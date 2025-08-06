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
