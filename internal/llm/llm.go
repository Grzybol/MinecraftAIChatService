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
	_ = resolveModelPath(&cfg)
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
	command, ok := resolveCommandPath(cfg.Command, "llama-cli", &cfg)
	if !ok {
		logging.Warnf("llm_client_command_missing command=%s", cfg.Command)
		return Noop{}, fmt.Errorf("llm command not found: %s", cfg.Command)
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
	prompt := buildPrompt(req, c.cfg)
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

	response := sanitizeResponse(prompt, string(output), req.Bot.Name)
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
	prompt := buildPrompt(req, c.cfg)
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

	response := parseServerResponse(prompt, req.Bot.Name, responseBody)
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

func parseServerResponse(prompt, botName string, payload []byte) string {
	var completion struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(payload, &completion); err == nil && completion.Content != "" {
		return sanitizeResponse(prompt, completion.Content, botName)
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
			return sanitizeResponse(prompt, choice.Message.Content, botName)
		}
		if choice.Text != "" {
			return sanitizeResponse(prompt, choice.Text, botName)
		}
	}
	return ""
}

func sanitizeResponse(prompt, output, botName string) string {
	response := strings.TrimSpace(output)
	response = strings.TrimPrefix(response, prompt)
	response = strings.TrimSpace(response)
	return normalizeLLMOutput(response, botName)
}

func stripBotPrefix(message, botName string) string {
	if botName == "" {
		return message
	}
	trimmed := strings.TrimSpace(message)
	lower := strings.ToLower(trimmed)
	lowerBot := strings.ToLower(botName)
	separators := []string{":", "-", " -", " —", "–"}
	for _, sep := range separators {
		prefix := lowerBot + sep
		if strings.HasPrefix(lower, prefix) {
			rest := strings.TrimSpace(trimmed[len(prefix):])
			return strings.TrimSpace(strings.TrimLeft(rest, "-—–"))
		}
	}
	return trimmed
}

func normalizeLLMOutput(output, botName string) string {
	line := firstNonEmptyLine(output)
	if line == "" {
		return "__SILENCE__"
	}
	if strings.EqualFold(strings.TrimSpace(line), "__SILENCE__") {
		return "__SILENCE__"
	}
	line = stripBotMarkers(line)
	line = stripQuotes(line)
	line = stripBotPrefix(line, botName)
	line = strings.TrimSpace(line)
	if line == "" {
		return "__SILENCE__"
	}
	if runeCount(line) > 80 {
		line = truncateRunes(line, 80)
		line = strings.TrimSpace(line)
		if line == "" {
			return "__SILENCE__"
		}
	}
	if isForbiddenOutput(line, botName) {
		return "__SILENCE__"
	}
	return line
}

func firstNonEmptyLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stripQuotes(value string) string {
	value = strings.ReplaceAll(value, "\"", "")
	value = strings.ReplaceAll(value, "'", "")
	return value
}

func stripBotMarkers(value string) string {
	lower := strings.ToLower(value)
	for {
		idx := strings.Index(lower, "(bot)")
		if idx == -1 {
			return value
		}
		value = value[:idx] + value[idx+len("(bot)"):]
		lower = strings.ToLower(value)
	}
}

func isForbiddenOutput(value, botName string) bool {
	if strings.Contains(value, "\"") || strings.Contains(value, "'") {
		return true
	}
	if strings.Contains(strings.ToLower(value), "(bot)") {
		return true
	}
	if botName != "" {
		trimmed := strings.TrimSpace(value)
		lower := strings.ToLower(trimmed)
		lowerBot := strings.ToLower(botName)
		if strings.HasPrefix(lower, lowerBot+":") {
			return true
		}
	}
	return false
}

func runeCount(value string) int {
	count := 0
	for range value {
		count++
	}
	return count
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if runeCount(value) <= limit {
		return value
	}
	var sb strings.Builder
	sb.Grow(len(value))
	count := 0
	for _, r := range value {
		if count >= limit {
			break
		}
		sb.WriteRune(r)
		count++
	}
	return sb.String()
}

