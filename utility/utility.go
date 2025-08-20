package utility

import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/aurieli333/goapimon/config"
	"github.com/aurieli333/goapimon/model"

	"github.com/influxdata/tdigest"
)

type WindowStats struct {
	Count     int
	ErrCount  int
	ErrorRate float64
	Status    map[int]int
	Avg       float64
	Min       float64
	Max       float64
	RPS       float64
	P50       float64
	P90       float64
	P95       float64
	P99       float64
}

func CalcWindowStats(recs []model.RequestRecord, window time.Duration, now time.Time) WindowStats {
	stats := WindowStats{
		Status: make(map[int]int),
		Min:    -1,
		Max:    -1,
	}

	if len(recs) == 0 {
		return stats
	}

	start := now.Add(-window)
	var sum time.Duration
	var minDur time.Duration = 1<<63 - 1
	var maxDur time.Duration = 0

	td := tdigest.NewWithCompression(1000)

	for _, rec := range recs {
		if rec.Timestamp.After(start) {
			stats.Count++

			if rec.Status >= 400 {
				stats.ErrCount++
			}
			stats.Status[rec.Status]++

			sum += rec.Duration
			ms := float64(rec.Duration.Nanoseconds()) / 1_000_000.
			td.Add(ms, 1)

			if rec.Duration < minDur {
				minDur = rec.Duration
			}
			if rec.Duration > maxDur {
				maxDur = rec.Duration
			}
		}
	}

	if stats.Count == 0 {
		return stats
	}

	stats.Avg = float64(sum.Nanoseconds()) / 1_000_000. / float64(stats.Count)
	stats.Min = float64(sum.Nanoseconds()) / 1_000_000.
	stats.Max = float64(sum.Nanoseconds()) / 1_000_000.
	stats.RPS = float64(stats.Count) / window.Seconds()
	stats.ErrorRate = float64(stats.ErrCount) / float64(stats.Count) * 100

	stats.P50 = td.Quantile(0.50)
	stats.P90 = td.Quantile(0.90)
	stats.P95 = td.Quantile(0.95)
	stats.P99 = td.Quantile(0.99)
	return stats
}

// Helper to marshal JSON or panic
func MustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func IsInternalPath(path string) bool {
	_, ok := config.InternalPaths[path]
	return ok
}

var (
	uuidRegex = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	intRegex  = regexp.MustCompile(`/\d+`)
)

func NormalizePath(path string) string {
	path = uuidRegex.ReplaceAllString(path, ":id")
	path = intRegex.ReplaceAllString(path, "/:id")
	return path
}
