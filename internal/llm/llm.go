package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"

	"aichatplayers/internal/config"
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

type Noop struct{}

func (Noop) Enabled() bool { return false }

func (Noop) Generate(ctx context.Context, req Request) (string, error) {
	return "", errors.New("llm disabled")
}

func (Noop) Close() error { return nil }

func NewClient(cfg config.LLMConfig) (Generator, error) {
	if cfg.ModelPath == "" {
		return Noop{}, nil
	}
	if cfg.Command == "" {
		cfg.Command = "llama-cli"
	}
	if _, err := os.Stat(cfg.ModelPath); err != nil {
		return Noop{}, fmt.Errorf("llm model path unavailable: %w", err)
	}
	command, err := exec.LookPath(cfg.Command)
	if err != nil {
		return Noop{}, fmt.Errorf("llm command not found: %w", err)
	}
	if cfg.MaxRAMMB > 0 {
		debug.SetMemoryLimit(int64(cfg.MaxRAMMB) * 1024 * 1024)
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

	timeout := c.cfg.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
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
			return "", fmt.Errorf("llm timeout after %s", timeout)
		}
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return "", fmt.Errorf("llm command failed: %w output=%s", err, trimmed)
		}
		return "", fmt.Errorf("llm command failed: %w", err)
	}

	response := strings.TrimSpace(string(output))
	response = strings.TrimPrefix(response, prompt)
	response = strings.TrimSpace(response)
	if idx := strings.Index(response, "\n"); idx >= 0 {
		response = strings.TrimSpace(response[:idx])
	}
	response = strings.Trim(response, "\"")
	if response == "" {
		return "", errors.New("llm returned empty response")
	}
	return response, nil
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
