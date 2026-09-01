# Project Instructions (Claude Code)

> **Read [AGENTS.md](AGENTS.md) first** — cross-CLI invariants + the 12 Core Agent Rules. This file is the Claude Code overlay (digest).
> **Full runtime detail — env-var table, operator commands, ship classes, publishing pipeline — lives in [docs/operations/runtime-reference.md](docs/operations/runtime-reference.md).** Read it before touching loop behavior, flags, gates, or releases. Release notes: [CHANGELOG.md](CHANGELOG.md).
> **Operating policy (canonical, environment-independent): [docs/operations/operating-policy.md](docs/operations/operating-policy.md)** — pipeline issues are fixed console-first at maximum reasoning; routing is typed plumbing; salvage before requeue; wiring proofs mandatory. The repo, not session memory, is the source of truth for these rules.

## Automation Loop Guardrails

- When running the evolve loop or any batch/merge-train automation, evaluate output after EVERY cycle. If 2 consecutive cycles produce zero ships (0 merged PRs), STOP the loop immediately and root-cause the pipeline before running another wave.
- Never let a batch run more than 2 unproductive waves "to see if it self-corrects".
- ADR-0072 reconciliation: a zero-ship streak is SYSTEM-fail evidence — HALT + P0 applies. This does not conflict with "never stop the queue", which governs task-level failures.

## Status Reporting

When reporting on a batch or pipeline run, always include: cycles run, ships/lanes completed (e.g. 2/2), open PR numbers, failing CI job names with failure class, and the specific next action. Do not report "running" or "in progress" as a status without these numbers.

## Session conventions

- **Confirm direction first**: multi-step/multi-cycle work needs a 3-bullet plan + approval. Single-cycle bug fixes, file-path-specified tasks, and approved-plan tasks are exempt.
- **Output discipline**: summaries with `file:line` refs; >300-line findings go to a markdown file, not chat.
- **Long-running jobs**: verify health after launch (exit codes, log tail); checkpoint every cycle so `--resume` works; surface failures immediately.
- **Pre-commit review fleet**: change → code-simplifier → (**architecture-reviewer ∥ code-reviewer**, parallel, both read-only) → address findings → commit. architecture-reviewer ([.claude/agents/architecture-reviewer.md](.claude/agents/architecture-reviewer.md)) is blocking: any CRITICAL finding per the profile's rubric must be fixed before commit. Pure-docs diffs may skip the fleet.

## Autonomous execution (bypass mode)

Bypass = "don't ask the user", NOT "skip integrity checks". Mandatory (full text in runtime-reference.md):

1. Continue all cycles without pausing; never ask "should I continue?".
2. FULL pipeline every cycle — real `scout-report.md` / `build-report.md` / `audit-report.md`.
3. Phase order enforced at every transition by the Go orchestrator state machine (`go/internal/core`); `evolve guard phase` is a live PreToolUse deny of in-process `Agent`/`Task` dispatch while a cycle is active (rewired per ADR-0075; `go/internal/guards/phase.go`).
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

## CI Failures

- Classify every CI failure as flake vs. real defect before retrying. Retrying an unclassified red is not allowed.
- File a regression issue for any failure class seen 2+ times, and link it in the PR description.
- Ship gates: if a gate reports RED, verify the gate's own logic before assuming the code is broken (false-RED gates have shipped before).

## Go Project Conventions

After editing any Go file, run `gofmt -l .`, `go vet ./...`, and `go test -count=1 ./...` (from the `go/` module root) before opening a PR. **`-count=1` is required, not optional:** Go's test cache does not track file reads that escape the module root via `..`, and many tests read `.evolve/profiles/*.json` that way — so after a config-only edit a bare `go test ./...` serves a stale `ok (cached)` while the regression is live on disk (reproduced 2026-08-28). CI and `make test` already pass `-count=1`; local verification was the gap. Add table-driven edge-case tests for any bug that was root-caused during a pipeline stall.

## Shell conventions

