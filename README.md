# goapimon — Simple API Monitoring for Go (net/http)

📈 **goapimon** is a lightweight, plug-and-play middleware for Go `net/http` servers.  
It monitors API requests, collects real-time metrics, and provides a local dashboard.  
Perfect for solo developers, hobby projects, and self-hosted apps.

---

## 📦 Installation

```bash
go get github.com/aurieli333/goapimon
```

---

## 🧪 Basic Usage

```go
package main

import (
	"net/http"
	"github.com/aurieli333/goapimon"
)

func main() {
	mux := http.NewServeMux()

	// Your API endpoint
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello!"))
	})

	// Enable dashboard at /__goapimon (optional)
	goapimon.EnableDashboard()

	// Export metrics for Prometheus (optional)
	goapimon.EnablePrometheus("/metrics")
 
    http.Handle("/__goapimon", goapimon.DashboardHandler)
	http.Handle("/metrics", goapimon.PrometheusHandler)

	// Wrap your handler with the monitoring middleware
	logged := goapimon.Monitor(mux)

	http.ListenAndServe(":8080", logged)
}
```

---

## 🔎 What It Monitors

| Metric         | Description                          |
|----------------|--------------------------------------|
| Request count  | Per route + method                   |
| Latency        | Average & last response time (ms)    |
| Status codes   | 2xx, 4xx, 5xx breakdown               |
| Error rate     | Errors per route                     |

---

## 🖥️ Dashboard Preview

Visit `/__goapimon` in your browser to see a live dashboard of your API performance.

_You can disable or protect this route in production._

---

## ⚙️ Configuration

Configure with environment variables or code:
- `GOAPIMON_LOG_FILE=logs.txt` (write logs to file)
- `GOAPIMON_ENABLE_DASHBOARD=false` (disable UI)
- `GOAPIMON_METRICS_PATH=/custommetrics` (custom exporter path)

---

## 💰 goapimon Pro (Coming Soon)

Upgrade to **goapimon Pro** for:
- 📬 Telegram / Slack / Discord alerts
- 📊 CSV data export
- 🧪 Route performance analysis
- 🔐 Basic auth for dashboard
- 🎨 Custom theming (dark mode, branding)

💵 One-time purchase on Boosty _(coming soon)_

---

## 🛠 Roadmap

- [ ] In-memory metrics
- [ ] Prometheus exporter
- [ ] Local HTML dashboard
- [ ] Alerts & export (Pro)
- [ ] Auth + theming (Pro)
- [ ] Public hosted mode

---

## 🤝 Contributing

PRs welcome! Please open an issue first for major changes.

---

## 📄 License

MIT — free for commercial and personal use.

---

## 🌐 Links

- Website / Demo: _(coming soon)_
- Telegram updates: _(coming soon)_
- Buy Pro: _(coming soon)_

---

**Monitor your Go API in seconds — no config, no cloud, no bloat.**
