package monitor

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aurieli333/goapimon/config"
	"github.com/aurieli333/goapimon/model"
)

type Monitor struct {
	Stats map[string]map[string]*model.RouteStats
	Mu    *config.SafeMutex
}

func NewMonitor(stats map[string]map[string]*model.RouteStats) *Monitor {
	return &Monitor{
		Stats: stats,
		Mu:    &config.SafeMutex{},
	}
}

// statusRecorder wraps http.ResponseWriter to capture status code
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func isInternalPath(path string) bool {
	_, ok := config.InternalPaths[path]
	return ok
}

var uuidRegex = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
var intRegex = regexp.MustCompile(`/\d+`)

func normalizePath(path string) string {
	path = uuidRegex.ReplaceAllString(path, ":id")
	path = intRegex.ReplaceAllString(path, "/:id")
	return path
}

func (m *Monitor) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isInternalPath(strings.Split(r.URL.Path, `/`)[1]) || isInternalPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		sr := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(sr, r)
		elapsed := time.Since(start)

		m.Mu.Lock()
		defer m.Mu.Unlock()

		methodStats, ok := m.Stats[r.Method]
		if !ok {
			methodStats = make(map[string]*model.RouteStats)
			m.Stats[r.Method] = methodStats
		}
		rs, ok := methodStats[normalizePath(r.URL.Path)]
		if !ok {
			rs = &model.RouteStats{
				TotalStatus: make(map[int]int),
				TotalMin:    elapsed,
				TotalMax:    elapsed,
				FirstSeen:   start,
			}
			methodStats[normalizePath(r.URL.Path)] = rs
		}
		rec := model.RequestRecord{Timestamp: start, Duration: elapsed, Status: sr.status}
		rs.Recent = append(rs.Recent, rec)
		// Prune old records (>5min)
		cutoff := time.Now().Add(-5 * time.Minute)
		idx := 0
		for i, rec := range rs.Recent {
			if rec.Timestamp.After(cutoff) {
				idx = i
				break
			}
		}
		if len(rs.Recent) > 0 && rs.Recent[0].Timestamp.Before(cutoff) {
			rs.Recent = rs.Recent[idx:]
		}
		// Update totals
		rs.TotalCount++
		rs.TotalStatus[sr.status]++
		rs.TotalTime += elapsed
		if elapsed < rs.TotalMin {
			rs.TotalMin = elapsed
		}
		if elapsed > rs.TotalMax {
			rs.TotalMax = elapsed
		}
		rs.LastSeen = time.Now()
		if r.URL.Query().Get("fail") == "1" || sr.status >= 400 {
			rs.TotalErrorCount++
		}
	})
}
