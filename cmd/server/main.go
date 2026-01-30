package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"aichatplayers/internal/api"
	"aichatplayers/internal/config"
	"aichatplayers/internal/llm"
	"aichatplayers/internal/logging"
	"aichatplayers/internal/planner"
)

const bodyLimitBytes = 1 << 20

func main() {
	listenAddr := flag.String("listen", ":8090", "http listen address")
	flag.Parse()

	logFile, err := initLogging()
	if err != nil {
		logging.Fatalf("failed to init logging: %v", err)
	}
	if logFile != nil {
		defer logFile.Close()
	}

	cfg, err := config.Load()
	if err != nil {
		logging.Fatalf("failed to load config: %v", err)
	}

	serverProcess, err := llm.EnsureServerReady(cfg.LLM)
	if err != nil {
		logging.Errorf("llm_server_start_failed error=%v fallback=heuristics", err)
	}
	if serverProcess != nil {
		defer serverProcess.Close()
	}

	llmClient, err := llm.NewClient(cfg.LLM)
	if err != nil {
		logging.Errorf("llm_init_failed error=%v fallback=heuristics", err)
	}
	defer llmClient.Close()
	if llmClient.Enabled() {
		logging.Infof("llm_enabled model_path=%s ctx=%d threads=%d timeout=%s soft_timeout=%s", cfg.LLM.ModelPath, cfg.LLM.CtxSize, cfg.LLM.NumThreads, cfg.LLM.Timeout, cfg.LLM.SoftTimeout)
	}

	plan := planner.NewPlanner(llmClient, planner.Config{LLMTimeout: cfg.LLM.SoftTimeout})
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

	logging.Infof("listening on %s", *listenAddr)
	if err := server.ListenAndServe(); err != nil {
		logging.Fatalf("server stopped: %v", err)
	}
}

func initLogging() (*os.File, error) {
	if err := os.MkdirAll("logs", 0o755); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}
	logTimestamp := time.Now().Unix()
	logPath := filepath.Join("logs", fmt.Sprintf("logs_%d", logTimestamp))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	logging.SetLevelFromEnv("LOG_LEVEL")
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)
	logging.Infof("logging initialized path=%s", logPath)
	return logFile, nil
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
