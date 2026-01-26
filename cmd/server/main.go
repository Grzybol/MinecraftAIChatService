package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"aichatplayers/internal/api"
	"aichatplayers/internal/planner"
)

const bodyLimitBytes = 1 << 20

func main() {
	listenAddr := flag.String("listen", ":8090", "http listen address")
	flag.Parse()

	plan := planner.NewPlanner()
	h := &api.Handler{Planner: plan}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", methodGuard("GET", h.Healthz))
	mux.HandleFunc("/v1/plan", methodGuard("POST", h.Plan))
	mux.HandleFunc("/v1/bots/register", methodGuard("POST", h.RegisterBots))

	wrapped := api.WithRequestID(api.RequestLogging(api.LimitBodySize(bodyLimitBytes, mux)))

	server := &http.Server{
		Addr:         *listenAddr,
		Handler:      wrapped,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("listening on %s", *listenAddr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func methodGuard(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}
