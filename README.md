# AIChatPlayers

AIChatPlayers is a lightweight Go HTTP JSON service that plans bot chat replies for Minecraft servers. It analyzes recent chat, bot personas, and server context to schedule short messages using a local LLM when configured, with heuristics as a fallback.

## Requirements

- Go 1.20+

## Run the server

```bash
go run ./cmd/server -listen :8090
```

## Local LLM configuration

The planner can call a local `llama.cpp`-compatible model (GGUF) via the `llama-cli` binary or an already-running `llama.cpp` server. If the model is unavailable or times out, the service falls back to heuristics. The service does not download models automatically; you must provide the GGUF file yourself (for example, in the `models/` directory).

Set the configuration via environment variables or a `.env` file in the repo root:

```bash
LLM_MODEL_PATH=/models/deepseek-1b.gguf
LLM_SERVER_URL=http://127.0.0.1:8080
LLM_SERVER_COMMAND=llama-server
LLM_COMMAND=llama-cli
LLM_MAX_RAM_MB=1024
LLM_NUM_THREADS=6
LLM_CTX_SIZE=2048
LLM_TIMEOUT_MS=2000
LLM_SERVER_STARTUP_TIMEOUT_MS=60000
LLM_TEMPERATURE=0.6
LLM_TOP_P=0.9
```

Notes:
- `LLM_COMMAND` defaults to `llama-cli` on your `PATH`.
- `LLM_SERVER_COMMAND` defaults to `llama-server` on your `PATH` when auto-starting the server.
- `LLM_MAX_RAM_MB` sets the Go memory limit before model execution.
- `LLM_SERVER_URL` enables calling a running `llama.cpp` server (uses the `/completion` endpoint) instead of spawning `llama-cli` for every request.
- If both `LLM_SERVER_URL` and `LLM_MODEL_PATH` are set, the server will attempt to start `LLM_SERVER_COMMAND` automatically and wait for it to become ready before accepting requests.
- `LLM_SERVER_STARTUP_TIMEOUT_MS` controls how long the service waits for the server to become ready before falling back.

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
