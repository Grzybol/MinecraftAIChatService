package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
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

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logFile, elasticLogger, err := initLogging(cfg.Elastic)
	if err != nil {
		log.Fatalf("failed to init logging: %v", err)
	}
	if logFile != nil {
		defer logFile.Close()
	}
	if elasticLogger != nil {
		defer elasticLogger.Close()
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

	plan := planner.NewPlanner(llmClient, planner.Config{
		LLMTimeout:       cfg.LLM.SoftTimeout,
		ChatHistoryLimit: cfg.LLM.ChatHistoryLimit,
	})
	h := &api.Handler{Planner: plan}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", methodGuard("GET", h.Healthz))
	mux.HandleFunc("/v1/plan", methodGuard("POST", h.Plan))
	mux.HandleFunc("/v1/engagement", methodGuard("POST", h.Engagement))
	mux.HandleFunc("/v1/bots/register", methodGuard("POST", h.RegisterBots))

	wrapped := api.WithRequestID(api.RequestLogging(api.LimitBodySize(bodyLimitBytes, api.RequestErrorLogging(api.RequestDebugLogging(mux)))))

	server := &http.Server{
		Addr:         *listenAddr,
		Handler:      wrapped,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	logging.Infof("listening on %s", *listenAddr)
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logging.Infof("shutdown_signal_received signal=%s", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logging.Errorf("server_shutdown_failed error=%v", err)
		}
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logging.Errorf("server_stopped error=%v", err)
		}
	}
}

func initLogging(elasticCfg config.ElasticConfig) (*os.File, *logging.ElasticLogger, error) {
	logDir := strings.TrimSpace(os.Getenv("LOG_DIR"))
	if logDir == "" {
		logDir = "logs"
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create logs dir: %w", err)
	}
	logTimestamp := time.Now().Unix()
	logPath := filepath.Join(logDir, fmt.Sprintf("logs_%d", logTimestamp))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	stdoutLevel := logging.LevelInfo
	if level, ok := logging.ParseLevel(os.Getenv("LOG_LEVEL")); ok {
		stdoutLevel = level
	}
	fileLevel := stdoutLevel
	if raw := strings.TrimSpace(os.Getenv("LOG_FILE_LEVEL")); raw != "" {
		if level, ok := logging.ParseLevel(raw); ok {
			fileLevel = level
		}
	}
	minLevel := stdoutLevel
	if fileLevel < minLevel {
		minLevel = fileLevel
	}
	elasticLevel := stdoutLevel
	if raw := strings.TrimSpace(os.Getenv("ELASTIC_LOG_LEVEL")); raw != "" {
		if level, ok := logging.ParseLevel(raw); ok {
			elasticLevel = level
		}
	}
	if elasticLevel < minLevel {
		minLevel = elasticLevel
	}
	logging.SetLevel(minLevel)
	var elasticLogger *logging.ElasticLogger
	if elasticCfg.URL != "" && elasticCfg.Index != "" {
		elasticLogger, err = logging.NewElasticLogger(elasticCfg.URL, elasticCfg.Index, elasticCfg.APIKey, elasticCfg.VerifyCert)
		if err != nil {
			return nil, nil, fmt.Errorf("init elastic logger: %w", err)
		}
	}
	outputs := []io.Writer{logging.NewSplitWriter(os.Stdout, stdoutLevel, logFile, fileLevel)}
	if elasticLogger != nil {
		outputs = append(outputs, logging.NewElasticWriter(elasticLogger, elasticLevel))
	}
	log.SetOutput(io.MultiWriter(outputs...))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)
	logging.Infof("logging initialized path=%s stdout_level=%s file_level=%s", logPath, stdoutLevel, fileLevel)
	if elasticLogger != nil {
		logging.Infof("elastic_logging_enabled url=%s index=%s verify_cert=%t", elasticCfg.URL, elasticCfg.Index, elasticCfg.VerifyCert)
	}
	return logFile, elasticLogger, nil
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
