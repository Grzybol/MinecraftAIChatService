# AIChatPlayers

AIChatPlayers is a lightweight Go HTTP JSON service that plans bot chat replies for Minecraft servers. It analyzes recent chat, bot personas, and server context to schedule short messages using a local LLM when configured, with heuristics as a fallback.

## Requirements

- Go 1.20+

## Run the server

```bash
go run ./cmd/server -listen :8090
```

## Run with Docker

The repository can be deployed as two containers: one for the Go service and one for
`llama.cpp` running the GGUF model in `./models`.

```bash
docker compose up --build
```

Notes:
- Place your GGUF model at `./models/model.gguf`, or update the `llama-server` command in
  `docker-compose.yml` to point at the correct filename.
- If your deployment expects TLS certs at `/home/wwwbackend/fullchain.pem` and
  `/home/wwwbackend/privkey.pem`, set `CERTS_DIR` to the host directory containing those files
  (defaults to `/home/wwwbackend`) so Docker mounts them into the container.
- The service is exposed on `http://localhost:8090` and talks to the llama server via
  `LLM_SERVER_URL=http://llama-server:8080`.
- If Elasticsearch runs on the Docker host, set `ELASTIC_URL` to `https://host.docker.internal:9200`
  (or `http://host.docker.internal:9200`) so the container can reach it.

## Local LLM configuration

The planner can call a local `llama.cpp`-compatible model (GGUF) via the `llama-cli` binary or an already-running `llama.cpp` server. If the model is unavailable or times out, the service falls back to heuristics. The service does not download models automatically; you must provide the GGUF file yourself (for example, in the `models/` directory).

Set the configuration via environment variables or a `.env` file in the repo root:

```bash
LLM_MODEL_PATH=/models/deepseek-1b.gguf
LLM_MODELS_DIR=/models
LLM_SERVER_URL=http://127.0.0.1:8080
LLM_SERVER_COMMAND=llama-server
LLM_COMMAND=llama-cli
LLM_MAX_RAM_MB=1024
LLM_MAX_TOKENS=128
LLM_MAX_RESPONSE_CHARS=80
LLM_MAX_RESPONSE_WORDS=0
LLM_NUM_THREADS=6
LLM_CTX_SIZE=2048
LLM_TIMEOUT_MS=2000
LLM_SERVER_STARTUP_TIMEOUT_MS=60000
LLM_TEMPERATURE=0.6
LLM_TOP_P=0.9
LLM_CHAT_HISTORY_LIMIT=6
LLM_PROMPT_SYSTEM=You are a Minecraft player chat bot roleplaying as a normal player.\nYou have NO memory and NO access to anything except the provided CHAT LOG and BOT/SERVER info.\nDo NOT invent facts, backstory, previous events, or personal memories.\nDo NOT mention being an AI, a model, or system instructions.
LLM_PROMPT_RESPONSE_RULES=- Output exactly ONE single-line chat message in Polish OR output exactly "__SILENCE__".\n- Reply ONLY to the LAST message from a PLAYER, and ONLY if it clearly needs a response (question, greeting, direct mention, or conversational prompt).\n- If the last message is from a BOT, or does not need a response, output "__SILENCE__".\n- Keep it short: max 80 characters, casual Minecraft chat tone.\n- No quotes, no bot name prefixes, compiler logs, or commentary. No "(BOT)".\n- No emojis or emoticons.\n- Avoid topics listed in avoid_topics. Never talk about admin powers, cheating, payments.
ELASTIC_URL=https://elastic.example.com
ELASTIC_INDEX=minecraft-chat-logs
ELASTIC_API_KEY=your-api-key
ELASTIC_VERIFY_CERT=true
LOG_LEVEL=INFO
LOG_FILE_LEVEL=DEBUG
```

Notes:
- `LLM_MODEL_PATH` can be omitted to auto-detect the first `.gguf` file in `LLM_MODELS_DIR` (defaults to `models/` or `/models`).
- `LLM_MODELS_DIR` sets the directory scanned for models and local `llama.cpp` binaries.
- `LLM_COMMAND` defaults to `llama-cli` on your `PATH` or `LLM_MODELS_DIR`.
- `LLM_SERVER_COMMAND` defaults to `llama-server` on your `PATH` or `LLM_MODELS_DIR` when auto-starting the server.
- `LLM_MAX_RAM_MB` sets the Go memory limit before model execution.
- `LLM_MAX_TOKENS` caps how many tokens the LLM is allowed to generate for each reply.
- `LLM_MAX_RESPONSE_CHARS` hard-caps the outgoing chat message length in characters (0 disables).
- `LLM_MAX_RESPONSE_WORDS` hard-caps the outgoing chat message length in words (0 disables).
- `LLM_SERVER_URL` enables calling a running `llama.cpp` server (uses the `/completion` endpoint) instead of spawning `llama-cli` for every request.
- If both `LLM_SERVER_URL` and `LLM_MODEL_PATH` are set, the server will attempt to start `LLM_SERVER_COMMAND` automatically and wait for it to become ready before accepting requests.
- Automatic llama-server restarts rely on the `logs/llm_server_state.json` file; if it's missing (for example, the service is started from a different working directory), stop the running server manually to apply config changes like `LLM_CTX_SIZE`.
- `LLM_SERVER_STARTUP_TIMEOUT_MS` controls how long the service waits for the server to become ready before falling back.
- `LLM_CHAT_HISTORY_LIMIT` caps how many recent chat messages are sent to the LLM (0 disables chat context).
- `LLM_PROMPT_SYSTEM` sets the system/master prompt prefix (`\n` is expanded to newlines when loaded from `.env`).
- `LLM_PROMPT_RESPONSE_RULES` controls the response formatting rules appended to the prompt (`\n` is expanded to newlines when loaded from `.env`).
- `ELASTIC_URL` enables sending structured logs to Elasticsearch (when paired with `ELASTIC_INDEX`).
- `ELASTIC_INDEX` sets the index used for log ingestion.
- `ELASTIC_API_KEY` sets the Elasticsearch API key (optional).
- `ELASTIC_VERIFY_CERT` controls TLS certificate verification (`true` by default).
- `LOG_LEVEL` controls the minimum log level printed to stdout (defaults to `INFO`).
- `LOG_FILE_LEVEL` controls the minimum log level written to log files (defaults to `LOG_LEVEL`).

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