bash 3.2 target. Banned: `declare -A`, `mapfile`, `${var^^}`, `sed -i ''`, `date -d`. Required: `set -uo pipefail` (not `set -e`), atomic writes via `mv "${f}.tmp.$$"`, `git diff HEAD` for tree-state SHA. `skills/<name>/` is canonical; `.agents/skills/` are symlinks. Full table with reasons/portable alternatives → [runtime-reference.md](docs/operations/runtime-reference.md).

## /evo:loop task priority

1. New features 2. Bug fixes 3. Security issues

## Critical runtime facts (full table → runtime-reference.md)

- Gates default-ON as **compiled Go defaults** (`internal/policy` + `internal/config`), surfaced when the policy block is absent: `eval_gate=enforce`, `contract_gate=enforce`, `repo_contract_gate=enforce` (ship-time repo-contract scanner pack, `internal/phases/ship/repocontract.go` — lane ships can no longer red main), EGPS `red_count==0` to ship, tdd phase enabled. `.evolve/policy.json` MAY override these via a `gates`/`workflow` block, but the checked-in file sets only floor/cli_health/catalog/acs/parallel_evaluate/pins/fleet/disposition/failure_disposition — it does not contain the gate keys (don't assume policy.json is the source of the defaults). Since 2026-08-10: `failure_disposition.stage=enforce` (escalation boundary LIVE, was shadow). `fleet.count=3` (restored from the August codex-quota reduction; verified live 2026-08-24 — 76 codex dispatches / 44% share over cycles 1530-1552, zero quota halts, gpt-5.6 sol/terra/luna tiers healthy; codex owns scout/build/coverage lanes while audit/tdd stay claude by the adversarial cross-family design).
- Boot self-heal: `boot.binary_refresh=auto` **compiled default** (`cmd_loop_boot_refresh.go`) — rebuild + re-exec when the running binary's build stamp is behind HEAD with a go/ delta; `off` only for deliberate old-binary pins (unknown words resolve to `auto`).
- Contract blocks: second consecutive contract-gate block escalates the re-dispatch CLI (soft overlay, `internal/core/contract_escalation.go`) — escalation before breaker demotion; salvage rungs are breaker-neutral. Recurring identical-fingerprint halt → `evolve loop --reset --fingerprint <fp>` (acked into `.evolve/resolved-fingerprints.json`).
- Default execution = tmux-LLM drivers (`claude-tmux` etc.); headless `claude -p` is opt-in only. Claude OAuth detected from macOS Keychain. 4 CLI families: claude, codex, agy (Antigravity), ollama.
- Commits: bare `git commit` / `git push origin main` are ship-gate-denied. Interactive commits: `/commit` → attestation → `evolve ship --class manual` (`--bypass-commit-gate` routine use is a violation). Cycle commits: `--class cycle` (full audit-binding). Releases: `evolve release X.Y.Z` — "publish" ≠ "push".
- Unfinished cycle → `evolve loop --resume` or `evolve cycle reset`; `evolve loop --force-fresh` as last-resort escape hatch (history NOT sealed).
- Routing: `EVOLVE_DYNAMIC_ROUTING=advisory` default (since 2026-06-06, retro steps 1-3 landed; `=off` is the static escape hatch); integrity floor `ship ⇒ build ∧ audit ∧ (tdd unless trivial)`; policy pins live in `.evolve/policy.json` (`pins`; empty since 2026-08-14 — `setup apply --preset recommended` superseded the memo→agy/fast pin; policy bypass is off by default). Swarm: stage=shadow — a **compiled default**, overridable via `.evolve/policy.json` `swarm.stage` (not set in the checked-in file).
- Observer auto-spawn defaults on as a **compiled Go default** (`internal/policy`; stall 600s, tmux liveness probe), overridable via a `.evolve/policy.json` `observer` block (not set in the checked-in file).
- Run `/clear` before a new evolve-loop batch (session cost isolation).

## References

- [docs/operations/runtime-reference.md](docs/operations/runtime-reference.md) — env-var table, operator commands, ship classes, publishing
- [docs/architecture/](docs/architecture/) — design docs; [control-flags.md](docs/architecture/control-flags.md) — all `EVOLVE_*` flags
- [CHANGELOG.md](CHANGELOG.md) · [release-notes/](docs/operations/release-notes/index.md)
