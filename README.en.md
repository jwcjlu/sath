# sath AI Agent Framework

A lightweight, extensible Go framework for building AI agents. It provides:

- Unified model interface with OpenAI (and optional DashScope/Ollama) adapters
- Default chat Agent with short-term buffer memory
- Middleware chain (logging, recovery, metrics, tracing)
- Config loading (env + YAML/JSON with env overrides)
- Tool registry and Function Calling examples
- CLI: `sath init`, `sath demo`, `sath serve`
- Plugin and event hooks for extensibility

---

## Quick Start

### Prerequisites

- Go 1.20+
- OpenAI API key (or DashScope/Ollama for other backends)

### Build and run

```bash
git clone <your-repo-url> sath
cd sath
go build ./...
go build -o sath ./cmd/sath
```

**Run chat demo (REPL)**

```bash
export OPENAI_API_KEY="your-key"
./sath demo
```

**Run tool-calling demo**

```bash
cd cmd/tool_demo && go run .
```

---

## CLI

| Command       | Description                                              |
|---------------|----------------------------------------------------------|
| `sath init`   | Create a new project skeleton (main.go + config.yaml)    |
| `sath demo`   | Run the built-in chat REPL                              |
| `sath serve`  | Start HTTP server: POST /chat, GET /health, GET /metrics |

Example: `sath serve -a :8080 -c config.yaml` (with optional `--debug`).

---

## Configuration

- **Env**: `OPENAI_MODEL`, `AGENT_MAX_HISTORY`, `OPENAI_API_KEY`
- **File**: `config.Load(path)` or `config.LoadWithEnv(path)` for YAML/JSON; env overrides apply after load.
- **Multi-env**: `config.LoadForEnv("dev", "config")` loads `config/config.dev.yaml` (or .yml/.json).

---

## Roadmap and versioning

- **Roadmap**: Multi-model and multi-modal support, RAG, observability, plugins (see [plan.md](plan.md)).
- **Versioning**: Semantic Versioning; 0.x may still see minor API tweaks.

---

## Community

- [CONTRIBUTING.md](CONTRIBUTING.md) — how to contribute
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) — code of conduct
- [Issue templates](.github/ISSUE_TEMPLATE/) — bugs and feature requests

For full docs (concepts, API, extending, best practices), see the [docs/](docs/) directory.
