package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"

	"aichatplayers/internal/config"
	"aichatplayers/internal/logging"
	"aichatplayers/internal/models"
)

const defaultMaxTokens = 128

type Generator interface {
	Enabled() bool
	Generate(ctx context.Context, req Request) (string, error)
	Close() error
}

type Request struct {
	Server     models.ServerContext
	Bot        models.BotProfile
	Topic      string
	RecentChat []models.ChatMessage
}

type Client struct {
	cfg     config.LLMConfig
	command string
	enabled bool
}

type ServerClient struct {
	cfg     config.LLMConfig
	url     string
	client  *http.Client
	enabled bool
}

type Noop struct{}

func (Noop) Enabled() bool { return false }

func (Noop) Generate(ctx context.Context, req Request) (string, error) {
	return "", errors.New("llm disabled")
}

func (Noop) Close() error { return nil }

func NewClient(cfg config.LLMConfig) (Generator, error) {
	logging.Debugf("llm_client_init server_url=%q model_path=%q command=%q server_command=%q", cfg.ServerURL, cfg.ModelPath, cfg.Command, cfg.ServerCommand)
	if strings.TrimSpace(cfg.ServerURL) != "" {
		logging.Debugf("llm_client_mode server url configured")
		return newServerClient(cfg), nil
	}
	if cfg.ModelPath == "" {
		logging.Debugf("llm_client_disabled reason=missing_model_path")
		return Noop{}, nil
	}
	if cfg.Command == "" {
		cfg.Command = "llama-cli"
	}
	if _, err := os.Stat(cfg.ModelPath); err != nil {
		logging.Warnf("llm_client_model_unavailable path=%s error=%v", cfg.ModelPath, err)
		return Noop{}, fmt.Errorf("llm model path unavailable: %w", err)
	}
	command, err := exec.LookPath(cfg.Command)
	if err != nil {
		logging.Warnf("llm_client_command_missing command=%s error=%v", cfg.Command, err)
		return Noop{}, fmt.Errorf("llm command not found: %w", err)
	}
	logging.Debugf("llm_client_command_resolved command=%s path=%s", cfg.Command, command)
	if cfg.MaxRAMMB > 0 {
		debug.SetMemoryLimit(int64(cfg.MaxRAMMB) * 1024 * 1024)
		logging.Debugf("llm_client_memory_limit_set max_ram_mb=%d", cfg.MaxRAMMB)
	}
	return &Client{cfg: cfg, command: command, enabled: true}, nil
}

func (c *Client) Enabled() bool {
	if c == nil {
		return false
	}
	return c.enabled
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) Generate(ctx context.Context, req Request) (string, error) {
	if c == nil || !c.enabled {
		return "", errors.New("llm disabled")
	}
	prompt := buildPrompt(req)
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("llm prompt empty")
	}

	ctx, cancel := withTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	args := []string{
		"--model", c.cfg.ModelPath,
		"--prompt", prompt,
		"--n-predict", fmt.Sprint(defaultMaxTokens),
		"--temp", fmt.Sprint(c.cfg.Temperature),
		"--top-p", fmt.Sprint(c.cfg.TopP),
	}
	if c.cfg.CtxSize > 0 {
		args = append(args, "--ctx-size", fmt.Sprint(c.cfg.CtxSize))
	}
	if c.cfg.NumThreads > 0 {
		args = append(args, "--threads", fmt.Sprint(c.cfg.NumThreads))
	}

	cmd := exec.CommandContext(ctx, c.command, args...)
	configureCommand(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("llm timeout after %s", timeoutLabel(c.cfg.Timeout))
		}
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return "", fmt.Errorf("llm command failed: %w output=%s", err, trimmed)
		}
		return "", fmt.Errorf("llm command failed: %w", err)
	}

	response := sanitizeResponse(prompt, string(output))
	if response == "" {
		return "", errors.New("llm returned empty response")
	}
	return response, nil
}

func (c *ServerClient) Enabled() bool {
	if c == nil {
		return false
	}
	return c.enabled
}

func (c *ServerClient) Close() error {
	return nil
}

