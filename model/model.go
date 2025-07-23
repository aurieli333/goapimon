package model

import "time"

type RequestRecord struct {
	Timestamp time.Time
	Duration  time.Duration
	Status    int
}

type RouteStats struct {
	Recent          []RequestRecord
	TotalCount      int
	TotalErrorCount int
	TotalStatus     map[int]int
	TotalTime       time.Duration
	TotalMin        time.Duration
	TotalMax        time.Duration
	FirstSeen       time.Time
	LastSeen        time.Time
}

type Window struct {
	Name   string
	Length time.Duration
}
