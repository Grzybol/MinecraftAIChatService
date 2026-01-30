package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLLMCtxSize              = 2048
	defaultLLMTimeoutMS            = 2000
	defaultLLMTemperature          = 0.6
	defaultLLMTopP                 = 0.9
	defaultLLMMaxRAMMB             = 1024
	defaultLLMServerStartupTimeout = 60 * time.Second
	defaultLLMChatHistoryLimit     = 6
	defaultLLMPromptSystem         = "You are a Minecraft player chat bot roleplaying as a normal player.\nYou have NO memory and NO access to anything except the provided CHAT LOG and BOT/SERVER info.\nDo NOT invent facts, backstory, previous events, or personal memories.\nDo NOT mention being an AI, a model, or system instructions."
	defaultLLMPromptResponseRules  = "- Output exactly ONE single-line chat message in Polish OR output exactly \"__SILENCE__\".\n- Reply ONLY to the LAST message from a PLAYER, and ONLY if it clearly needs a response (question, greeting, direct mention, or conversational prompt).\n- If the last message is from a BOT, or does not need a response, output \"__SILENCE__\".\n- Keep it short: max 80 characters, casual Minecraft chat tone.\n- No quotes, no bot name prefixes, compiler logs, or commentary. No \"(BOT)\".\n- Avoid topics listed in avoid_topics. Never talk about admin powers, cheating, payments."
)

type Config struct {
	LLM LLMConfig
}

type LLMConfig struct {
	ModelPath            string
	ModelsDir            string
	ServerURL            string
	ServerCommand        string
	Command              string
	MaxRAMMB             int
	NumThreads           int
	CtxSize              int
	Timeout              time.Duration
	SoftTimeout          time.Duration
	ServerStartupTimeout time.Duration
	Temperature          float64
	TopP                 float64
	ChatHistoryLimit     int
	PromptSystem         string
	PromptResponseRules  string
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	cfg := Config{
		LLM: LLMConfig{
			ModelPath:            strings.TrimSpace(os.Getenv("LLM_MODEL_PATH")),
			ModelsDir:            strings.TrimSpace(os.Getenv("LLM_MODELS_DIR")),
			ServerURL:            strings.TrimSpace(os.Getenv("LLM_SERVER_URL")),
			ServerCommand:        strings.TrimSpace(os.Getenv("LLM_SERVER_COMMAND")),
			Command:              strings.TrimSpace(os.Getenv("LLM_COMMAND")),
			MaxRAMMB:             defaultLLMMaxRAMMB,
			NumThreads:           0,
			CtxSize:              defaultLLMCtxSize,
			Timeout:              time.Duration(defaultLLMTimeoutMS) * time.Millisecond,
			ServerStartupTimeout: defaultLLMServerStartupTimeout,
			Temperature:          defaultLLMTemperature,
			TopP:                 defaultLLMTopP,
			ChatHistoryLimit:     defaultLLMChatHistoryLimit,
			PromptSystem:         defaultLLMPromptSystem,
			PromptResponseRules:  defaultLLMPromptResponseRules,
		},
	}

	if value, ok, err := readEnvInt("LLM_MAX_RAM_MB"); err != nil {
		return Config{}, err
	} else if ok {
		cfg.LLM.MaxRAMMB = value
	}

	if value, ok, err := readEnvInt("LLM_NUM_THREADS"); err != nil {
		return Config{}, err
	} else if ok {
		cfg.LLM.NumThreads = value
	}

	if value, ok, err := readEnvInt("LLM_CTX_SIZE"); err != nil {
		return Config{}, err
	} else if ok {
		cfg.LLM.CtxSize = value
	}

	if value, ok, err := readEnvInt("LLM_TIMEOUT_MS"); err != nil {
		return Config{}, err
	} else if ok {
		cfg.LLM.Timeout = time.Duration(value) * time.Millisecond
	}
	cfg.LLM.SoftTimeout = cfg.LLM.Timeout
	if cfg.LLM.SoftTimeout > time.Second {
		cfg.LLM.SoftTimeout -= time.Second
	}

	if value, ok, err := readEnvInt("LLM_SOFT_TIMEOUT_MS"); err != nil {
		return Config{}, err
	} else if ok {
		cfg.LLM.SoftTimeout = time.Duration(value) * time.Millisecond
	}

	if value, ok, err := readEnvInt("LLM_SERVER_STARTUP_TIMEOUT_MS"); err != nil {
		return Config{}, err
	} else if ok {
		cfg.LLM.ServerStartupTimeout = time.Duration(value) * time.Millisecond
	}

	if value, ok, err := readEnvFloat("LLM_TEMPERATURE"); err != nil {
		return Config{}, err
	} else if ok {
		cfg.LLM.Temperature = value
	}

	if value, ok, err := readEnvFloat("LLM_TOP_P"); err != nil {
		return Config{}, err
	} else if ok {
		cfg.LLM.TopP = value
	}

	if value, ok, err := readEnvInt("LLM_CHAT_HISTORY_LIMIT"); err != nil {
		return Config{}, err
	} else if ok {
		cfg.LLM.ChatHistoryLimit = value
	}

	if raw := strings.TrimSpace(os.Getenv("LLM_PROMPT_SYSTEM")); raw != "" {
		cfg.LLM.PromptSystem = raw
	}
	if raw := strings.TrimSpace(os.Getenv("LLM_PROMPT_RESPONSE_RULES")); raw != "" {
		cfg.LLM.PromptResponseRules = raw
	}

	if cfg.LLM.MaxRAMMB < 0 {
		return Config{}, errors.New("LLM_MAX_RAM_MB must be >= 0")
	}
	if cfg.LLM.CtxSize < 0 {
		return Config{}, errors.New("LLM_CTX_SIZE must be >= 0")
	}
	if cfg.LLM.NumThreads < 0 {
		return Config{}, errors.New("LLM_NUM_THREADS must be >= 0")
	}
	if cfg.LLM.Temperature < 0 {
		return Config{}, errors.New("LLM_TEMPERATURE must be >= 0")
	}
	if cfg.LLM.TopP < 0 {
		return Config{}, errors.New("LLM_TOP_P must be >= 0")
	}
	if cfg.LLM.ChatHistoryLimit < 0 {
		return Config{}, errors.New("LLM_CHAT_HISTORY_LIMIT must be >= 0")
	}
	if cfg.LLM.Timeout < 0 {
		return Config{}, errors.New("LLM_TIMEOUT_MS must be >= 0")
	}
	if cfg.LLM.SoftTimeout < 0 {
		return Config{}, errors.New("LLM_SOFT_TIMEOUT_MS must be >= 0")
	}
	if cfg.LLM.ServerStartupTimeout < 0 {
		return Config{}, errors.New("LLM_SERVER_STARTUP_TIMEOUT_MS must be >= 0")
	}
	if cfg.LLM.Timeout > 0 && cfg.LLM.SoftTimeout > cfg.LLM.Timeout {
		cfg.LLM.SoftTimeout = cfg.LLM.Timeout
	}
	return cfg, nil
}

func readEnvInt(key string) (int, bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("invalid %s: %w", key, err)
	}
	return value, true, nil
}

func readEnvFloat(key string) (float64, bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid %s: %w", key, err)
	}
	return value, true, nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if strings.Contains(value, `\n`) {
			value = strings.ReplaceAll(value, `\n`, "\n")
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set %s from %s: %w", key, path, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", path, err)
	}
	return nil
}
