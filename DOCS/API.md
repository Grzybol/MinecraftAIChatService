# API Reference

## GET /healthz

**Response**

```json
{"status":"ok"}
```

## POST /v1/plan

Plans chat replies for online bots based on recent chat messages.

### Request body

```json
{
  "request_id": "string",
  "server": {
    "server_id": "betterbox-1",
    "mode": "LOBBY|PVP|BOXPVP|SKYBLOCK",
    "online_players": 42
  },
  "tick": 123456,
  "time_ms": 1712345678901,
  "bots": [
    {
      "bot_id": "bot_01",
      "name": "Kuba",
      "online": true,
      "cooldown_ms": 0,
      "persona": {
        "language": "pl",
        "tone": "casual",
        "style_tags": ["short", "memes_light"],
        "avoid_topics": ["payments", "admin_powers", "cheating"],
        "knowledge_level": "average_player"
      }
    }
  ],
  "chat": [
    {
      "ts_ms": 1712345670000,
      "sender": "RealPlayer123",
      "sender_type": "PLAYER",
      "message": "siema ktos idzie na pvp?"
    }
  ],
  "settings": {
    "max-actions": 3,
    "min-delay-ms": 800,
    "max-delay-ms": 4500,
    "global-silence-chance": 0.25,
    "reply-chance": 0.65
  }
}
```

### Response body

```json
{
  "request_id": "string",
  "actions": [
    {
      "bot_id": "bot_02",
      "send_after_ms": 2100,
      "message": "ja dopiero wbijam, co tu się teraz robi? 😅",
      "visibility": "PUBLIC",
      "reason": "newbie_smalltalk"
    }
  ],
  "debug": {
    "chosen_strategy": "heuristics",
    "suppressed_replies": 1
  }
}
```

### Notes

- Bots with `cooldown_ms > 0` are excluded from planning.
- `send_after_ms` is randomized between `min-delay-ms` and `max-delay-ms`.
- If `global-silence-chance` triggers or toxic chat is detected, the service may return an empty `actions` list.
- For backward compatibility, `settings` also accepts legacy snake_case keys (`max_actions`, `min_delay_ms`, `max_delay_ms`, `global_silence_chance`, `reply_chance`).

## POST /v1/bots/register (optional)

Caches bot profiles in memory to reuse in subsequent requests. This endpoint is optional and not required for `/v1/plan` to work.

### Request body

```json
{
  "server_id": "betterbox-1",
  "bots": [
    {
      "bot_id": "bot_01",
      "name": "Kuba",
      "online": true,
      "cooldown_ms": 0,
      "persona": {
        "language": "pl",
        "tone": "casual",
        "style_tags": ["short", "memes_light"],
        "avoid_topics": ["payments", "admin_powers", "cheating"],
        "knowledge_level": "average_player"
      }
    }
  ]
}
```

### Response body

```json
{
  "registered": 1
}
```
