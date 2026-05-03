# Plugin Boundary Incident — 2026-05-03

> Cycle 1 of a `/evolve-loop` run from `/Users/danleemh/ai/claude/learning` aborted at $0.35 / 112 s with rc=2; cycles 2–5 never started. Dispatcher classified it as integrity-breach. Root cause was structural, not behavioral.

## What happened

A user invoked `/evolve-loop` from a project at `/Users/danleemh/ai/claude/learning`. The orchestrator subagent attempted to execute the calibrate phase and aborted before reaching scout. Two distinct failures:

1. **Bash sandbox mismatch.** The orchestrator's bash was scoped to the project root (`/Users/danleemh/ai/claude/learning`). Phase scripts (`scripts/cycle-state.sh`, `scripts/subagent-run.sh`) live in the plugin install directory (`~/.claude/plugins/cache/evolve-loop/evolve-loop/8.17.0/`) and the orchestrator-prompt invoked them with relative paths (`bash scripts/cycle-state.sh advance ...`). Relative paths resolved against cwd, where the scripts do not exist, so bash failed.

2. **Sensitive-path write block.** The orchestrator attempted to write `orchestrator-report.md` into the workspace path `~/.claude/plugins/cache/evolve-loop/evolve-loop/8.17.0/.evolve/runs/cycle-1/`. Claude Code blocks all writes under `~/.claude/` as a sensitive-path policy. The dispatcher's chosen workspace location was therefore unwritable.

Both stem from a single architectural mistake: every kernel script computed `REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"` and treated that single root as both the read-source (scripts/, agents/) AND the write-target (state.json, ledger.jsonl, runs/). That conflation works in development (cwd == repo) and breaks under the plugin install pattern where the script-install location and the user-project location diverge.

## Why prior tests didn't catch it

The regression suite runs from the dev repo. In dev mode, plugin-root and project-root are the same directory; the conflation is invisible. There was no test exercising "scripts at path A, cwd at path B." The bug was structural and was always going to surface the first time someone invoked `/evolve-loop` from a project that wasn't the evolve-loop repo itself.

## The fix (v8.18.0)

Two distinct roots resolved by a new shared helper, sourced into every kernel script:

| Variable | Source | Purpose |
|---|---|---|
| `EVOLVE_PLUGIN_ROOT` | `dirname/..` of the script (the install location) | Read-only resources: `scripts/`, `agents/`, `.evolve/profiles/`, `skills/` |
| `EVOLVE_PROJECT_ROOT` | env override → git toplevel of cwd → `$PWD` | Writable: `state.json`, `ledger.jsonl`, `runs/`, `cycle-state.json`, `instincts/` |

`scripts/resolve-roots.sh` is the single source of truth for this resolution. Every kernel script sources it as the first non-comment line after `set -uo pipefail`. The helper is idempotent — sourcing twice is a no-op.

Scripts updated:

- `scripts/run-cycle.sh`
- `scripts/evolve-loop-dispatch.sh`
- `scripts/cycle-state.sh`
- `scripts/subagent-run.sh`
- `scripts/phase-gate.sh`
- `scripts/guards/phase-gate-precondition.sh`
- `scripts/record-failure-to-state.sh`
- `scripts/merge-lesson-into-state.sh`
- `scripts/ship.sh`

Orchestrator prompt updated:

- `agents/evolve-orchestrator.md` — every relative `bash scripts/...` invocation became `bash $EVOLVE_PLUGIN_ROOT/scripts/...`. The context block now injects both `pluginRoot` and `projectRoot` so the orchestrator subagent has the values available without recomputing them.

Tests updated:

- `scripts/resolve-roots-test.sh` (new) — six test groups covering dev mode, plugin mode separation, env override, no-git fallback, idempotency, writability.
- `scripts/merge-lesson-test.sh` — switched to `EVOLVE_PROJECT_ROOT="$ROOT"` env override per invocation (cleaner than copying scripts).
- `scripts/ship-integration-test.sh` — also copies `resolve-roots.sh` into the fake repo since `ship.sh` sources it.

## What this does NOT do

- Does not change the trust kernel. SHA256 ledger binding, sandbox-exec wrapping, phase gates, role gates — all unchanged.
- Does not change adapter behavior. Cross-CLI (Gemini/Codex) hybrid pattern is preserved.
- Does not relax any permission. Reads stay reads, writes stay writes; only the path resolution is corrected.

## Verification

```
bash scripts/resolve-roots-test.sh           # 9/9 PASS, no regression
bash scripts/run-all-regression-tests.sh     # 24/27 PASS, 3 pre-existing failures unrelated
```

Pre-existing failures (verified by git-stash bisect — same failure count on `main` before this change):

- `scripts/orchestrator-sandbox-coverage-test.sh` — drift in `.evolve/release-journal` classification
- `scripts/release-pipeline-test.sh` — 8 unrelated failures
- `scripts/release/preflight-test.sh` — 2 unrelated failures
