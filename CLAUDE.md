# Project Instructions (Claude Code)

> **Read [AGENTS.md](AGENTS.md) first** — it carries the cross-CLI invariants and the 12 Core Agent Rules that bind every agent in this repo. This file is the Claude Code-specific overlay: runtime hooks, plugin specifics, and current-behavior env-var table.
>
> Release notes: [CHANGELOG.md](CHANGELOG.md). Version-specific narrative archived in [docs/operations/release-archive.md](docs/operations/release-archive.md).

## Session conventions

- **Confirm direction first.** Multi-step or multi-cycle work — produce a 3-bullet plan (approach, key decision, success criteria) and wait for approval. Single-cycle bug fixes, file-path-specified tasks, and tasks invoked from an approved plan are exempt. The "25 cycles in the wrong direction" failure mode this prevents is documented in `/insights`.
- **Output discipline.** Default to summaries with `file:line` references; do not paste large code blocks. For investigations producing >300 lines of findings, write to a markdown file and link rather than dumping into chat.
- **Long-running jobs.** After launching any background dispatcher/`/evolve-loop`, verify health (exit codes, log tail) before declaring it running. Multi-cycle runs MUST checkpoint after each cycle so `--resume` can recover from any cycle. Surface failures immediately.

## Autonomous execution (bypass-permissions / yolo / auto-approve)

When operating in autonomous mode, AGENTS.md Rule 4 ("stop and ask") is overridden. All 8 items below remain **mandatory** — bypass means "don't ask the user," not "skip integrity checks."

