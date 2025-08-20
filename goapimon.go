package goapimon

import (
	"sync"
	"time"

	"github.com/aurieli333/goapimon/adapters"
	"github.com/aurieli333/goapimon/dashboard"
	"github.com/aurieli333/goapimon/model"
	"github.com/aurieli333/goapimon/monitor"
	"github.com/aurieli333/goapimon/prometheus"
)

// Global mutex shared across all statistics consumers
var mu = &sync.Mutex{}

// Define default time windows for analysis
var windows = []model.Window{
	{Name: "1m", Length: 1 * time.Minute},
	{Name: "2m", Length: 2 * time.Minute},
	{Name: "5m", Length: 5 * time.Minute},
	// Add more windows here if needed
}

// Global stats shared between monitor, dashboard, and prometheus modules
var stats = make(map[string]map[string]*model.RouteStats)

// Monitor — shared monitoring instance
var Monitor = monitor.NewMonitor(stats)

// Dashboard — shared dashboard instance
var Dashboard = dashboard.NewDashboard(mu, windows, stats)

// Prometheus — shared Prometheus metrics instance
var Prometheus = prometheus.NewPrometheus(mu, windows, stats)

// DashboardHandler — public HTTP handler for serving the dashboard UI
var DashboardHandler = Dashboard.Handler()

// PrometheusHandler — public HTTP handler for exposing Prometheus metrics
var PrometheusHandler = Prometheus.Handler()

// DashboardEnable — enables the dashboard at runtime
func DashboardEnable() {
	Dashboard.Enable()
}

// PrometheusEnable — enables Prometheus metrics and sets its endpoint path
func PrometheusEnable(path string) {
	Prometheus.Enable(path)
}

var MiddlewareGin = adapters.MiddlewareGin
var MiddlewareNetHTTP = adapters.MiddlewareNetHTTP
