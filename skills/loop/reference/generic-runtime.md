# Generic Runtime

> Fallback for any agentic CLI without a tested adapter (Copilot, custom internal CLIs, future entrants). Skill content is portable; runtime guarantees are not.

## What works

You can read SKILL.md, the phase docs (`phases.md`, `phase0-calibrate.md`, etc.), and the reference files. The information is platform-neutral.

You can run `evolve loop ...` directly if your CLI has shell access (the STRICT MODE command in SKILL.md). The dispatcher is platform-neutral: it runs one cycle per iteration and verifies the ledger entries. It does not require a specific CLI to be the caller.

## What does NOT work

- **Subagent isolation.** The dispatcher dispatches each phase agent through the Go bridge. If the bridge has no driver for the CLI, the dispatch fails with a no-driver error. The gemini alias (the bridge runs the claude-tmux driver for a bare `gemini`) is one viable workaround; see `reference/gemini-runtime.md` for the template. (The legacy bash CLI adapters under `adapters/*.sh` were removed in the script→Go migration; only the `*.capabilities.json` manifests remain.)

- **PreToolUse kernel hooks.** `evolve guard role`, `evolve guard ship` and `evolve guard phase` are configured in `.claude/settings.json` and fire on Claude Code's PreToolUse mechanism; other CLIs may have different hook surfaces or none. Without the hooks, the trust boundary is advisory rather than structural. A sufficiently confused or adversarial agent can edit source files directly or push without `evolve ship`. The Go state machine still enforces the phase order inside `evolve loop`. This is the substance of the forgery incident (`docs/incidents/gemini-forgery.md`).

- **Tool name translation.** The skill text uses Claude Code names (`Read`, `Bash`, `Skill`, `Agent`). If your CLI uses different names, you need a translation file at `reference/<your-cli>-tools.md`. Existing examples: `claude-tools.md`, `gemini-tools.md`, `codex-tools.md`.

## Three viable options on an unsupported CLI

### Option 1 — Read-only

Treat evolve-loop as a documentation source. Read the phase docs, learn the architecture, but execute cycles only on Claude Code or through the gemini → claude-tmux alias.

### Option 2 — Hybrid driver (recommended for any new CLI)

Mirror the gemini alias: add the bare CLI name to `bareDriverMap` in `go/internal/bridge/driver.go`, so the bridge runs the claude-tmux driver for it. Add an `adapters/<cli>.capabilities.json` manifest too. Benefit: full trust-boundary preservation. Trade-off: requires the Claude binary at runtime.

### Option 3 — Native adapter

Implement a real Go bridge driver against your CLI's flag surface (`go/internal/bridge/driver_<cli>.go`), with a bridge manifest (`go/internal/bridge/manifests/<cli>.json`) and an `adapters/<cli>.capabilities.json` manifest. Benefit: no Claude binary required. Trade-off: must verify your CLI supports profile-scoped permissions, non-interactive prompt mode, and either a budget cap flag or external cost tracking. Tier 1 designation requires passing the same Go regression suite Claude does.

The env-var interface of the bash adapters is removed design. See [docs/architecture/platform-compatibility.md](../../../docs/architecture/platform-compatibility.md) for the CLI support, and [docs/architecture/agent-bridge-dispatch-contract.md](../../../docs/architecture/agent-bridge-dispatch-contract.md) for the bridge dispatch contract.

## Detecting your CLI

Auto-detection runs via `evolve detect-cli` — see `reference/platform-detect.md` for the probe table. If your CLI isn't detected, add a `reference/<your-cli-name>-tools.md` and `reference/<your-cli-name>-runtime.md`; without them you'll stay on this generic runtime.
