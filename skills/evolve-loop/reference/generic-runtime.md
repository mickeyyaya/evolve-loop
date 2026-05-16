# Generic Runtime

> Fallback for any agentic CLI without a tested adapter (Codex, Copilot, custom internal CLIs, future entrants). Skill content is portable; runtime guarantees are not.

## What works

You can read SKILL.md, the phase docs (`phases.md`, `phase0-calibrate.md`, etc.), and the reference files. The information is platform-neutral.

You can invoke `bash scripts/dispatch/evolve-loop-dispatch.sh ...` directly if your CLI has shell access. The dispatcher itself is platform-neutral — it loops over `run-cycle.sh` and verifies ledger entries. It does not require a specific CLI to be the caller.

## What does NOT work

- **Subagent isolation.** The dispatcher will try to invoke `subagent-run.sh`, which dispatches to `scripts/cli_adapters/<cli>.sh`. If your CLI's adapter is a stub (codex.sh) or doesn't exist, you'll get exit 99 ("provider not supported"). The hybrid pattern used for Gemini is one viable workaround; see `reference/gemini-runtime.md` for the template.

- **PreToolUse kernel hooks.** `role-gate`, `ship-gate`, and `phase-gate-precondition` are configured in `.claude-plugin/plugin.json` and fire on Claude Code's PreToolUse mechanism. Other CLIs may have different hook surfaces or none. Without the hooks, the trust boundary is advisory rather than structural — a sufficiently confused or adversarial agent can edit source files directly, push without going through `ship.sh`, or skip phases. This is the substance of the forgery incident referenced from `docs/platform-compatibility.md`.

- **Tool name translation.** The skill text uses Claude Code names (`Read`, `Bash`, `Skill`, `Agent`). If your CLI uses different names, you need a translation file at `reference/<your-cli>-tools.md`. Existing examples: `claude-tools.md`, `gemini-tools.md`, `codex-tools.md`.

## Three viable options on an unsupported CLI

### Option 1 — Read-only

Treat evolve-loop as a documentation source. Read the phase docs, learn the architecture, but execute cycles only on Claude Code or via the Gemini hybrid driver.

### Option 2 — Hybrid driver (recommended for any new CLI)

Mirror what `scripts/cli_adapters/gemini.sh` does: implement a thin shim that probes for `claude` and delegates. Cost: ~50 lines of bash. Benefit: full trust-boundary preservation. Trade-off: requires the Claude binary at runtime.

### Option 3 — Native adapter

Implement a real `scripts/cli_adapters/<cli>.sh` against your CLI's flag surface. Cost: 500–1000 lines (mirror `claude.sh`). Benefit: no Claude binary required. Trade-off: must verify your CLI supports profile-scoped permissions, non-interactive prompt mode, and either a budget cap flag or external cost tracking. Tier 1 designation requires passing the same regression suite Claude does (`scripts/utility/run-all-regression-tests.sh`).

See [docs/platform-compatibility.md](../../../docs/platform-compatibility.md) "Adapter contract" section for the env-var interface every adapter must satisfy.

## Detecting your CLI

Set `EVOLVE_PLATFORM=<your-cli-name>` to bypass auto-detection. The skill will then look for `reference/<your-cli-name>-tools.md` and `reference/<your-cli-name>-runtime.md`. If neither exists, you'll fall back to this file.
