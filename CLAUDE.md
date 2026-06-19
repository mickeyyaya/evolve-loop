# Project Instructions (Claude Code)

> **Read [AGENTS.md](AGENTS.md) first** — cross-CLI invariants + the 12 Core Agent Rules. This file is the Claude Code overlay (digest).
> **Full runtime detail — env-var table, operator commands, ship classes, publishing pipeline — lives in [docs/operations/runtime-reference.md](docs/operations/runtime-reference.md).** Read it before touching loop behavior, flags, gates, or releases. Release notes: [CHANGELOG.md](CHANGELOG.md).

## Session conventions

- **Confirm direction first**: multi-step/multi-cycle work needs a 3-bullet plan + approval. Single-cycle bug fixes, file-path-specified tasks, and approved-plan tasks are exempt.
- **Output discipline**: summaries with `file:line` refs; >300-line findings go to a markdown file, not chat.
- **Long-running jobs**: verify health after launch (exit codes, log tail); checkpoint every cycle so `--resume` works; surface failures immediately.

## Autonomous execution (bypass mode)

Bypass = "don't ask the user", NOT "skip integrity checks". Mandatory (full text in runtime-reference.md):

1. Continue all cycles without pausing; never ask "should I continue?".
2. FULL pipeline every cycle — real `scout-report.md` / `build-report.md` / `audit-report.md`.
3. Phase gate at every transition (Go orchestrator + `evolve guard phase`).
4. Never fabricate cycle numbers (CRITICAL violation).
5. Phase agents go through the native bridge (`evolve subagent run` / `evolve loop`); in-process `Agent` is denied.
6. OS sandboxing wraps subprocesses (`EVOLVE_SANDBOX=1`; EPERM fallback auto-enabled when nested).
7. Eval-quality pre-flight on every eval (`evolve eval quality-check`).
8. Adversarial Auditor default-on (Opus auditor vs Sonnet builder; `ADVERSARIAL_AUDIT=0` disables).

Maximum velocity, zero shortcuts. Worktrees are provisioned natively — agents may NOT call `git worktree`; follow failure-adapter verdicts (PROCEED/RETRY/BLOCK) verbatim; `evolve ledger verify` checks the chain.

## Verification before claiming done

1. Probe before declaring a CLI unavailable: `evolve doctor probe <tool>`; list what you checked.
2. Read actual exports before importing/calling from a module.
3. Run tests and report counts: `cd go && go test ./internal/<pkg>/... — N/N PASS, no regression`.

## Shell conventions

bash 3.2 target. Banned: `declare -A`, `mapfile`, `${var^^}`, `sed -i ''`, `date -d`. Required: `set -uo pipefail` (not `set -e`), atomic writes via `mv "${f}.tmp.$$"`, `git diff HEAD` for tree-state SHA. `skills/<name>/` is canonical; `.agents/skills/` are symlinks. Full table with reasons/portable alternatives → [runtime-reference.md](docs/operations/runtime-reference.md).

## /evolve-loop task priority

1. New features 2. Bug fixes 3. Security issues

## Critical runtime facts (full table → runtime-reference.md)

- Gates default-ON: `EVOLVE_EVAL_GATE=enforce`, `EVOLVE_CONTRACT_GATE=enforce` (retry defaults live in `.evolve/policy.json`), EGPS `red_count==0` to ship, `EVOLVE_TEST_PHASE_ENABLED=1`.
- Default execution = tmux-LLM drivers (`claude-tmux` etc.); headless `claude -p` is opt-in only. Claude OAuth detected from macOS Keychain.
- Commits: bare `git commit` / `git push origin main` are ship-gate-denied. Interactive commits: `/commit` → attestation → `evolve ship --class manual` (`--bypass-commit-gate` routine use is a violation). Cycle commits: `--class cycle` (full audit-binding). Releases: `evolve release X.Y.Z` — "publish" ≠ "push".
- Unfinished cycle → `evolve loop --resume` or `evolve cycle reset`; never routine `EVOLVE_FORCE_FRESH=1`.
- Routing: `EVOLVE_DYNAMIC_ROUTING=advisory` default (since 2026-06-06, retro steps 1-3 landed; `=off` is the static escape hatch); integrity floor `ship ⇒ build ∧ audit ∧ (tdd unless trivial)`; policy pins in `.evolve/policy.json` (`EVOLVE_POLICY_BYPASS` off). Swarm: stage=shadow (set via `.evolve/policy.json` `swarm.stage`).
- Observer auto-spawn defaults on via `.evolve/policy.json` `observer` settings (stall 600s, tmux liveness probe).
- Run `/clear` before a new evolve-loop batch (session cost isolation).

## References

- [docs/operations/runtime-reference.md](docs/operations/runtime-reference.md) — env-var table, operator commands, ship classes, publishing
- [docs/architecture/](docs/architecture/) — design docs; [control-flags.md](docs/architecture/control-flags.md) — all `EVOLVE_*` flags
- [CHANGELOG.md](CHANGELOG.md) · [release-notes/](docs/operations/release-notes/index.md)
