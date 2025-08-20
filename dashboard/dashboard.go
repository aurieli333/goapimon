package dashboard

import (
	"bytes"
	"embed"
	"encoding/csv"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aurieli333/goapimon/model"
	"github.com/aurieli333/goapimon/utility"

	"github.com/influxdata/tdigest"
)

//go:embed template.html
var tmplFS embed.FS

//go:embed static/*
var embeddedFiles embed.FS

type Row struct {
	Method     string      `json:"Method"`
	Path       string      `json:"Path"`
	Count      int         `json:"Count"`
	ErrorCount int         `json:"ErrorCount"`
	ErrorRate  float64     `json:"ErrorRate"` // %
	Status     map[int]int `json:"Status"`
	Avg        float64     `json:"Avg"`        // ms
	Min        float64     `json:"Min"`        // ms
	Max        float64     `json:"Max"`        // ms
	P50        float64     `json:"P50"`        // ms
	P90        float64     `json:"P90"`        // ms
	P95        float64     `json:"P95"`        // ms
	P99        float64     `json:"P99"`        // ms
	Throughput float64     `json:"Throughput"` // rps
	HasError   bool        `json:"Has_error"`
}

type Dashboard struct {
	Mu      *sync.Mutex
	Windows []model.Window
	Stats   map[string]map[string]*model.RouteStats
	Enabled bool
}

func NewDashboard(mu *sync.Mutex, windows []model.Window, stats map[string]map[string]*model.RouteStats) *Dashboard {
	return &Dashboard{
		Mu:      mu,
		Windows: windows,
		Stats:   stats,
		Enabled: false,
	}
}

func (d *Dashboard) Enable() {
	d.Enabled = true
}

func (d *Dashboard) exportCsv(w http.ResponseWriter, r *http.Request) {
	d.Mu.Lock()
	defer d.Mu.Unlock()

	b := &bytes.Buffer{}
	writer := csv.NewWriter(b)
	writer.Write([]string{"window", "URL", "Method", "status", "count", "avg", "throughput"})

	data := d.calcData()

	for window, rows := range data {
		for _, row := range rows {
			count := strconv.Itoa(row.Count)
			avg := strconv.FormatFloat(row.Avg, 'f', 2, 64)
			tp := strconv.FormatFloat(row.Throughput, 'f', 2, 64)
			var status string
			for tStatus := range row.Status {
				status = strconv.Itoa(tStatus)
				break
			}

			writer.Write([]string{window, row.Path, row.Method, status, count, avg, tp})
		}
	}

	writer.Flush()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=monitors.csv")
	w.WriteHeader(http.StatusOK)

	w.Write(b.Bytes())
}

func (d *Dashboard) calcData() map[string][]Row {
	data := make(map[string][]Row)
	now := time.Now()

	// Windows
	for _, win := range d.Windows {
		rows := []Row{}
		for method, paths := range d.Stats {
			for path, s := range paths {
				ws := utility.CalcWindowStats(s.Recent, win.Length, now)
				if ws.Count == 0 {
					continue
				}
				rows = append(rows, Row{
					Method:     method,
					Path:       path,
					Count:      ws.Count,
					ErrorCount: ws.ErrCount,
					ErrorRate:  ws.ErrorRate,
					Status:     ws.Status,
					Avg:        ws.Avg,
					Min:        ws.Min,
					Max:        ws.Max,
					P50:        ws.P50,
					P90:        ws.P90,
					P95:        ws.P95,
					P99:        ws.P99,
					Throughput: ws.RPS,
					HasError:   ws.ErrCount > 0,
				})
			}
		}
		data[win.Name] = rows
	}

	// Total
	rows := []Row{}
	for method, paths := range d.Stats {
		for path, s := range paths {
			avg := float64(0)
			if s.TotalCount > 0 {
				avg = float64(s.TotalTime.Milliseconds()) / float64(s.TotalCount)
			}

			var rps float64 = -1
			if s.TotalCount > 1 && now.After(s.FirstSeen) {
				rps = float64(s.TotalCount) / now.Sub(s.FirstSeen).Seconds()
			}

			// t-digest for quantiles
			td := tdigest.NewWithCompression(1000)
			for _, rec := range s.Recent {
				ms := float64(rec.Duration.Milliseconds())
				td.Add(ms, 1)
			}

			rows = append(rows, Row{
				Method:     method,
				Path:       path,
				Count:      s.TotalCount,
				ErrorCount: s.TotalErrorCount,
				ErrorRate:  float64(s.TotalErrorCount) / float64(s.TotalCount) * 100,
				Status:     s.TotalStatus,
				Avg:        avg,
				Min:        float64(s.TotalMin.Milliseconds()),
				Max:        float64(s.TotalMax.Milliseconds()),
				P50:        td.Quantile(0.50),
				P90:        td.Quantile(0.90),
				P95:        td.Quantile(0.95),
				P99:        td.Quantile(0.99),
				Throughput: rps,
				HasError:   s.TotalErrorCount > 0,
			})
		}
	}
	data["total"] = rows

	return data
}

func (d *Dashboard) Handler() http.HandlerFunc {
	staticFS, _ := fs.Sub(embeddedFiles, "static")
	fileServer := http.StripPrefix("/__goapimon/static/", http.FileServer(http.FS(staticFS)))
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.Enabled {
			http.NotFound(w, r)
			return
		}

		if r.URL.Path == "/__goapimon/export/csv" {
			// Serve CSV export
			d.exportCsv(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/__goapimon/static/") {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Serve the main dashboard HTML
		d.Mu.Lock()
		data := d.calcData()
		jsonData, err := json.Marshal(data)
		if err != nil {
			http.Error(w, "Failed to encode data", http.StatusInternalServerError)
			return
		}
		d.Mu.Unlock()

		tmplData := struct {
			Data template.JS
		}{
			Data: template.JS(jsonData),
		}

		tmpl, err := template.ParseFS(tmplFS, "template.html")
		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		if err := tmpl.Execute(w, tmplData); err != nil {
			http.Error(w, "Render error", http.StatusInternalServerError)
			return
		}
	}
}
