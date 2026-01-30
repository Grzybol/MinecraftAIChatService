# Local model files

Store GGUF model files for the local `llama.cpp` runner in this directory, for example:

```
models/deepseek-1b.gguf
```

The service does **not** download models automatically. You can either set `LLM_MODEL_PATH` to point at the file you place here (or elsewhere on disk), or omit it and let the service auto-detect the first `.gguf` file in `LLM_MODELS_DIR` (defaults to `models/` or `/models`).

You can also place `llama-cli`/`llama-server` binaries in this directory to avoid updating your `PATH`. Set `LLM_MODELS_DIR` to a different folder if you store models and binaries elsewhere.
