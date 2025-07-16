package prometheus

import (
	"fmt"
	"goapimon/config"
	"goapimon/utility"
	"net/http"
)

func Enable(path string) {
	config.PrometheusEnabled = true
	config.PrometheusPath = path
}

var Handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	if !config.PrometheusEnabled {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Prometheus metrics disabled"))
		return
	}
	config.Mu.Lock()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for method, paths := range config.Stats {
		for path, s := range paths {
			for _, win := range config.Windows {
				count, errCount, status, avg, min, max, thru := utility.CalcWindowStats(s.Recent, win.Length)
				for code, cnt := range status {
					w.Write([]byte(fmt.Sprintf("goapimon_status_total{window=\"%s\",method=\"%s\",path=\"%s\",code=\"%d\"} %d\n", win.Name, method, path, code, cnt)))
				}
				w.Write([]byte(fmt.Sprintf("goapimon_requests_total{window=\"%s\",method=\"%s\",path=\"%s\"} %d\n", win.Name, method, path, count)))
				w.Write([]byte(fmt.Sprintf("goapimon_errors_total{window=\"%s\",method=\"%s\",path=\"%s\"} %d\n", win.Name, method, path, errCount)))
				w.Write([]byte(fmt.Sprintf("goapimon_avg_ms{window=\"%s\",method=\"%s\",path=\"%s\"} %.1f\n", win.Name, method, path, avg)))
				w.Write([]byte(fmt.Sprintf("goapimon_min_ms{window=\"%s\",method=\"%s\",path=\"%s\"} %.1f\n", win.Name, method, path, min)))
				w.Write([]byte(fmt.Sprintf("goapimon_max_ms{window=\"%s\",method=\"%s\",path=\"%s\"} %.1f\n", win.Name, method, path, max)))
				w.Write([]byte(fmt.Sprintf("goapimon_throughput_rps{window=\"%s\",method=\"%s\",path=\"%s\"} %.2f\n", win.Name, method, path, thru)))
			}
			// Total (all time)
			for code, cnt := range s.TotalStatus {
				w.Write([]byte(fmt.Sprintf("goapimon_status_total{window=\"total\",method=\"%s\",path=\"%s\",code=\"%d\"} %d\n", method, path, code, cnt)))
			}
			avg := float64(0)
			if s.TotalCount > 0 {
				avg = float64(s.TotalTime.Milliseconds()) / float64(s.TotalCount)
			}
			thru := float64(0)
			dur := s.LastSeen.Sub(s.FirstSeen).Seconds()
			if dur > 0 {
				thru = float64(s.TotalCount) / dur
			}
			w.Write([]byte(fmt.Sprintf("goapimon_requests_total{window=\"total\",method=\"%s\",path=\"%s\"} %d\n", method, path, s.TotalCount)))
			w.Write([]byte(fmt.Sprintf("goapimon_errors_total{window=\"total\",method=\"%s\",path=\"%s\"} %d\n", method, path, s.TotalErrorCount)))
			w.Write([]byte(fmt.Sprintf("goapimon_avg_ms{window=\"total\",method=\"%s\",path=\"%s\"} %.1f\n", method, path, avg)))
			w.Write([]byte(fmt.Sprintf("goapimon_min_ms{window=\"total\",method=\"%s\",path=\"%s\"} %.1f\n", method, path, float64(s.TotalMin.Milliseconds()))))
			w.Write([]byte(fmt.Sprintf("goapimon_max_ms{window=\"total\",method=\"%s\",path=\"%s\"} %.1f\n", method, path, float64(s.TotalMax.Milliseconds()))))
			w.Write([]byte(fmt.Sprintf("goapimon_throughput_rps{window=\"total\",method=\"%s\",path=\"%s\"} %.2f\n", method, path, thru)))
		}
	}
	config.Mu.Unlock()
}
