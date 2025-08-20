package adapters

import (
	"strings"
	"time"

	"github.com/aurieli333/goapimon/monitor"
	"github.com/aurieli333/goapimon/utility"

	"github.com/gin-gonic/gin"
)

// MiddlewareGin — adapter for Gin
func MiddlewareGin(m *monitor.Monitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		if utility.IsInternalPath(strings.Split(c.Request.URL.Path, `/`)[1]) || utility.IsInternalPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()
		elapsed := time.Since(start)

		method := c.Request.Method
		path := utility.NormalizePath(c.FullPath()) // FullPath for routes with params
		status := c.Writer.Status()
		m.CoreMiddleware(method, path, status, start, elapsed)
	}
}
