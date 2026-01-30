package config

import (
	"testing"
	"time"
)

func TestLoadLLMConfigOverrides(t *testing.T) {
	t.Setenv("LLM_MODEL_PATH", "/tmp/model.gguf")
	t.Setenv("LLM_MODELS_DIR", "/tmp/models")
	t.Setenv("LLM_SERVER_URL", "http://127.0.0.1:8080")
	t.Setenv("LLM_SERVER_COMMAND", "/usr/local/bin/llama-server")
	t.Setenv("LLM_COMMAND", "/usr/local/bin/llama-cli")
	t.Setenv("LLM_MAX_RAM_MB", "1536")
	t.Setenv("LLM_NUM_THREADS", "6")
	t.Setenv("LLM_CTX_SIZE", "4096")
	t.Setenv("LLM_TIMEOUT_MS", "3500")
	t.Setenv("LLM_SOFT_TIMEOUT_MS", "3000")
	t.Setenv("LLM_SERVER_STARTUP_TIMEOUT_MS", "45000")
	t.Setenv("LLM_TEMPERATURE", "0.25")
	t.Setenv("LLM_TOP_P", "0.8")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.LLM.ModelPath != "/tmp/model.gguf" {
		t.Fatalf("ModelPath = %q", cfg.LLM.ModelPath)
	}
	if cfg.LLM.ModelsDir != "/tmp/models" {
		t.Fatalf("ModelsDir = %q", cfg.LLM.ModelsDir)
	}
	if cfg.LLM.ServerURL != "http://127.0.0.1:8080" {
		t.Fatalf("ServerURL = %q", cfg.LLM.ServerURL)
	}
	if cfg.LLM.ServerCommand != "/usr/local/bin/llama-server" {
		t.Fatalf("ServerCommand = %q", cfg.LLM.ServerCommand)
	}
	if cfg.LLM.Command != "/usr/local/bin/llama-cli" {
		t.Fatalf("Command = %q", cfg.LLM.Command)
	}
	if cfg.LLM.MaxRAMMB != 1536 {
		t.Fatalf("MaxRAMMB = %d", cfg.LLM.MaxRAMMB)
	}
	if cfg.LLM.NumThreads != 6 {
		t.Fatalf("NumThreads = %d", cfg.LLM.NumThreads)
	}
	if cfg.LLM.CtxSize != 4096 {
		t.Fatalf("CtxSize = %d", cfg.LLM.CtxSize)
	}
	if cfg.LLM.Timeout != 3500*time.Millisecond {
		t.Fatalf("Timeout = %v", cfg.LLM.Timeout)
	}
	if cfg.LLM.SoftTimeout != 3000*time.Millisecond {
		t.Fatalf("SoftTimeout = %v", cfg.LLM.SoftTimeout)
	}
	if cfg.LLM.ServerStartupTimeout != 45*time.Second {
		t.Fatalf("ServerStartupTimeout = %v", cfg.LLM.ServerStartupTimeout)
	}
	if cfg.LLM.Temperature != 0.25 {
		t.Fatalf("Temperature = %v", cfg.LLM.Temperature)
	}
	if cfg.LLM.TopP != 0.8 {
		t.Fatalf("TopP = %v", cfg.LLM.TopP)
	}
}