1. **Continue all cycles without pausing.** Never output "should I continue?" between cycles. Complete every requested cycle end-to-end.
2. **Run the FULL pipeline every cycle.** Launch Scout, Builder, Auditor for every cycle. No inline edits skipping agents, no batch-claiming multiple cycles. Each cycle must produce `scout-report.md`, `build-report.md`, `audit-report.md`.
3. **Run `phase-gate.sh` at every transition.** `scripts/lifecycle/phase-gate.sh` is non-negotiable in bypass mode.
4. **Never fabricate cycles.** Every cycle number must correspond to real Scout → Build → Audit → Ship → Learn execution. Claiming a number without doing the work is a CRITICAL violation.
5. **Phase agents MUST be invoked via `scripts/dispatch/subagent-run.sh`.** In-process `Agent` is denied by `orchestrator.json:disallowed_tools` AND by `phase-gate-precondition.sh` whenever `cycle-state.json` exists. No bypass. The runner enforces per-agent CLI permission profiles in `.evolve/profiles/`, generates a challenge token, and writes a tamper-evident ledger entry.
6. **OS-level sandboxing wraps every claude subprocess.** When `EVOLVE_SANDBOX=1`, runner wraps `claude -p` in `sandbox-exec` (macOS) / `bwrap` (Linux). `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` is REQUIRED for nested-claude (auto-enabled by `scripts/dispatch/evolve-loop-dispatch.sh` via `detect-nested-claude.sh`). Auditor/Evaluator profiles run `read_only_repo: true`.
7. **Mutation testing pre-flight on every eval.** `gate_discover_to_build` runs `scripts/verification/mutate-eval.sh` against each new eval. Kill rate < 0.8 flags the eval as tautological. Promotion path: WARN-only → fail-gate after one verification cycle.
8. **Adversarial Auditor mode is default-on.** Runner prepends "ADVERSARIAL AUDIT MODE" framing requiring positive evidence for PASS. Auditor defaults to Opus (different family from Builder's Sonnet) to break same-model-judge sycophancy. Disable only with `ADVERSARIAL_AUDIT=0`.

**The rule is: maximum velocity, zero shortcuts.** Go fast by being efficient, not by skipping steps.

**Runtime constraints.** Per-cycle git worktrees provisioned by `run-cycle.sh` (recorded in `cycle-state.json:active_worktree`). Orchestrator and phase agents MAY NOT call `git worktree add/remove` (denied by profiles). Failure-adapter (`scripts/failure/failure-adapter.sh`) computes deterministic PROCEED/RETRY/BLOCK from `state.json:failedApproaches[]` — orchestrator follows verbatim. Ledger hash-chain via `prev_hash` + `entry_seq`; verify with `bash scripts/observability/verify-ledger-chain.sh`. `ship.sh` advances `state.json:lastCycleNumber` after successful ship.

## Verification before claiming done

These three are AGENTS.md Rules 8 + 12 applied to Claude Code workflows. Apply ALL before reporting complete:

1. **Probe before declaring a CLI unavailable.** Run `bash scripts/utility/probe-tool.sh <tool>` (canonical helper) or `command -v <tool> || which <tool> || ls /usr/local/bin/<tool> ~/.local/bin/<tool>`. List what you checked. The `/insights` audit caught "no `gws` command" claims when `gws` was at `~/.local/bin/`.
2. **Read actual exports.** Before importing/calling from `module X`, `Read` X and list real exports. Caught the Builder-against-imagined-API failure mode that required full rewrites.
3. **Run tests and report counts.** Format: `bash scripts/<suite>.sh — N/N PASS, no regression`. "Tests pass" without numbers is unverified. If no test infra exists, say so explicitly rather than skipping the check.

## Shell & environment conventions

Shell scripts target **bash 3.2** (macOS default). Banned/required:

| Status | Pattern | Reason |
|---|---|---|
| Banned | `declare -A` | bash 4+ |
| Banned | `mapfile` / `readarray` | bash 4+ |
| Banned | `${var^^}` / `${var,,}` | bash 4+ |
| Banned | `sed -i ''` | BSD-incompatible — use `> "${file}.tmp.$$" && mv` |
| Banned | `date -d` | GNU-only — use `date -u -j -f` on macOS; fallback chain `gdate || date -d || date -j -f` |
| Required | `set -uo pipefail` | NOT `set -e` for orchestrator scripts capturing sub-script exit codes |
| Required | Atomic writes via `mv` of `${file}.tmp.$$` | Crash-safe |
| Required | `git diff HEAD` for tree-state SHA | Match audit-binding model (untracked files don't count) |

`skills/<name>/` is canonical; `.agents/skills/<name>/` are symlinks → `../../skills/<name>/` for cross-CLI auto-discovery. Git tracks content at the canonical path.

**SSE / streaming endpoints**: disable middleware buffering, add explicit timeouts, provide a cancel-UI button. Don't rely on browser-side timeout alone.

## /evolve-loop task priority

When selecting cycle tasks:

1. **New features** — top priority
2. **Bug fixes** — second
3. **Security issues** — last

## Current behavior (env-var reference)

Defaults reflect production posture as of v10.8.0. Detail docs linked per row.

| Subsystem | Env var | Default | Effect / reference |
|---|---|---|---|
| EGPS gate | `acs-verdict.json` | enforced | Cycle ships only if `red_count == 0`. WARN level removed in v10.0.0. See [docs/architecture/egps-v10.md](docs/architecture/egps-v10.md). |
| EGPS Tester | `EVOLVE_TEST_PHASE_ENABLED` | `1` (default-on) | When `1`, TDD-Engineer writes behavioral predicates before Builder; Tester validates after. When `0`, Builder writes own predicates (v10.1 fallback, degrades quality). Flipped default in cycle-86 (predicate-quality Layer 4). |
| Phase-B observability | `EVOLVE_TRACKER_ENABLED` | `0` (opt-in) | When `1`, replays NDJSON via `tracker-writer.sh` post-phase. v10.6+ candidate flips to default-on. See [docs/architecture/phase-tracker.md](docs/architecture/phase-tracker.md). |
| Triage phase | `EVOLVE_TRIAGE_DISABLE` | `0` (Triage on) | Runs Layer C Triage on every cycle. Soft WARN if skipped without override. |
| Auto-retrospective | `EVOLVE_DISABLE_AUTO_RETROSPECTIVE` | `0` (on) | FAIL/WARN cycles auto-run retrospective + merge-lesson. Cost ~$0.30-0.50/cycle. |
| Fan-out | `EVOLVE_FANOUT_ENABLED` | `0` (off) | Parallel sub-personas. Concurrency cap `EVOLVE_FANOUT_CONCURRENCY=2`. Per-worker cap `EVOLVE_FANOUT_PER_WORKER_BUDGET_USD=0.20`. Builder/Intent/Orchestrator/TDD excluded by `parallel_eligible` (single-writer invariant). See [docs/architecture/sequential-write-discipline.md](docs/architecture/sequential-write-discipline.md). |
| Plan review | `EVOLVE_PLAN_REVIEW` | `0` (off) | Four-lens review (CEO/Eng/Design/Security) between discover and build. Verdict gates via `gate_plan_review_to_tdd`. |
| Strict audit | `EVOLVE_STRICT_AUDIT` | `0` (fluent) | WARN ships by default (v8.28.0+). Set `1` to block on WARN. (Note: WARN removed in v10.0.0 — applies to pre-EGPS bootstrap cycles only.) |
| Budget — single ceiling | `EVOLVE_MAX_BUDGET_USD` | `999999` | Per-invocation override. Highest priority over `budget_tiers` and profile defaults. |
| Budget — batch cap | `EVOLVE_BATCH_BUDGET_CAP` | `20.00` | Cumulative USD ceiling across cycles in a single dispatcher invocation. Trips with `DISPATCH_RC=4`. |
| Budget — user-stated | `--budget-usd N` CLI flag | unset | Primary stop condition. Exits with `stop_reason=budget`. |
| Builder cost guard | `EVOLVE_BUILDER_COST_THRESHOLD` | `2.00` | `gate_build_to_audit` appends audit defect on overrun. `EVOLVE_BUILDER_COST_GUARD_STRICT=1` for fail-fast. |
| Checkpoint trigger | `EVOLVE_CHECKPOINT_AT_PCT` | `95` | Pre-emptive checkpoint at cumulative cost %. `--resume` reads it. See [docs/architecture/checkpoint-resume.md](docs/architecture/checkpoint-resume.md). |
| Context autotrim | `EVOLVE_CONTEXT_AUTOTRIM` | `0` (opt-in) | Head-60%/tail-35% prompt trim above `EVOLVE_PROMPT_MAX_TOKENS=30000`. See [docs/architecture/context-window-control.md](docs/architecture/context-window-control.md). |
| Worktree base | `EVOLVE_WORKTREE_BASE` | auto | Resolved by `preflight-environment.sh`: `.evolve/worktrees/` > `$TMPDIR/evolve-loop/<hash>`. Never auto-enable `EVOLVE_SKIP_WORKTREE=1` (operator-only emergency hatch). |
| Inner sandbox | `EVOLVE_INNER_SANDBOX` | auto | `false` when nested-Claude detected; force-enable/disable with `1`/`0`. `EVOLVE_FORCE_INNER_SANDBOX` deprecated. |
<!-- Shared Agent Values: researchCache schema uses research_fingerprint + research-cache.sh utility -->
| Research cache | `EVOLVE_RESEARCH_CACHE_ENABLED` | `0` (opt-in) | Adds `state.json:researchCache.entries[<sha>]`. Cache hit when fingerprint matches + `cycle - produced_at_cycle ≤ EVOLVE_RESEARCH_CACHE_MAX_AGE` (default 5). |
| Subscription proxy | `EVOLVE_ANTHROPIC_BASE_URL` | unset | When set, exported as `ANTHROPIC_BASE_URL` before every `claude -p` invocation. **Proxy-agnostic**: target must speak Anthropic Messages API format (`POST /v1/messages`). **Not required for subscription auth** — `claude -p` reads `~/.claude.json` OAuth credentials natively. Use only for custom endpoints (LiteLLM, corporate gateway). Example: `export EVOLVE_ANTHROPIC_BASE_URL=http://127.0.0.1:4000/v1` (LiteLLM default). Note: `hermes proxy start` does not exist in hermes-agent; do not use it. See `knowledge-base/research/hermes-agent-proxy-integration.md`. Run `bash scripts/utility/doctor-subscription-auth.sh` to detect your active auth mode. |
| Incremental intent | `EVOLVE_INTENT_DELTA` | `0` (opt-in) | When `1`, `intent-batch-resolve.sh` runs before the intent phase to compute `INTENT_MODE=full\|delta` by comparing `GOAL_HASH` against `state.json:currentBatch.goalHash`. In delta mode, the intent persona emits `intent-delta.md` (patch) or `[intent-unchanged]` instead of a full `intent.md`; `gate_intent_to_research` accepts both. Requires `EVOLVE_REQUIRE_INTENT=1`. See [docs/architecture/incremental-intent.md](docs/architecture/incremental-intent.md). |

> **Session cost isolation (v10.8.0+):** `claude -p` subagent invocations bill to the OAuth session that launched the dispatcher (the parent Claude Code session), not the batch budget meter. To isolate `/evolve-loop` costs from your prior session context, run `/clear` before starting a new evolve-loop batch. The batch meter (`state.json:currentBatch.cycleAccruedCostUSD`) tracks per-cycle accumulation but cannot capture OAuth session charges.

## Ship classes (`scripts/lifecycle/ship.sh --class <X>`)

| Class | Use case | Verification |
|---|---|---|
| `cycle` (default) | `/evolve-loop` cycle commits | Full audit-binding: recent PASS, SHA match, HEAD/tree bound, `acs-verdict.json` red_count==0 |
| `manual` | Operator-driven manual commits | Skips audit; interactive y/N. CI mode: `EVOLVE_SHIP_AUTO_CONFIRM=1`. |
| `release` | `scripts/release-pipeline.sh` only | Skips audit (version-bump.sh mutates files post-audit); logs RELEASE class loudly |

Bare `git push origin main` is denied by ship-gate (v8.13.0+). `EVOLVE_BYPASS_SHIP_VERIFY=1` is a permanent compatibility bridge but emits deprecation WARN — prefer `--class manual`.

## Publishing releases

"publish" ≠ "push". Use the self-healing pipeline:

```bash
bash scripts/release-pipeline.sh X.Y.Z              # full publish
bash scripts/release-pipeline.sh X.Y.Z --dry-run    # simulate
bash scripts/release-pipeline.sh X.Y.Z --skip-tests # hot-fix (CI-pre-verified)
```

Pipeline lifecycle: pre-flight → version bump → auto-changelog (conventional commits) → consistency check → atomic ship via `ship.sh` → marketplace propagation polling (5 min) → cache refresh → auto-rollback on post-push failure.

Auto-bumped version markers: `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `skills/evolve-loop/SKILL.md` (heading), `README.md`, `CHANGELOG.md`. `scripts/utility/release.sh` is a standalone consistency verifier.

Full vocabulary (push, tag, release, propagate, publish, ship): [docs/guides/publishing-releases.md](docs/guides/publishing-releases.md).

## References

- [AGENTS.md](AGENTS.md) — cross-CLI invariants + 12 Core Agent Rules
- [docs/architecture/portable-core.md](docs/architecture/portable-core.md) — minimal-core file list for vendoring evolve-loop into another project
- [docs/operations/release-archive.md](docs/operations/release-archive.md) — version-specific narrative (v8.13 – v10.5)
- [docs/architecture/](docs/architecture/) — per-feature design docs (egps-v10, checkpoint-resume, tri-layer, sequential-write-discipline, platform-compatibility, phase-tracker, …)
- [docs/operations/release-notes/](docs/operations/release-notes/index.md) — per-version index
- [CHANGELOG.md](CHANGELOG.md) — full chronology
