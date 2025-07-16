package dashboard

import (
	"encoding/json"
	"goapimon/config"
	"goapimon/utility"
	"html/template"
	"net/http"
	"path/filepath"
)

type DashboardData struct {
	Name  string
	Value int
}

func Enable() {
	config.DashboardEnabled = true
}

var Handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	if !config.DashboardEnabled {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Dashboard disabled"))
		return
	}
	config.Mu.Lock()
	// Prepare data for JS rendering
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
	data := map[string][]Row{}
	for _, win := range config.Windows {
		rows := []Row{}
		for method, paths := range config.Stats {
			for path, s := range paths {
				count, errCount, status, avg, min, max, rps := utility.CalcWindowStats(s.Recent, win.Length)
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
	// Total (all time) - use all-time stats, not calcWindowStats
	rows := []Row{}
	for method, paths := range config.Stats {
		for path, s := range paths {
			status := s.TotalStatus
			avg := float64(0)
			if s.TotalCount > 0 {
				avg = float64(s.TotalTime.Milliseconds()) / float64(s.TotalCount)
			}
			var rps float64 = -1
			if s.TotalCount > 1 && s.LastSeen.After(s.FirstSeen) {
				rps = float64(s.TotalCount) / s.LastSeen.Sub(s.FirstSeen).Seconds()
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
	config.Mu.Unlock()

	// Marshal to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "Failed to encode data", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	tmplData := struct {
		Data template.JS // Use template.JS to avoid escaping
	}{
		Data: template.JS(jsonData),
	}

	// Parse and execute template
	tmplPath := filepath.Join("goapimon", "dashboard", "template.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, tmplData)
}
