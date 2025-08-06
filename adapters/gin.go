package adapters

import (
	"net/http"

	"github.com/aurieli333/goapimon/monitor"

	"github.com/gin-gonic/gin"
)

// Adapter for connecting goapimon.Middleware to Gin
func Middleware(m *monitor.Monitor) gin.HandlerFunc {
	// Wrap in http.Handler, and then in gin.HandlerFunc
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	return gin.WrapH(h)
}
