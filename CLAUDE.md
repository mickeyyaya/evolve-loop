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
3. **Run `phase-gate.sh` at every transition.** `legacy/scripts/lifecycle/phase-gate.sh` is non-negotiable in bypass mode.
4. **Never fabricate cycles.** Every cycle number must correspond to real Scout → Build → Audit → Ship → Learn execution. Claiming a number without doing the work is a CRITICAL violation.
5. **Phase agents MUST be invoked via `legacy/scripts/dispatch/subagent-run.sh`.** In-process `Agent` is denied by `orchestrator.json:disallowed_tools` AND by `phase-gate-precondition.sh` whenever `cycle-state.json` exists. No bypass. The runner enforces per-agent CLI permission profiles in `.evolve/profiles/`, generates a challenge token, and writes a tamper-evident ledger entry.
6. **OS-level sandboxing wraps every claude subprocess.** When `EVOLVE_SANDBOX=1`, runner wraps `claude -p` in `sandbox-exec` (macOS) / `bwrap` (Linux). `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` is REQUIRED for nested-claude (auto-enabled by `archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh` via `detect-nested-claude.sh` when the bash rollback hatch fires). Auditor/Evaluator profiles run `read_only_repo: true`.
7. **Mutation testing pre-flight on every eval.** `gate_discover_to_build` runs `legacy/scripts/verification/mutate-eval.sh` against each new eval. Kill rate < 0.8 flags the eval as tautological. Promotion path: WARN-only → fail-gate after one verification cycle.
8. **Adversarial Auditor mode is default-on.** Runner prepends "ADVERSARIAL AUDIT MODE" framing requiring positive evidence for PASS. Auditor defaults to Opus (different family from Builder's Sonnet) to break same-model-judge sycophancy. Disable only with `ADVERSARIAL_AUDIT=0`.

**The rule is: maximum velocity, zero shortcuts.** Go fast by being efficient, not by skipping steps.

**Runtime constraints.** Per-cycle git worktrees provisioned by `run-cycle.sh` (recorded in `cycle-state.json:active_worktree`). Orchestrator and phase agents MAY NOT call `git worktree add/remove` (denied by profiles). Failure-adapter (`legacy/scripts/failure/failure-adapter.sh`) computes deterministic PROCEED/RETRY/BLOCK from `state.json:failedApproaches[]` — orchestrator follows verbatim. Ledger hash-chain via `prev_hash` + `entry_seq`; verify with `bash legacy/scripts/observability/verify-ledger-chain.sh`. `ship.sh` advances `state.json:lastCycleNumber` after successful ship.

## Verification before claiming done

These three are AGENTS.md Rules 8 + 12 applied to Claude Code workflows. Apply ALL before reporting complete:

1. **Probe before declaring a CLI unavailable.** Run `bash legacy/scripts/utility/probe-tool.sh <tool>` (canonical helper) or `command -v <tool> || which <tool> || ls /usr/local/bin/<tool> ~/.local/bin/<tool>`. List what you checked. The `/insights` audit caught "no `gws` command" claims when `gws` was at `~/.local/bin/`.
2. **Read actual exports.** Before importing/calling from `module X`, `Read` X and list real exports. Caught the Builder-against-imagined-API failure mode that required full rewrites.
3. **Run tests and report counts.** Format: `bash legacy/scripts/<suite>.sh — N/N PASS, no regression`. "Tests pass" without numbers is unverified. If no test infra exists, say so explicitly rather than skipping the check.

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
| Build planner | `EVOLVE_BUILD_PLANNER` | 1 (advisory, v12.3+) | Opt C build-planner phase (wired v10.19; cycle-103). `0` = shadow (no-op); `1` = advisory (build-plan.md produced). 3-cycle rollout: shadow→advisory→enforce. Revert: `EVOLVE_BUILD_PLANNER=0`. See [docs/architecture/adr/0019-build-planner-phase.md](docs/architecture/adr/0019-build-planner-phase.md). |
| Reflection journal | `EVOLVE_REFLECTION_JOURNAL` | `1` (advisory v10.20) | Per-phase Reflection Journal: each phase agent appends `## Reflection` section + `<phase>-reflection.yaml` sidecar. Reflector (every cycle) + Retrospective/Memo consume YAML. v10.20 = advisory (missing YAML → WARN); v10.21 = enforce (phase-gate blocks missing YAML). Revert: `EVOLVE_REFLECTION_JOURNAL=0`. See [docs/architecture/reflection-journal.md](docs/architecture/reflection-journal.md) and [docs/architecture/learn-phase.md](docs/architecture/learn-phase.md). |
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
| Stall detector | `EVOLVE_OBSERVER_ENFORCE` | `1` (default-on since v10.18.0) | When `1`, phase-observer replaces phase-watchdog as cycle-scope stall detector. `0` opts back to watchdog (deprecated, emits WARN). See [docs/architecture/phase-observer.md](docs/architecture/phase-observer.md). |
| Stall threshold | `EVOLVE_OBSERVER_STALL_S` | `600` | Primary stall threshold for phase-observer. Bridges from `EVOLVE_INACTIVITY_THRESHOLD_S` (DEPRECATED). |
| Soft-stall nudge | `EVOLVE_OBSERVER_NUDGE_S` | `0` (opt-in) | When `>0` and `NUDGE_S <= idle < STALL_S`, phase-observer appends ONE `nudge` envelope to the agent inbox (`<workspace>/.bridge-inbox/<agent>.ndjson`) before the hard SIGTERM — a draining `*-tmux` driver injects it to prompt the agent to summarize+continue or finalize. Body overridable via `EVOLVE_OBSERVER_NUDGE_BODY`. Inert for headless drivers (no drain loop). See [docs/architecture/adr/0023-live-injection-and-launch-rules.md](docs/architecture/adr/0023-live-injection-and-launch-rules.md). |
| Live command injection | `evolve bridge send` (CLI) | n/a | Facet A: queue a live command for an already-running `*-tmux` agent — `evolve bridge send --workspace=DIR --agent=NAME [--kind=command\|interrupt\|nudge\|system_rule] [--source=cli] <body>`. The driver drains its inbox from the artifact-wait poll loop and injects: `command`/`nudge`/`system_rule` are idle-gated (wait for the prompt marker), `interrupt` sends ESC first. Cursor seeks to EOF on launch, so only post-launch sends are delivered (no backlog replay on named-session resume). Go-tmux-only. See ADR-0023. |
| Launch-time system prompt | `EVOLVE_SYSTEM_PROMPT` / `EVOLVE_<AGENT>_SYSTEM_PROMPT` | unset | Facet B: per-agent system-level rules prepended as a `## Rules` block to the prompt at launch (CLI-agnostic, at the same seam as the interactive-policy block). Precedence: `EVOLVE_<AGENT>_SYSTEM_PROMPT` > `EVOLVE_SYSTEM_PROMPT` > profile `system_prompt` > read(profile `system_prompt_file`) > none. Order in the prompt: rules < policy < body. See ADR-0023. |
<!-- Shared Agent Values: researchCache schema uses research_fingerprint + research-cache.sh utility -->
| Research cache | `EVOLVE_RESEARCH_CACHE_ENABLED` | `0` (opt-in) | Adds `state.json:researchCache.entries[<sha>]`. Cache hit when fingerprint matches + `cycle - produced_at_cycle ≤ EVOLVE_RESEARCH_CACHE_MAX_AGE` (default 5). |
| Research tool | `EVOLVE_ALLOW_DEEP_RESEARCH` | `0` | When `1`, lifts per-agent quota cap; records `deep_overrides` counter. Does not disable hook telemetry. See [docs/architecture/research-tool.md](docs/architecture/research-tool.md). |
| Research tool | `EVOLVE_RESEARCH_QUOTA_SOFT` | *(planned)* | Soft quota: allows over-quota web calls but emits WARN in guards.log. Not yet implemented in `research-quota-gate.sh` as of cycle-89. |
| Research tool | `EVOLVE_RESEARCH_HOOK_DISABLED` | `0` | When `1`, `research-quota-gate.sh` is a no-op but counters still increment (telemetry-only mode). |
| Research tool | `EVOLVE_KB_SEARCH_PATHS` | `knowledge-base/research/:.evolve/instincts/lessons/:docs/research/` | Colon-separated root paths for `legacy/scripts/research/kb-search.sh`. |
| Subscription proxy | `EVOLVE_ANTHROPIC_BASE_URL` | unset | When set, exported as `ANTHROPIC_BASE_URL` before every `claude -p` invocation. **Proxy-agnostic**: target must speak Anthropic Messages API format (`POST /v1/messages`). **Not required for subscription auth** — `claude -p` reads `~/.claude.json` OAuth credentials natively. Use only for custom endpoints (LiteLLM, corporate gateway). Example: `export EVOLVE_ANTHROPIC_BASE_URL=http://127.0.0.1:4000/v1` (LiteLLM default). Note: `hermes proxy start` does not exist in hermes-agent; do not use it. See `knowledge-base/research/hermes-agent-proxy-integration.md`. Run `bash legacy/scripts/utility/doctor-subscription-auth.sh` to detect your active auth mode. |
| Incremental intent | `EVOLVE_INTENT_DELTA` | `0` (opt-in) | When `1`, `intent-batch-resolve.sh` runs before the intent phase to compute `INTENT_MODE=full\|delta` by comparing `GOAL_HASH` against `state.json:currentBatch.goalHash`. In delta mode, the intent persona emits `intent-delta.md` (patch) or `[intent-unchanged]` instead of a full `intent.md`; `gate_intent_to_research` accepts both. Requires `EVOLVE_REQUIRE_INTENT=1`. See [docs/architecture/incremental-intent.md](docs/architecture/incremental-intent.md). |
| Antigravity adapter — require-full | `EVOLVE_AGY_REQUIRE_FULL` | `0` | When `1`, `agy.sh` exits 99 if neither `agy` nor `claude` binary is found (same opt-in as `EVOLVE_GEMINI_REQUIRE_FULL`). Default: graceful degradation. |
| Antigravity adapter — binary override | `EVOLVE_AGY_BINARY` | unset | Testing seam: override the `agy` binary path. Honoured only when `EVOLVE_TESTING=1`. Used by ACS predicates to force NATIVE/DEGRADED mode in tests. |
| Go-vs-bash dispatch | `EVOLVE_USE_LEGACY_BASH` | `0` (Go primary, v11.0.0+) | When `0` (default), the Go binary at `EVOLVE_GO_BIN` (or `go/bin/evolve`) is the primary entrypoint for `evolve cycle run`, `evolve loop`, `evolve doctor`, `evolve guard`, `evolve ledger`, `evolve acs`. When `1`, `evolve loop` exec's to `archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh` (archived in v11.5.0 M6 via `git mv`; full history preserved). Bash dispatcher + `run-cycle.sh` + `resume-cycle.sh` live under `archive/legacy/scripts/dispatch/` from v11.5.0+. See [docs/migration-from-bash.md](docs/migration-from-bash.md). |
| Go binary path override | `EVOLVE_GO_BIN` | unset | Path to the Go binary. When unset, dispatchers look for `<project_root>/go/bin/evolve`. Set to the cross-compiled artifact path (e.g. `<HOME>/.local/bin/evolve-darwin-arm64`) for system-wide install. |
| Bash script location | n/a | `legacy/scripts/` (v11.1.0+) | Bash scripts physically live at `legacy/scripts/`. `legacy/scripts/` is a backcompat symlink. All `legacy/scripts/...` references in hooks, agents, docs continue to resolve. New code SHOULD reference `legacy/scripts/...` directly. |
| Native Go ship | `EVOLVE_NATIVE_SHIP` | `1` (Go native, v11.3.0+) | When unset or `1`, the ship phase runs the native Go implementation (`go/internal/phases/ship/`): self-SHA TOFU, audit-binding, EGPS gate, atomic commit+ff-merge+push, gh release. When `0`, falls back to shelling out to `legacy/scripts/lifecycle/ship.sh` (rollback hatch through v11.x). Parity contract: the 23-test matrix in `go/internal/phases/ship/native_test.go` mirrors `legacy/scripts/tests/ship-integration-test.sh` byte-for-byte on commit-message footers + exit codes + ledger semantics. CLI surface: `evolve ship [--class cycle\|manual\|release\|trivial] [--dry-run] "<msg>"`. See [docs/v12.0.0-roadmap.md](docs/v12.0.0-roadmap.md) for the v11.x→v12 sequencing. |
| Interactive policy (global) | `EVOLVE_INTERACTIVE_POLICY` | `recommended_or_first` (default-on, v12.1+) | Bridge prepends a deterministic policy block to every phase prompt so subagents self-resolve `AskUserQuestion`/y/N prompts without hanging the autonomous loop. Values: `recommended_or_first` (pick option labeled "(Recommended)" or first), `escalate` (no block, fail loudly on ambiguity — legacy posture), `auto_yes` (binary y/N → yes; multi-option falls back to recommended-or-first). Unknown values silently default to `recommended_or_first` to protect autonomy from typos. Block is < 200 tokens and deterministic to preserve Claude prompt-prefix cache. Implemented in `go/internal/adapters/bridge/bridge.go:injectPolicyPrefix`. |
| Interactive policy (per-agent) | `EVOLVE_<AGENT>_INTERACTIVE_POLICY` | unset | Per-agent override of the global policy. `<AGENT>` is the agent name uppercased with hyphens → underscores: `scout` → `EVOLVE_SCOUT_INTERACTIVE_POLICY`, `tdd-engineer` → `EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY`. Precedence: `req.Env` per-agent > process env per-agent > `req.Env` global > process env global > default. Use to pin one phase (e.g., `EVOLVE_AUDITOR_INTERACTIVE_POLICY=escalate`) while every other phase stays on autonomy. |
| Plan-mode dispatch (per-phase) | `EVOLVE_<PHASE>_PERMISSION_MODE` | unset (v12.1+) | Sets the Claude Code `--permission-mode` flag for a single phase invocation. Values per Claude Code: `plan`, `acceptEdits`, `default`. Wired into all 6 phase runners (intent, scout, triage, tdd, build, audit) via the shared `go/internal/phaseflags/` package — call site is one line: `phaseflags.For("<phase>").Resolve(profileDir, req.Env)`. Precedence: `req.Env[EVOLVE_<PHASE>_PERMISSION_MODE]` > `profile.permission_mode` (JSON field in `.evolve/profiles/<phase>.json`) > unset. profile.extra_flags pass through unconditionally. The combined slice is appended to `BridgeRequest.ExtraFlags`; only Claude drivers honor `--permission-mode` (claude-p, claude-tmux). Codex/Gemini drivers refuse it gracefully (`plan_mode_degraded` ledger entry — wired in a later v12.1 slice). |
| Stdout filter | `EVOLVE_STDOUT_FILTER` | `on` (v12.2+) | After each phase the runner writes `<phase>-stdout.clean.txt` next to the raw `<phase>-stdout.log` — a human-readable trace (~8–20% the size of raw) with `stream_event.*` redraw noise dropped, hook envelopes compressed to one line, tool_result payloads middle-truncated, and `assistant.thinking.signature` blobs stripped. Raw file is BYTE-FOR-BYTE UNCHANGED so cyclecost.go and phaseobserver continue to read raw without churn. Set `off` to skip. Filter is best-effort; failures WARN to stderr and never block the phase. Implementation: `go/internal/logfilter/`, wired in `go/internal/phases/runner/runner.go` after `bridge.Launch()` succeeds via the `stdoutFilterFn` test seam. Per-line dispatch (try JSON parse → known event type → JSON rules; else plain-text rules with consecutive-dedup + spinner/box drop) unifies stream-json and tmux-scrollback in one pipeline. Measured retention on cycle-106 build-stdout.log: 200 KB → 15 KB (7.6%). See `knowledge-base/research/stdout-noise-profile-2026-05-26.md`. |
| Cycle outcome label | `FinalVerdict` (orchestrator output field) | `PASS\|FAIL\|WARN\|SHIPPED_VIA_BUILD\|SKIPPED_AUDIT_ADVISORY\|SKIPPED_UNKNOWN` (v12.2+) | The dispatcher's per-cycle JSON summary used to emit a bare `SKIPPED` whenever the formal ship phase didn't run, conflating "build phase committed inline" with "audit blocked" with "no signal." Now disambiguated: HEAD moved during cycle → `SHIPPED_VIA_BUILD`; retro decision text contains `would-have-blocked` → `SKIPPED_AUDIT_ADVISORY`; else bare `SKIPPED_UNKNOWN` (loud label invites inspection). PASS/FAIL/WARN pass through unchanged. Implementation: `Orchestrator.finalizeOutcome` in `go/internal/core/orchestrator.go`; `gitHEAD` seam injectable for tests. Source incident: cycle-107 (2026-05-26) shipped commit `4519cea` but reported bare `SKIPPED`. |
| Ledger manual entries | `cycle_label` field on `LedgerEntry` | unset | Manual operator entries (release audits, ad-hoc events outside the integer cycle sequence) MUST use `"cycle": 0` plus `"cycle_label": "<semantic>"` (e.g., `"manual-release-v10.16.0"`). Reserved: numeric `cycle` field is exclusively for the integer cycle sequence. Legacy entries that wrote `"cycle": "<string>"` (which broke the dispatcher's `ledger iter` walker with `json: cannot unmarshal string into Go struct field LedgerEntry.cycle of type int`) are absorbed by a defensive unmarshaler in `go/internal/core/ports.go` — string cycles land in `CycleLabel` with `Cycle=0`. The on-disk ledger is never rewritten (would break the SHA256 hash chain). |
| Unfinished-cycle guard | `EVOLVE_FORCE_FRESH` | `0` (guard on) | A fresh `evolve loop` run REFUSES (exit 2, `stop_reason=unfinished_cycle`) when an unfinished cycle is detected — `cycle-state.json` ahead of `lastCycleNumber`, OR unreadable (truncated by a SIGKILL'd dispatcher) — and prints the resume∥reset fork. This prevents silently clobbering a stuck cycle's history. Resolve by `evolve loop --resume` (continue) or `evolve cycle reset` (seal it — archives workspace + cycle-state snapshot + `reset-manifest.json` to `.evolve/runs/cycle-<N>.reset-<ts>/`, never deleted; advances `lastCycleNumber`; auditable ledger entry). `EVOLVE_FORCE_FRESH=1` restores the prior silent-clobber (history NOT sealed). Implementation: `unfinishedCycle` + the guard in `go/cmd/evolve/cmd_loop.go`; `core.SealCycle` in `go/internal/core/reset.go`. |
| Commit gate | `EVOLVE_BYPASS_COMMIT_GATE` | `0` (gate on, v13.0.0+) | `evolve ship --class manual` is the single chokepoint for interactive commits (bare `git commit` is ship-gate-denied), and it requires a fresh commit-gate review attestation: `.commit-gate/attestation.json` whose `tree_state_sha == sha256(git diff HEAD)`. Produce it with `/commit` (code-simplifier + code-reviewer + language reviewer + lint + targeted tests). Missing/malformed/stale (reviewed tree ≠ staged tree) → ship refuses with an `IntegrityError`. `EVOLVE_BYPASS_COMMIT_GATE=1` skips the check (routine use is a CLAUDE.md violation); `--dry-run` is exempt (commits nothing). Implementation: `go/internal/phases/ship/commitgate.go`; portable bash runner `commit-gate/commit-gate-runner.sh`. See [skills/commit/SKILL.md](skills/commit/SKILL.md). |
| Dynamic phase routing | `EVOLVE_DYNAMIC_ROUTING` | `off` (static state machine drives, v13.0.0/PR #4) | Rollout stage for the Go routing kernel (`go/internal/router`, `core.RoutingProposer`): `off`/`0` = legacy static sequence drives (byte-identical to pre-routing); `shadow` = router computes + logs the would-have-routed plan, static still drives; `advisory` = router drives the optional-phase surface, mandatory spine stays static; `enforce` = router drives, kernel-clamped to the integrity floor. Companion flags: `EVOLVE_ROUTING_MODE` (`llm` default = LLM proposes + kernel clamps / `static` = triggers + spine only, no LLM), `EVOLVE_MANDATORY_PHASES` (CSV, default `scout,build,audit,ship`; omitting `audit`/`ship` emits a `weak-spine` WARN), `EVOLVE_CONDITIONAL_MANDATORY` (`phase:expr`, default `tdd:cycle_size!=trivial`), `EVOLVE_MAX_OPTIONAL_INSERTIONS` (int, default `4`), `EVOLVE_USE_PHASE_REGISTRY` (`0` to ignore `docs/architecture/phase-registry.json`). Precedence: env > registry file > built-in default. First live advisory cycle was cycle-108. See [docs/architecture/dynamic-phase-routing.md](docs/architecture/dynamic-phase-routing.md) and ADR-0024 (proposed PhaseAdvisor evolution). |

> **Operator commands (read-only):**
>
> - `evolve guard list-audit-fails [--evolve-dir DIR] [--json]` — enumerates `state.json:failedApproaches[]` entries with `classification=code-audit-fail` that are still within their 30-day retention window. Surfaces the count + identity of the audit failures the retro-phase failure-adapter has been tallying. Pure read; never mutates state. Use `--json` for scripted triage.
> - `evolve eval diversity-check <evalsDir> [slug]` — suite-level adversarial-diversity check (v13.0.0+): flags an eval suite that is positive-only by counting how many evals carry negative + edge cases. Exit `0` PASS, `1` WARN, `2` HALT (cohesive suite, zero negative cases), `10` bad args. Companions: `evolve eval quality-check <eval.md>` (Level-0 single-file tautology detection) and `evolve eval verify <eval.md> <workspace>` (independent eval re-execution). Read-only.

> **Operator commands (write a verdict file):**
>
> - `evolve acs suite --cycle N [--root .] [--evolve-dir .evolve]` — runs the bash EGPS predicate suite (cycle-N predicates + regression-suite + red-team) and writes `<evolve-dir>/runs/cycle-N/acs-verdict.json`; exit `2` when any predicate is RED, `0` when all green. Host-side deterministic replacement for the deleted `run-acs-suite.sh` (ADR-0025). `evolve acs run --cycle N <pkg>` runs `go test -json` on a package into the same verdict file.

> **Session cost isolation (v10.8.0+):** `claude -p` subagent invocations bill to the OAuth session that launched the dispatcher (the parent Claude Code session), not the batch budget meter. To isolate `/evolve-loop` costs from your prior session context, run `/clear` before starting a new evolve-loop batch. The batch meter (`state.json:currentBatch.cycleAccruedCostUSD`) tracks per-cycle accumulation but cannot capture OAuth session charges.

## Ship classes (`evolve ship --class <X>` native; or `legacy/scripts/lifecycle/ship.sh --class <X>` with `EVOLVE_NATIVE_SHIP=0`)

| Class | Use case | Verification |
|---|---|---|
| `cycle` (default) | `/evolve-loop` cycle commits | Full audit-binding: recent PASS, SHA match, HEAD/tree bound, `acs-verdict.json` red_count==0 |
| `manual` | Operator-driven manual commits | Skips audit-binding, but requires a fresh commit-gate review attestation (`.commit-gate/attestation.json`; bypass with `EVOLVE_BYPASS_COMMIT_GATE=1`); interactive y/N. CI mode: `EVOLVE_SHIP_AUTO_CONFIRM=1`. |
| `release` | `legacy/scripts/release-pipeline.sh` only | Skips audit (version-bump.sh mutates files post-audit); logs RELEASE class loudly |

Bare `git push origin main` is denied by ship-gate (v8.13.0+). `EVOLVE_BYPASS_SHIP_VERIFY=1` is a permanent compatibility bridge but emits deprecation WARN — prefer `--class manual`.

## Publishing releases

"publish" ≠ "push". Use the self-healing pipeline:

```bash
bash legacy/scripts/release-pipeline.sh X.Y.Z              # full publish
bash legacy/scripts/release-pipeline.sh X.Y.Z --dry-run    # simulate
bash legacy/scripts/release-pipeline.sh X.Y.Z --skip-tests # hot-fix (CI-pre-verified)
```

Pipeline lifecycle: pre-flight → version bump → auto-changelog (conventional commits) → consistency check → atomic ship via `ship.sh` → marketplace propagation polling (5 min) → cache refresh → auto-rollback on post-push failure.

Auto-bumped version markers: `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `skills/evolve-loop/SKILL.md` (heading), `README.md`, `CHANGELOG.md`. `legacy/scripts/utility/release.sh` is a standalone consistency verifier.

Full vocabulary (push, tag, release, propagate, publish, ship): [docs/guides/publishing-releases.md](docs/guides/publishing-releases.md).

## References

- [AGENTS.md](AGENTS.md) — cross-CLI invariants + 12 Core Agent Rules
- [docs/architecture/portable-core.md](docs/architecture/portable-core.md) — minimal-core file list for vendoring evolve-loop into another project
- [docs/operations/release-archive.md](docs/operations/release-archive.md) — version-specific narrative (v8.13 – v10.5)
- [docs/architecture/](docs/architecture/) — per-feature design docs (egps-v10, checkpoint-resume, tri-layer, sequential-write-discipline, platform-compatibility, phase-tracker, …)
- [docs/operations/release-notes/](docs/operations/release-notes/index.md) — per-version index
- [CHANGELOG.md](CHANGELOG.md) — full chronology
