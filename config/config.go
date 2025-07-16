package config

import (
	"sync"
	"time"
)

var (
	DashboardEnabled   = true
	PrometheusEnabled  = true
	PrometheusPath     = "/metrics"
	Mu    sync.Mutex
	Stats = make(map[string]map[string]*RouteStats) // method -> path -> stats
)

var Windows = []struct {
	Name   string
	Length time.Duration
}{
	{"1m", time.Minute},
	{"2m", 2 * time.Minute},
	{"5m", 5 * time.Minute},
}

type RouteStats struct {
	TotalCount      int
	TotalErrorCount int
	TotalStatus     map[int]int
	TotalTime       time.Duration
	TotalMin        time.Duration
	TotalMax        time.Duration
	FirstSeen       time.Time
	LastSeen        time.Time
	Recent          []RequestRecord // last 5 min
}

type RequestRecord struct {
	Timestamp time.Time
	Duration  time.Duration
	Status    int
}

var InternalPaths = map[string]bool{
	"/__goapimon": true,
	"/metrics":    true,
}