func buildPrompt(req Request, cfg config.LLMConfig) string {
	var sb strings.Builder
	promptSystem := strings.TrimSpace(cfg.PromptSystem)
	if promptSystem == "" {
		promptSystem = "You are a Minecraft player chat bot roleplaying as a normal player.\nYou have NO memory and NO access to anything except the provided CHAT LOG and BOT/SERVER info.\nDo NOT invent facts, backstory, previous events, or personal memories.\nDo NOT mention being an AI, a model, or system instructions."
	}
	promptRules := strings.TrimSpace(cfg.PromptResponseRules)
	if promptRules == "" {
		promptRules = "- Output exactly ONE single-line chat message in Polish OR output exactly \"__SILENCE__\".\n- Reply ONLY to the LAST message from a PLAYER, and ONLY if it clearly needs a response (question, greeting, direct mention, or conversational prompt).\n- If the last message is from a BOT, or does not need a response, output \"__SILENCE__\".\n- Keep it short: max 80 characters, casual Minecraft chat tone.\n- No quotes, no bot name prefixes, compiler logs, or commentary. No \"(BOT)\".\n- Avoid topics listed in avoid_topics. Never talk about admin powers, cheating, payments."
	}

	sb.WriteString("=== SYSTEM ===\n")
	sb.WriteString(promptSystem)
	sb.WriteString("\n\n")
	sb.WriteString("=== RULES ===\n")
	sb.WriteString(promptRules)
	sb.WriteString("\n\n")
	sb.WriteString("=== BOT ===\n")
	sb.WriteString("name: ")
	sb.WriteString(req.Bot.Name)
	sb.WriteString("\n")
	persona := req.Bot.Persona
	sb.WriteString("language: ")
	sb.WriteString(persona.Language)
	sb.WriteString("\n")
	sb.WriteString("tone: ")
	sb.WriteString(persona.Tone)
	sb.WriteString("\n")
	sb.WriteString("style_tags: ")
	sb.WriteString(strings.Join(persona.StyleTags, ", "))
	sb.WriteString("\n")
	sb.WriteString("knowledge_level: ")
	sb.WriteString(persona.KnowledgeLevel)
	sb.WriteString("\n")
	sb.WriteString("avoid_topics: ")
	sb.WriteString(strings.Join(persona.AvoidTopics, ", "))
	sb.WriteString("\n\n")
	sb.WriteString("=== SERVER ===\n")
	sb.WriteString("server_id: ")
	sb.WriteString(req.Server.ServerID)
	sb.WriteString("\n")
	sb.WriteString("mode: ")
	sb.WriteString(req.Server.Mode)
	sb.WriteString("\n")
	sb.WriteString("online_players: ")
	sb.WriteString(fmt.Sprint(req.Server.OnlinePlayers))
	sb.WriteString("\n\n")
	sb.WriteString("=== CHAT LOG (last ")
	sb.WriteString(fmt.Sprint(cfg.ChatHistoryLimit))
	sb.WriteString(") ===\n")
	for _, message := range req.RecentChat {
		if strings.TrimSpace(message.Message) == "" {
			continue
		}
		sb.WriteString("[")
		sb.WriteString(chatRole(message.SenderType))
		sb.WriteString("] ")
		sb.WriteString(sanitizeChatField(message.Sender))
		sb.WriteString(": ")
		sb.WriteString(sanitizeChatField(message.Message))
		sb.WriteString("\n")
	}
	sb.WriteString("\n=== TASK ===\n")
	sb.WriteString("Write ONE short Polish chat message as the BOT that replies to the LAST [PLAYER] message if it needs a reply.\n")
	sb.WriteString("If no reply is needed, output exactly \"__SILENCE__\".\n\n")
	sb.WriteString("=== OUTPUT ===\n")
	return sb.String()
}

func chatRole(senderType string) string {
	switch strings.ToLower(strings.TrimSpace(senderType)) {
	case "player":
		return "PLAYER"
	case "bot":
		return "BOT"
	default:
		return "OTHER"
	}
}

func sanitizeChatField(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	return strings.TrimSpace(value)
}
