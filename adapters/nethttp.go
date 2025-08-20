package adapters

import (
	"net/http"
	"strings"
	"time"

	"github.com/aurieli333/goapimon/model"
	"github.com/aurieli333/goapimon/monitor"
	"github.com/aurieli333/goapimon/utility"
)

// MiddlewareNetHTTP — adapter for net/http
func MiddlewareNetHTTP(m *monitor.Monitor, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if utility.IsInternalPath(strings.Split(r.URL.Path, `/`)[1]) || utility.IsInternalPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		sr := &model.StatusRecorder{ResponseWriter: w, Status: 200}
		start := time.Now()
		next.ServeHTTP(sr, r)
		elapsed := time.Since(start)

		method := r.Method
		path := utility.NormalizePath(r.URL.Path)
		m.CoreMiddleware(method, path, sr.Status, start, elapsed)
	})
}
