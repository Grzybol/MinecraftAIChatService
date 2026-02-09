package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"aichatplayers/internal/api"
	"aichatplayers/internal/config"
	"aichatplayers/internal/logging"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:8090", "base url of aichatplayers")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		logging.Fatalf("failed to load config: %v", err)
	}
	fmt.Printf("elastic config: url=%s index=%s api_key_set=%t verify_cert=%t\n", cfg.Elastic.URL, cfg.Elastic.Index, cfg.Elastic.APIKey != "", cfg.Elastic.VerifyCert)

	payload := sampleRequest()
	body, err := json.Marshal(payload)
	if err != nil {
		logging.Fatalf("marshal request: %v", err)
	}

	resp, err := http.Post(*url+"/v1/plan", "application/json", bytes.NewReader(body))
	if err != nil {
		logging.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.Fatalf("read response: %v", err)
	}

	fmt.Printf("status: %s\n", resp.Status)
	fmt.Println(string(data))
}

func sampleRequest() api.PlanRequest {
	now := time.Now().UnixMilli()
	return api.PlanRequest{
		RequestID: "sample-req-001",
		Server: api.ServerContext{
			ServerID:      "betterbox-1",
			Mode:          "LOBBY",
			OnlinePlayers: 42,
		},
		Tick:   123456,
		TimeMS: now,
		Bots: []api.BotProfile{
			{
				BotID:      "bot_01",
				Name:       "Kuba",
				Online:     true,
				CooldownMS: 0,
				Persona: api.Persona{
					Language:       "pl",
					Tone:           "casual",
					StyleTags:      []string{"short", "memes_light"},
					AvoidTopics:    []string{"payments", "admin_powers", "cheating"},
					KnowledgeLevel: "average_player",
				},
			},
			{
				BotID:      "bot_02",
				Name:       "Maja",
				Online:     true,
				CooldownMS: 2000,
				Persona: api.Persona{
					Language:       "pl",
					Tone:           "friendly",
					StyleTags:      []string{"helpful", "short"},
					AvoidTopics:    []string{"pvp_duel_requests"},
					KnowledgeLevel: "newbie",
				},
			},
		},
		Chat: []api.ChatMessage{
			{
				TimestampMS: now - 2000,
				Sender:      "RealPlayer123",
				SenderType:  "PLAYER",
				Message:     "siema ktos idzie na pvp?",
			},
			{
				TimestampMS: now - 1000,
				Sender:      "Admin",
				SenderType:  "SYSTEM",
				Message:     "Event start za 5 minut!",
			},
		},
		Settings: api.PlanSettings{
			MaxActions:          3,
			MinDelayMS:          800,
			MaxDelayMS:          4500,
			GlobalSilenceChance: 0.25,
			ReplyChance:         0.65,
		},
	}
}
