# goapimon — Simple API Monitoring for Go (net/http, gin)

📈 **goapimon** is a lightweight, plug-and-play middleware for Go `net/http`, `gin` servers.  
It monitors API requests, collects real-time metrics, and provides a local dashboard.  
Perfect for solo developers, hobby projects, and self-hosted apps.

---

## 📦 Installation

```bash
go get github.com/aurieli333/goapimon
```

---

## 🧪 Basic Usage

### net/http
```go
package main

import (
	"goapimon"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	// Your API endpoint
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello!"))
	})

	// Turn on Dashboard (optional)
	goapimon.DashboardEnable()

	// Turn on Prometheus with path (optional)
	goapimon.PrometheusEnable("/metrics")

	// Creating handlers
	mux.HandleFunc("/__goapimon/", goapimon.DashboardHandler)
	mux.HandleFunc("/metrics", goapimon.PrometheusHandler)

	// Wrapping your mux in monitoring(middleware)
	logged := goapimon.Monitor.Middleware(mux)

	log.Println("Starting server on :8080")
	err := http.ListenAndServe(":8080", logged)
	if err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
```

### Gin Web Framework
```
package main

import (
	"github.com/aurieli333/goapimon"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Turn on Dashboard (optional)
	goapimon.DashboardEnable()

	// Turn on Prometheus with path (optional)
	goapimon.PrometheusEnable("/metrics")

	// Use goapimon middleware for gin
	r.Use(goapimon.GinMiddleware(goapimon.Monitor))

	// Your API endpoint
	r.GET("/hello", func(c *gin.Context) {
		c.String(200, "Hello!")
	})

	// Creating handlers
	r.Any("/__goapimon/*any", gin.WrapF(goapimon.DashboardHandler))
	r.GET("/metrics", gin.WrapF(goapimon.PrometheusHandler))

	r.Run(":8080")
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
- [ ] Support frameworks
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
- Telegram updates: https://t.me/goapimon
- Buy Pro: _(coming soon)_

---

**Monitor your Go API in seconds — no config, no cloud, no bloat.**
