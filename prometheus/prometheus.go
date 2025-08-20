package prometheus

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aurieli333/goapimon/model"
	"github.com/aurieli333/goapimon/utility"
)

// Prometheus exposes metrics in plain text for Prometheus scrapes.
type Prometheus struct {
	Mu      *sync.Mutex
	Windows []model.Window
	Stats   map[string]map[string]*model.RouteStats

	Enabled bool
	Path    string
}

func NewPrometheus(mu *sync.Mutex, windows []model.Window, stats map[string]map[string]*model.RouteStats) *Prometheus {
	return &Prometheus{
		Mu:      mu,
		Windows: windows,
		Stats:   stats,
	}
}

func (p *Prometheus) Enable(path string) {
	p.Enabled = true
	p.Path = path
}

// Handler returns an http.HandlerFunc that serves metrics in Prometheus text format.
func (p *Prometheus) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !p.Enabled {
			http.NotFound(w, r)
			return
		}

		// Set content type for Prometheus
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		// Copy stats under lock so we can release mutex during heavy computation
		p.Mu.Lock()
		statsCopy := deepCopyStats(p.Stats)
		windowsCopy := append([]model.Window(nil), p.Windows...)
		p.Mu.Unlock()

		now := time.Now()

		// Iterate copied stats and write metrics
		for method, paths := range statsCopy {
			for path, s := range paths {
				// Windowed metrics (per configured windows)
				for _, win := range windowsCopy {
					ws := utility.CalcWindowStats(s.Recent, win.Length, now)
					writeMetrics(w, win.Name, method, path, ws)
				}

				// Total / lifetime metrics
				total := calcTotalStats(s)
				writeMetrics(w, "total", method, path, total)
			}
		}
	}
}

// writeMetrics emits all metrics for a given (window, method, path) using WindowStats.
func writeMetrics(w io.Writer, window, method, path string, m utility.WindowStats) {
	labelsBase := map[string]string{
		"window": window,
		"method": method,
		"path":   path,
	}

	// status counts
	for code, cnt := range m.Status {
		labels := withExtra(labelsBase, map[string]string{"code": strconv.Itoa(code)})
		writeMetric(w, "goapimon_http_status_total", labels, cnt)
	}

	// counters and gauges
	writeMetric(w, "goapimon_requests_total", labelsBase, m.Count)
	writeMetric(w, "goapimon_errors_total", labelsBase, m.ErrCount)
	writeMetric(w, "goapimon_error_rate", labelsBase, fmt.Sprintf("%.2f", m.ErrorRate))
	writeMetric(w, "goapimon_avg_ms", labelsBase, fmt.Sprintf("%.1f", m.Avg))
	writeMetric(w, "goapimon_min_ms", labelsBase, fmt.Sprintf("%.1f", m.Min))
	writeMetric(w, "goapimon_max_ms", labelsBase, fmt.Sprintf("%.1f", m.Max))
	writeMetric(w, "goapimon_p50_ms", labelsBase, fmt.Sprintf("%.1f", m.P50))
	writeMetric(w, "goapimon_p90_ms", labelsBase, fmt.Sprintf("%.1f", m.P90))
	writeMetric(w, "goapimon_p95_ms", labelsBase, fmt.Sprintf("%.1f", m.P95))
	writeMetric(w, "goapimon_p99_ms", labelsBase, fmt.Sprintf("%.1f", m.P99))
	writeMetric(w, "goapimon_throughput_rps", labelsBase, fmt.Sprintf("%.2f", m.RPS))
}

// calcTotalStats builds WindowStats from RouteStats aggregates (lifetime metrics).
func calcTotalStats(s *model.RouteStats) utility.WindowStats {
	var rps float64
	duration := s.LastSeen.Sub(s.FirstSeen).Seconds()
	if duration > 0 {
		rps = float64(s.TotalCount) / duration
	}

	var errorRate float64
	if s.TotalCount > 0 {
		errorRate = float64(s.TotalErrorCount) / float64(s.TotalCount) * 100
	}

	return utility.WindowStats{
		Count:     s.TotalCount,
		ErrCount:  s.TotalErrorCount,
		ErrorRate: errorRate,
		Status:    s.TotalStatus,
		Avg:       msSafeDiv(s.TotalTime.Milliseconds(), s.TotalCount),
		Min:       float64(s.TotalMin.Milliseconds()),
		Max:       float64(s.TotalMax.Milliseconds()),
		P50:       -1, // can be filled from t-digest stored in RouteStats if added later
		P90:       -1,
		P95:       -1,
		P99:       -1,
		RPS:       rps,
	}
}

func msSafeDiv(ms int64, count int) float64 {
	if count == 0 {
		return 0
	}
	return float64(ms) / float64(count)
}

// withExtra merges two label maps and returns a new map.
func withExtra(base, extra map[string]string) map[string]string {
	res := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		res[k] = v
	}
	for k, v := range extra {
		res[k] = v
	}
	return res
}

// formatLabels builds Prometheus label block from map.
func formatLabels(labels map[string]string) string {
	var b strings.Builder
	b.WriteString("{")
	first := true
	for k, v := range labels {
		if !first {
			b.WriteString(",")
		}
		// Note: labels are not escaped here; if you may have quotes or backslashes, escape them.
		b.WriteString(fmt.Sprintf("%s=\"%s\"", k, v))
		first = false
	}
	b.WriteString("}")
	return b.String()
}

// writeMetric writes a single metric line.
func writeMetric(w io.Writer, name string, labels map[string]string, value interface{}) {
	fmt.Fprintf(w, "%s%s %v\n", name, formatLabels(labels), value)
}

// deepCopyStats makes a shallow-deep copy of the stats map so we can work without lock.
// It copies RouteStats struct and creates a new slice for Recent.
func deepCopyStats(src map[string]map[string]*model.RouteStats) map[string]map[string]*model.RouteStats {
	out := make(map[string]map[string]*model.RouteStats, len(src))
	for method, pathStats := range src {
		out[method] = make(map[string]*model.RouteStats, len(pathStats))
		for path, stat := range pathStats {
			// copy struct value
			newStat := *stat
			// copy recent slice
			newRecent := make([]model.RequestRecord, len(stat.Recent))
			copy(newRecent, stat.Recent)
			newStat.Recent = newRecent
			out[method][path] = &newStat
		}
	}
	return out
}
