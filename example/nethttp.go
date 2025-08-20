package main

import (
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/aurieli333/goapimon"
)

func main() {
	mux := http.NewServeMux()

	// Your API endpoint
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		rand.Seed(time.Now().UnixNano())
		randomDuration := time.Duration(rand.Float64() * float64(time.Second))
		time.Sleep(randomDuration)
		w.Write([]byte("Hello!"))
	})

	// Turn on Dashboard (optional)
	goapimon.DashboardEnable()

	// Turn on Prometheus with metrics path (optional)
	goapimon.PrometheusEnable("/metrics")

	// Add handlers
	mux.HandleFunc("/__goapimon/", goapimon.DashboardHandler)
	mux.HandleFunc("/metrics", goapimon.PrometheusHandler)

	logged := goapimon.MiddlewareNetHTTP(goapimon.Monitor, mux)

	log.Println("Starting server on :8080")
	err := http.ListenAndServe(":8080", logged)
	if err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
