package models

import (
	"encoding/json"
	"testing"
)

func TestPlanSettingsUnmarshalSupportsSnakeCase(t *testing.T) {
	payload := []byte(`{"max_actions":3,"min_delay_ms":800,"max_delay_ms":4500,"global_silence_chance":0.25,"reply_chance":0.65}`)

	var settings PlanSettings
	if err := json.Unmarshal(payload, &settings); err != nil {
		t.Fatalf("expected successful unmarshal, got error: %v", err)
	}

	if settings.MaxActions != 3 || settings.MinDelayMS != 800 || settings.MaxDelayMS != 4500 || settings.GlobalSilenceChance != 0.25 || settings.ReplyChance != 0.65 {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestPlanSettingsUnmarshalSupportsKebabCase(t *testing.T) {
	payload := []byte(`{"max-actions":3,"min-delay-ms":800,"max-delay-ms":4500,"global-silence-chance":0.25,"reply-chance":0.65}`)

	var settings PlanSettings
	if err := json.Unmarshal(payload, &settings); err != nil {
		t.Fatalf("expected successful unmarshal, got error: %v", err)
	}

	if settings.MaxActions != 3 || settings.MinDelayMS != 800 || settings.MaxDelayMS != 4500 || settings.GlobalSilenceChance != 0.25 || settings.ReplyChance != 0.65 {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestPlanSettingsUnmarshalRejectsUnknownField(t *testing.T) {
	payload := []byte(`{"max_actions":3,"unexpected":true}`)

	var settings PlanSettings
	if err := json.Unmarshal(payload, &settings); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestPlanSettingsMarshalUsesKebabCase(t *testing.T) {
	payload, err := json.Marshal(PlanSettings{
		MaxActions:          3,
		MinDelayMS:          800,
		MaxDelayMS:          4500,
		GlobalSilenceChance: 0.25,
		ReplyChance:         0.65,
	})
	if err != nil {
		t.Fatalf("expected successful marshal, got error: %v", err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("expected valid json, got error: %v", err)
	}

	if _, ok := decoded["max-actions"]; !ok {
		t.Fatalf("expected kebab-case key max-actions in payload: %s", string(payload))
	}
	if _, ok := decoded["max_actions"]; ok {
		t.Fatalf("did not expect snake-case key max_actions in payload: %s", string(payload))
	}
}
