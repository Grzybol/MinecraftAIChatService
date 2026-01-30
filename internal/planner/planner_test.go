package planner

import (
	"context"
	"errors"
	"testing"

	"aichatplayers/internal/llm"
	"aichatplayers/internal/models"
)

type fakeLLM struct {
	enabled bool
	message string
	err     error
}

func (f fakeLLM) Enabled() bool { return f.enabled }

func (f fakeLLM) Generate(ctx context.Context, req llm.Request) (string, error) {
	return f.message, f.err
}

func (f fakeLLM) Close() error { return nil }

func TestPlannerFallbacksToHeuristics(t *testing.T) {
	planner := NewPlanner(fakeLLM{enabled: true, err: errors.New("boom")}, Config{})
	req := models.PlanRequest{
		RequestID: "req-1",
		Server: models.ServerContext{
			ServerID:      "srv-1",
			Mode:          "LOBBY",
			OnlinePlayers: 10,
		},
		Tick:   123,
		TimeMS: 1712345000000,
		Bots: []models.BotProfile{
			{
				BotID:  "bot-1",
				Name:   "Kuba",
				Online: true,
				Persona: models.Persona{
					Language:       "pl",
					Tone:           "casual",
					StyleTags:      []string{"short"},
					AvoidTopics:    []string{"payments"},
					KnowledgeLevel: "average_player",
				},
			},
		},
		Chat: []models.ChatMessage{
			{
				TimestampMS: 1712344999000,
				Sender:      "RealPlayer123",
				SenderType:  "PLAYER",
				Message:     "hej kto pvp?",
			},
		},
		Settings: models.PlanSettings{
			MaxActions:  1,
			ReplyChance: 1,
			MinDelayMS:  10,
			MaxDelayMS:  20,
		},
	}

	resp := planner.Plan(req)
	if len(resp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(resp.Actions))
	}
	if resp.Debug.ChosenStrategy != "heuristics_fallback" {
		t.Fatalf("expected fallback strategy, got %s", resp.Debug.ChosenStrategy)
	}
	if resp.Actions[0].Reason == "llm" {
		t.Fatalf("expected heuristic reason, got %s", resp.Actions[0].Reason)
	}
}

func TestPlannerTreatsBotsOnlineWhenFlagOmitted(t *testing.T) {
	planner := NewPlanner(noopLLM{}, Config{})
	req := models.PlanRequest{
		RequestID: "req-2",
		Server: models.ServerContext{
			ServerID:      "srv-1",
			Mode:          "LOBBY",
			OnlinePlayers: 10,
		},
		Tick:   123,
		TimeMS: 1712345000000,
		Bots: []models.BotProfile{
			{
				BotID:  "bot-1",
				Name:   "Kuba",
				Online: false,
				Persona: models.Persona{
					Language:       "pl",
					Tone:           "casual",
					StyleTags:      []string{"short"},
					AvoidTopics:    []string{"payments"},
					KnowledgeLevel: "average_player",
				},
			},
		},
		Chat: []models.ChatMessage{
			{
				TimestampMS: 1712344999000,
				Sender:      "RealPlayer123",
				SenderType:  "PLAYER",
				Message:     "hej kto pvp?",
			},
		},
		Settings: models.PlanSettings{
			MaxActions:  1,
			ReplyChance: 1,
			MinDelayMS:  10,
			MaxDelayMS:  20,
		},
	}

	resp := planner.Plan(req)
	if len(resp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(resp.Actions))
	}
	if resp.Debug.ChosenStrategy != "heuristics" {
		t.Fatalf("expected heuristics strategy, got %s", resp.Debug.ChosenStrategy)
	}
}
