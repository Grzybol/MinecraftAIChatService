# AIChatPlayers

AIChatPlayers is a lightweight Go HTTP JSON service that plans bot chat replies for Minecraft servers using deterministic heuristics. It analyzes recent chat, bot personas, and server context to schedule short messages without relying on an LLM.

## Requirements

- Go 1.20+

## Run the server

```bash
go run ./cmd/server -listen :8090
```

### Windows

```powershell
go run .\cmd\server -listen :8090
```

### Linux

```bash
go run ./cmd/server -listen :8090
```

## Run the sample client

```bash
go run ./cmd/client -url http://127.0.0.1:8090
```

## Example curl

```bash
curl -X POST http://127.0.0.1:8090/v1/plan \
  -H 'Content-Type: application/json' \
  -d '{
    "request_id": "example-001",
    "server": {
      "server_id": "betterbox-1",
      "mode": "LOBBY",
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
      "max_actions": 3,
      "min_delay_ms": 800,
      "max_delay_ms": 4500,
      "global_silence_chance": 0.25,
      "reply_chance": 0.65
    }
  }'
```

## Documentation

Technical and API documentation lives in the [`DOCS`](DOCS) directory.
