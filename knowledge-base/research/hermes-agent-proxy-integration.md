# Hermes-Agent Proxy Integration Research

> **Status:** Verified against hermes-agent v0.11.0 (2026-05-19)
> **Scope:** Whether hermes-agent can act as an Anthropic API proxy for `claude -p` / `EVOLVE_ANTHROPIC_BASE_URL`

---

## Table of Contents

1. [What hermes-agent is](#1-what-hermes-agent-is)
2. [Endpoint format: OpenAI vs Anthropic](#2-endpoint-format-openai-vs-anthropic)
3. [`hermes proxy start` — command verification](#3-hermes-proxy-start--command-verification)
4. [Port 8645 — provenance](#4-port-8645--provenance)
5. [Auth flow: hermes login providers](#5-auth-flow-hermes-login-providers)
6. [Does hermes read Claude Code credentials?](#6-does-hermes-read-claude-code-credentials)
7. [Verdict: does hermes work as an Anthropic passthrough?](#7-verdict-does-hermes-work-as-an-anthropic-passthrough)
8. [What actually works for subscription auth with `claude -p`](#8-what-actually-works-for-subscription-auth-with-claude--p)
9. [What tools DO work if a proxy is needed](#9-what-tools-do-work-if-a-proxy-is-needed)
10. [Recommended CLAUDE.md correction](#10-recommended-claudemd-correction)

---

## 1. What hermes-agent is

Hermes Agent (by Nous Research, `~/.local/bin/hermes`, v0.11.0) is a **self-improving AI agent** — a chat agent with tool-calling, skills, scheduling, and messaging-platform integrations (Telegram, Discord, WhatsApp). It is NOT an HTTP API proxy.

Hermes uses the OpenAI Python SDK (`openai==2.32.0`) to call inference APIs. It supports many providers including Nous Portal, OpenRouter, NVIDIA NIM, OpenAI, and also Anthropic (via the `anthropic` Python SDK). It calls these APIs for its **own** conversations; it does not expose a local HTTP server that forwards API calls from other tools.

---

## 2. Endpoint format: OpenAI vs Anthropic

| Aspect | Detail |
|--------|--------|
| Primary API format | OpenAI SDK / OpenAI-compatible (`/v1/chat/completions`) |
| Anthropic format | hermes uses `anthropic` Python SDK internally for Anthropic calls — it speaks `/v1/messages` format to Anthropic's API, but this is for **inbound** to Anthropic, not exposed **outbound** |
| Local HTTP server | hermes exposes a **Web UI** server (`hermes dashboard`) and a **messaging gateway** (`hermes gateway`) — neither proxies Anthropic API calls |
| `ANTHROPIC_BASE_URL` | hermes does not set or consume `ANTHROPIC_BASE_URL` for its own operations |

**`claude -p`'s `ANTHROPIC_BASE_URL`** must point to a service that serves the Anthropic Messages API format (`POST /v1/messages`). hermes does not provide this.

---

## 3. `hermes proxy start` — command verification

**VERDICT: `hermes proxy start` is a fabricated command. It does not exist.**

Verified by running `hermes proxy --help` against hermes v0.11.0:

```
hermes: error: argument command: invalid choice: 'proxy'
(choose from 'chat', 'model', 'gateway', 'setup', 'whatsapp', 'login',
 'logout', 'auth', 'status', 'cron', 'webhook', 'hooks', 'doctor', 'dump',
 'debug', 'backup', 'import', 'config', 'pairing', 'skills', 'plugins',
 'memory', 'tools', 'mcp', 'sessions', 'insights', 'claw', 'version',
 'update', 'uninstall', 'acp', 'profile', 'completion', 'dashboard', 'logs')
```

The CLAUDE.md row that ships the setup instruction `hermes proxy start` is **incorrect and must not be executed** — it will error immediately.

---

## 4. Port 8645 — provenance

The value `8645` appears in hermes source at:

```
hermes_cli/gateway.py:2663
{"name": "WECOM_CALLBACK_PORT", "prompt": "Callback server port (default: 8645)", ...}
```

This is the **WeCom (WeChat Work)** messaging-platform callback server port — completely unrelated to Anthropic API proxying. The origin of `EVOLVE_ANTHROPIC_BASE_URL=http://127.0.0.1:8645/v1` in CLAUDE.md is unclear; this URL never points at any hermes service.

---

## 5. Auth flow: hermes login providers

`hermes login` supports `--provider {nous,openai-codex}` only. There is no `anthropic` provider for the hermes OAuth login flow. Running `hermes login anthropic` would fail.

The correct hermes login for Nous subscription access is `hermes login nous` (or `hermes login` with no flag, which defaults to `nous`). This authenticates hermes with the Nous inference portal — it does not create any Anthropic credential that could be injected into `claude -p` calls.

---

## 6. Does hermes read Claude Code credentials?

Yes — but for hermes's own use, not for proxying. hermes's `agent/anthropic_adapter.py:read_claude_code_credentials()` reads:

1. macOS Keychain entry `"Claude Code-credentials"`
2. `~/.claude/.credentials.json` (`claudeAiOauth.accessToken`)

This allows hermes to call Anthropic's API using the user's Claude Code OAuth token when the user configures hermes with the `claude-code` model. This is **hermes acting as a client to Anthropic API** — it does not make hermes a proxy for other tools.

---

## 7. Verdict: does hermes work as an Anthropic passthrough?

**NO.** hermes-agent:

- Does not expose any HTTP server that accepts `POST /v1/messages`
- Has no `proxy` subcommand
- Port 8645 is for WeCom, not Anthropic
- `hermes login nous` authenticates hermes itself, not `claude -p`

hermes cannot be used as the value of `EVOLVE_ANTHROPIC_BASE_URL`.

---

## 8. What actually works for subscription auth with `claude -p`

**Claude Code's `claude -p` already handles subscription auth natively** — no proxy required.

The `claude` binary reads OAuth credentials from:
- `~/.claude/.credentials.json` (`claudeAiOauth.accessToken`)
- macOS Keychain (`"Claude Code-credentials"`)
- `~/.claude.json` (`primaryApiKey`)

When `ANTHROPIC_API_KEY` is unset and no `--bare` flag is passed, `claude -p` uses OAuth subscription auth automatically. The evolve-loop adapter already handles this correctly:

```bash
# claude.sh: drops --bare when ANTHROPIC_API_KEY is unset
if [ -z "${ANTHROPIC_API_KEY:-}" ] && [ "${EVOLVE_FORCE_BARE:-0}" != "1" ]; then
    # strips --bare from EXTRA_FLAGS_ARR
fi
```

This means: **for most operators, `EVOLVE_ANTHROPIC_BASE_URL` is not required**. Subscription auth works out of the box via `~/.claude.json`.

---

## 9. What tools DO work if a proxy is needed

`EVOLVE_ANTHROPIC_BASE_URL` is valid and useful for routing `claude -p` through a **custom Anthropic-format endpoint** — but the tool must serve `POST /v1/messages` with Anthropic protocol. Options:

| Tool | Use case | Notes |
|------|----------|-------|
| LiteLLM proxy | Route to Anthropic or compatible models | Requires `ANTHROPIC_API_KEY` in the proxy env |
| Custom FastAPI proxy | Auth injection, request logging | Must speak Anthropic Messages format |
| Corporate API gateway | Enterprise routing | Must speak Anthropic Messages format |

None of these provide "free" subscription auth bypass — they still require credentials in the proxy layer. The native `~/.claude.json` path is the only credential-free subscription path.

---

## 10. Recommended CLAUDE.md correction

Remove the fabricated `hermes proxy start` command and clarify:

```markdown
| Subscription proxy | `EVOLVE_ANTHROPIC_BASE_URL` | unset | When set, exported as `ANTHROPIC_BASE_URL` before every `claude -p` invocation. Proxy-agnostic: target must speak Anthropic Messages API format (`POST /v1/messages`). **Not required for subscription auth** — `claude -p` reads `~/.claude.json` OAuth credentials natively. Use this only for custom endpoints (LiteLLM, corporate gateway). Example: `export EVOLVE_ANTHROPIC_BASE_URL=http://127.0.0.1:4000/v1` (LiteLLM default port). **Do not use `hermes proxy start`** — that command does not exist in hermes-agent. |
```

---

## References

- hermes-agent v0.11.0 source: `~/.hermes/hermes-agent/`
- `hermes_cli/auth.py`: provider registry, `read_claude_code_credentials`
- `hermes_cli/gateway.py:2663`: WeCom port 8645
- `agent/anthropic_adapter.py:530`: `read_claude_code_credentials` function
- `scripts/cli_adapters/claude.sh:129-139`: `EVOLVE_ANTHROPIC_BASE_URL` implementation
- `scripts/dispatch/subagent-run.sh:65-75`: proxy passthrough and WARN logic
- evolve-loop commit `b2197be`: origin of `EVOLVE_ANTHROPIC_BASE_URL` feature
