package config

import (
	"sync"
	"time"
)

type SafeMutex struct {
	mu sync.Mutex
}

func (m *SafeMutex) Lock() {
	m.mu.Lock()
}

func (m *SafeMutex) Unlock() {
	m.mu.Unlock()
}

var Windows = []struct {
	Name   string
	Length time.Duration
}{
	{"1m", time.Minute},
	{"2m", 2 * time.Minute},
	{"5m", 5 * time.Minute},
}

var InternalPaths = map[string]bool{
	"/__goapimon/": true,
	"/__goapimon":  true,
	"/metrics":     true,
}
