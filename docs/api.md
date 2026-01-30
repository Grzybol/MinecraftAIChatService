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
    "max_actions": 1,
    "min_delay_ms": 400,
    "max_delay_ms": 1200,
    "global_silence_chance": 0.1,
    "reply_chance": 0.7
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
  - `max_actions` controls how many planned actions to return.
  - `min_delay_ms` / `max_delay_ms` set action delay bounds.
  - `global_silence_chance` and `reply_chance` control response probability.

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
