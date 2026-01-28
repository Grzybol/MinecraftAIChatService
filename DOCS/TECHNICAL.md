# Heuristics & Technical Notes

## Request Flow

1. HTTP middleware assigns a request ID, logs the request, and limits body size to 1MB.
2. `/v1/plan` validates JSON and forwards data into the planner.
3. The planner computes topics from the most recent chat lines and builds a deterministic plan using seeded randomness.
4. If configured, the planner asks the local LLM for a short reply and falls back to heuristics on errors or timeouts.

## Topic Detection

Topic detection uses keyword matching on the last 10 chat messages:

- Greeting: `siema`, `hej`, `czesc`, `elo`, `yo`, `witam`
- PvP invite: `kto pvp`, `pvp`, `klepac`, `1v1`, `duel`, `pojedynek`
- Event: `event`, `start`, `drop`, `turniej`, `boss`
- Help: `jak`, `gdzie`, `co robic`, `pomoc`, `help`
- Toxic: common Polish profanity (suppresses replies)

## Anti-spam Rules

- Bots with `cooldown_ms > 0` are excluded from selection.
- `max_actions` caps the number of planned messages.
- Per-bot memory suppresses repeating the same topic within 60 seconds.

## Small Talk Logic

If no topics are detected, the planner either:

- Returns no actions (based on `global_silence_chance`).
- Or picks a single bot to send a short, casual prompt.

## Response Generation

- When the local LLM is enabled, the planner constructs a persona-aware prompt (language, tone, style tags, avoid topics, knowledge level) and requests a single short chat message.
- If the LLM is unavailable, returns an error, or times out, the planner falls back to static templates.
- Bots with `tone` of `friendly` or `casual` may append a friendly emoji when using heuristics.
- `avoid_topics` can suppress replies related to the topic (supports strings containing `pvp` or `event`).
- `knowledge_level: newbie` adds a beginner-style prefix to heuristic replies.
