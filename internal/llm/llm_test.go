package llm

import (
	"strings"
	"testing"

	"aichatplayers/internal/config"
	"aichatplayers/internal/models"
)

func TestBuildPromptFormat(t *testing.T) {
	req := Request{
		Server: models.ServerContext{
			ServerID:      "srv-1",
			Mode:          "LOBBY",
			OnlinePlayers: 5,
		},
		Bot: models.BotProfile{
			Name: "Kuba",
			Persona: models.Persona{
				Language: "pl",
				Tone:     "casual",
				StyleTags: []string{
					"short",
					"memes_light",
				},
				AvoidTopics: []string{
					"admin_powers",
				},
				KnowledgeLevel: "average_player",
			},
		},
		RecentChat: []models.ChatMessage{
			{
				Sender:     "Player123",
				SenderType: "PLAYER",
				Message:    "Cześć wszystkim!",
			},
			{
				Sender:     "Kuba",
				SenderType: "BOT",
				Message:    "Hej!",
			},
		},
	}

	prompt := buildPrompt(req, config.LLMConfig{ChatHistoryLimit: 6})
	sections := []string{
		"=== SYSTEM ===",
		"=== RULES ===",
		"=== BOT ===",
		"=== SERVER ===",
		"=== CHAT LOG (last 6) ===",
		"=== TASK ===",
		"=== OUTPUT ===",
	}
	for _, section := range sections {
		if !strings.Contains(prompt, section) {
			t.Fatalf("expected prompt section %q, got: %q", section, prompt)
		}
	}
	if !strings.HasSuffix(prompt, "=== OUTPUT ===\n") {
		t.Fatalf("expected prompt to end with output header, got: %q", prompt)
	}
	if !strings.Contains(prompt, "[PLAYER] Player123: Cześć wszystkim!") {
		t.Fatalf("expected player chat line, got: %q", prompt)
	}
	if !strings.Contains(prompt, "[BOT] Kuba: Hej!") {
		t.Fatalf("expected bot chat line, got: %q", prompt)
	}
	if !strings.Contains(prompt, "__SILENCE__") {
		t.Fatalf("expected silence contract in prompt, got: %q", prompt)
	}
}

func TestNormalizeLLMOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		bot    string
		want   string
	}{
		{
			name:   "first non-empty line",
			output: "\nhej\nkolejna",
			bot:    "Kuba",
			want:   "hej",
		},
		{
			name:   "silence token",
			output: "__SILENCE__\nignored",
			bot:    "Kuba",
			want:   "__SILENCE__",
		},
		{
			name:   "strip quotes and bot prefix",
			output: "\"Kuba: siema\"",
			bot:    "Kuba",
			want:   "siema",
		},
		{
			name:   "strip bot marker",
			output: "(BOT) hejka",
			bot:    "Kuba",
			want:   "hejka",
		},
		{
			name:   "truncate long output",
			output: strings.Repeat("a", 90),
			bot:    "Kuba",
			want:   strings.Repeat("a", 80),
		},
		{
			name:   "empty after sanitize",
			output: "Kuba: \"\"",
			bot:    "Kuba",
			want:   "__SILENCE__",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeLLMOutput(tt.output, tt.bot)
			if got != tt.want {
				t.Fatalf("normalizeLLMOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}
