package api

type ServerContext struct {
	ServerID      string `json:"server_id"`
	Mode          string `json:"mode"`
	OnlinePlayers int    `json:"online_players"`
}

type Persona struct {
	Language       string   `json:"language"`
	Tone           string   `json:"tone"`
	StyleTags      []string `json:"style_tags"`
	AvoidTopics    []string `json:"avoid_topics"`
	KnowledgeLevel string   `json:"knowledge_level"`
}

type BotProfile struct {
	BotID      string  `json:"bot_id"`
	Name       string  `json:"name"`
	Online     bool    `json:"online"`
	CooldownMS int64   `json:"cooldown_ms"`
	Persona    Persona `json:"persona"`
}

type ChatMessage struct {
	TimestampMS int64  `json:"ts_ms"`
	Sender      string `json:"sender"`
	SenderType  string `json:"sender_type"`
	Message     string `json:"message"`
}

type PlanSettings struct {
	MaxActions          int     `json:"max_actions"`
	MinDelayMS          int64   `json:"min_delay_ms"`
	MaxDelayMS          int64   `json:"max_delay_ms"`
	GlobalSilenceChance float64 `json:"global_silence_chance"`
	ReplyChance         float64 `json:"reply_chance"`
}

type PlanRequest struct {
	RequestID string        `json:"request_id"`
	Server    ServerContext `json:"server"`
	Tick      int64         `json:"tick"`
	TimeMS    int64         `json:"time_ms"`
	Bots      []BotProfile  `json:"bots"`
	Chat      []ChatMessage `json:"chat"`
	Settings  PlanSettings  `json:"settings"`
}

type PlannedAction struct {
	BotID       string `json:"bot_id"`
	SendAfterMS int64  `json:"send_after_ms"`
	Message     string `json:"message"`
	Visibility  string `json:"visibility"`
	Reason      string `json:"reason"`
}

type PlanDebug struct {
	ChosenStrategy    string `json:"chosen_strategy"`
	SuppressedReplies int    `json:"suppressed_replies"`
}

type PlanResponse struct {
	RequestID string          `json:"request_id"`
	Actions   []PlannedAction `json:"actions"`
	Debug     PlanDebug       `json:"debug"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

type BotRegisterRequest struct {
	ServerID string       `json:"server_id"`
	Bots     []BotProfile `json:"bots"`
}

type BotRegisterResponse struct {
	Registered int `json:"registered"`
}
