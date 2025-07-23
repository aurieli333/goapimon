package main

import (
	"log"
	"net/http"

	"github.com/aurieli333/goapimon"
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
