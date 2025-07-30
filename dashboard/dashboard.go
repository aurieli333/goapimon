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
)

//go:embed template.html
var tmplFS embed.FS

//go:embed static/*
var embeddedFiles embed.FS

type Row struct {
	Method     string
	Path       string
	Count      int
	ErrorCount int
	Status     map[int]int
	Avg        float64
	Min        float64
	Max        float64
	Throughput float64
	HasError   bool
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
	for _, win := range d.Windows {
		rows := []Row{}
		for method, paths := range d.Stats {
			for path, s := range paths {
				count, errCount, status, avg, min, max, rps := utility.CalcWindowStats(s.Recent, win.Length, time.Now())
				if count == 0 {
					continue
				}
				hasErr := errCount > 0
				rows = append(rows, Row{
					Method:     method,
					Path:       path,
					Count:      count,
					ErrorCount: errCount,
					Status:     status,
					Avg:        avg,
					Min:        min,
					Max:        max,
					Throughput: rps,
					HasError:   hasErr,
				})
			}
		}
		data[win.Name] = rows
	}

	// Total stats (all-time)
	rows := []Row{}
	for method, paths := range d.Stats {
		for path, s := range paths {
			status := s.TotalStatus
			avg := float64(0)
			if s.TotalCount > 0 {
				avg = float64(s.TotalTime.Milliseconds()) / float64(s.TotalCount)
			}
			var rps float64 = -1
			if s.TotalCount > 1 && time.Now().After(s.FirstSeen) {
				rps = float64(s.TotalCount) / time.Now().Sub(s.FirstSeen).Seconds()
			}
			hasErr := s.TotalErrorCount > 0
			rows = append(rows, Row{
				Method:     method,
				Path:       path,
				Count:      s.TotalCount,
				ErrorCount: s.TotalErrorCount,
				Status:     status,
				Avg:        avg,
				Min:        float64(s.TotalMin.Milliseconds()),
				Max:        float64(s.TotalMax.Milliseconds()),
				Throughput: rps,
				HasError:   hasErr,
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
