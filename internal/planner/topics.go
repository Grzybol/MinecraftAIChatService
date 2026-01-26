package planner

var (
	greetingKeywords = []string{"siema", "hej", "czesc", "elo", "yo", "witam"}
	pvpKeywords      = []string{"kto pvp", "pvp", "klepac", "1v1", "duel", "pojedynek"}
	eventKeywords    = []string{"event", "start", "drop", "turniej", "boss"}
	helpKeywords     = []string{"jak", "gdzie", "co robic", "pomoc", "help"}
	toxicKeywords    = []string{"kurwa", "chuj", "chujowy", "jebac", "idiota"}
)

type Topic string

const (
	TopicGreeting  Topic = "greeting"
	TopicPVPInvite Topic = "pvp_invite"
	TopicEvent     Topic = "event"
	TopicHelp      Topic = "help"
	TopicToxic     Topic = "toxic"
)
