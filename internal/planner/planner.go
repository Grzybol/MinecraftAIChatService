package planner

import (
	"fmt"
	"math/rand"
	"sync"

	"aichatplayers/internal/models"
	"aichatplayers/internal/util"
)

type BotMemory struct {
	LastSentMS int64
	LastTopic  Topic
}

type Planner struct {
	mu       sync.Mutex
	memory   map[string]map[string]BotMemory
	registry map[string]map[string]models.BotProfile
}

func NewPlanner() *Planner {
	return &Planner{
		memory:   make(map[string]map[string]BotMemory),
		registry: make(map[string]map[string]models.BotProfile),
	}
}

func (p *Planner) RegisterBots(serverID string, bots []models.BotProfile) int {
	if serverID == "" {
		serverID = "default"
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.registry[serverID] == nil {
		p.registry[serverID] = make(map[string]models.BotProfile)
	}
	count := 0
	for _, bot := range bots {
		if bot.BotID == "" {
			continue
		}
		p.registry[serverID][bot.BotID] = bot
		count++
	}
	return count
}

func (p *Planner) Plan(req models.PlanRequest) models.PlanResponse {
	rng := util.NewSeededRand(req.RequestID, fmt.Sprint(req.Tick), fmt.Sprint(req.TimeMS))
	availableBots := filterAvailableBots(req.Bots)
	if len(availableBots) == 0 {
		return models.PlanResponse{RequestID: req.RequestID}
	}

	topics := detectTopics(req.Chat)
	settings := normalizeSettings(req.Settings)

	actions, strategy, suppressed := p.buildPlan(req, topics, availableBots, settings, rng)

	return models.PlanResponse{
		RequestID: req.RequestID,
		Actions:   actions,
		Debug: models.PlanDebug{
			ChosenStrategy:    strategy,
			SuppressedReplies: suppressed,
		},
	}
}

func filterAvailableBots(bots []models.BotProfile) []models.BotProfile {
	available := make([]models.BotProfile, 0, len(bots))
	for _, bot := range bots {
		if !bot.Online {
			continue
		}
		if bot.CooldownMS > 0 {
			continue
		}
		available = append(available, bot)
	}
	return available
}

func normalizeSettings(settings models.PlanSettings) models.PlanSettings {
	if settings.MaxActions <= 0 {
		settings.MaxActions = 2
	}
	if settings.MinDelayMS <= 0 {
		settings.MinDelayMS = 800
	}
	if settings.MaxDelayMS <= settings.MinDelayMS {
		settings.MaxDelayMS = settings.MinDelayMS + 1200
	}
	if settings.ReplyChance <= 0 {
		settings.ReplyChance = 0.6
	}
	if settings.GlobalSilenceChance < 0 {
		settings.GlobalSilenceChance = 0
	}
	if settings.GlobalSilenceChance > 1 {
		settings.GlobalSilenceChance = 1
	}
	return settings
}

func (p *Planner) buildPlan(req models.PlanRequest, topics []Topic, bots []models.BotProfile, settings models.PlanSettings, rng *rand.Rand) ([]models.PlannedAction, string, int) {
	strategy := "heuristics"
	if len(topics) == 0 {
		if rng.Float64() < settings.GlobalSilenceChance {
			return nil, "silence", 1
		}
		return p.smallTalkPlan(req, bots, settings, rng), "small_talk", 0
	}

	if containsTopic(topics, TopicToxic) {
		return nil, "toxic_silence", len(bots)
	}

	if rng.Float64() > settings.ReplyChance {
		return nil, "reply_suppressed", 1
	}

	actions := make([]models.PlannedAction, 0, settings.MaxActions)
	suppressed := 0

	selectedBots := pickBots(bots, settings.MaxActions, rng)
	for _, topic := range topics {
		for _, bot := range selectedBots {
			if len(actions) >= settings.MaxActions {
				break
			}
			if p.shouldSuppress(req.Server.ServerID, bot.BotID, topic, req.TimeMS) {
				suppressed++
				continue
			}
			message, reason := generateResponse(topic, bot, rng)
			if message == "" {
				continue
			}
			actions = append(actions, models.PlannedAction{
				BotID:       bot.BotID,
				SendAfterMS: randomDelay(settings, rng),
				Message:     message,
				Visibility:  "PUBLIC",
				Reason:      reason,
			})
			p.remember(req.Server.ServerID, bot.BotID, topic, req.TimeMS)
		}
	}
	return actions, strategy, suppressed
}

func (p *Planner) smallTalkPlan(req models.PlanRequest, bots []models.BotProfile, settings models.PlanSettings, rng *rand.Rand) []models.PlannedAction {
	selected := pickBots(bots, 1, rng)
	actions := make([]models.PlannedAction, 0, 1)
	for _, bot := range selected {
		message, reason := generateResponse("", bot, rng)
		if message == "" {
			continue
		}
		actions = append(actions, models.PlannedAction{
			BotID:       bot.BotID,
			SendAfterMS: randomDelay(settings, rng),
			Message:     message,
			Visibility:  "PUBLIC",
			Reason:      reason,
		})
		p.remember(req.Server.ServerID, bot.BotID, "small_talk", req.TimeMS)
	}
	return actions
}

func (p *Planner) shouldSuppress(serverID, botID string, topic Topic, nowMS int64) bool {
	if botID == "" {
		return true
	}
	if serverID == "" {
		serverID = "default"
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	memory := p.memory[serverID]
	if memory == nil {
		return false
	}
	last, ok := memory[botID]
	if !ok {
		return false
	}
	if last.LastTopic == topic && nowMS-last.LastSentMS < 60000 {
		return true
	}
	return false
}

func (p *Planner) remember(serverID, botID string, topic Topic, nowMS int64) {
	if serverID == "" {
		serverID = "default"
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.memory[serverID] == nil {
		p.memory[serverID] = make(map[string]BotMemory)
	}
	p.memory[serverID][botID] = BotMemory{LastSentMS: nowMS, LastTopic: topic}
}

func containsTopic(topics []Topic, target Topic) bool {
	for _, topic := range topics {
		if topic == target {
			return true
		}
	}
	return false
}

func pickBots(bots []models.BotProfile, max int, rng *rand.Rand) []models.BotProfile {
	if len(bots) <= max {
		return bots
	}
	selected := make([]models.BotProfile, 0, max)
	indices := rng.Perm(len(bots))
	for i := 0; i < len(indices) && len(selected) < max; i++ {
		selected = append(selected, bots[indices[i]])
	}
	return selected
}

func randomDelay(settings models.PlanSettings, rng *rand.Rand) int64 {
	span := settings.MaxDelayMS - settings.MinDelayMS
	if span <= 0 {
		return settings.MinDelayMS
	}
	return settings.MinDelayMS + rng.Int63n(span+1)
}
