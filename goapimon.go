package goapimon

import (
	"goapimon/dashboard"
	"goapimon/monitor"
	"goapimon/prometheus"
)

// Monitor — публичный API
var Monitor = monitor.Monitor

// DashboardHandler — публичный HTTP-хендлер
var DashboardHandler = dashboard.Handler

// PrometheusHandler — экспорт /metrics
var PrometheusHandler = prometheus.Handler
