package monitor

import (
	"slices"
	"time"

	"github.com/aurieli333/goapimon/config"
	"github.com/aurieli333/goapimon/model"
)

type Monitor struct {
	Stats map[string]map[string]*model.RouteStats // method -> path -> stats
	Mu    *config.SafeMutex
}

func NewMonitor(stats map[string]map[string]*model.RouteStats) *Monitor {
	return &Monitor{
		Stats: stats,
		Mu:    &config.SafeMutex{},
	}
}

func (m *Monitor) CoreMiddleware(method string, path string, status int, start time.Time, elapsed time.Duration) {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	methodStats, ok := m.Stats[method]
	if !ok {
		methodStats = make(map[string]*model.RouteStats)
		m.Stats[method] = methodStats
	}

	rs, ok := methodStats[path]
	if !ok {
		rs = &model.RouteStats{
			TotalStatus: make(map[int]int),
			TotalMin:    elapsed,
			TotalMax:    elapsed,
			FirstSeen:   start,
		}
		methodStats[path] = rs
	}

	// add new data
	rs.Recent = append(rs.Recent, model.RequestRecord{
		Timestamp: start,
		Duration:  elapsed,
		Status:    status,
		Method:    method,
	})

	// Delete old data (more then 5 minutes)
	cutoff := time.Now().Add(-5 * time.Minute)
	idx := slices.IndexFunc(rs.Recent, func(rec model.RequestRecord) bool {
		return rec.Timestamp.After(cutoff)
	})
	if idx > 0 {
		rs.Recent = slices.Delete(rs.Recent, 0, idx)
	}

	// Refresh aggregates
	rs.TotalCount++
	rs.TotalStatus[status]++
	rs.TotalTime += elapsed
	if elapsed < rs.TotalMin {
		rs.TotalMin = elapsed
	}
	if elapsed > rs.TotalMax {
		rs.TotalMax = elapsed
	}
	rs.LastSeen = time.Now()
	if status >= 400 {
		rs.TotalErrorCount++
	}
}
