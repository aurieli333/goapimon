package model

import (
	"net/http"
	"time"
)

// Request data
type RequestRecord struct {
	Timestamp time.Time     // Время запроса
	Duration  time.Duration // Длительность
	Status    int           // HTTP статус
	Method    string        // Метод (GET, POST...)
}

// Store only N minutes
type RouteStats struct {
	Recent []RequestRecord // Raw data for last interval

	// Aggregates
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

// statusRecorder — for storing status
type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *StatusRecorder) WriteHeader(code int) {
	r.Status = code
	r.ResponseWriter.WriteHeader(code)
}
