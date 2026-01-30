# Request processing flowchart

```mermaid
flowchart TD
    A[HTTP /v1/plan request] --> B[Parse PlanRequest JSON]
    B --> C[Planner filters bots
(online + cooldown)]
    C --> D[Detect chat topics]
    D --> E[Select strategy
(small talk / reply)]
    E --> F{LLM enabled?}
    F -- yes --> G[Trim recent chat
LLM_CHAT_HISTORY_LIMIT]
    G --> H[Build prompt
LLM_PROMPT_SYSTEM
+ persona
+ server context
+ topic
+ recent chat
+ LLM_PROMPT_RESPONSE_RULES]
    H --> I[Call local LLM]
    I --> J{Valid response?}
    J -- yes --> K[Return planned action]
    J -- no --> L[Fallback heuristics]
    F -- no --> L
    L --> K
    K --> M[PlanResponse JSON]
```

## Notes

- Only selected data from the incoming request is forwarded to the model: bot persona, server mode/online player count, selected topic, and the last N chat messages (N = `LLM_CHAT_HISTORY_LIMIT`).
- Request metadata (request_id, tick, time_ms), settings, and the full bot list are **not** sent to the LLM.
- Prompt behavior can be adjusted with `LLM_PROMPT_SYSTEM` and `LLM_PROMPT_RESPONSE_RULES`.