func (c *ServerClient) Generate(ctx context.Context, req Request) (string, error) {
	if c == nil || !c.enabled {
		return "", errors.New("llm disabled")
	}
	prompt := buildPrompt(req)
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("llm prompt empty")
	}

	ctx, cancel := withTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	payload := map[string]any{
		"prompt":      prompt,
		"n_predict":   defaultMaxTokens,
		"temperature": c.cfg.Temperature,
		"top_p":       c.cfg.TopP,
		"stream":      false,
	}
	if c.cfg.CtxSize > 0 {
		payload["n_ctx"] = c.cfg.CtxSize
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("llm server request encode: %w", err)
	}

	endpoint := strings.TrimRight(c.url, "/") + "/completion"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm server request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("llm timeout after %s", timeoutLabel(c.cfg.Timeout))
		}
		return "", fmt.Errorf("llm server request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("llm server read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		trimmed := strings.TrimSpace(string(responseBody))
		if trimmed != "" {
			return "", fmt.Errorf("llm server response status=%d body=%s", resp.StatusCode, trimmed)
		}
		return "", fmt.Errorf("llm server response status=%d", resp.StatusCode)
	}

	response := parseServerResponse(prompt, responseBody)
	if response == "" {
		return "", errors.New("llm returned empty response")
	}
	return response, nil
}

func newServerClient(cfg config.LLMConfig) *ServerClient {
	return &ServerClient{
		cfg:     cfg,
		url:     strings.TrimSpace(cfg.ServerURL),
		client:  &http.Client{},
		enabled: true,
	}
}

func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	effective := timeoutLabel(timeout)
	return context.WithTimeout(ctx, effective)
}

func timeoutLabel(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return 2 * time.Second
	}
	return timeout
}

func parseServerResponse(prompt string, payload []byte) string {
	var completion struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(payload, &completion); err == nil && completion.Content != "" {
		return sanitizeResponse(prompt, completion.Content)
	}

	var openAI struct {
		Choices []struct {
			Text    string `json:"text"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(payload, &openAI); err == nil && len(openAI.Choices) > 0 {
		choice := openAI.Choices[0]
		if choice.Message.Content != "" {
			return sanitizeResponse(prompt, choice.Message.Content)
		}
		if choice.Text != "" {
			return sanitizeResponse(prompt, choice.Text)
		}
	}
	return ""
}

func sanitizeResponse(prompt, output string) string {
	response := strings.TrimSpace(output)
	response = strings.TrimPrefix(response, prompt)
	response = strings.TrimSpace(response)
	if idx := strings.Index(response, "\n"); idx >= 0 {
		response = strings.TrimSpace(response[:idx])
	}
	return strings.Trim(response, "\"")
}

func buildPrompt(req Request) string {
	var sb strings.Builder
	sb.WriteString("You are a Minecraft chat bot.\n")
	if req.Bot.Name != "" {
		sb.WriteString("Bot name: ")
		sb.WriteString(req.Bot.Name)
		sb.WriteString("\n")
	}
	persona := req.Bot.Persona
	if persona.Language != "" {
		sb.WriteString("Language: ")
		sb.WriteString(persona.Language)
		sb.WriteString("\n")
	}
	if persona.Tone != "" {
		sb.WriteString("Tone: ")
		sb.WriteString(persona.Tone)
		sb.WriteString("\n")
	}
	if len(persona.StyleTags) > 0 {
		sb.WriteString("Style tags: ")
		sb.WriteString(strings.Join(persona.StyleTags, ", "))
		sb.WriteString("\n")
	}
	if persona.KnowledgeLevel != "" {
		sb.WriteString("Knowledge level: ")
		sb.WriteString(persona.KnowledgeLevel)
		sb.WriteString("\n")
	}
	if len(persona.AvoidTopics) > 0 {
		sb.WriteString("Avoid topics: ")
		sb.WriteString(strings.Join(persona.AvoidTopics, ", "))
		sb.WriteString("\n")
	}
	if req.Server.Mode != "" || req.Server.OnlinePlayers > 0 {
		sb.WriteString("Server mode: ")
		sb.WriteString(req.Server.Mode)
		sb.WriteString(", online players: ")
		sb.WriteString(fmt.Sprint(req.Server.OnlinePlayers))
		sb.WriteString("\n")
	}
	if req.Topic != "" {
		sb.WriteString("Topic: ")
		sb.WriteString(req.Topic)
		sb.WriteString("\n")
	} else {
		sb.WriteString("Topic: small_talk\n")
	}
	if len(req.RecentChat) > 0 {
		sb.WriteString("Recent chat:\n")
		for _, message := range req.RecentChat {
			if message.Message == "" {
				continue
			}
			sb.WriteString("- ")
			sb.WriteString(message.Sender)
			if message.SenderType != "" {
				sb.WriteString(" (")
				sb.WriteString(message.SenderType)
				sb.WriteString(")")
			}
			sb.WriteString(": ")
			sb.WriteString(message.Message)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("Respond with a single short chat message. Do not add quotes or extra commentary.\n")
	return sb.String()
}
