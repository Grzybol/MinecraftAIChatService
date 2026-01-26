package planner

import (
	"math/rand"
	"sort"
	"strings"

	"aichatplayers/internal/api"
	"aichatplayers/internal/util"
)

const maxRecentMessages = 10

func detectTopics(messages []api.ChatMessage) []Topic {
	if len(messages) == 0 {
		return nil
	}

	recent := messages
	if len(messages) > maxRecentMessages {
		recent = messages[len(messages)-maxRecentMessages:]
	}

	topicCounts := make(map[Topic]int)
	for _, message := range recent {
		text := util.NormalizeText(message.Message)
		switch {
		case util.ContainsAny(text, toxicKeywords):
			topicCounts[TopicToxic]++
		case util.ContainsAny(text, eventKeywords):
			topicCounts[TopicEvent]++
		case util.ContainsAny(text, pvpKeywords):
			topicCounts[TopicPVPInvite]++
		case util.ContainsAny(text, helpKeywords):
			topicCounts[TopicHelp]++
		case util.ContainsAny(text, greetingKeywords):
			topicCounts[TopicGreeting]++
		}
	}

	if len(topicCounts) == 0 {
		return nil
	}

	ordered := make([]Topic, 0, len(topicCounts))
	for topic := range topicCounts {
		ordered = append(ordered, topic)
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		return topicCounts[ordered[i]] > topicCounts[ordered[j]]
	})
	return ordered
}

func generateResponse(topic Topic, bot api.BotProfile, rng *rand.Rand) (string, string) {
	if shouldAvoidTopic(topic, bot.Persona.AvoidTopics) {
		return "", ""
	}
	tone := strings.ToLower(bot.Persona.Tone)
	styleTags := strings.Join(bot.Persona.StyleTags, ",")
	knowledge := strings.ToLower(bot.Persona.KnowledgeLevel)

	switch topic {
	case TopicGreeting:
		return prefixNewbie(knowledge, rng, pickTemplate(greetingTemplates, rng)) + emojiSuffix(tone, rng), "greeting"
	case TopicPVPInvite:
		return pickTemplate(pvpNeutralTemplates, rng) + emojiSuffix(tone, rng), "avoid_real_pvp"
	case TopicEvent:
		return pickTemplate(eventTemplates, rng), "react_to_event"
	case TopicHelp:
		return prefixNewbie(knowledge, rng, pickTemplate(helpTemplates, rng)), "helpful_hint"
	case "":
		message := pickTemplate(smallTalkTemplates, rng)
		if strings.Contains(styleTags, "short") {
			message = shorten(message)
		}
		return prefixNewbie(knowledge, rng, message) + emojiSuffix(tone, rng), "small_talk"
	default:
		return "", ""
	}
}

func shouldAvoidTopic(topic Topic, avoid []string) bool {
	if topic == "" {
		return false
	}
	for _, item := range avoid {
		normalized := strings.ToLower(item)
		if strings.EqualFold(item, string(topic)) {
			return true
		}
		if strings.Contains(normalized, "pvp") && topic == TopicPVPInvite {
			return true
		}
		if strings.Contains(normalized, "event") && topic == TopicEvent {
			return true
		}
	}
	return false
}

func prefixNewbie(level string, rng *rand.Rand, message string) string {
	if level != "newbie" || message == "" {
		return message
	}
	prefix := pickTemplate(newbieAddOns, rng)
	if strings.HasPrefix(message, prefix) {
		return message
	}
	return prefix + ", " + message
}

func pickTemplate(templates []string, rng *rand.Rand) string {
	if len(templates) == 0 {
		return ""
	}
	return templates[rng.Intn(len(templates))]
}

func emojiSuffix(tone string, rng *rand.Rand) string {
	if tone == "friendly" || tone == "casual" {
		return " " + pickTemplate(friendlyEmojis, rng)
	}
	return ""
}

func shorten(message string) string {
	parts := strings.Fields(message)
	if len(parts) <= 3 {
		return message
	}
	return strings.Join(parts[:3], " ")
}
