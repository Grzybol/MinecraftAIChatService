# API documentation

## Overview

The service exposes a small HTTP JSON API for planning chat actions for Minecraft bots. All endpoints accept and return JSON, and the server rejects unknown fields in request payloads.

Base URL example: `http://localhost:8090`.

## POST /v1/plan

Generate planned chat actions based on recent chat history, server state, and bot personas.

### Expected request

```json
{
  "request_id": "req-12345",
  "server": {
    "server_id": "survival-1",
    "mode": "survival",
    "online_players": 12
  },
  "tick": 99123,
  "time_ms": 1738230000123,
  "bots": [
    {
      "bot_id": "bot-01",
      "name": "test4",
      "online": true,
      "cooldown_ms": 0,
      "persona": {
        "language": "pl",
        "tone": "casual",
        "style_tags": ["friendly", "short"],
        "avoid_topics": ["admin", "cheats"],
        "knowledge_level": "player"
      }
    }
  ],
  "chat": [
    {
      "ts_ms": 1738230000456,
      "sender": "grzybol",
      "sender_type": "PLAYER",
      "message": "halo, jestes tam?"
    }
  ],
  "settings": {
    "max-actions": 1,
    "min-delay-ms": 400,
    "max-delay-ms": 1200,
    "global-silence-chance": 0.1,
    "reply-chance": 0.7
  }
}
```

### Fields

- `request_id` (string): Client-supplied identifier for correlation. If omitted, the service uses the request ID from middleware (when available).
- `server` (object): Server context.
  - `server_id` (string)
  - `mode` (string)
  - `online_players` (int)
- `tick` (int64): Current server tick.
- `time_ms` (int64): Current server time in milliseconds.
- `bots` (array): Bot profiles with persona data.
- `chat` (array): Chat log entries; the planner reads the latest entries in chronological order.
  - `sender_type` should be a high-level role label such as `PLAYER` or `BOT`.
  - `message` is the raw chat content and is the field used when constructing prompts.
- `settings` (object): Planning constraints.
  - `max-actions` controls how many planned actions to return.
  - `min-delay-ms` / `max-delay-ms` set action delay bounds.
  - `global-silence-chance` and `reply-chance` control response probability.
  - Legacy snake_case variants are also accepted for compatibility (`max_actions`, `min_delay_ms`, `max_delay_ms`, `global_silence_chance`, `reply_chance`).

### Expected response

```json
{
  "request_id": "req-12345",
  "actions": [
    {
      "bot_id": "bot-01",
      "send_after_ms": 800,
      "message": "siema, jestem!",
      "visibility": "PUBLIC",
      "reason": "reply"
    }
  ],
  "debug": {
    "chosen_strategy": "reply",
    "suppressed_replies": 0
  }
}
```

### Response notes

- `actions` may be empty if the planner decides to stay silent.
- The planner may return `"__SILENCE__"` as a message when it explicitly decides not to reply. Clients should treat it as a no-op and suppress output in game chat.
- `visibility` is currently `PUBLIC` for planned actions.

## POST /v1/engagement

Generate planned chat actions to initiate conversations after chat has been quiet. This endpoint accepts the same payload as `/v1/plan`, with two extra fields for engagement context.

### Expected request

```json
{
  "request_id": "req-12345",
  "server": {
    "server_id": "survival-1",
    "mode": "survival",
    "online_players": 12
  },
  "tick": 99123,
  "time_ms": 1738230000123,
  "bots": [
    {
      "bot_id": "bot-01",
      "name": "test4",
      "online": true,
      "cooldown_ms": 0,
      "persona": {
        "language": "pl",
        "tone": "casual",
        "style_tags": ["friendly", "short"],
        "avoid_topics": ["admin", "cheats"],
        "knowledge_level": "player"
      }
    }
  ],
  "chat": [
    {
      "ts_ms": 1738230000456,
      "sender": "grzybol",
      "sender_type": "PLAYER",
      "message": "halo, jestes tam?"
    }
  ],
  "settings": {
    "max-actions": 1,
    "min-delay-ms": 400,
    "max-delay-ms": 1200,
    "global-silence-chance": 0.1,
    "reply-chance": 0.7
  },
  "target_player": "PlayerX",
  "example_prompt": "Napisz krótką wiadomość angażującą gracza/bota o nicku PlayerX."
}
```

### Fields

All fields from `/v1/plan` plus:

- `target_player` (string): Chosen player (non-bot) for the engagement attempt.
- `example_prompt` (string): Short prompt hint for kick-starting the conversation.

### Expected response

Response payload is identical to `/v1/plan` (PlannerResponse).

## POST /v1/bots/register

Register known bots and their personas for a given server.

### Expected request

```json
{
  "server_id": "survival-1",
  "bots": [
    {
      "bot_id": "bot-01",
      "name": "test4",
      "online": true,
      "cooldown_ms": 0,
      "persona": {
        "language": "pl",
        "tone": "casual",
        "style_tags": ["friendly", "short"],
        "avoid_topics": ["admin", "cheats"],
        "knowledge_level": "player"
      }
    }
  ]
}
```

### Expected response

```json
{
  "registered": 1
}
```

## GET /healthz

Simple health check.

### Expected response

```json
{
  "status": "ok"
}
```
