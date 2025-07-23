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
		Enabled: false,
	}
}

func (p *Prometheus) Enable(path string) {
	p.Enabled = true
	p.Path = path
}

func (p *Prometheus) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !p.Enabled {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		p.Mu.Lock()
		stats := deepCopyStats(p.Stats)
		windows := append([]model.Window(nil), p.Windows...)
		p.Mu.Unlock()

		for method, paths := range stats {
			for path, s := range paths {
				for _, win := range windows {
					count, errCount, statusMap, avg, min, max, rps := utility.CalcWindowStats(s.Recent, win.Length, time.Now())

					for code, cnt := range statusMap {
						writeMetric(w, "goapimon_http_status_total", map[string]string{
							"window": win.Name, "method": method, "path": path, "code": strconv.Itoa(code),
						}, cnt)
					}

					writeMetric(w, "goapimon_requests_total", map[string]string{
						"window": win.Name, "method": method, "path": path,
					}, count)

					writeMetric(w, "goapimon_errors_total", map[string]string{
						"window": win.Name, "method": method, "path": path,
					}, errCount)

					writeMetric(w, "goapimon_avg_ms", map[string]string{
						"window": win.Name, "method": method, "path": path,
					}, fmt.Sprintf("%.1f", avg))

					writeMetric(w, "goapimon_min_ms", map[string]string{
						"window": win.Name, "method": method, "path": path,
					}, fmt.Sprintf("%.1f", min))

					writeMetric(w, "goapimon_max_ms", map[string]string{
						"window": win.Name, "method": method, "path": path,
					}, fmt.Sprintf("%.1f", max))

					writeMetric(w, "goapimon_throughput_rps", map[string]string{
						"window": win.Name, "method": method, "path": path,
					}, fmt.Sprintf("%.2f", rps))
				}

				// Total window (lifetime stats)
				for code, cnt := range s.TotalStatus {
					writeMetric(w, "goapimon_http_status_total", map[string]string{
						"window": "total", "method": method, "path": path, "code": strconv.Itoa(code),
					}, cnt)
				}

				writeMetric(w, "goapimon_requests_total", map[string]string{
					"window": "total", "method": method, "path": path,
				}, s.TotalCount)

				writeMetric(w, "goapimon_errors_total", map[string]string{
					"window": "total", "method": method, "path": path,
				}, s.TotalErrorCount)

				avgTotal := float64(0)
				if s.TotalCount > 0 {
					avgTotal = float64(s.TotalTime.Milliseconds()) / float64(s.TotalCount)
				}

				writeMetric(w, "goapimon_avg_ms", map[string]string{
					"window": "total", "method": method, "path": path,
				}, fmt.Sprintf("%.1f", avgTotal))

				writeMetric(w, "goapimon_min_ms", map[string]string{
					"window": "total", "method": method, "path": path,
				}, fmt.Sprintf("%.1f", float64(s.TotalMin.Milliseconds())))

				writeMetric(w, "goapimon_max_ms", map[string]string{
					"window": "total", "method": method, "path": path,
				}, fmt.Sprintf("%.1f", float64(s.TotalMax.Milliseconds())))

				rps := float64(0)
				duration := s.LastSeen.Sub(s.FirstSeen).Seconds()
				if duration > 0 {
					rps = float64(s.TotalCount) / duration
				}

				writeMetric(w, "goapimon_throughput_rps", map[string]string{
					"window": "total", "method": method, "path": path,
				}, fmt.Sprintf("%.2f", rps))
			}
		}
	}
}

// Helper to format labels for Prometheus
func formatLabels(labels map[string]string) string {
	var b strings.Builder
	b.WriteString("{")
	first := true
	for k, v := range labels {
		if !first {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%s=\"%s\"", k, v))
		first = false
	}
	b.WriteString("}")
	return b.String()
}

// Writes a single Prometheus metric
func writeMetric(w io.Writer, name string, labels map[string]string, value interface{}) {
	fmt.Fprintf(w, "%s%s %v\n", name, formatLabels(labels), value)
}

// Deep copy stats to safely unlock before writing response
func deepCopyStats(src map[string]map[string]*model.RouteStats) map[string]map[string]*model.RouteStats {
	statsCopy := make(map[string]map[string]*model.RouteStats)
	for method, pathStats := range src {
		statsCopy[method] = make(map[string]*model.RouteStats)
		for path, stat := range pathStats {
			newStat := *stat
			newRecent := make([]model.RequestRecord, len(stat.Recent))
			copy(newRecent, stat.Recent)
			newStat.Recent = newRecent
			statsCopy[method][path] = &newStat
		}
	}
	return statsCopy
}
