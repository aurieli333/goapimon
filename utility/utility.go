package utility

import (
	"encoding/json"
	"goapimon/config"
	"time"
)

func CalcWindowStats(recs []config.RequestRecord, window time.Duration) (count, errCount int, status map[int]int, avg, min, max, rps float64) {
	if len(recs) == 0 {
		return 0, 0, map[int]int{}, 0, -1, -1, -1
	}
	status = make(map[int]int)
	var sum time.Duration
	minDur := time.Duration(1<<63 - 1)
	maxDur := time.Duration(0)
	start := time.Now().Add(-window)
	firstTs := time.Time{}
	lastTs := time.Time{}
	for _, rec := range recs {
		if rec.Timestamp.After(start) {
			if count == 0 {
				firstTs = rec.Timestamp
			}
			lastTs = rec.Timestamp
			count++
			if rec.Status >= 400 {
				errCount++
			}
			status[rec.Status]++
			sum += rec.Duration
			if rec.Duration < minDur {
				minDur = rec.Duration
			}
			if rec.Duration > maxDur {
				maxDur = rec.Duration
			}
		}
	}
	if count == 0 {
		return 0, 0, status, 0, -1, -1, -1
	}
	avg = float64(sum.Milliseconds()) / float64(count)
	min = float64(minDur.Milliseconds())
	max = float64(maxDur.Milliseconds())
	if count > 1 && lastTs.After(firstTs) {
		rps = float64(count) / lastTs.Sub(firstTs).Seconds()
	} else {
		rps = -1
	}
	return
}

// Helper to marshal JSON or panic
func MustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
