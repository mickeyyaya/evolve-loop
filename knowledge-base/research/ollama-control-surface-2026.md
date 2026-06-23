# Ollama Control Surface — Deep Research Dossier (2026-05-28)

> **Purpose.** This is the load-bearing research dossier for Workstream F: an `ollama-tmux` bridge driver for evolve-loop, used as a per-phase reviewer LLM and reasoning/review phase. It also informs the per-phase user-facing toggles for "which LLM CLI + which model" the evolve-loop bridge will expose.
>
> **Scope.** Exhaustive coverage of the Ollama control surface as of 2026-Q1/Q2 — REPL, CLI flags, HTTP API, env vars, Modelfile DSL, Ollama Cloud routing, tool-calling, structured output, and integration patterns.
>
> **Method.** Context7 against `/ollama/ollama` + `/websites/ollama_api`, plus WebFetch on docs.ollama.com and the official GitHub repo, plus WebSearch corroboration. All sources cited inline.
>
> **Reading order.** Section 0 ("Recommendation") is the actionable summary. Section 11 is the control-parameter map (what evolve-loop exposes vs. hides). Sections 1-10 are reference.

---

## Table of Contents

- [0. Summary & Recommendation for Workstream F](#0-summary--recommendation-for-workstream-f)
- [1. ollama CLI Control Surface](#1-ollama-cli-control-surface)
- [2. Local vs Cloud Routing](#2-local-vs-cloud-routing)
- [3. HTTP API Surface](#3-http-api-surface)
- [4. Model Parameters — Exhaustive](#4-model-parameters--exhaustive)
- [5. Tool Use / Function Calling](#5-tool-use--function-calling)
- [6. Structured Output](#6-structured-output)
- [7. Modelfile Composition](#7-modelfile-composition)
- [8. Multi-Model / Per-Request Model Selection](#8-multi-model--per-request-model-selection)
- [9. Output Streaming & Completion Detection](#9-output-streaming--completion-detection)
- [10. Integration Patterns](#10-integration-patterns)
- [11. Control-Parameter Map (evolve-loop profile JSON)](#11-control-parameter-map-evolve-loop-profile-json)
- [12. Open Questions & Follow-Ups](#12-open-questions--follow-ups)
- [13. Sources](#13-sources)

---

## 0. Summary & Recommendation for Workstream F

### TL;DR

**Build `ollama-tmux` as a thin peer of `claude-tmux`/`codex-tmux`/`agy-tmux` on the shared `driver_tmux_repl.go` engine — but back the per-phase invocation with the HTTP API (`/api/chat`) under the hood once we need anything beyond a smoke-test reviewer.**

The tmux REPL gives us a uniform driver shape across CLIs (operator visibility, named-session resume, live-injection, the same artifact-wait completion detector). But Ollama's REPL is deliberately simpler than Claude Code's — no in-REPL tool calling, no streaming JSON envelopes, no per-message structured-output flag. For a **reasoning/review phase** that just consumes prompt → emits answer, the REPL is sufficient. For anything that needs JSON schema validation, tool calls, or deterministic completion markers, the HTTP API is the right transport.

### Two-tier design

| Tier | Driver | Use Case | Why |
|---|---|---|---|
| **Tier 1 (default)** | `ollama-tmux` (REPL on `driver_tmux_repl.go`) | Per-phase reviewer LLM, reasoning, advisory phases | Uniform operator UX with claude/codex/agy; visible scrollback; live-injection works; cheap |
| **Tier 2 (opt-in)** | `ollama-api` (HTTP `/api/chat` direct) | Structured-output phases, tool-using phases, headless CI | Deterministic done=true marker; JSON-schema enforced; no terminal/tmux dep |

Ship Tier 1 now (matches WS-F brief). Tier 2 is a follow-up.

### Completion-detection contract (REPL)

The Ollama REPL **returns the `>>> ` prompt marker** when generation completes ([source: docs.ollama.com/cli](https://docs.ollama.com/cli) — the prompt shown is `>>> Send a message (/? for help)`). This is the exact same artifact-wait pattern `driver_tmux_repl.go` already uses for the other CLIs — match on `^>>> ` re-appearance at the bottom of the pane after the prompt is injected. The completion-detector Strategy landed in PR #18 (ADR-0027 PR1) is directly reusable.

### Multi-line input contract

Ollama supports triple-quote `"""` delimiters for multi-line prompts ([source: docs.ollama.com/cli](https://docs.ollama.com/cli) — quoted: *"Text wrapped with `\"\"\"` triple quotes enables multi-line entries"*). The bridge already pastes large prompts via tmux `send-keys` + `Enter`; we just need to wrap the prompt body in `"""…"""` when it contains a newline. This is a one-line tweak to the existing tmux paste helper.

### Cloud vs local routing — the user's main ask

**Routing is by model tag**, not by env-var flag:

- Bare local model: `ollama run llama3.1:8b` → loads from `$OLLAMA_MODELS`, runs on local hardware.
- Cloud-suffixed model: `ollama run gpt-oss:120b-cloud` → ollama transparently routes to ollama.com cloud, gated by `OLLAMA_API_KEY` or a one-time `ollama signin` ([source: docs.ollama.com/api/authentication](https://docs.ollama.com/api/authentication) — quoted: *"ollama run gpt-oss:120b-cloud"*).
- `OLLAMA_HOST` points at a **specific ollama daemon**, NOT at the cloud — its purpose is "which `ollama serve` instance does the CLI talk to" (default `http://127.0.0.1:11434`). Set it to a remote workstation's address for a shared ollama server. It does NOT route to ollama.com — cloud routing happens **inside** the daemon when a `:cloud`-tagged model is requested.
- Direct cloud HTTP: `https://ollama.com/api` (native) or `https://ollama.com/v1` (OpenAI-compatible), `Authorization: Bearer $OLLAMA_API_KEY` ([source: docs.ollama.com/api/authentication](https://docs.ollama.com/api/authentication)).

**Implication for evolve-loop.** A user picks "ollama-tmux" + `model: "gpt-oss:120b-cloud"` and we get cloud routing for free — no separate cloud driver. Local picks (`llama3.1:8b`, `qwen2.5-coder:32b`, etc.) work identically with the same driver, same code path. **This is the right UX answer to the user's main ask.**

> **Cost gotcha.** Cloud `:cloud` tags bill against the user's ollama.com plan (Free / Pro $20/mo / Max $100/mo per [ollama.com/cloud](https://ollama.com/cloud)) — totally separate from Anthropic / OpenAI / Google quota meters. evolve-loop's cost tracker should treat `ollama:cloud:*` as a third billable plane; local `ollama:*` is free-at-the-edge but consumes local CPU/GPU.

> **Structured output gotcha.** Per docs.ollama.com/capabilities/structured-outputs (quoted: *"Ollama's Cloud currently does not support structured outputs"*) — `format: { schema }` is local-only. If a phase declares `requires: structured-output`, the bridge must refuse `ollama:cloud:*` and either downgrade to local or fail closed.

### Per-phase control surface evolve-loop should expose

Minimum viable (Tier 1, REPL):

```json
{
  "driver": "ollama-tmux",
  "model": "llama3.1:8b",
  "system_prompt": "…optional…",
  "options": {
    "temperature": 0.2,
    "top_p": 0.9,
    "num_ctx": 8192,
    "num_predict": 4096,
    "seed": 0
  },
  "keep_alive": "5m"
}
```

These five options cover ≥95% of operator intent. Full table in [§11](#11-control-parameter-map-evolve-loop-profile-json). REPL injects them via `/set parameter <key> <value>` before sending the prompt body — Tier 2 (HTTP) passes them in the `options` object directly.

---

## 1. ollama CLI Control Surface

### 1.1 Top-level subcommands

| Command | Purpose | Notes |
|---|---|---|
| `ollama run <model> [prompt]` | Interactive REPL or one-shot | Opens `>>>` REPL when no prompt arg; one-shot when arg or stdin pipe |
| `ollama pull <model>` | Download from registry | Required before first local run |
| `ollama push <model>` | Upload to registry | Requires `ollama signin` |
| `ollama list` / `ollama ls` | List installed local models | Shows name, ID, size, modified time |
| `ollama ps` | List currently loaded (in-memory) models | Shows VRAM usage, expiry |
| `ollama show <model>` | Show modelfile, params, template, system | Equivalent to `/show modelfile` in REPL |
| `ollama cp <src> <dst>` | Clone a model locally under a new name | Cheap (CoW) |
| `ollama rm <model>` | Delete a local model | Frees disk |
| `ollama create <name> -f <Modelfile>` | Build a model from a Modelfile on disk | See §7 |
| `ollama serve` | Start the daemon | Default port 11434, env-config via OLLAMA_* |
| `ollama stop <model>` | Unload a running model from memory | Same as `keep_alive: 0` API call |
| `ollama signin` | OAuth flow to ollama.com (browser) | Creates `~/.ollama/id_ed25519` keypair |
| `ollama signout` | Revoke local credentials | Removes saved keypair binding |
| `ollama launch [integration]` | Zero-config launch of AI coding assistants (e.g. `ollama launch claude`) | Newer; useful as inspiration but not what WS-F needs |

Source: [docs.ollama.com/cli](https://docs.ollama.com/cli), [glukhov.org cheatsheet 2026](https://www.glukhov.org/llm-hosting/ollama/ollama-cheatsheet/).

### 1.2 `ollama run` flags

| Flag | Effect |
|---|---|
| `--verbose` / `-v` | Print timing stats (tokens/s, load time, eval time) after each response |
| `--format <string>` | Force output format, e.g. `--format json` |
| `--keepalive <duration>` | Override `OLLAMA_KEEP_ALIVE` default (`5m`) |
| `--nowordwrap` | Disable automatic terminal word-wrap |
| `--hidethinking` | Suppress `thinking` blocks from thinking-enabled models |
| `--think` | Enable thinking output for thinking models |
| `--insecure` | Allow connections to registries over plain HTTP |
| `-p, --parameters <k=v>` | Pass model parameters inline (CLI-side equivalent of `options:` in API) |

Sources: [docs.ollama.com/cli](https://docs.ollama.com/cli), [glukhov.org cheatsheet](https://www.glukhov.org/llm-hosting/ollama/ollama-cheatsheet/).

### 1.3 Interactive REPL — slash-command reference

When you start `ollama run <model>` without a prompt arg, you get the REPL with prompt marker `>>> Send a message (/? for help)`. Type `/?` to see:

| Command | Effect |
|---|---|
| `/?` or `/help` | Show this help |
| `/bye` or `/exit` | Exit Ollama |
| `/set` | Set session variables (see subcommands below) |
| `/show` | Show session info (see subcommands below) |
| `/save <name>` | Save current session (history + system + params) to a named slot |
| `/load <name>` | Load a previously saved session |
| `/clear` | Clear the conversation history (keeps system prompt + params) |

**`/set` subcommands** (consolidated from multiple cheatsheets — exact set depends on Ollama version):

| `/set …` | Effect |
|---|---|
| `/set system <text>` | Set the system prompt for this session |
| `/set parameter <name> <value>` | Set a runtime parameter (see §4) — e.g. `/set parameter num_ctx 8192` |
| `/set template <text>` | Set the prompt template (Go-template syntax) |
| `/set history` / `/set nohistory` | Enable / disable readline history persistence |
| `/set wordwrap` / `/set nowordwrap` | Toggle word-wrap on |
| `/set format json` / `/set noformat` | Toggle JSON-only output mode |
| `/set verbose` / `/set quiet` | Toggle per-response timing stats |
| `/set think` / `/set nothink` | Toggle visible thinking output (for thinking models) |

**`/show` subcommands**:

| `/show …` | Effect |
|---|---|
| `/show info` | Model architecture, parameter count, quantization, capabilities |
| `/show license` | License text |
| `/show modelfile` | The synthesized Modelfile for the active model |
| `/show parameters` | Currently active runtime parameters |
| `/show system` | The active system prompt |
| `/show template` | The active prompt template |

**Multi-line input**: wrap text in triple quotes `"""`. Lines between the opening and closing `"""` are sent as one prompt. Quoted from docs.ollama.com/cli: *"Text wrapped with `\"\"\"` triple quotes enables multi-line entries"*.

**Exit**: `/bye`, `/exit`, or Ctrl+D.

Sources: [docs.ollama.com/cli](https://docs.ollama.com/cli), [computingforgeeks cheatsheet](https://computingforgeeks.com/ollama-commands-cheat-sheet/), [glukhov.org](https://www.glukhov.org/llm-hosting/ollama/ollama-cheatsheet/), [hostinger ollama tutorial](https://www.hostinger.com/tutorials/ollama-cli-tutorial).

### 1.4 Non-interactive use (one-shot)

Three equivalent forms:

```bash
# (1) Positional arg
ollama run llama3.1 "Why is the sky blue?"

# (2) Piped stdin
echo "Why is the sky blue?" | ollama run llama3.1

# (3) File input
cat prompt.txt | ollama run llama3.1
```

All three: model loads, prints the response to stdout, exits. The REPL doesn't open. No prompt marker is printed.

Source: [hostinger ollama tutorial](https://www.hostinger.com/tutorials/ollama-cli-tutorial), [github.com/ollama/ollama issues#7823](https://github.com/ollama/ollama/issues/7823).

> **WS-F note.** The tmux driver should NOT use this one-shot form — we want the interactive REPL for the same operator-visibility reasons claude-tmux/codex-tmux exist. The one-shot form is the right shape for a future `ollama-p` headless driver paralleling `claude-p`.

### 1.5 Streaming behavior in `ollama run`

In REPL mode, `ollama run` streams tokens to stdout as they're generated. There is no `--stream=false` flag for the CLI — streaming is the only mode. Use the HTTP API with `"stream": false` if you need a single-blob response.

`--verbose` adds a stats block after each response:

```
total duration:       4.2s
load duration:        1.1s
prompt eval count:    26 token(s)
prompt eval duration: 130ms
prompt eval rate:     200 tokens/s
eval count:           259 token(s)
eval duration:        4.2s
eval rate:            61.5 tokens/s
```

### 1.6 OLLAMA_* environment variables (full list)

From the source code at [github.com/ollama/ollama/blob/main/envconfig/config.go](https://github.com/ollama/ollama/blob/main/envconfig/config.go):

| Variable | Type | Default | Purpose |
|---|---|---|---|
| `OLLAMA_HOST` | URL | `http://127.0.0.1:11434` | Daemon bind / CLI connect address |
| `OLLAMA_MODELS` | path | `$HOME/.ollama/models` | Local model storage dir |
| `OLLAMA_KEEP_ALIVE` | duration | `5m` | How long a loaded model stays in VRAM after last request |
| `OLLAMA_NUM_PARALLEL` | uint | `1` | Max concurrent requests per model |
| `OLLAMA_MAX_LOADED_MODELS` | uint | `0` (auto-per-GPU) | Max distinct models loaded simultaneously |
| `OLLAMA_DEBUG` | level | `INFO` | Log verbosity: 0=INFO, 1=DEBUG, 2=TRACE |
| `OLLAMA_FLASH_ATTENTION` | bool | `false` | Enable experimental flash attention |
| `OLLAMA_KV_CACHE_TYPE` | string | `f16` | KV cache quantization (`f16`, `q8_0`, `q4_0`) |
| `OLLAMA_ORIGINS` | CSV | `localhost,127.0.0.1` | Allowed CORS origins for the HTTP server |
| `OLLAMA_NOPRUNE` | bool | `false` | Skip model blob pruning on startup |
| `OLLAMA_NOHISTORY` | bool | `false` | Disable readline history persistence |
| `OLLAMA_GPU_OVERHEAD` | uint64 | `0` | Bytes of VRAM to reserve per GPU |
| `OLLAMA_LOAD_TIMEOUT` | duration | `5m` | Model-load stall detector |
| `OLLAMA_MAX_QUEUE` | uint | `512` | HTTP request queue ceiling |
| `OLLAMA_SCHED_SPREAD` | bool | `false` | Spread models across all GPUs vs pack one GPU first |
| `OLLAMA_NEW_ENGINE` | bool | `false` | Opt into the new engine (perf path) |
| `OLLAMA_CONTEXT_LENGTH` | uint | `0` | Default context window if model doesn't declare one |
| `OLLAMA_LLM_LIBRARY` | string | auto | Force a specific inference backend (CUDA / Metal / CPU / Vulkan) |
| `OLLAMA_DEBUG_LOG_REQUESTS` | bool | `false` | Dump every inference request to log |
| `OLLAMA_MAX_TRANSFER_STREAMS` | uint | `4` | Parallel safetensors transfer threads |
| `OLLAMA_REMOTES` | CSV | `ollama.com` | Allowed remote model hosts (cloud routing target) |
| `OLLAMA_NO_CLOUD` | bool | `false` | Disable cloud features entirely |
| `OLLAMA_MULTIUSER_CACHE` | bool | `false` | Optimize caching for multi-user setups |
| `OLLAMA_AUTH` | bool | `false` | Enable client-server authentication |
| `OLLAMA_VULKAN` | bool | `false` | Enable experimental Vulkan backend |
| `OLLAMA_EDITOR` | path | `$EDITOR` | Editor invoked by `/set system` long-form |
| `OLLAMA_API_KEY` | string | unset | Bearer token for cloud / authenticated daemon |

Source: WebFetch of [envconfig/config.go](https://github.com/ollama/ollama/blob/main/envconfig/config.go), plus [docs.ollama.com/api/authentication](https://docs.ollama.com/api/authentication) for `OLLAMA_API_KEY`.

> **WS-F note.** evolve-loop should pass `OLLAMA_KEEP_ALIVE=10m` or so when launching the tmux session so the model stays hot between phases. `OLLAMA_NUM_PARALLEL=1` is fine for sequential phases — bump if we run review phases in parallel. `OLLAMA_HOST` should be forwarded from the operator's env (so they can target a beefy workstation across the LAN without changing evolve-loop config).

---

## 2. Local vs Cloud Routing

This is the user's primary research question. Definitive answer:

### 2.1 Mechanism — routing by model tag

**Cloud routing is selected by the model tag**, specifically the `:cloud` suffix:

```bash
# Local — uses local GPU/CPU, no auth needed, free
ollama run llama3.1:8b

# Cloud — routes through ollama.com, requires OLLAMA_API_KEY or `ollama signin`, billable
ollama run gpt-oss:120b-cloud
ollama run deepseek-v4-pro:cloud
```

Source: docs.ollama.com/api/authentication, quoted: *"ollama run gpt-oss:120b-cloud"*; [openclaw/ollama provider docs](https://docs.openclaw.ai/providers/ollama).

The Ollama daemon (`ollama serve`) handles the routing internally — your CLI talks to the local daemon as usual, the daemon decides whether to dispatch locally or proxy to ollama.com based on the tag.

### 2.2 Authentication for cloud routing

Two equivalent paths:

**Path A — OAuth via browser** (interactive):
```bash
ollama signin    # opens browser, OAuth flow, stores token locally
ollama run gpt-oss:120b-cloud "hello"   # auto-authenticated
```

The signin flow registers the local machine's Ed25519 public key (`~/.ollama/id_ed25519.pub`) against the user's ollama.com account. Source: [DeepWiki ollama/ollama 3.6](https://deepwiki.com/ollama/ollama/3.6-authentication-and-api-keys).

**Path B — API key** (automated / CI):
```bash
export OLLAMA_API_KEY="$(cat ~/.config/ollama/api_key)"
ollama run gpt-oss:120b-cloud "hello"   # daemon uses OLLAMA_API_KEY automatically
```

Create the key at https://ollama.com/settings/keys. Quoted from docs.ollama.com/api/authentication: *"API keys don't currently expire, however you can revoke them at any time"*.

### 2.3 Direct cloud HTTP (bypassing the local daemon)

The cloud also exposes the standard HTTP API directly:

```bash
# Native Ollama API on cloud
curl https://ollama.com/api/chat \
  -H "Authorization: Bearer $OLLAMA_API_KEY" \
  -d '{"model":"gpt-oss:120b","messages":[{"role":"user","content":"hi"}]}'

# OpenAI-compatible layer on cloud
curl https://ollama.com/v1/chat/completions \
  -H "Authorization: Bearer $OLLAMA_API_KEY" \
  -d '{"model":"gpt-oss:120b","messages":[{"role":"user","content":"hi"}]}'
```

**Important**: the `/api` prefix is for native Ollama endpoints; `/v1` is the OpenAI-compatibility layer. Both live under `https://ollama.com/...`. Source: docs.ollama.com/api/authentication.

### 2.4 Cloud-only limitations

Per [docs.ollama.com/capabilities/structured-outputs](https://docs.ollama.com/capabilities/structured-outputs): *"Ollama's Cloud currently does not support structured outputs."*

So `format: { schema }` requests are LOCAL-ONLY. evolve-loop should gate cloud routing with a capability check (refuse `:cloud` for phases that need structured output) or fall back to free-form JSON + post-hoc validation.

### 2.5 `OLLAMA_HOST` vs cloud

Common confusion: `OLLAMA_HOST` points at an **ollama daemon** (local or remote-workstation), NOT at the cloud. It cannot be set to `https://ollama.com` to magically use cloud — that would attempt to use the cloud's URL as a local-daemon URL and fail. Cloud routing is exclusively via the `:cloud` model-tag mechanism described above.

Use cases for `OLLAMA_HOST`:
- `http://127.0.0.1:11434` (default) — local daemon
- `http://workstation.local:11434` — beefy LAN box hosting the daemon
- `https://ollama.example.com` — reverse-proxied remote daemon with `OLLAMA_AUTH=true`

### 2.6 Cloud pricing model (informational)

Per [ollama.com/cloud](https://ollama.com/cloud):

| Tier | Price | Capacity |
|---|---|---|
| Free | $0/mo | Light usage — eval / chat / small assistants |
| Pro | $20/mo | "50× more cloud usage than Free", 3 concurrent cloud models |
| Max | $100/mo | "5× more capacity than Pro", 10 concurrent models |

evolve-loop should consider this when sizing its per-phase cost ceilings: cloud Ollama is bursty and rate-limited rather than per-token billed.

---

## 3. HTTP API Surface

The HTTP API is served by `ollama serve` (default `http://127.0.0.1:11434`). All endpoints accept JSON and return JSON or NDJSON.

### 3.1 Endpoint catalog

| Method + Path | Purpose |
|---|---|
| `POST /api/generate` | Single-prompt completion (legacy-shaped, simpler than chat) |
| `POST /api/chat` | Multi-turn chat with message history; supports tools |
| `POST /api/embeddings` | Generate embeddings (legacy field name) |
| `POST /api/embed` | Generate embeddings (newer field name) |
| `GET /api/tags` | List installed local models (= `ollama list`) |
| `POST /api/show` | Get model metadata (Modelfile, params, capabilities) |
| `POST /api/pull` | Download a model (streams progress) |
| `POST /api/push` | Upload a model (streams progress) |
| `POST /api/create` | Build a model from a Modelfile or in-line spec |
| `POST /api/copy` | Clone a model locally |
| `DELETE /api/delete` | Remove a local model |
| `GET /api/ps` | List currently loaded models |
| `GET /api/version` | Daemon version |
| `HEAD /api/blobs/:digest` | Check blob existence |
| `POST /api/blobs/:digest` | Upload a blob by digest (used by /api/create) |
| `POST /v1/chat/completions` | OpenAI-compatible chat |
| `POST /v1/embeddings` | OpenAI-compatible embeddings |
| `GET /v1/models` | OpenAI-compatible model list |
| `POST /v1/messages` | Anthropic-compatible messages (tool calling, thinking) |

Sources: [docs.ollama.com/api](https://docs.ollama.com/api), [github.com/ollama/ollama/blob/main/docs/api.md](https://github.com/ollama/ollama/blob/main/docs/api.md).

### 3.2 `POST /api/generate` — full field reference

| Field | Type | Required | Notes |
|---|---|---|---|
| `model` | string | yes | e.g. `"llama3.1:8b"` |
| `prompt` | string | no | The user prompt |
| `suffix` | string | no | Text appended after the model response (fill-in-the-middle) |
| `system` | string | no | Overrides Modelfile SYSTEM |
| `template` | string | no | Overrides Modelfile TEMPLATE |
| `context` | int[] | no | **Deprecated**; previous-response context tokens |
| `stream` | bool | no | Default `true`; set `false` for single-blob response |
| `raw` | bool | no | `true` = no prompt templating applied |
| `format` | string \| object | no | `"json"` or JSON schema object |
| `keep_alive` | string \| int | no | `"5m"`, `"0"` (unload now), `-1` (indefinite) |
| `images` | string[] | no | Base64-encoded images for multimodal models |
| `options` | object | no | Runtime params (temperature, top_p, num_ctx, …) — see §4 |
| `think` | bool \| string | no | For thinking models — `true`, `"high"`, `"medium"`, `"low"` |
| `logprobs` | bool | no | Return logprobs of output tokens |
| `top_logprobs` | int | no | How many alternatives per token |

Source: [github.com/ollama/ollama/blob/main/docs/api.md](https://github.com/ollama/ollama/blob/main/docs/api.md).

### 3.3 `POST /api/chat` — full field reference

| Field | Type | Required | Notes |
|---|---|---|---|
| `model` | string | yes | Model name |
| `messages` | object[] | yes | Conversation history (see message schema below) |
| `tools` | object[] | no | OpenAI-shaped function-tool array (see §5) |
| `format` | string \| object | no | `"json"` or JSON schema |
| `options` | object | no | Runtime params |
| `stream` | bool | no | Default `true` |
| `keep_alive` | string \| int | no | Same semantics as /api/generate |
| `think` | bool \| string | no | Thinking models only |
| `logprobs` | bool | no | |
| `top_logprobs` | int | no | |

**Message object**:

| Field | Notes |
|---|---|
| `role` | `system` \| `user` \| `assistant` \| `tool` |
| `content` | string |
| `thinking` | string (assistant only, thinking models) |
| `images` | base64-encoded image array (multimodal) |
| `tool_calls` | array (assistant only — model-emitted tool calls) |
| `tool_name` | string (role:tool only — names the tool whose result this is) |

### 3.4 `POST /api/embed` / `POST /api/embeddings`

```json
{
  "model": "nomic-embed-text",
  "input": "string or string[]",
  "truncate": true,
  "options": {...},
  "keep_alive": "5m"
}
```

Returns `{"model":"…","embeddings":[[…],[…]]}`.

### 3.5 NDJSON streaming response format

When `stream: true` (default), `/api/generate` and `/api/chat` return newline-delimited JSON, one object per chunk. The final object has `"done": true` and includes timing stats. Example from docs.ollama.com/api/streaming:

```
{"model":"gemma3","created_at":"…","response":"That","done":false}
{"model":"gemma3","created_at":"…","response":"'","done":false}
{"model":"gemma3","created_at":"…","response":"s","done":false}
{"model":"gemma3","created_at":"…","response":" a","done":false}
…
{"model":"gemma3","created_at":"…","response":" question","done":true,"done_reason":"stop"}
```

**Completion detection contract** for an HTTP-API driver: parse each line as JSON; consume the `response` (generate) or `message.content` (chat) field; stop when `done: true` arrives. `done_reason` is one of `stop` (natural EOS), `length` (hit `num_predict`), `load` (model load failure), or `unload` (forced unload).

For tool-calling streams: chunks may carry `message.tool_calls` arrays — accumulate them across chunks (per docs: *"if chunk.message.thinking: thinking += chunk.message.thinking"* and similarly for content and tool_calls) and execute after `done: true`.

---

## 4. Model Parameters — Exhaustive

Every runtime parameter, where it can be set, and what it does.

### 4.1 Parameter table

| Parameter | Type | Default | Modelfile? | REPL `/set parameter`? | API `options:`? | Effect |
|---|---|---|---|---|---|---|
| `temperature` | float | 0.8 | yes | yes | yes | Sampling temperature — higher = more random |
| `top_p` | float | 0.9 | yes | yes | yes | Nucleus sampling cutoff |
| `top_k` | int | 40 | yes | yes | yes | Top-K sampling |
| `min_p` | float | 0.0 | yes | yes | yes | Minimum-probability sampling |
| `typical_p` | float | — | yes | yes | yes | Typical sampling cutoff |
| `num_ctx` | int | 2048 | yes | yes | yes | Context window size (tokens) — **the single most impactful param** |
| `num_predict` | int | -1 (∞) | yes | yes | yes | Max tokens to generate |
| `num_keep` | int | — | yes | yes | yes | Tokens to keep from prompt when context fills |
| `repeat_penalty` | float | 1.1 | yes | yes | yes | Repetition discouragement strength |
| `repeat_last_n` | int | 64 | yes | yes | yes | Look-back window for repeat penalty |
| `presence_penalty` | float | — | yes | yes | yes | OpenAI-style presence penalty |
| `frequency_penalty` | float | — | yes | yes | yes | OpenAI-style frequency penalty |
| `penalize_newline` | bool | true | yes | yes | yes | Apply repeat penalty to `\n` |
| `seed` | int | 0 (random) | yes | yes | yes | RNG seed — set non-zero for deterministic generation |
| `stop` | string[] | — | yes (multiple lines) | yes | yes | Stop sequences |
| `mirostat` | int | 0 (off) | yes | yes | yes | 0=off, 1=Mirostat, 2=Mirostat 2.0 |
| `mirostat_eta` | float | 0.1 | yes | yes | yes | Mirostat learning rate |
| `mirostat_tau` | float | 5.0 | yes | yes | yes | Mirostat target entropy |
| `tfs_z` | float | — | yes | yes | yes | Tail-free sampling |
| `num_gpu` | int | auto | yes | yes | yes | Number of layers to offload to GPU (−1=all, 0=CPU only) |
| `main_gpu` | int | 0 | yes | yes | yes | Which GPU index to prefer |
| `num_thread` | int | auto | yes | yes | yes | CPU threads |
| `num_batch` | int | — | yes | yes | yes | Prompt batch size |
| `use_mmap` | bool | true | yes | yes | yes | Memory-map the model file |
| `numa` | bool | false | yes | yes | yes | NUMA-aware allocation |

Sources: [github.com/ollama/ollama/blob/main/docs/modelfile.mdx](https://github.com/ollama/ollama/blob/main/docs/modelfile.mdx), Context7 dump of `/ollama/ollama` showing full options example for `/api/generate`.

### 4.2 Three places to set the same parameter — precedence

1. **Modelfile** `PARAMETER` instruction — baked in at `ollama create` time, applies to every invocation of that model.
2. **REPL** `/set parameter <k> <v>` — overrides the Modelfile for this REPL session only.
3. **API** `options: { … }` — overrides everything for this single request.

Precedence (highest wins): **API request > REPL `/set` > Modelfile PARAMETER > built-in default**.

### 4.3 The five params that matter for evolve-loop

For a per-phase reviewer LLM, the high-leverage params are:

| Param | Recommended | Why |
|---|---|---|
| `temperature` | 0.2 | Reviewers should be deterministic-ish; high temperature → hallucinated criticisms |
| `top_p` | 0.9 | Standard nucleus sampling |
| `num_ctx` | 8192 or 16384 | Reviewer must see full code context — most models default 2048 which truncates |
| `num_predict` | 4096 | Cap the verdict length |
| `seed` | 0 or fixed | 0 = random; fix it for ledger-replay-determinism |

The rest stay at defaults unless we have a specific reason.

---

## 5. Tool Use / Function Calling

### 5.1 Schema (OpenAI-compatible)

Ollama's `/api/chat` `tools` array uses OpenAI's function-tool schema:

```json
{
  "model": "qwen3",
  "messages": [{"role":"user","content":"What is the temperature in NYC?"}],
  "stream": false,
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_temperature",
        "description": "Get the current temperature for a city",
        "parameters": {
          "type": "object",
          "required": ["city"],
          "properties": {
            "city": {"type": "string", "description": "The name of the city"}
          }
        }
      }
    }
  ]
}
```

### 5.2 Model emits tool calls

Response (when the model decides to call a tool):

```json
{
  "model": "qwen3",
  "message": {
    "role": "assistant",
    "content": "",
    "tool_calls": [
      {
        "function": {
          "name": "get_temperature",
          "arguments": {"city": "New York"}
        }
      }
    ]
  },
  "done": true
}
```

### 5.3 Feeding result back

Append the tool result as a `role:tool` message and re-call `/api/chat`:

```json
{
  "model": "qwen3",
  "messages": [
    {"role":"user","content":"What is the temperature in NYC?"},
    {"role":"assistant","content":"","tool_calls":[…]},
    {"role":"tool","tool_name":"get_temperature","content":"22°C"}
  ],
  "tools": [...]
}
```

Source: [docs.ollama.com/capabilities/tool-calling](https://docs.ollama.com/capabilities/tool-calling).

### 5.4 Models that support tool calling

Per the [tools tag on ollama.com/library](https://ollama.com/search?c=tools): tool-capable models include `qwen3` (current docs example), `qwen2.5`, `llama3.1`, `llama3.2`, `mistral-nemo`, `mistral-small` (24B has strong agentic + JSON), `command-r`, `firefunction`. Different models have different reliability; qwen2.5/3 and mistral-small are currently the most reliable for production tool use.

Source: [Local AI Master ollama function calling guide](https://localaimaster.com/blog/ollama-function-calling-tools), [Morph best ollama models 2026](https://www.morphllm.com/best-ollama-models).

### 5.5 Critical limitation for evolve-loop

Tool calling is **API-only** — there is no way to define tools inside the REPL (`/set tools` doesn't exist). If a phase needs the LLM to call external tools, the phase MUST use Tier 2 (HTTP `/api/chat`), not Tier 1 (`ollama-tmux`).

For evolve-loop's planned use cases (per-phase reviewer LLM, reasoning-only review phases), tool calling is **not required** — the reviewer reads context + emits verdict, no tool dispatch. So Tier 1 is sufficient. Document this in the WS-F profile spec so future operators know not to try to use the REPL driver for tool-using phases.

### 5.6 Difference from Claude tool-use

| Aspect | Ollama | Claude (Anthropic) |
|---|---|---|
| Schema | OpenAI-shape `tools: [{type:function,function:{…}}]` | Anthropic-shape `tools: [{name,input_schema,…}]` |
| Streaming tool_calls | Accumulated across chunks (manual) | Discrete `tool_use` content blocks |
| In-CLI tool dispatch | None (HTTP API only) | Claude Code REPL has built-in tool plumbing (Bash, Read, Edit, …) |
| Reliability | Model-dependent (qwen3/mistral best) | Claude 4.5/4.6/4.7 designed for tool use, high reliability |

For Ollama via the Anthropic-compatible endpoint `/v1/messages`, you get Anthropic-shape tools, BUT the model still has to be tool-trained — `/v1/messages` is just a protocol shim, not a capability upgrade.

---

## 6. Structured Output

### 6.1 Three modes

| Mode | Syntax | Behavior |
|---|---|---|
| Free-form JSON | `"format": "json"` | Model emits any valid JSON |
| Schema-constrained | `"format": { …json schema… }` | Output structurally matches the schema |
| Free-form text | (omit `format`) | Default |

### 6.2 Example — schema-constrained

```json
{
  "model": "llama3.1",
  "messages": [{"role":"user","content":"Extract author and year from: \"Karpathy 2023 nanoGPT\""}],
  "stream": false,
  "format": {
    "type": "object",
    "required": ["author","year"],
    "properties": {
      "author": {"type":"string"},
      "year": {"type":"integer"}
    }
  }
}
```

Source: [docs.ollama.com/capabilities/structured-outputs](https://docs.ollama.com/capabilities/structured-outputs).

### 6.3 Best practices (from the docs)

- Pass the schema inside the prompt too (the model needs to "see" the constraint, not just be sampled against it).
- Set `temperature: 0` for deterministic JSON.
- Validate the JSON on receive — schema-constrained sampling reduces but doesn't eliminate parse errors.
- Define schemas with Pydantic (Python) or Zod (JS) so the same schema doubles as validator.

### 6.4 Limitations

1. **Cloud-only models DO NOT support structured outputs** — `format: { schema }` is local-only. Quoted from docs.ollama.com: *"Ollama's Cloud currently does not support structured outputs."*
2. **No GBNF grammar support** — Ollama's structured output is JSON-schema only. If you need arbitrary CFG-constrained generation (regex, code-grammar), use llama.cpp directly or a different runtime.
3. **REPL `/set format json`** sets free-form JSON mode only — there's no in-REPL way to pass a JSON schema. Schema-constrained → API only.

### 6.5 evolve-loop implication

For a verdict-emitting reviewer that must produce structured output (e.g., `{verdict: PASS|WARN|FAIL, findings: [...]}`), the WS-F driver should use **Tier 2 (HTTP API)** with `format: <schema>`. Tier 1 (REPL) can use `/set format json` for free-form JSON, but then we must validate + retry on schema-violations downstream.

---

## 7. Modelfile Composition

### 7.1 Instructions

| Instruction | Required | Purpose |
|---|---|---|
| `FROM <source>` | yes | Base model — can be registry name, GGUF file path, Safetensors directory, or `FROM scratch` |
| `PARAMETER <key> <value>` | no | Runtime parameter (see §4) — can appear many times |
| `TEMPLATE """…"""` | no | Go-template prompt format with `{{ .System }}`, `{{ .Prompt }}`, `{{ .Response }}` |
| `SYSTEM """…"""` | no | System prompt |
| `ADAPTER <path>` | no | (Q)LoRA adapter to apply |
| `LICENSE """…"""` | no | License text |
| `MESSAGE <role> "<content>"` | no | Few-shot example messages — `role` is `system`, `user`, or `assistant` |
| `REQUIRES <version>` | no | Minimum Ollama version |

Source: [docs.ollama.com/modelfile](https://docs.ollama.com/modelfile), [github.com/ollama/ollama/blob/main/docs/modelfile.mdx](https://github.com/ollama/ollama/blob/main/docs/modelfile.mdx).

### 7.2 Example Modelfile for an evolve-loop reviewer

```Modelfile
FROM llama3.1:8b

PARAMETER temperature 0.2
PARAMETER top_p 0.9
PARAMETER num_ctx 16384
PARAMETER num_predict 4096
PARAMETER seed 42

SYSTEM """You are an adversarial code reviewer.
Your job is to find real defects, not to be agreeable.
Emit JSON: {"verdict": "PASS|WARN|FAIL", "findings": [...]}.
Require positive evidence for PASS.
"""

MESSAGE user "Review this commit: <example>"
MESSAGE assistant '{"verdict":"WARN","findings":[{"severity":"low","location":"a.go:42","msg":"…"}]}'
```

Save as `Modelfile.evolve-reviewer-llama3.1`, then:

```bash
ollama create evolve-reviewer-llama3.1 -f Modelfile.evolve-reviewer-llama3.1
ollama run evolve-reviewer-llama3.1
```

### 7.3 On-the-fly Modelfile via `/api/create`

You can compose a Modelfile equivalent directly via the API, no disk file needed:

```bash
curl http://localhost:11434/api/create -d '{
  "model": "evolve-reviewer-llama3.1",
  "from": "llama3.1:8b",
  "system": "You are an adversarial code reviewer…",
  "parameters": {
    "temperature": 0.2,
    "num_ctx": 16384
  }
}'
```

Source: WebFetch of [github.com/ollama/ollama/blob/main/docs/api.md](https://github.com/ollama/ollama/blob/main/docs/api.md) — `POST /api/create` schema.

This is the right pattern for evolve-loop: at startup, the bridge synthesizes a per-phase Modelfile via `/api/create` from the phase's profile, names it `evolve-<phase>-<cycle>` for traceability, then targets it for the REPL launch. Cleanup with `DELETE /api/delete` at cycle end.

### 7.4 Layered FROM

You can FROM an already-created custom model to layer further customization:

```Modelfile
FROM evolve-reviewer-llama3.1
PARAMETER temperature 0.1
SYSTEM """[More specific override]"""
```

This is a clean composition mechanism — a base "evolve-reviewer-llama3.1" + per-phase overlays.

---

## 8. Multi-Model / Per-Request Model Selection

### 8.1 Switching models inside a REPL session — NO

There is **no in-REPL `/load <new-model>`** to switch the active model — `/load` and `/save` operate on saved **sessions** (history + system + params for the same model), not on swapping models.

To switch model, you must exit (`/bye`) and re-invoke `ollama run <other-model>`.

### 8.2 Switching models per-request via HTTP — YES

HTTP `/api/chat` includes `model` in every request. Send model A on one call, model B on the next — Ollama loads/unloads as needed (subject to `OLLAMA_MAX_LOADED_MODELS` and `OLLAMA_KEEP_ALIVE`).

### 8.3 Pre-loading multiple models

Send a `keep_alive: -1` request to each model upfront to keep them all hot:

```bash
curl http://localhost:11434/api/generate -d '{"model":"llama3.1:8b","prompt":"","keep_alive":-1}'
curl http://localhost:11434/api/generate -d '{"model":"qwen2.5:7b","prompt":"","keep_alive":-1}'
```

Subject to VRAM limits and `OLLAMA_MAX_LOADED_MODELS`.

### 8.4 evolve-loop recommendation

**Spawn a fresh `ollama run <model>` per phase invocation.** This matches WS-F brief (peer of claude-tmux/codex-tmux/agy-tmux, one tmux session per phase).

Reasons:
- Operator-visible tmux session per phase = uniform UX.
- REPL session has its own `/set system`, `/set parameter`, `/set format`, history — clean isolation between phases.
- Cheap: with `OLLAMA_KEEP_ALIVE=10m`, the model stays loaded between consecutive same-model phases.
- Cleanup is `tmux kill-session`, no API state to GC.

The alternative (long-lived REPL with `/clear` between phases) introduces session-state leak risk (forgotten `/set` values) and complicates failure recovery. Don't.

---

## 9. Output Streaming & Completion Detection

### 9.1 REPL completion detection

The Ollama REPL emits the `>>> ` prompt marker when generation completes and it's ready for the next input. The full prompt line is `>>> Send a message (/? for help)` on the first turn, then just `>>> ` on subsequent turns.

**WS-F detection contract**:

```
After tmux paste of prompt + Enter:
1. Wait for the prompt to leave the bottom of the pane (model started generating).
2. Wait for `>>> ` to re-appear at the bottom-most non-blank line, with no further changes for N seconds (debounce).
3. Capture everything between the user's prompt and the new `>>> ` as the response.
```

This is structurally identical to the claude-tmux / codex-tmux completion detection, so the existing `driver_tmux_repl.go` engine + ADR-0027 PR1 completion-detector Strategy applies directly. Add an `ollamatmux` Strategy variant that recognizes `^>>> ` as the EOF marker (vs Claude's `> ` and Codex's `▶`).

### 9.2 Multi-line prompt handling

Ollama auto-detects single-line vs multi-line. For multi-line input, wrap in `"""`:

```
>>> """
... explain in detail:
... why is the sky blue?
... and why isn't it green?
... """
```

The model receives the body between the `"""` markers as one prompt.

**WS-F tmux paste contract**: when the prompt contains `\n`, prepend `"""\n` and append `\n"""\n`. Use `tmux paste-buffer` (or `send-keys -l`) to send the buffer literally without shell interpretation. The existing tmux-driver paste helper needs only the prefix/suffix tweak.

### 9.3 No streaming envelope to parse

Unlike Claude Code's `--output-format=stream-json`, Ollama's REPL streams raw text to the pane — no JSON wrappers, no event types, no tool_use blocks. The scout/build/audit phases that consume `<phase>-stdout.log` already get human-readable text, so the v12.2 stdout filter (`~/ai/claude/evolve-loop/go/internal/logfilter/`) has very little to clean for ollama-tmux. Plain text in, plain text out.

### 9.4 HTTP API completion detection (Tier 2)

Per §3.5: parse NDJSON line-by-line, accumulate `response` / `message.content`, stop on `done: true`. `done_reason` distinguishes natural EOS vs `length` (hit num_predict cap) vs `load` failure.

---

## 10. Integration Patterns

Surveyed how other agentic projects drive Ollama.

### 10.1 `aider`

Uses HTTP `/api/chat` (via the OpenAI-compat `/v1/chat/completions`). No REPL — talks to the daemon directly. Prompt pre-cached on the daemon side. Streams tokens to the user's terminal as they arrive. No tmux involvement.

### 10.2 `Open WebUI`

Same pattern — HTTP `/api/chat`. Web UI, not terminal. Includes a per-conversation model selector that translates to `model` field per request.

### 10.3 `llm` CLI (Simon Willison)

Uses HTTP `/api/generate` or `/api/chat` via the `llm-ollama` plugin. One-shot pattern: `llm -m ollama:llama3.1 "prompt"`. Streams stdout. Mirrors the `echo "prompt" | ollama run model` pattern from §1.4.

### 10.4 `Continue.dev`

VSCode plugin, HTTP `/api/chat`. The plugin's config file picks the model per-request, mirroring evolve-loop's per-phase model selection. They handle tool calling via their own plugin layer, not via Ollama's tool field — historically because Ollama tool-calling was unreliable on smaller models.

### 10.5 `agent-browser` / `claude-in-chrome` style tmux drivers

evolve-loop is one of the few projects that drives a CLI REPL via tmux. Other tmux-CLI projects (e.g. `nanoclaw`, `dmux`) use the same shape: send-keys + paste-buffer for input, capture-pane for output, completion detection by prompt-marker re-appearance.

### 10.6 Canonical "send prompt, capture answer" recipe

Three flavors:

**Flavor A — HTTP, blocking** (simplest, recommended for headless):
```bash
curl -s http://localhost:11434/api/generate -d '{
  "model": "llama3.1:8b",
  "prompt": "…",
  "stream": false,
  "options": {"temperature":0.2,"num_ctx":8192}
}' | jq -r '.response'
```

**Flavor B — HTTP, streaming** (best for token-by-token UX):
```bash
curl -sN http://localhost:11434/api/generate -d '{"model":"…","prompt":"…","stream":true}' \
  | while read -r line; do
      echo "$line" | jq -j '.response // empty'
      echo "$line" | jq -e '.done' >/dev/null && break
    done
```

**Flavor C — one-shot CLI** (no daemon-direct):
```bash
echo "…" | ollama run llama3.1:8b --format json
```

**Flavor D — tmux REPL** (evolve-loop's pattern):
```bash
tmux new -d -s ollama-phase-N
tmux send-keys -t ollama-phase-N "ollama run llama3.1:8b" Enter
# wait for first >>>
tmux paste-buffer -t ollama-phase-N -b prompt_buf   # buffer holds the """…""" wrapped prompt
tmux send-keys -t ollama-phase-N Enter
# poll capture-pane until >>> re-appears at bottom + N-second debounce
tmux capture-pane -t ollama-phase-N -p > response.txt
tmux send-keys -t ollama-phase-N "/bye" Enter
```

WS-F implements Flavor D.

---

## 11. Control-Parameter Map (evolve-loop profile JSON)

This is the actionable spec: which Ollama controls evolve-loop should expose to operators, where each setting lives in the profile JSON, and how the bridge realizes it.

### 11.1 Profile JSON shape (proposed)

```json
{
  "agent": "auditor",
  "driver": "ollama-tmux",
  "model": "llama3.1:8b",
  "fallback_models": ["qwen2.5:7b","mistral-nemo:12b"],
  "ollama": {
    "host": null,
    "api_key": null,
    "keep_alive": "10m",
    "system_prompt": "You are an adversarial code reviewer…",
    "options": {
      "temperature": 0.2,
      "top_p": 0.9,
      "top_k": 40,
      "num_ctx": 16384,
      "num_predict": 4096,
      "seed": 0,
      "repeat_penalty": 1.1,
      "stop": ["<|user|>","<|system|>"]
    },
    "format": null,
    "verbose": false,
    "think": false,
    "use_modelfile": false,
    "modelfile_template": null
  }
}
```

### 11.2 Field-by-field map

| Profile field | Maps to | Realization in tmux driver |
|---|---|---|
| `driver: "ollama-tmux"` | Driver selection | `BridgeRequest.Driver = "ollama-tmux"` |
| `model` | Model tag | `ollama run <model>` — supports local (`llama3.1:8b`) and cloud (`gpt-oss:120b-cloud`) |
| `fallback_models` | If primary unavailable | Driver tries each in order at launch; first that loads wins |
| `ollama.host` | `OLLAMA_HOST` env at launch | Set in `BridgeRequest.Env`; defaults to inherit from operator env |
| `ollama.api_key` | `OLLAMA_API_KEY` env at launch | Set in `BridgeRequest.Env`; required for `:cloud` tags |
| `ollama.keep_alive` | `OLLAMA_KEEP_ALIVE` env | Set at launch; ensures model stays hot between phases |
| `ollama.system_prompt` | `/set system <text>` on REPL entry | Driver sends `/set system "…"` before first user prompt |
| `ollama.options.*` | `/set parameter <k> <v>` for each | Driver iterates and sends each `/set parameter` line |
| `ollama.format` | `/set format json` if `"json"`; otherwise nothing | Tier 1 limit — schema-constrained needs Tier 2 |
| `ollama.verbose` | `--verbose` CLI flag | Append to `ollama run` invocation |
| `ollama.think` | `--think` CLI flag | Append to `ollama run` invocation |
| `ollama.use_modelfile` + `modelfile_template` | Tier 1.5 — pre-create custom model via `/api/create` | Driver POSTs to `http://$OLLAMA_HOST/api/create` at session start, gets back `evolve-<phase>-<cycle>` model name, runs that |

### 11.3 What we deliberately do NOT expose (yet)

| Param | Reason |
|---|---|
| `mirostat*`, `tfs_z`, `typical_p` | Niche; few operators tune these |
| `num_gpu`, `main_gpu`, `num_thread`, `num_batch`, `use_mmap`, `numa` | Hardware-specific; auto-detection is good |
| `presence_penalty`, `frequency_penalty` | Rarely needed for reasoning/review |
| `penalize_newline` | Default fine |
| `logprobs`, `top_logprobs` | Future feature (eval / introspection) |
| Tools array | Tier 2 only |
| Multimodal `images` | Out of WS-F scope |

If an operator needs one of these, they can override via `BridgeRequest.ExtraFlags` (passed through to `ollama run`) or via a Modelfile (Tier 1.5).

### 11.4 Stage A → B → C rollout

1. **Stage A (MVP)**: model + system_prompt + options{temperature, top_p, num_ctx, num_predict, seed}. Ships first. ~95% of the value.
2. **Stage B**: keep_alive, format, verbose, think, fallback_models, host, api_key. Adds operator polish.
3. **Stage C (Tier 1.5)**: use_modelfile + modelfile_template + per-phase `/api/create`. Enables schema-constrained output and cleaner per-phase isolation.
4. **Stage D (Tier 2)**: HTTP `ollama-api` driver for tool-using phases + structured-output phases.

### 11.5 Cross-CLI parity matrix (per WS-F brief)

Each driver should expose the equivalent shape, so operators can swap CLIs cleanly:

| Concept | Claude (claude-tmux) | Codex (codex-tmux) | Antigravity (agy-tmux) | Ollama (ollama-tmux, NEW) |
|---|---|---|---|---|
| Model select | `--model haiku/sonnet/opus` | `--model gpt-5.4-mini/-5.4/-5.5` | `--model gemini-3.5-flash` | model tag (`llama3.1:8b`, `gpt-oss:120b-cloud`) |
| System prompt | `--append-system-prompt` | `--system` | `--system` | `/set system` in REPL |
| Temperature | (Claude doesn't expose) | `--temperature` | `--temperature` | `/set parameter temperature` |
| Context window | (auto) | (auto) | (auto) | `/set parameter num_ctx` |
| Max output | `--max-tokens` | `--max-tokens` | `--max-tokens` | `/set parameter num_predict` |
| Determinism | `--seed` (in some) | `--seed` | `--seed` | `/set parameter seed` |
| Cost | Anthropic subscription | OpenAI API | Google AI Studio | Local: free / Cloud: Ollama Pro/Max plan |
| Auth | OAuth (`~/.claude.json`) | API key | API key | `ollama signin` OAuth or `OLLAMA_API_KEY` |
| Tool use | Native, reliable | Native, reliable | Native, reliable | Model-dependent (qwen3/mistral best), API-only |
| Structured output | JSON-schema via tool-result shape | OpenAI structured output | Gemini structured output | Local: yes; Cloud: NO |

---

## 12. Open Questions & Follow-Ups

Items to validate during WS-F implementation:

1. **REPL multi-line edge cases.** Does `tmux paste-buffer` of `"""\nbig prompt\n"""` work reliably across Ollama versions, or are there race conditions with the REPL's input echo? Bench with a 10KB prompt and check for truncation.

2. **`/set parameter` echo behavior.** Does the REPL echo a confirmation after each `/set parameter`? If yes, we need to consume that echo in the completion-detector before sending the user prompt. Worst case: tunnel each `/set` and wait for prompt-return between each.

3. **Cloud rate limits.** What's the exact rate limit on Ollama Free? evolve-loop should expose `EVOLVE_OLLAMA_RATE_LIMIT_RPM` or rely on 429-retry with backoff. Not documented in ollama.com/cloud public pages.

4. **`/api/create` cleanup.** If the bridge dies mid-cycle, do orphan `evolve-<phase>-<cycle>` models accumulate? Add a startup-time GC pass that `DELETE /api/delete`s any `evolve-*` model older than 24h.

5. **`OLLAMA_NEW_ENGINE` impact.** The new engine flag is `false` by default. Should we set it `true` in evolve-loop profiles for the perf boost? Test correctness first.

6. **Embedding endpoint for context retrieval.** Out of scope for WS-F but worth noting: if evolve-loop ever wants local-only RAG / instinct retrieval, `nomic-embed-text` via `/api/embed` is the no-network alternative to OpenAI embeddings. Cheap, fast, offline-capable.

7. **`/v1/messages` Anthropic compatibility.** Ollama exposes an Anthropic-shape endpoint. If we want a single bridge code path that targets BOTH Claude-cloud and Ollama-local, point both at `POST /v1/messages` with `Authorization: Bearer …` and let the model-tag pick the backend. Future architectural cleanup, not WS-F.

8. **Per-phase `--permission-mode` analog.** Claude has `--permission-mode plan|acceptEdits|default`. Ollama has nothing equivalent (no tool dispatch in REPL). The evolve-loop profile field `permission_mode` should be no-op'd for `ollama-tmux` (driver ignores it) with a one-line debug-log notice.

---

## 13. Sources

Primary (cited inline):
- [docs.ollama.com/cli](https://docs.ollama.com/cli) — CLI reference
- [docs.ollama.com/api](https://docs.ollama.com/api) — HTTP API index
- [docs.ollama.com/api/generate](https://docs.ollama.com/api/generate) — /api/generate field reference
- [docs.ollama.com/api/chat](https://docs.ollama.com/api/chat) — /api/chat field reference + streaming format
- [docs.ollama.com/api/authentication](https://docs.ollama.com/api/authentication) — OLLAMA_API_KEY, cloud auth, `:cloud` tags
- [docs.ollama.com/api/anthropic-compatibility](https://docs.ollama.com/api/anthropic-compatibility) — `/v1/messages` endpoint
- [docs.ollama.com/api/streaming](https://docs.ollama.com/api/streaming) — NDJSON streaming format
- [docs.ollama.com/capabilities/tool-calling](https://docs.ollama.com/capabilities/tool-calling) — Function calling schema
- [docs.ollama.com/capabilities/structured-outputs](https://docs.ollama.com/capabilities/structured-outputs) — `format` field, JSON schema, cloud limitation
- [docs.ollama.com/modelfile](https://docs.ollama.com/modelfile) — Modelfile DSL reference
- [ollama.com/cloud](https://ollama.com/cloud) — Pricing tiers (Free / Pro $20 / Max $100)
- [ollama.com/library](https://ollama.com/library), [ollama.com/search?c=tools](https://ollama.com/search?c=tools) — Model catalog, tool-capable filter
- [github.com/ollama/ollama/blob/main/docs/api.md](https://github.com/ollama/ollama/blob/main/docs/api.md) — `/api/create`, `/api/show` schemas
- [github.com/ollama/ollama/blob/main/docs/modelfile.mdx](https://github.com/ollama/ollama/blob/main/docs/modelfile.mdx) — Modelfile spec
- [github.com/ollama/ollama/blob/main/envconfig/config.go](https://github.com/ollama/ollama/blob/main/envconfig/config.go) — All OLLAMA_* env vars
- [github.com/ollama/ollama/issues/7823](https://github.com/ollama/ollama/issues/7823) — STDIN piping behavior

Corroborating (cheatsheets, third-party guides):
- [Rost Glukhov — Ollama CLI Cheatsheet (2026 update)](https://www.glukhov.org/llm-hosting/ollama/ollama-cheatsheet/)
- [computingforgeeks — Ollama Commands Cheat Sheet](https://computingforgeeks.com/ollama-commands-cheat-sheet/)
- [Hostinger — Ollama CLI tutorial](https://www.hostinger.com/tutorials/ollama-cli-tutorial)
- [Geshan — Ollama commands part 2](https://geshan.com.np/blog/2025/02/ollama-commands/)
- [LLM Hardware — Ollama Cheat Sheet 2026](https://llmhardware.io/guides/ollama-cheat-sheet)
- [DeepWiki ollama/ollama 3.6 — Authentication and API Keys](https://deepwiki.com/ollama/ollama/3.6-authentication-and-api-keys)
- [Local AI Master — Function Calling Guide](https://localaimaster.com/blog/ollama-function-calling-tools)
- [Morph — Best Ollama Models 2026](https://www.morphllm.com/best-ollama-models)
- [OpenClaw — Ollama provider docs](https://docs.openclaw.ai/providers/ollama)
- [Promptfoo — Ollama provider](https://www.promptfoo.dev/docs/providers/ollama/)

Context7 dumps (full):
- `/ollama/ollama` (569 snippets, source reputation High, benchmark 83.5)
- `/websites/ollama_api` (115 snippets, source reputation High, benchmark 85.9)

---

*Dossier complete. Suitable for direct ingestion by the WS-F implementation kickoff plan. Update the Stage-A profile JSON section as fields are validated against a live Ollama daemon.*
