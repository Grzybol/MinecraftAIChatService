package planner

import (
	"context"
	"log"
	"math/rand"

	"aichatplayers/internal/llm"
	"aichatplayers/internal/models"
)

type LLMGenerator interface {
	Enabled() bool
	Generate(ctx context.Context, req llm.Request) (string, error)
	Close() error
}

type noopLLM struct{}

func (noopLLM) Enabled() bool { return false }

func (noopLLM) Generate(ctx context.Context, req llm.Request) (string, error) {
	return "", errLLMDisabled
}

func (noopLLM) Close() error { return nil }

func (p *Planner) generateMessage(req models.PlanRequest, topic Topic, bot models.BotProfile, rng *rand.Rand) (string, string, bool, bool) {
	if shouldAvoidTopic(topic, bot.Persona.AvoidTopics) {
		return "", "", false, false
	}
	if p.llm != nil && p.llm.Enabled() {
		ctx := context.Background()
		var cancel context.CancelFunc
		if p.llmTimeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, p.llmTimeout)
			defer cancel()
		}
		llmReq := llm.Request{
			Server:     req.Server,
			Bot:        bot,
			Topic:      string(topic),
			RecentChat: recentChat(req.Chat, 6),
		}
		message, err := p.llm.Generate(ctx, llmReq)
		if err != nil {
			log.Printf("planner_llm_error request_id=%s transaction_id=%s bot_id=%s topic=%s error=%v", req.RequestID, req.RequestID, bot.BotID, topic, err)
		} else if message != "" {
			log.Printf("planner_llm_response request_id=%s transaction_id=%s bot_id=%s topic=%s", req.RequestID, req.RequestID, bot.BotID, topic)
			return message, "llm", true, true
		}
		message, reason := generateResponse(topic, bot, rng)
		if message != "" {
			log.Printf("planner_heuristic_response request_id=%s transaction_id=%s bot_id=%s topic=%s reason=%s", req.RequestID, req.RequestID, bot.BotID, topic, reason)
		}
		return message, reason, true, false
	}
	message, reason := generateResponse(topic, bot, rng)
	if message != "" {
		log.Printf("planner_heuristic_response request_id=%s transaction_id=%s bot_id=%s topic=%s reason=%s", req.RequestID, req.RequestID, bot.BotID, topic, reason)
	}
	return message, reason, false, false
}

func recentChat(messages []models.ChatMessage, limit int) []models.ChatMessage {
	if limit <= 0 || len(messages) == 0 {
		return nil
	}
	if len(messages) <= limit {
		return messages
	}
	return messages[len(messages)-limit:]
}

func strategyLabel(base string, llmAttempted, llmUsed bool) string {
	if llmUsed {
		return "llm"
	}
	if llmAttempted {
		return base + "_fallback"
	}
	return base
}

var errLLMDisabled = &llmError{message: "llm disabled"}

type llmError struct {
	message string
}

func (e *llmError) Error() string { return e.message }
