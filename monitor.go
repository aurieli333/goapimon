package goapimon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type RequestRecord struct {
	Timestamp time.Time
	Duration  time.Duration
	Status    int
}

type RouteStats struct {
	TotalCount      int
	TotalErrorCount int
	TotalStatus     map[int]int
	TotalTime       time.Duration
	TotalMin        time.Duration
	TotalMax        time.Duration
	FirstSeen       time.Time
	LastSeen        time.Time
	Recent          []RequestRecord // last 5 min
}

var (
	dashboardEnabled   bool
	prometheusEnabled  bool
	prometheusPath     string

	mu    sync.Mutex
	stats = make(map[string]map[string]*RouteStats) // method -> path -> stats
)

// statusRecorder wraps http.ResponseWriter to capture status code
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

var windows = []struct {
	Name   string
	Length time.Duration
}{
	{"1m", time.Minute},
	{"2m", 2 * time.Minute},
	{"5m", 5 * time.Minute},
}

func Monitor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__goapimon" {
			next.ServeHTTP(w, r)
			return
		}
		sr := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(sr, r)
		elapsed := time.Since(start)
		mu.Lock()
		methodStats, ok := stats[r.Method]
		if !ok {
			methodStats = make(map[string]*RouteStats)
			stats[r.Method] = methodStats
		}
		rs, ok := methodStats[r.URL.Path]
		if !ok {
			rs = &RouteStats{
				TotalStatus: make(map[int]int),
				TotalMin:    elapsed,
				TotalMax:    elapsed,
				FirstSeen:   start,
			}
			methodStats[r.URL.Path] = rs
		}
		rec := RequestRecord{Timestamp: start, Duration: elapsed, Status: sr.status}
		rs.Recent = append(rs.Recent, rec)
		// Prune old records (>5min)
		cutoff := time.Now().Add(-5 * time.Minute)
		idx := 0
		for i, rec := range rs.Recent {
			if rec.Timestamp.After(cutoff) {
				idx = i
				break
			}
		}
		if len(rs.Recent) > 0 && rs.Recent[0].Timestamp.Before(cutoff) {
			rs.Recent = rs.Recent[idx:]
		}
		// Update totals
		rs.TotalCount++
		rs.TotalStatus[sr.status]++
		rs.TotalTime += elapsed
		if elapsed < rs.TotalMin {
			rs.TotalMin = elapsed
		}
		if elapsed > rs.TotalMax {
			rs.TotalMax = elapsed
		}
		rs.LastSeen = time.Now()
		if r.URL.Query().Get("fail") == "1" || sr.status >= 400 {
			rs.TotalErrorCount++
		}
		mu.Unlock()
	})
}

func EnableDashboard() {
	dashboardEnabled = true
}

func EnablePrometheus(path string) {
	prometheusEnabled = true
	prometheusPath = path
}

func calcWindowStats(recs []RequestRecord, window time.Duration) (count, errCount int, status map[int]int, avg, min, max, rps float64) {
	if len(recs) == 0 {
		return 0, 0, map[int]int{}, 0, -1, -1, -1
	}
	status = make(map[int]int)
	var sum time.Duration
	minDur := time.Duration(1<<63 - 1)
	maxDur := time.Duration(0)
	start := time.Now().Add(-window)
	firstTs := time.Time{}
	lastTs := time.Time{}
	for _, rec := range recs {
		if rec.Timestamp.After(start) {
			if count == 0 {
				firstTs = rec.Timestamp
			}
			lastTs = rec.Timestamp
			count++
			if rec.Status >= 400 {
				errCount++
			}
			status[rec.Status]++
			sum += rec.Duration
			if rec.Duration < minDur {
				minDur = rec.Duration
			}
			if rec.Duration > maxDur {
				maxDur = rec.Duration
			}
		}
	}
	if count == 0 {
		return 0, 0, status, 0, -1, -1, -1
	}
	avg = float64(sum.Milliseconds()) / float64(count)
	min = float64(minDur.Milliseconds())
	max = float64(maxDur.Milliseconds())
	if count > 1 && lastTs.After(firstTs) {
		rps = float64(count) / lastTs.Sub(firstTs).Seconds()
	} else {
		rps = -1
	}
	return
}

var DashboardHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	if !dashboardEnabled {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Dashboard disabled"))
		return
	}
	mu.Lock()
	// Prepare data for JS rendering
	type Row struct {
		Method      string
		Path        string
		Count       int
		ErrorCount  int
		Status      map[int]int
		Avg         float64
		Min         float64
		Max         float64
		Throughput  float64
		HasError    bool
	}
	data := map[string][]Row{}
	for _, win := range windows {
		rows := []Row{}
		for method, paths := range stats {
			for path, s := range paths {
				count, errCount, status, avg, min, max, rps := calcWindowStats(s.Recent, win.Length)
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
	for method, paths := range stats {
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
	mu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html lang='en'>
<head>
<meta charset='UTF-8'>
<title>goapimon Dashboard</title>
<style>
:root {
  --bg: #f8f9fa;
  --fg: #222;
  --header-bg: #fff;
  --header-fg: #222;
  --accent: #007bff;
  --tab-bg: #f1f3f6;
  --tab-active: #fff;
  --error-bg: #ffeaea;
  --table-bg: #fff;
  --table-stripe: #f6f8fa;
  --border: #eee;
  --badge-2xx: #2ecc40;
  --badge-3xx: #3498db;
  --badge-4xx: #f1c40f;
  --badge-5xx: #e74c3c;
  --badge-other: #888;
}
body.dark {
  --bg: #181a1b;
  --fg: #e4e6e7;
  --header-bg: #23272a;
  --header-fg: #e4e6e7;
  --accent: #4ea1ff;
  --tab-bg: #23272a;
  --tab-active: #181a1b;
  --error-bg: #3a2323;
  --table-bg: #23272a;
  --table-stripe: #181a1b;
  --border: #333;
  --badge-2xx: #27d97a;
  --badge-3xx: #4ea1ff;
  --badge-4xx: #ffe066;
  --badge-5xx: #ff7675;
  --badge-other: #aaa;
}
body {
  font-family: 'Inter', system-ui, sans-serif;
  margin: 0;
  background: var(--bg);
  color: var(--fg);
  min-height: 100vh;
}
#header {
  background: var(--header-bg);
  color: var(--header-fg);
  padding: 1.2em 1.5em 1em 1.5em;
  display: flex;
  align-items: center;
  justify-content: space-between;
  box-shadow: 0 2px 16px rgba(0,0,0,0.04);
  border-bottom: 1px solid var(--border);
}
#header h1 {
  margin: 0;
  font-size: 2.1em;
  font-weight: 800;
  letter-spacing: -1px;
  display: flex;
  align-items: center;
  gap: 0.5em;
  font-family: 'Inter', system-ui, sans-serif;
}
#header .logo {
  font-size: 1.25em;
  color: var(--accent);
  font-weight: 900;
  letter-spacing: 0;
  font-family: 'Inter', system-ui, sans-serif;
}
#header .subtitle {
  font-size: 1.1em;
  font-weight: 400;
  color: var(--accent);
  margin-left: 1.2em;
  letter-spacing: 0.5px;
}
#theme-toggle {
  background: var(--tab-bg);
  color: var(--accent);
  border: 1px solid var(--border);
  border-radius: 1.5em;
  padding: 0.3em 1.1em;
  font-size: 1em;
  cursor: pointer;
  margin-left: 1em;
  transition: background 0.2s, color 0.2s;
  outline: none;
}
#theme-toggle:focus {
  box-shadow: 0 0 0 2px var(--accent);
}
#theme-toggle:hover {
  background: var(--accent);
  color: #fff;
}
#tabs { display: flex; border-bottom: 2px solid var(--border); margin-bottom: 1em; }
.tab { padding: 0.7em 1.5em; cursor: pointer; border: none; background: var(--tab-bg); font-size: 1em; color: var(--fg); transition: background 0.2s, color 0.2s; outline: none; }
.tab:focus { box-shadow: 0 0 0 2px var(--accent); }
.tab.active { border-bottom: 3px solid var(--accent); color: var(--accent); background: var(--tab-active); font-weight: bold; }
#filters { margin: 1em 0; display: flex; gap: 1em; align-items: center; }
input, select { padding: 0.3em 0.6em; font-size: 1em; border-radius: 0.3em; border: 1px solid var(--border); background: var(--table-bg); color: var(--fg); outline: none; }
input:focus, select:focus { box-shadow: 0 0 0 2px var(--accent); }
button { border-radius: 0.3em; border: 1px solid var(--border); background: var(--tab-bg); color: var(--fg); cursor: pointer; outline: none; }
button:focus { box-shadow: 0 0 0 2px var(--accent); }
table { border-collapse: collapse; width: 100%; background: var(--table-bg); box-shadow: 0 2px 8px rgba(0,0,0,0.03); border-radius: 0.5em; overflow: hidden; }
th, td { padding: 0.5em 0.7em; text-align: left; }
th { position: sticky; top: 0; background: var(--tab-bg); z-index: 1; font-weight: 700; letter-spacing: 0.5px; }
tr:nth-child(even) { background: var(--table-stripe); }
tr.error { background: var(--error-bg); }
td.status { font-size: 0.98em; }
.status-badges { display: flex; gap: 0.3em; flex-wrap: wrap; }
.badge { display: inline-block; min-width: 2.2em; padding: 0.18em 0.7em; border-radius: 1em; font-size: 0.98em; font-weight: 600; color: #fff; background: var(--badge-other); text-align: center; box-shadow: 0 1px 2px rgba(0,0,0,0.03); transition: background 0.2s; cursor: default; position: relative; }
.badge-2xx { background: var(--badge-2xx); }
.badge-3xx { background: var(--badge-3xx); }
.badge-4xx { background: var(--badge-4xx); color: #222; }
.badge-5xx { background: var(--badge-5xx); }
.badge-other { background: var(--badge-other); }
.badge[title] { border-bottom: 1px dotted #fff; cursor: help; }
@media (max-width: 700px) {
table, thead, tbody, th, td, tr { display: block; }
th { top: auto; position: static; }
tr { margin-bottom: 1em; }
td { border-bottom: 1px solid var(--border); }
#header { flex-direction: column; align-items: flex-start; gap: 0.7em; }
}
</style>
</head>
<body>
<div id='header'>
  <h1><span class='logo'>goapimon</span> <span class='subtitle'>API Monitor</span></h1>
  <button id='theme-toggle' onclick='toggleTheme()'>🌙 Dark</button>
</div>
<div id='tabs'></div>
<div id='filters'>
<label>Path: <input id='pathFilter' placeholder='Filter by path'></label>
<label>Method: <select id='methodFilter'><option value=''>All</option></select></label>
<button onclick='refresh()'>Refresh</button>
<label style='display:flex;align-items:center;gap:0.3em;cursor:pointer;font-size:0.97em;'><input type='checkbox' id='autorefreshbox' style='accent-color:var(--accent);margin:0;'> Auto-refresh</label>
<span style='color:#888;font-size:0.95em;' id='autorefresh'></span>
</div>
<div id='tableWrap'></div>
<script>
const data = ` + "`" + string(mustJSON(data)) + "`" + `;
const parsed = JSON.parse(data);
const windows = ["1m","2m","5m","total"];
let current = localStorage.getItem('goapimon-tab') || "1m";
let timer = null;
let autoRefreshEnabled = localStorage.getItem('goapimon-autorefresh') === '1';
function renderTabs() {
  const tabs = document.getElementById('tabs');
  tabs.innerHTML = windows.map(function(w) {
    return '<button class="tab' + (w===current ? ' active' : '') + '" onclick="switchTab(\'' + w + '\')">' + w + '</button>';
  }).join('');
}
function statusBadges(status) {
  let html = '<span class="status-badges">';
  const codes = Object.keys(status).map(Number).sort((a,b)=>a-b);
  for (let i=0; i<codes.length; ++i) {
    const code = codes[i];
    const count = status[code];
    let cls = 'badge-other';
    if (code >= 200 && code < 300) cls = 'badge-2xx';
    else if (code >= 300 && code < 400) cls = 'badge-3xx';
    else if (code >= 400 && code < 500) cls = 'badge-4xx';
    else if (code >= 500 && code < 600) cls = 'badge-5xx';
    html += '<span class="badge ' + cls + '" title="' + code + ' status">' + code + ' <span style="opacity:0.7;font-weight:400;">(' + count + ')</span></span>';
  }
  html += '</span>';
  return html;
}
function renderTable() {
  const rows = parsed[current] || [];
  const pathVal = document.getElementById('pathFilter').value.toLowerCase();
  const methodVal = document.getElementById('methodFilter').value;
  const methods = new Set();
  let html = '<table><thead><tr><th>Method</th><th>Path</th><th>Count</th><th>Error</th><th>Status</th><th>Avg ms</th><th>Min ms</th><th>Max ms</th><th>RPS</th></tr></thead><tbody>';
  for (let i=0; i<rows.length; ++i) {
    const row = rows[i];
    methods.add(row.Method);
    if (pathVal && row.Path.toLowerCase().indexOf(pathVal) === -1) continue;
    if (methodVal && row.Method !== methodVal) continue;
    html += '<tr' + (row.HasError ? ' class="error"' : '') + '><td>' + row.Method + '</td><td>' + row.Path + '</td><td>' + row.Count + '</td><td>' + row.ErrorCount + '</td><td class="status">' + statusBadges(row.Status) + '</td><td>' + row.Avg.toFixed(2) + '</td><td>' + (row.Min === -1 ? 'N/A' : row.Min.toFixed(2)) + '</td><td>' + (row.Max === -1 ? 'N/A' : row.Max.toFixed(2)) + '</td><td>' + (row.Throughput === -1 ? 'N/A' : row.Throughput.toFixed(2)) + '</td></tr>';
  }
  html += '</tbody></table>';
  document.getElementById('tableWrap').innerHTML = html;
  // Fill method filter
  const sel = document.getElementById('methodFilter');
  const prev = sel.value;
  sel.innerHTML = '<option value="">All</option>' + Array.from(methods).sort().map(function(m){return '<option value="'+m+'">'+m+'</option>';}).join('');
  sel.value = prev;
}
function switchTab(w) { current = w; localStorage.setItem('goapimon-tab', w); renderTabs(); renderTable(); }
document.getElementById('pathFilter').oninput = renderTable;
document.getElementById('methodFilter').onchange = renderTable;
function refresh() { localStorage.setItem('goapimon-tab', current); location.reload(); }
function autoRefresh() {
  clearInterval(timer);
  if (!autoRefreshEnabled) {
    document.getElementById('autorefresh').textContent = '';
    return;
  }
  let left = 5;
  document.getElementById('autorefresh').textContent = 'Auto-refresh in ' + left + 's';
  timer = setInterval(function(){
    left--;
    document.getElementById('autorefresh').textContent = 'Auto-refresh in ' + left + 's';
    if (left===0) {
      localStorage.setItem('goapimon-tab', current);
      location.reload();
    }
  },1000);
}
document.getElementById('autorefreshbox').checked = autoRefreshEnabled;
document.getElementById('autorefreshbox').onchange = function() {
  autoRefreshEnabled = this.checked;
  localStorage.setItem('goapimon-autorefresh', autoRefreshEnabled ? '1' : '');
  autoRefresh();
};
function toggleTheme() {
  const body = document.body;
  const btn = document.getElementById('theme-toggle');
  const dark = body.classList.toggle('dark');
  btn.textContent = dark ? '☀️ Light' : '🌙 Dark';
  localStorage.setItem('goapimon-theme', dark ? 'dark' : '');
}
(function(){
  if(localStorage.getItem('goapimon-theme')==='dark') {
    document.body.classList.add('dark');
    document.getElementById('theme-toggle').textContent = '☀️ Light';
  }
})();
renderTabs();
renderTable();
autoRefresh();
</script>
</body>
</html>`))
}

// Helper to marshal JSON or panic
func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

var PrometheusHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	if !prometheusEnabled {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Prometheus metrics disabled"))
		return
	}
	mu.Lock()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for method, paths := range stats {
		for path, s := range paths {
			for _, win := range windows {
				count, errCount, status, avg, min, max, thru := calcWindowStats(s.Recent, win.Length)
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
	mu.Unlock()
} 