package llm

import (
	"strings"
	"testing"

	"aichatplayers/internal/config"
	"aichatplayers/internal/models"
)

func TestBuildPromptIncludesLanguageInstruction(t *testing.T) {
	req := Request{
		Bot: models.BotProfile{
			Name: "Kuba",
			Persona: models.Persona{
				Language: "pl",
			},
		},
	}

	prompt := buildPrompt(req, config.LLMConfig{})
	if !strings.Contains(prompt, "Odpowiadaj wyłącznie w języku pl.") {
		t.Fatalf("expected language instruction in prompt, got: %q", prompt)
	}
}

func TestSanitizeResponseStripsBotPrefix(t *testing.T) {
	prompt := "You are a Minecraft chat bot."
	output := ":Kuba: hej"
	got := sanitizeResponse(prompt, output, "Kuba")
	if got != "hej" {
		t.Fatalf("expected bot prefix stripped, got: %q", got)
	}
}
