package planner

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"aichatplayers/internal/logging"
	"aichatplayers/internal/models"
	"aichatplayers/internal/util"
)

type BotMemory struct {
	LastSentByTopic map[Topic]int64
}

type Planner struct {
	mu         sync.Mutex
	memory     map[string]map[string]BotMemory
	registry   map[string]map[string]models.BotProfile
	llm        LLMGenerator
	llmTimeout time.Duration
}

const topicCooldownMS int64 = 15000

type Config struct {
	LLMTimeout time.Duration
}

func NewPlanner(generator LLMGenerator, cfg Config) *Planner {
	if generator == nil {
		generator = noopLLM{}
	}
	return &Planner{
		memory:     make(map[string]map[string]BotMemory),
		registry:   make(map[string]map[string]models.BotProfile),
		llm:        generator,
		llmTimeout: cfg.LLMTimeout,
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
	logging.Infof("planner_register server_id=%s bots_total=%d registered=%d", serverID, len(bots), count)
	return count
}

func (p *Planner) Plan(req models.PlanRequest) models.PlanResponse {
	logging.Infof("planner_plan_start request_id=%s transaction_id=%s server_id=%s tick=%d time_ms=%d bots=%d chat_messages=%d", req.RequestID, req.RequestID, req.Server.ServerID, req.Tick, req.TimeMS, len(req.Bots), len(req.Chat))
	rng := util.NewSeededRand(req.RequestID, fmt.Sprint(req.Tick), fmt.Sprint(req.TimeMS))
	availableBots := filterAvailableBots(req.Bots)
	availableBots = filterSelfReplyBots(req, availableBots)
	if len(availableBots) == 0 {
		logging.Infof("planner_plan_no_available_bots request_id=%s transaction_id=%s", req.RequestID, req.RequestID)
		return models.PlanResponse{RequestID: req.RequestID}
	}

	topics := detectTopics(req.Chat)
	settings := normalizeSettings(req.Settings)
	logging.Debugf("planner_plan_context request_id=%s transaction_id=%s topics=%v available_bots=%v settings=%+v", req.RequestID, req.RequestID, topics, botIDs(availableBots), settings)

	actions, strategy, suppressed := p.buildPlan(req, topics, availableBots, settings, rng)
	logging.Infof("planner_plan_result request_id=%s transaction_id=%s strategy=%s actions=%d suppressed=%d", req.RequestID, req.RequestID, strategy, len(actions), suppressed)

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

func filterSelfReplyBots(req models.PlanRequest, bots []models.BotProfile) []models.BotProfile {
	last := latestChatMessage(req.Chat)
	if last == nil {
		return bots
	}
	if !strings.EqualFold(last.SenderType, "BOT") {
		return bots
	}
	filtered := make([]models.BotProfile, 0, len(bots))
	for _, bot := range bots {
		if isSameSender(bot, *last) {
			logging.Debugf("planner_plan_skip_self_reply request_id=%s transaction_id=%s bot_id=%s sender=%s", req.RequestID, req.RequestID, bot.BotID, last.Sender)
			continue
		}
		filtered = append(filtered, bot)
	}
	return filtered
}

func latestChatMessage(messages []models.ChatMessage) *models.ChatMessage {
	if len(messages) == 0 {
		return nil
	}
	latestIndex := 0
	for i := 1; i < len(messages); i++ {
		if messages[i].TimestampMS >= messages[latestIndex].TimestampMS {
			latestIndex = i
		}
	}
	return &messages[latestIndex]
}

func isSameSender(bot models.BotProfile, message models.ChatMessage) bool {
	if bot.BotID != "" && strings.EqualFold(message.Sender, bot.BotID) {
		return true
	}
	if bot.Name != "" && strings.EqualFold(message.Sender, bot.Name) {
		return true
	}
	return false
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
			logging.Infof("planner_plan_silence request_id=%s transaction_id=%s reason=global_silence", req.RequestID, req.RequestID)
			return nil, "silence", 1
		}
		logging.Debugf("planner_plan_small_talk request_id=%s transaction_id=%s", req.RequestID, req.RequestID)
		actions, llmAttempted, llmUsed := p.smallTalkPlan(req, bots, settings, rng)
		return actions, strategyLabel("small_talk", llmAttempted, llmUsed), 0
	}

	if containsTopic(topics, TopicToxic) {
		logging.Infof("planner_plan_toxic_silence request_id=%s transaction_id=%s topic=%s", req.RequestID, req.RequestID, TopicToxic)
		return nil, "toxic_silence", len(bots)
	}

	if rng.Float64() > settings.ReplyChance {
		logging.Infof("planner_plan_reply_suppressed request_id=%s transaction_id=%s reply_chance=%.2f", req.RequestID, req.RequestID, settings.ReplyChance)
		return nil, "reply_suppressed", 1
	}

	actions := make([]models.PlannedAction, 0, settings.MaxActions)
	suppressed := 0
	llmAttempted := false
	llmUsed := false

	selectedBots := pickBots(bots, settings.MaxActions, rng)
	logging.Debugf("planner_plan_selected_bots request_id=%s transaction_id=%s bots=%v topics=%v", req.RequestID, req.RequestID, botIDs(selectedBots), topics)
	for _, topic := range topics {
		for _, bot := range selectedBots {
			if len(actions) >= settings.MaxActions {
				break
			}
			if p.shouldSuppress(req.Server.ServerID, bot.BotID, topic, req.TimeMS) {
				logging.Debugf("planner_plan_suppress request_id=%s transaction_id=%s bot_id=%s topic=%s", req.RequestID, req.RequestID, bot.BotID, topic)
				suppressed++
				continue
			}
			message, reason, attempted, used := p.generateMessage(req, topic, bot, rng)
			if attempted {
				llmAttempted = true
			}
			if used {
				llmUsed = true
			}
			if message == "" {
				logging.Debugf("planner_plan_no_message request_id=%s transaction_id=%s bot_id=%s topic=%s", req.RequestID, req.RequestID, bot.BotID, topic)
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
			logging.Infof("planner_plan_action request_id=%s transaction_id=%s bot_id=%s topic=%s reason=%s", req.RequestID, req.RequestID, bot.BotID, topic, reason)
		}
	}
	return actions, strategyLabel(strategy, llmAttempted, llmUsed), suppressed
}

func (p *Planner) smallTalkPlan(req models.PlanRequest, bots []models.BotProfile, settings models.PlanSettings, rng *rand.Rand) ([]models.PlannedAction, bool, bool) {
	selected := pickBots(bots, 1, rng)
	logging.Debugf("planner_plan_small_talk_bots request_id=%s transaction_id=%s bots=%v", req.RequestID, req.RequestID, botIDs(selected))
	actions := make([]models.PlannedAction, 0, 1)
	llmAttempted := false
	llmUsed := false
	for _, bot := range selected {
		message, reason, attempted, used := p.generateMessage(req, "", bot, rng)
		if attempted {
			llmAttempted = true
		}
		if used {
			llmUsed = true
		}
		if message == "" {
			logging.Debugf("planner_plan_small_talk_no_message request_id=%s transaction_id=%s bot_id=%s", req.RequestID, req.RequestID, bot.BotID)
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
		logging.Infof("planner_plan_small_talk_action request_id=%s transaction_id=%s bot_id=%s reason=%s", req.RequestID, req.RequestID, bot.BotID, reason)
	}
	return actions, llmAttempted, llmUsed
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
	lastSent, ok := last.LastSentByTopic[topic]
	if ok && nowMS-lastSent < topicCooldownMS {
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
	last := p.memory[serverID][botID]
	if last.LastSentByTopic == nil {
		last.LastSentByTopic = make(map[Topic]int64)
	}
	last.LastSentByTopic[topic] = nowMS
	p.memory[serverID][botID] = last
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

func botIDs(bots []models.BotProfile) []string {
	ids := make([]string, 0, len(bots))
	for _, bot := range bots {
		if bot.BotID != "" {
			ids = append(ids, bot.BotID)
		}
	}
	return ids
}

func randomDelay(settings models.PlanSettings, rng *rand.Rand) int64 {
	span := settings.MaxDelayMS - settings.MinDelayMS
	if span <= 0 {
		return settings.MinDelayMS
	}
	return settings.MinDelayMS + rng.Int63n(span+1)
}
