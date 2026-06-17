# Control Flags Reference — `EVOLVE_*`

> **93+ distinct flags** as of 2026-05-27 (count approximate). See cluster annotations for consolidation targets in cycles 8–10.
> Canonical source — bash surface: `grep -rohE 'EVOLVE_[A-Z_]+' legacy/scripts/ agents/ skills/ | sort -u`.
> Go-native surface (NOT captured by the bash grep — e.g. the dynamic-routing family and `EVOLVE_BYPASS_COMMIT_GATE` live in `go/internal/`): `grep -rohE 'EVOLVE_[A-Z_]+' go/ | sort -u`.

## Status Key

| Status | Meaning |
|--------|---------|
| ACTIVE | Read in production code; do not remove without a deprecation window |
| DEPRECATED | Still honored via bridge; emits stderr WARN; remove in a future cycle |
| OVERLAPPING | Aliases an ACTIVE flag under a different name; consolidation target |
| DEAD | No production reader; safe to remove from docs |
| INTERNAL | Set by the runner for subprocess injection; not operator-facing |

> **The hand-maintained cluster tables below are a legacy annotation layer.** The authoritative flag status is the [Generated Flag Index](#generated-flag-index) (SSOT = `go/internal/flagregistry`, regenerated via `evolve flags generate`). When a cluster table's Status disagrees with the Generated Flag Index, **the Generated Flag Index wins.**

---

## Core Infrastructure (never consolidate)

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_PROJECT_ROOT` | ACTIVE | Writable project directory (dual-root pattern) |
| `EVOLVE_PLUGIN_ROOT` | ACTIVE | Read-only plugin install location |
| `EVOLVE_CYCLE_STATE_FILE` | DEAD | Override cycle-state.json path (test seam) — no reader on any surface (2026-06-11 inventory) |
| `EVOLVE_TESTING` | ACTIVE | Test harness mode — disables real CLI calls |
| `EVOLVE_RESOLVE_ROOTS_LOADED` | ACTIVE | Idempotency guard for resolve-roots.sh sourcing |
| `EVOLVE_FAILURE_CLASSIFICATIONS_LOADED` | ACTIVE | Idempotency guard for failure-classifications.sh |

## Sandbox Cluster

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_SANDBOX` | ACTIVE | Enable outer sandbox-exec/bwrap wrapper |
| `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` | ACTIVE | EPERM fallback for nested-Claude (Darwin 25.4+) |
| `EVOLVE_INNER_SANDBOX` | ACTIVE | Tri-state inner sandbox: `1`=force-enable, `0`=force-disable, unset=auto from environment.json |
| `EVOLVE_FORCE_INNER_SANDBOX` | DEPRECATED | Bridged to `EVOLVE_INNER_SANDBOX=1` (v8.60+); emits stderr WARN; removal target v8.61+ |

> **Cycle 8 delivered**: `EVOLVE_FORCE_INNER_SANDBOX` is now deprecated with a bridge in `claude.sh`.
> Use `EVOLVE_INNER_SANDBOX=1` for force-enable, `EVOLVE_INNER_SANDBOX=0` for force-disable.

## Budget Cluster

The token-budget **cost gates were removed**. The dollar-cost calculation was
unreliable across LLM models (tmux/subscription claude reports `$0`, gemini used
a hardcoded price table, ollama is free), so any decision keyed on cost behaved
differently per model. Cost is now **display-only telemetry** (`total_cost_usd`
in loop output, per-phase `cost_usd`). The flags below are accepted but ignored
(deprecated no-ops); use `--cycles N` to bound a run.

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_MAX_BUDGET_USD` | DEPRECATED (no-op) | Former per-subagent budget cap; adapters no longer enforce a cap |
| `EVOLVE_BUDGET_CAP` | DEPRECATED (no-op) | Former bridge to `EVOLVE_MAX_BUDGET_USD` |
| `EVOLVE_BUDGET_ENFORCE` | DEPRECATED (no-op) | Former opt-in to profile-resolved per-phase caps |
| `EVOLVE_BUDGET_MAX_CYCLES` | DEPRECATED (no-op) | `--budget-usd` no longer drives cycle count; use `--cycles N` |
| `EVOLVE_BATCH_BUDGET_CAP` | DEPRECATED (no-op) | Former cumulative batch budget ceiling |
| `EVOLVE_BATCH_BUDGET_DISABLE` | DEPRECATED (no-op) | Former batch-budget tripwire disable |
| `EVOLVE_BUILDER_COST_THRESHOLD` | DEPRECATED (no-op) | Former builder cost-overrun guard threshold |
| `EVOLVE_BUILDER_COST_GUARD_STRICT` | DEPRECATED (no-op) | Former builder cost-overrun hard-fail switch |
| `EVOLVE_CHECKPOINT_WARN_AT_PCT` / `EVOLVE_CHECKPOINT_AT_PCT` | DEPRECATED (no-op) | Former cost-percentage checkpoint thresholds |
| `EVOLVE_PHASE_COST_CEILING` | DEPRECATED (no-op) | Former cyclehealth per-phase cost-ceiling anomaly |
| `EVOLVE_FANOUT_PER_WORKER_BUDGET_USD` | DEPRECATED (no-op) | Former per-worker cost cap; fanout no longer reads or injects it |
| `EVOLVE_BUILD_PLANNER` | ACTIVE (advisory; default on) | Opt C build-planner phase (NOT a cost flag). `1` = advisory (build-plan.md produced); `0` = opt-out. See ADR-0019. |

> **Token-budget cost gates removed**: the dollar-cost calculation and every gate
> that decided off it (loop stop, phase FAIL, checkpoint-by-cost, optional-phase
> skip) were removed because cost is model-dependent and unreliable. The `budget`
> package, the gemini price table, and claude.sh budget-tier resolution are gone.
> The flags above remain accepted-but-ignored for backward compatibility.
>
> **Cycle 10 CLOSED**: Workflow Defaults cluster — `EVOLVE_STRICT_*` (2 flags) and `EVOLVE_DISPATCH_*` (2 policy flags; REPEAT_THRESHOLD excluded as numeric threshold) consolidated.
> `EVOLVE_STRICT_FAILURES` bridged to `EVOLVE_STRICT_AUDIT` (canonical). `EVOLVE_DISPATCH_VERIFY` + `EVOLVE_DISPATCH_STOP_ON_FAIL` bridged to `EVOLVE_DISPATCH_POLICY={off|verify|stop}` (canonical).
> Note: cycle-9 callout misstated the counts as "3 STRICT + 2 DISPATCH" — actual was 2 STRICT + 3 DISPATCH (REPEAT_THRESHOLD is a numeric threshold, not a policy switch).
>
> **Cycle 11 target**: `EVOLVE_REQUIRE_*` family audit — `EVOLVE_REQUIRE_INTENT` and `EVOLVE_REQUIRE_TEAM_CONTEXT` share "force phase on every cycle" semantics; investigate unified `EVOLVE_REQUIRED_PHASES` list flag. Lower priority (rarely set by operators); treat as `investigate` not `commit`.

## State File Cluster (cycle 7 consolidation)

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_STATE_FILE_OVERRIDE` | ACTIVE (canonical) | Override state.json path |
| `EVOLVE_STATE_OVERRIDE` | DEPRECATED | Alias for `EVOLVE_STATE_FILE_OVERRIDE`; emits stderr WARN |

## Bypass / Emergency Hatches

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_BYPASS_SHIP_VERIFY` | DEPRECATED | Bridged to `--class manual` since v8.25; emits deprecation WARN |
| `EVOLVE_BYPASS_SHIP_GATE` | ACTIVE | Emergency hatch: bypass ship-gate |
| `EVOLVE_BYPASS_PHASE_GATE` | ACTIVE | Emergency hatch: bypass phase-gate-precondition |
| `EVOLVE_BYPASS_ROLE_GATE` | ACTIVE | Emergency hatch: bypass role-gate |
| `EVOLVE_BYPASS_POSTEDIT_VALIDATE` | ACTIVE | Emergency hatch: bypass postedit validation |
| `EVOLVE_BYPASS_COMMIT_GATE` | ACTIVE (Go-native, v13.0.0+) | Emergency hatch: skip the `--class manual` commit-gate review attestation (`.commit-gate/attestation.json`). Routine use is a CLAUDE.md violation. `--dry-run` is exempt by construction. Reader: `go/internal/phases/ship/commitgate.go` |

## Triage Cluster (cycle 7 trim)

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_TRIAGE_DISABLE` | ACTIVE | Opt-out of triage default-on (v8.59+) |
| `EVOLVE_TRIAGE_TOP_N` | ACTIVE | Override triage top_n selection count |
| `EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL` | ACTIVE (v10.19) | Opt A: auto-skip Triage when 3 conditions hold (≤1 scout task AND empty carryoverTodos AND predicate-dependency-check.sh exit 0). Default on (=1); opt-out with =0. Writes a stub `triage-decision.{md,json}` with `auto_skip: true` so downstream phases see consistent inputs. |
| `EVOLVE_TRIAGE_ENABLED` | DEAD | v8.56–v8.58 opt-in; replaced by `EVOLVE_TRIAGE_DISABLE`; removed from docs |

## Fan-out Cluster (intentionally separate — do not consolidate per-phase flags)

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_FANOUT_ENABLED` | ACTIVE | Master switch for fan-out |
| `EVOLVE_FANOUT_SCOUT` | DEAD | Enable fan-out for scout phase — no reader on any surface (2026-06-11 inventory) |
| `EVOLVE_FANOUT_AUDITOR` | ACTIVE (wired v10.19) | Enable fan-out for auditor phase (orchestrator picks `dispatch-parallel auditor`; reads `auditor.json:parallel_subtasks[]`) |
| `EVOLVE_FANOUT_RETROSPECTIVE` | DEAD | Enable fan-out for retrospective phase — no reader on any surface (2026-06-11 inventory) |
| `EVOLVE_FANOUT_CONCURRENCY` | ACTIVE | Max parallel workers in flight (default 2) |
| `EVOLVE_FANOUT_TIMEOUT` | ACTIVE | Per-worker timeout in fanout |
| `EVOLVE_FANOUT_CANCEL_ON_CONSENSUS` | ACTIVE | Cancel remaining workers on K-agreement |
| `EVOLVE_FANOUT_CACHE_PREFIX` | ACTIVE | Write shared cache-prefix.md for siblings |
| `EVOLVE_FANOUT_CACHE_PREFIX_FILE` | ACTIVE | Path for cache-prefix.md |
| `EVOLVE_FANOUT_TRACK_WORKERS` | ACTIVE | Track active fanout worker PIDs |
| `EVOLVE_FANOUT_TEST_EXECUTOR` | ACTIVE | Test seam: override fanout worker command |
| `EVOLVE_FANOUT_CONSENSUS_K` | ACTIVE | Consensus threshold K |
| `EVOLVE_FANOUT_CONSENSUS_POLL_S` | ACTIVE | Consensus poll interval |

> Per-phase flags (`_SCOUT`/`_AUDITOR`/`_RETROSPECTIVE`) are intentionally separate to allow
> gradual role-by-role rollout. Do not consolidate into a string-list flag.

## Platform / CLI Hybrid

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_PLATFORM` | ACTIVE | Override platform detection |
| `EVOLVE_GEMINI_CLAUDE_PATH` | ACTIVE | Gemini hybrid: claude binary path |
| `EVOLVE_GEMINI_REQUIRE_FULL` | ACTIVE | Require Gemini full-mode (not degraded) |
| `EVOLVE_CODEX_CLAUDE_PATH` | ACTIVE | Codex hybrid: claude binary path |
| `EVOLVE_CODEX_REQUIRE_FULL` | ACTIVE | Require Codex full-mode |
| `EVOLVE_ALLOW_INTERACTIVE_FALLBACK` | ACTIVE | Opt-in to interactive-mode fallback |
| `EVOLVE_FORCE_BARE` | ACTIVE | Force bare API mode (no subscription) |

## Worktree / Workspace

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_WORKTREE_BASE` | ACTIVE | Per-cycle worktree base path |
| `EVOLVE_SKIP_WORKTREE` | ACTIVE | Emergency hatch: skip per-cycle worktree isolation |
| `EVOLVE_DRY_RUN_PROVISION_WORKTREE` | ACTIVE | Dry-run worktree provisioning |
| `EVOLVE_PROFILE_WORKTREE_AWARE` | ACTIVE | Mark profile as worktree-aware |

## Readiness Gate (pre-batch)

> Deterministic host-side gate run by `evolve loop` BEFORE the first cycle (Go: `go/internal/looppreflight`, wired in `cmd_loop.go` after the unfinished-cycle guard). On a Halt it aborts with `stop_reason=preflight_failed`, rc=2, cycle=0, and persists `.evolve/loop-preflight.json`. Catches the cycle-258 `ExitREPLBootTimeout` at batch start instead of mid-cycle.

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_SKIP_PREFLIGHT` | ACTIVE | Emergency hatch: skip the whole readiness gate (no checks, no boot) |
| `EVOLVE_SKIP_PREFLIGHT_BOOT` | ACTIVE | Run the cheap checks (structure/CLI/host) but skip the real bridge-boot probe — CI/offline (bridge-boot downgrades Halt→Warn) |
| `EVOLVE_CLI_HEALTH` | ACTIVE (default on) | The one dial for the CLI-health bench layer (cycle-283: a quota-walled codex re-burned its boot on every dispatch all night because nothing remembered the wall). `0` disables ALL of it: the runner's bench-writer (exit-85 + classified `rate_limit` escalation → bench the CLI FAMILY in `.evolve/cli-health.json`, `benched_until` from the wall's own reset hint else a strike-doubled cooldown), the dispatch-chain demotion (benched families start at their fallback; bench is advice — all-benched dispatches least-recently-benched with a loud WARN; policy pins bypass entirely), the loop's per-cycle canary (one `bridge.LiveSmokeTest` per EXPIRED bench: recovered → cleared, walled again → strikes+1), and the advisor's environmental "CLI health" prompt section. Preflight's `cli-health` check (WARN-only) and `evolve doctor live <driver>` (the probe that can SEE a quota wall — boot smoke cannot, walls appear only after work is submitted) remain readable surfaces. |

## Workflow Defaults

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_STRICT_AUDIT` | ACTIVE (canonical) | WARN→FAIL promotion in ship.sh + failure-adapter blocking (v8.35+); single severity gate |
| `EVOLVE_STRICT_FAILURES` | DEPRECATED | Bridged to `EVOLVE_STRICT_AUDIT`; emits stderr WARN; removal target v8.61+ |
| `EVOLVE_TASK_MODE` | ACTIVE | Profile tier selector (default/research/deep) |
| `EVOLVE_REQUIRE_INTENT` | ACTIVE | Force intent phase on every cycle |
| `EVOLVE_REQUIRE_TEAM_CONTEXT` | ACTIVE | Require team context before builder |
| `EVOLVE_PLAN_REVIEW` | ACTIVE | Enable Sprint 2 plan-review phase (opt-in) |
| `EVOLVE_DISABLE_AUTO_RETROSPECTIVE` | DEPRECATED | Superseded by `policy.json:failure_floor` (ADR-0039) which wins when both set; honored one more release with a `deprecated-flag` WARN; removal target next release |
| `EVOLVE_DISPATCH_POLICY` | ACTIVE (canonical) | Dispatch verification policy: `off` (skip check) / `verify` (default) / `stop` (fail-fast) |
| `EVOLVE_DISPATCH_STOP_ON_FAIL` | DEPRECATED | Bridged to `EVOLVE_DISPATCH_POLICY=stop`; emits stderr WARN; removal target v8.61+ |
| `EVOLVE_DISPATCH_VERIFY` | DEPRECATED | Bridged to `EVOLVE_DISPATCH_POLICY=off` (when `=0`); emits stderr WARN; removal target v8.61+ |
| `EVOLVE_DISPATCH_REPEAT_THRESHOLD` | ACTIVE | Threshold for repeat-cycle detection |
| `EVOLVE_AUTO_PRUNE` | ACTIVE | Enable auto-prune of expired state entries |
| `EVOLVE_STRATEGY` | ACTIVE | Cycle strategy override |
| `EVOLVE_SHIP_AUTO_CONFIRM` | ACTIVE | CI mode: skip interactive y/N in ship.sh |
| `EVOLVE_SHIP_RELEASE_NOTES` | ACTIVE | Create GitHub release on ship |
| `EVOLVE_DIFF_COMPLEXITY_DISABLE` | ACTIVE | Disable diff-complexity check in auditor |
| `EVOLVE_CONSENSUS_AUDIT` | ACTIVE | Enable consensus-dispatch for auditor |
| `EVOLVE_AUDITOR_TIER_OVERRIDE` | ACTIVE | Override auditor model tier |

## Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off)

> Read by the composition-root loader `go/internal/config/config.go` (the ONLY env site). Precedence: env > `docs/architecture/phase-registry.json` > built-in default. Distinct from the legacy bash PSMAS skip path (`EVOLVE_PSMAS_SKIP`). See [docs/architecture/dynamic-phase-routing.md](dynamic-phase-routing.md) and ADR-0024 (proposed PhaseAdvisor evolution).

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_DYNAMIC_ROUTING` | ACTIVE (default `advisory` since 2026-06-06, registry-pinned; was `off`) | Rollout stage: `off`/`0` (static state machine drives — operator escape hatch) / `shadow` (router computes + logs, static drives) / `advisory` (router drives optional surface, spine static; DEFAULT) / `enforce` (router drives, kernel-clamped). Unknown value → `off` + WARN |
| `EVOLVE_ROUTING_MODE` | ACTIVE (default `llm`) | Routing brain: `llm`/`dynamic`/`dynamic-llm` (LLM proposes, kernel clamps) / `static`/`static-preset`/`preset` (triggers + spine only, no LLM). Unknown → `llm` + WARN |
| `EVOLVE_MANDATORY_PHASES` | ACTIVE (default `scout,build,audit,ship`) | CSV ordered mandatory spine. Omitting `audit` or `ship` emits a `weak-spine` WARN |
| `EVOLVE_CONDITIONAL_MANDATORY` | ACTIVE (default `tdd:cycle_size!=trivial`) | `phase:expr` conditional-mandatory predicate; op ∈ `!= == >= <= > <` |
| `EVOLVE_MAX_OPTIONAL_INSERTIONS` | ACTIVE (default `4`) | Cap on optional phases the router may insert |
| `EVOLVE_USE_PHASE_REGISTRY` | ACTIVE (default on) | Set `0` to skip reading `phase-registry.json` (built-in defaults only) |

## Phase Recovery (ADR-0044, Go-native — one dial for the whole program)

> The Unified Phase Recovery Protocol's single rollout dial. Read by the bridge subprocess directly from env (`go/internal/bridge/fatalpane.go`, same subprocess pattern as `EVOLVE_COMMIT_EVIDENCE`); later slices (C3/C4) add the orchestrator's `config.RolloutStages` view. Classification (the `recovery.FatalPaneDetector` registry) is always-on above `off`; only ACTING on a classification is staged. See [phase-recovery.md](phase-recovery.md) + ADR-0044.

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_PHASE_RECOVERY` | ACTIVE (default `shadow`, v18.3+) | The one dial for BOTH the ADR-0044 phase-recovery program (terminal-state recovery) AND the ADR-0045 corrective-interaction program (repair a live/just-completed phase through bounded interaction). ADR-0044: fatal-pane fast-fail at the stop-review checkpoint, the observer's chain-backed StallPolicy (subprocess injects it ONLY at explicit `enforce`), and the orchestrator's escalate→advise→promote hook (`config.RolloutStages.PhaseRecovery` view). ADR-0045 corrective ACTIONS ride the SAME stage: the graduated correction ladder (salvage→live-fix→re-dispatch), the pre-85 AskBroker rung, promoted-rule enforcement, and the live bidirectional channel (ADR-0037, folded in at I6 — `enforce` implies the channel; `EVOLVE_CHANNEL` deprecated, see below). **Telemetry is EXEMPT**: ADR-0045 I1 interaction telemetry (`<phase>-interactions.ndjson` + `interaction-summary.json`) records at EVERY stage including `off` — only ACTIONS gate. Stages: `off` (detectors not consulted, no corrective actions; byte-identical legacy — telemetry still records) / `shadow` (DEFAULT — detect + log the would-act for every rung, legacy behavior decides; byte-identical) / `enforce` (fatal-pane preempt with `stop`; salvage relocates a misplaced deliverable; the kernel answers a blocking question pre-85; promoted enforce-stage rules fire; exit 81 hands the phase to the runner's CLI fallback chain). Unknown value → `off` (a typo never enables a kill-path). A Busy pane is never preempted/interrupted regardless of stage |
| `EVOLVE_CHANNEL` | **DEPRECATED** (honored one release with a WARN; removed next release) | The live bidirectional channel (ADR-0037) was opt-in via `EVOLVE_CHANNEL=1`. ADR-0045 I6 folds it into `EVOLVE_PHASE_RECOVERY` (the no-flag-sprawl rule): the channel is now IMPLIED by the stage — `enforce` turns it on, `off`/`shadow` leave it off (byte-identical). An explicit `EVOLVE_CHANNEL` is still honored for one release (`1` → on, anything else → off) and emits a one-time deprecation WARN from both the bridge driver and the observer adapter. The resolution lives in one place — `bridge/channel.Enabled` + `channel.ResolveStage` — shared by the subprocess driver and the in-process observer so they can never drift. |

## Observability / Prompt Tuning

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_PROMPT_MAX_TOKENS` | ACTIVE | Soft prompt token cap for role-context-builder |
| `EVOLVE_PROMPT_BUDGET_ENFORCE` | ACTIVE | Make prompt-over-cap a hard error |
| `EVOLVE_CACHE_PREFIX_V2` | ACTIVE (default `1`) | v8.61.0 Campaign A — static-first / dynamic-last prompt layering. When `1`: (Cycle A1) subagent-run.sh emits a small INVOCATION CONTEXT user prompt; (Cycle A2) claude.sh attaches the role-specific bedrock from `build-invocation-context.sh` via `--append-system-prompt` AND adds `--exclude-dynamic-system-prompt-sections` so per-machine sections move out of the cached system layer. Promoted to default=1 in cycle 43 (v10.6+), overdue since v8.62 target. Set `EVOLVE_CACHE_PREFIX_V2=0` to revert to legacy v1 ordering. |
| `EVOLVE_CONTEXT_DIGEST` | ACTIVE (default `1`) | v8.62.0 Campaign B (Tier 2 — digest layer). When `1`, role-context-builder.sh: (B1) lazy-builds `cycle-digest.json` via `build-cycle-digest.sh`; (B2) replaces full intent.md cat with a compact `## Intent (compact)` block (intent_anchor + acceptance_criteria from digest) for scout/triage/plan-review/tdd/builder phases — auditor + retrospective still get the full file. Real-world reduction: scout 84%, triage 40%, builder 43%. Promoted to default=1 in cycle 24 (v9.4.0). Set `EVOLVE_CONTEXT_DIGEST=0` to revert to legacy full-file mode. |
| `EVOLVE_ANCHOR_EXTRACT` | ACTIVE (default `1`) | v8.63.0 Campaign C (Tier 3 — anchored artifacts). When `1`, role-context-builder.sh extracts only named `<!-- ANCHOR:<name> -->` regions from prior phase artifacts instead of `cat`-ing whole files. Persona templates (scout/builder/auditor/retrospective) emit anchor markers around output sections. Backwards-compat: pre-v8.63 artifacts without anchors fall back to full-file emission once per file (no duplication regression). Auditor reads `diff_summary`+`test_results` from build-report and `proposed_tasks`+`acceptance_criteria` from scout-report; triage reads `proposed_tasks` only. Promoted to default=1 in cycle 24 (v9.4.0). Set `EVOLVE_ANCHOR_EXTRACT=0` to revert to legacy full-file mode. |
| `EVOLVE_RUN_TIMEOUT` | ACTIVE | Per-subagent run timeout |
| `EVOLVE_INSTINCT_SUMMARY_CAP` | ACTIVE | Max instinct summaries in state.json |
| `EVOLVE_CARRYOVER_TODO_MAX_UNPICKED` | ACTIVE | Carryover todos threshold |
| `EVOLVE_RELEASE_REQUIRE_PREFLIGHT` | ACTIVE | Force release preflight gate |
| `EVOLVE_REINVOKE_CMD` | ACTIVE | Stored reinvoke command for rate-limit recovery |
| `EVOLVE_MARKETPLACE_DIR` | ACTIVE | Override marketplace dir (test/release seam) |

## Override / Test Seams

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_LEDGER_OVERRIDE` | ACTIVE | Override ledger.jsonl path |
| `EVOLVE_PROFILES_DIR_OVERRIDE` | ACTIVE | Override profiles dir path |
| `EVOLVE_PROFILE_OVERRIDE` | ACTIVE | Override pre-built profile path |
| `EVOLVE_DIR` | ACTIVE | Derived `.evolve/` path in phase-gate.sh (internal) |

## Dead Flags (remove from docs; no production reader)

| Flag | Evidence | Action |
|------|---------|--------|
| `EVOLVE_TRIAGE_ENABLED` | Comment-only; production uses `EVOLVE_TRIAGE_DISABLE` | Removed from CLAUDE.md in cycle 7 |
| `EVOLVE_DIR_OVERRIDE` | Test-only conditional; no production reader | Leave in test; document as test-only |
| `EVOLVE_PROJECT_ROOT_OVERRIDE` | 1 occurrence, no reader | Document as dead |
| `EVOLVE_REPO_ROOT_OVERRIDE` | 1 occurrence, no reader | Document as dead |

## Internal (subprocess injection — not operator-facing)

| Flag | Purpose |
|------|---------|
| `EVOLVE_FANOUT_CYCLE` | Internal env passed to fanout worker subprocess |
| `EVOLVE_FANOUT_PARENT_AGENT` | Internal env passed to fanout worker subprocess |
| `EVOLVE_FANOUT_WORKER_NAME` | Internal env passed to fanout worker subprocess |
| `EVOLVE_FANOUT_WORKER_ARTIFACT` | Internal env passed to fanout worker subprocess |
| `EVOLVE_FANOUT_WORKER_TOKEN` | Internal env passed to fanout worker subprocess |
| `EVOLVE_FANOUT_WORKSPACE` | Internal env passed to fanout worker subprocess |
| `EVOLVE_DISPATCH_DEPTH` | Bridge-recursion depth; set on each fan-out worker command (parent+1), read at the `subagent run` / `dispatch-parallel` chokepoint to enforce the recursion cap (max 3). Absent ⇒ depth 0 (top-level). |
| `EVOLVE_PROJECT_WRITABLE` | Set by resolve-roots.sh after verification |

---

## Consolidation Roadmap

| Cycle | Cluster | Action |
|-------|---------|--------|
| 7 (done) | State-file | Deprecated `EVOLVE_STATE_OVERRIDE` → `EVOLVE_STATE_FILE_OVERRIDE` |
| 8 (done) | Sandbox | Deprecated `EVOLVE_FORCE_INNER_SANDBOX` → `EVOLVE_INNER_SANDBOX=1` bridge (v8.60) |
| 9 (done) | Budget | Deprecated `EVOLVE_BUDGET_CAP` → `EVOLVE_MAX_BUDGET_USD` bridge (v8.60); added builder cost-overrun guard |
| 10 (done) | Workflow Defaults | Deprecated `EVOLVE_STRICT_FAILURES` → `EVOLVE_STRICT_AUDIT`; deprecated `EVOLVE_DISPATCH_VERIFY` + `EVOLVE_DISPATCH_STOP_ON_FAIL` → `EVOLVE_DISPATCH_POLICY={off\|verify\|stop}` (v8.60) |
| 11 | Require Phases | Investigate `EVOLVE_REQUIRE_INTENT` + `EVOLVE_REQUIRE_TEAM_CONTEXT` → unified `EVOLVE_REQUIRED_PHASES` list flag |

<!-- GENERATED:flag-index BEGIN — do not edit by hand; run `evolve flags generate` -->

## Generated Flag Index

Complete flag index — generated from `go/internal/flagregistry` (SSOT). Edit the registry, then run `evolve flags generate`; do not edit this table by hand.

| Flag | Status | Kind | Default | Cluster | Purpose |
|------|--------|------|---------|---------|----------|
| `EVOLVE_ACS_GO_TIMEOUT_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ACS_PREDICATE_TIMEOUT_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ADAPTERS_DIR_OVERRIDE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_AGY_REQUIRE_FULL` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_ALLOW_DEEP_RESEARCH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ALLOW_DOC_DELETE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ALLOW_INTERACTIVE_FALLBACK` | dead | — | — | Platform / CLI Hybrid | Opt-in to interactive-mode fallback [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_ANCHOR_EXTRACT` | dead | — | — | Observability / Prompt Tuning | v8.63.0 Campaign C (Tier 3 — anchored artifacts). When `1`, role-context-builder.sh extracts only named `<!-- ANCHOR:<name> -->` regions from prior phase artifacts instead of `cat`-ing whole files. Persona templates (scout/builder/auditor/retrospective) emit anchor markers around output sections. Backwards-compat: pre-v8.63 artifacts without anchors fall back to full-file emission once per file (no duplication regression). Auditor reads `diff_summary`+`test_results` from build-report and `proposed_tasks`+`acceptance_criteria` from scout-report; triage reads `proposed_tasks` only. Promoted to default=1 in cycle 24 (v9.4.0). Set `EVOLVE_ANCHOR_EXTRACT=0` to revert to legacy full-file mode. [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_ANTHROPIC_BASE_URL` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ARTIFACT_MAX_EXTENDS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ARTIFACT_TIMEOUT_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_AUDITOR_CLI` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_AUDITOR_INTERACTIVE_POLICY` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_AUDITOR_MODEL` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_AUDITOR_TIER_OVERRIDE` | active | — | — | Workflow Defaults | Override auditor model tier |
| `EVOLVE_AUDIT_INTERACTIVE_POLICY` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_AUDIT_MODEL` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_AUDIT_PERMISSION_MODE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_AUTO_PRUNE` | active | — | — | Workflow Defaults | Enable auto-prune of expired state entries |
| `EVOLVE_AUTO_RESUME_MAX_ATTEMPTS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BACKFILL_ENABLED` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BASH_PARITY` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BATCH_BUDGET_CAP` | dead | — | — | Budget Cluster | DEPRECATED no-op (token-budget cost gates removed; cost calculation was unreliable across LLM models). Accepted but ignored. |
| `EVOLVE_BATCH_BUDGET_DISABLE` | dead | — | — | Budget Cluster | DEPRECATED no-op (batch budget tripwire removed with the token-budget cost gates). Accepted but ignored. |
| `EVOLVE_BRIDGE_CATALOG_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BRIDGE_GO` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BRIDGE_INTEGRATION_LIVE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BRIDGE_LIVE_CLI` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BRIDGE_LIVE_CLI_INTERACTIVE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BRIDGE_LIVE_CLI_ROUNDTRIP` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BRIDGE_MANIFEST_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BRIDGE_PIDFILE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BRIDGE_RECIPE_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BUDGET_CAP` | dead | — | — | Budget Cluster | DEPRECATED no-op (deprecation bridge removed with the token-budget cost gates). Accepted but ignored. |
| `EVOLVE_BUDGET_ENFORCE` | dead | — | — | Budget Cluster | DEPRECATED no-op (budget enforcement removed with the token-budget cost gates). Accepted but ignored. |
| `EVOLVE_BUDGET_MAX_CYCLES` | dead | — | — | Budget Cluster | DEPRECATED no-op: --budget-usd no longer drives cycle count (token-budget removal). Use --cycles N. Accepted but ignored. |
| `EVOLVE_BUILD` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BUILDER_COST_GUARD_STRICT` | dead | — | — | Budget Cluster | DEPRECATED no-op (builder cost guard removed with the token-budget cost gates). Accepted but ignored. |
| `EVOLVE_BUILDER_COST_THRESHOLD` | dead | — | — | Budget Cluster | DEPRECATED no-op (builder cost guard removed with the token-budget cost gates). Accepted but ignored. |
| `EVOLVE_BUILDER_INTERACTIVE_POLICY` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BUILDER_ISOLATION_STRICT` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BUILDER_MODEL` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BUILDER_PERMISSION_MODE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BUILDER_REVIEW_SKILLS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BUILDER_REVIEW_THRESHOLD` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BUILDER_SELF_REVIEW` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BUILDER_WORKTREE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BUILD_CLI` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BUILD_MODEL` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BUILD_PERMISSION_MODE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BUILD_PLANNER` | active | — | — | Budget Cluster | Opt C build-planner phase. `1` = advisory (default; build-plan.md produced, Builder reads it as a sanity check); `0` = opt-out. Enforce mode in cycle-105 (Builder Step 3 removed). 3-cycle rollout: shadow→advisory→enforce. Revert: `EVOLVE_BUILD_PLANNER=0`. See ADR-0019. |
| `EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BUILD_PLANNER_MODEL` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BUILD_PLAN_INPUT` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BUILD_PLAN_OUTPUT` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BUILD_SYSTEM_PROMPT` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_BYPASS_COMMIT_GATE` | active | — | — | Bypass / Emergency Hatches | Emergency hatch: skip the `--class manual` commit-gate review attestation (`.commit-gate/attestation.json`). Routine use is a CLAUDE.md violation. `--dry-run` is exempt by construction. Reader: `go/internal/phases/ship/commitgate.go` |
| `EVOLVE_BYPASS_PHASE_GATE` | active | — | — | Bypass / Emergency Hatches | Emergency hatch: bypass phase-gate-precondition |
| `EVOLVE_BYPASS_POSTEDIT_VALIDATE` | active | — | — | Bypass / Emergency Hatches | Emergency hatch: bypass postedit validation |
| `EVOLVE_BYPASS_PREFIX_GATE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_BYPASS_ROLE_GATE` | active | — | — | Bypass / Emergency Hatches | Emergency hatch: bypass role-gate |
| `EVOLVE_BYPASS_SHIP_GATE` | active | — | — | Bypass / Emergency Hatches | Emergency hatch: bypass ship-gate |
| `EVOLVE_BYPASS_SHIP_VERIFY` | deprecated | — | — | Bypass / Emergency Hatches | Emits deprecation WARN. Replaced by `evolve ship --class manual`. |
| `EVOLVE_CACHE_PREFIX_V2` | active | — | — | Observability / Prompt Tuning | v8.61.0 Campaign A — static-first / dynamic-last prompt layering. When `1`: (Cycle A1) subagent-run.sh emits a small INVOCATION CONTEXT user prompt; (Cycle A2) claude.sh attaches the role-specific bedrock from `build-invocation-context.sh` via `--append-system-prompt` AND adds `--exclude-dynamic-system-prompt-sections` so per-machine sections move out of the cached system layer. Promoted to default=1 in cycle 43 (v10.6+), overdue since v8.62 target. Set `EVOLVE_CACHE_PREFIX_V2=0` to revert to legacy v1 ordering. |
| `EVOLVE_CARRYOVER_TODO_MAX_UNPICKED` | dead | — | — | Observability / Prompt Tuning | Carryover todos threshold [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_CHANNEL` | deprecated | — | — | Bridge / Channel | Folded into channel.Enabled (ADR-0045 I6); honored via bridge with WARN. Replaced by `EVOLVE_PHASE_RECOVERY dial family`. Remove in v18.x (one release after I6). |
| `EVOLVE_CHECKPOINT_AT_PCT` | dead | — | — | Budget Cluster | DEPRECATED no-op: the cost-percentage checkpoint trigger was removed with the token-budget cost gates. Accepted but ignored. |
| `EVOLVE_CHECKPOINT_DISABLE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CHECKPOINT_REASON` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CHECKPOINT_REQUEST` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CHECKPOINT_TRIGGERED` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_CHECKPOINT_WARN_AT_PCT` | dead | — | — | Budget Cluster | DEPRECATED no-op: the cost-percentage checkpoint WARN was removed with the token-budget cost gates. Accepted but ignored. |
| `EVOLVE_CLI` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CLI_HEALTH` | active | — | — | Readiness Gate (pre-batch) | The one dial for the CLI-health bench layer (cycle-283: a quota-walled codex re-burned its boot on every dispatch all night because nothing remembered the wall). `0` disables ALL of it: the runner's bench-writer (exit-85 + classified `rate_limit` escalation → bench the CLI FAMILY in `.evolve/cli-health.json`, `benched_until` from the wall's own reset hint else a strike-doubled cooldown), the dispatch-chain demotion (benched families start at their fallback; bench is advice — all-benched dispatches least-recently-benched with a loud WARN; policy pins bypass entirely), the loop's per-cycle canary (one `bridge.LiveSmokeTest` per EXPIRED bench: recovered → cleared, walled again → strikes+1), and the advisor's environmental "CLI health" prompt section. Preflight's `cli-health` check (WARN-only) and `evolve doctor live <driver>` (the probe that can SEE a quota wall — boot smoke cannot, walls appear only after work is submitted) remain readable surfaces. |
| `EVOLVE_CODEX_CLAUDE_PATH` | dead | — | — | Platform / CLI Hybrid | Codex hybrid: claude binary path [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_CODEX_CONFIG_PATH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CODEX_REQUIRE_FULL` | active | — | — | Platform / CLI Hybrid | Require Codex full-mode |
| `EVOLVE_CODEX_VERSION_PATH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_COMMIT_EVIDENCE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_COMPACT_PROMPTS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_COMPOSE_PHASES` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CONDITIONAL_MANDATORY` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | `phase:expr` conditional-mandatory predicate; op ∈ `!= == >= <= > <` |
| `EVOLVE_CONSENSUS_AUDIT` | active | — | — | Workflow Defaults | Enable consensus-dispatch for auditor |
| `EVOLVE_CONTEXT_DIGEST` | dead | — | — | Observability / Prompt Tuning | v8.62.0 Campaign B (Tier 2 — digest layer). When `1`, role-context-builder.sh: (B1) lazy-builds `cycle-digest.json` via `build-cycle-digest.sh`; (B2) replaces full intent.md cat with a compact `## Intent (compact)` block (intent_anchor + acceptance_criteria from digest) for scout/triage/plan-review/tdd/builder phases — auditor + retrospective still get the full file. Real-world reduction: scout 84%, triage 40%, builder 43%. Promoted to default=1 in cycle 24 (v9.4.0). Set `EVOLVE_CONTEXT_DIGEST=0` to revert to legacy full-file mode. [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_CONTEXT_MODE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_CONTRACT_CORRECTION_RETRIES` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CONTRACT_GATE` | active | enum | enforce | Gates | Deliverable-contract gate (ADR-0034): off\|shadow\|enforce, circuit breaker demotes after 3 consecutive blocks. |
| `EVOLVE_CYCLE_STATE_FILE` | dead | — | — | Core Infrastructure (never consolidate) | Override cycle-state.json path (test seam) [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_DIFF_COMPLEXITY_DISABLE` | active | — | — | Workflow Defaults | Disable diff-complexity check in auditor |
| `EVOLVE_DIR` | dead | — | — | Override / Test Seams | Derived `.evolve/` path in phase-gate.sh (internal) [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_DIR_OVERRIDE` | dead | — | — | Dead Flags (remove from docs; no production reader) | Leave in test; document as test-only [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_DISABLE_AUTO_RETROSPECTIVE` | deprecated | — | — | Workflow Defaults | Superseded by `policy.json:failure_floor` (ADR-0039) which wins when both set; honored one more release with a `deprecated-flag` WARN; removal target next release |
| `EVOLVE_DISABLE_WORKSPACE_GUARD` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_DISPATCH_DEPTH` | internal | — | — | — | Bridge-recursion depth (B2): set on each fan-out worker command (parent+1), read at the subagent run / dispatch-parallel chokepoint to enforce the recursion cap (max 3). Absent ⇒ 0 (top-level). |
| `EVOLVE_DISPATCH_LOG_TTL_DAYS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_DISPATCH_PLAN_LOG` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_DISPATCH_POLICY` | active | — | — | Workflow Defaults | Dispatch verification policy: `off` (skip check) / `verify` (default) / `stop` (fail-fast) |
| `EVOLVE_DISPATCH_REPEAT_THRESHOLD` | active | — | — | Workflow Defaults | Threshold for repeat-cycle detection |
| `EVOLVE_DISPATCH_STOP_ON_FAIL` | deprecated | — | — | Workflow Defaults | Bridged to `EVOLVE_DISPATCH_POLICY=stop`; emits stderr WARN; removal target v8.61+ |
| `EVOLVE_DISPATCH_VERIFY` | deprecated | — | — | Workflow Defaults | Bridged to `EVOLVE_DISPATCH_POLICY=off` (when `=0`); emits stderr WARN; removal target v8.61+ |
| `EVOLVE_DOCS_CONTRACT_STRICT` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_DRY_RUN_PROVISION_WORKTREE` | dead | — | — | Worktree / Workspace | Dry-run worktree provisioning [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_DYNAMIC_ROUTING` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Rollout stage: `off`/`0` (static state machine drives — operator escape hatch) / `shadow` (router computes + logs, static drives) / `advisory` (router drives optional surface, spine static; DEFAULT) / `enforce` (router drives, kernel-clamped). Unknown value → `off` + WARN |
| `EVOLVE_E2E_ADVISOR_MODEL_CLAUDE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_ADVISOR_MODEL_OLLAMA` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE_ADVISOR` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE_BUDGET_USD` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE_LLM` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE_MATRIX` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE_MODEL_OLLAMA` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE_RETRIES` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE_SMOKE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE_SMOKE_TIMEOUT_S` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE_SOAK` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE_TIMEOUT_S` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_LIVE_TMUX_TIMEOUT_S` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_TMUX_LIVE_TIMEOUT_S` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_E2E_TMUX_TIMEOUT_S` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_EVAL_GATE` | active | enum | enforce | Gates | Structural eval gates (internal/evalgate): off\|shadow\|enforce. Gate A scout eval materialization, Gate B tdd predicate quality, Gate C floor binding (R9.3). |
| `EVOLVE_FAILURE_CLASSIFICATIONS_LOADED` | dead | — | — | Core Infrastructure (never consolidate) | Idempotency guard for failure-classifications.sh [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_FANOUT_AUDITOR` | active | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Enable fan-out for auditor phase (orchestrator picks `dispatch-parallel auditor`; reads `auditor.json:parallel_subtasks[]`) |
| `EVOLVE_FANOUT_CACHE_PREFIX` | active | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Write shared cache-prefix.md for siblings |
| `EVOLVE_FANOUT_CACHE_PREFIX_FILE` | active | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Path for cache-prefix.md |
| `EVOLVE_FANOUT_CANCEL_ON_CONSENSUS` | active | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Cancel remaining workers on K-agreement |
| `EVOLVE_FANOUT_CONCURRENCY` | active | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Max parallel workers in flight (default 2) |
| `EVOLVE_FANOUT_CONSENSUS_K` | active | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Consensus threshold K |
| `EVOLVE_FANOUT_CONSENSUS_POLL_S` | active | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Consensus poll interval |
| `EVOLVE_FANOUT_CYCLE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_FANOUT_ENABLED` | active | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Master switch for fan-out |
| `EVOLVE_FANOUT_PARENT_AGENT` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_FANOUT_PER_WORKER_BUDGET_USD` | dead | — | — | Budget Cluster | DEPRECATED no-op (per-worker cost cap removed with the token-budget cost gates; fanout no longer reads or injects it). Accepted but ignored. |
| `EVOLVE_FANOUT_RETROSPECTIVE` | dead | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Enable fan-out for retrospective phase [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_FANOUT_SCOUT` | dead | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Enable fan-out for scout phase [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_FANOUT_TEST_EXECUTOR` | active | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Escape hatch: override the fanout worker command to bypass the LLM in test harnesses (read in production code, not test-only) |
| `EVOLVE_FANOUT_TIMEOUT` | active | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Per-worker timeout in fanout |
| `EVOLVE_FANOUT_TRACK_WORKERS` | active | — | — | Fan-out Cluster (intentionally separate — do not consolidate per-phase flags) | Track active fanout worker PIDs |
| `EVOLVE_FANOUT_WORKER_ARTIFACT` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_FANOUT_WORKER_NAME` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_FANOUT_WORKER_TOKEN` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_FANOUT_WORKSPACE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_FLEET` | active | bool | 0 | Fleet Cluster (Track C concurrency) | Fleet mode (CB.2+): bridges refuse the process-cwd fallback when no worktree is designated (typed ExitBadFlags, never CLI-fallback). Set by the `evolve fleet` supervisor (CE.2); single-driver runs leave it unset and keep the loud-WARN fallback. |
| `EVOLVE_FORCE_BARE` | dead | — | — | Platform / CLI Hybrid | Force bare API mode (no subscription) [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_FORCE_FRESH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_FORCE_INNER_SANDBOX` | dead | — | — | Sandbox Cluster | Bridged to `EVOLVE_INNER_SANDBOX=1` (v8.60+); emits stderr WARN; removal target v8.61+ [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_GC` | active | enum | off | GC / Retention | GC shadow stage (L3.4). off=disabled (default); shadow=discover+plan+log manifest to workspace without mutations; enforce=shadow+apply (opt-in; honors quarantine/ledger/live hard rules). |
| `EVOLVE_GEMINI_CLAUDE_PATH` | dead | — | — | Platform / CLI Hybrid | Gemini hybrid: claude binary path [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_GEMINI_REQUIRE_FULL` | dead | — | — | Platform / CLI Hybrid | Require Gemini full-mode (not degraded) [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_GO_BIN` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_GO_BIN_TEST` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_GUARDS_LOG` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_HANG_CLASSIFIER` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_INACTIVITY_DISABLE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_INACTIVITY_GRACE_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_INACTIVITY_POLL_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_INACTIVITY_THRESHOLD_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_INACTIVITY_WARN_PCT` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_INNER_SANDBOX` | dead | — | — | Sandbox Cluster | Tri-state inner sandbox: `1`=force-enable, `0`=force-disable, unset=auto from environment.json [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_INSTINCT_SUMMARY_CAP` | dead | — | — | Observability / Prompt Tuning | Max instinct summaries in state.json [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_INTENT_DELTA` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_INTENT_MODEL` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_INTENT_PERMISSION_MODE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_INTERACTIVE_POLICY` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_KB_SEARCH_PATHS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_LEDGER_OVERRIDE` | active | — | — | Override / Test Seams | Override ledger.jsonl path |
| `EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS` | active | int | 1 | Workflow Defaults | Consecutive verdict-FAIL cycles a batch absorbs before stopping (default 1 = stop on first FAIL). A PASS/SHIPPED resets the streak; the cap still halts a broken run. rc=3 when any FAIL was absorbed. |
| `EVOLVE_MANDATORY_PHASES` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | CSV ordered mandatory spine. Omitting `audit` or `ship` emits a `weak-spine` WARN |
| `EVOLVE_MARKETPLACE_DIR` | active | — | — | Observability / Prompt Tuning | Override marketplace dir (test/release seam) |
| `EVOLVE_MAX_BUDGET_USD` | dead | — | — | Budget Cluster | DEPRECATED no-op: the adapters no longer resolve or enforce a cost cap (token-budget cost gates removed); claude.sh always passes an unlimited --max-budget-usd sentinel. Accepted but ignored. |
| `EVOLVE_MAX_OPTIONAL_INSERTIONS` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Cap on optional phases the router may insert |
| `EVOLVE_MODELCATALOG_AUTOREFRESH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_MODELCATALOG_CLASSIFIER_CLI` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_MODEL_CATALOG_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_NAMELESS` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_NATIVE_SHIP` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_NO_EQUALS` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_OBSERVER_AUTOSPAWN` | active | bool | 1 | Observer | Auto-spawn the per-phase observer goroutine (ADR-0030); 0 opts out. |
| `EVOLVE_OBSERVER_ENABLED` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_OBSERVER_ENFORCE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_OBSERVER_EOF_GRACE_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_OBSERVER_NUDGE_BODY` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_OBSERVER_NUDGE_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_OBSERVER_POLL_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_OBSERVER_STALL_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_OBSERVER_TEST_KEY` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_OLLAMA_BASE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PASS_CONFIDENCE_THRESHOLD` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PERSONA_OVERRIDE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PHASE_BUILD_BIN` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PHASE_COST_CEILING` | dead | — | — | Budget Cluster | DEPRECATED no-op: the cyclehealth cost_envelope anomaly was removed with the token-budget cost gates. Accepted but ignored. |
| `EVOLVE_PHASE_IO` | active | — | — | Phase I/O (ADR-0050) | ADR-0050 Phase-3 unified-phase-I/O rollout dial. FULL off→shadow→advisory→enforce ladder (4-value, unlike the 3-value gate dials). off = dormant legacy dispatch, byte-identical (the rollback escape hatch); shadow = typed envelope assembled + compared against legacy disk reads (mismatch → ledger phaseio_shadow_mismatch); advisory = envelope populated + read alongside legacy (legacy still wins); enforce = the typed envelope is AUTHORITATIVE — phase readers consume it and the audit/ship verdict parse is sentinel-mandatory. DEFAULT enforce as of the 3.10 cutover (was off through 18.14.0); set EVOLVE_PHASE_IO=off to roll back. A typo falls back to off (fail-safe, never leaves the dial in an unintended state). |
| `EVOLVE_PHASE_LATENCY_CEILING` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PHASE_LATENCY_CEILING_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PHASE_MAX_ATTEMPTS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PHASE_RECOVERY` | active | — | — | Phase Recovery (ADR-0044, Go-native — one dial for the whole program) | The one dial for BOTH the ADR-0044 phase-recovery program (terminal-state recovery) AND the ADR-0045 corrective-interaction program (repair a live/just-completed phase through bounded interaction). ADR-0044: fatal-pane fast-fail at the stop-review checkpoint, the observer's chain-backed StallPolicy (subprocess injects it ONLY at explicit `enforce`), and the orchestrator's escalate→advise→promote hook (`config.RolloutStages.PhaseRecovery` view). ADR-0045 corrective ACTIONS ride the SAME stage: the graduated correction ladder (salvage→live-fix→re-dispatch), the pre-85 AskBroker rung, promoted-rule enforcement, and the live bidirectional channel (ADR-0037, folded in at I6 — `enforce` implies the channel; `EVOLVE_CHANNEL` deprecated, see below). **Telemetry is EXEMPT**: ADR-0045 I1 interaction telemetry (`<phase>-interactions.ndjson` + `interaction-summary.json`) records at EVERY stage including `off` — only ACTIONS gate. Stages: `off` (detectors not consulted, no corrective actions; byte-identical legacy — telemetry still records) / `shadow` (DEFAULT — detect + log the would-act for every rung, legacy behavior decides; byte-identical) / `enforce` (fatal-pane preempt with `stop`; salvage relocates a misplaced deliverable; the kernel answers a blocking question pre-85; promoted enforce-stage rules fire; exit 81 hands the phase to the runner's CLI fallback chain). Unknown value → `off` (a typo never enables a kill-path). A Busy pane is never preempted/interrupted regardless of stage |
| `EVOLVE_PHASE_ROOTS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PHASE_SCOUT_BIN` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_PLAN_REVIEW` | active | — | — | Workflow Defaults | Enable Sprint 2 plan-review phase (opt-in) |
| `EVOLVE_PLAN_REVIEWER_INTERACTIVE_POLICY` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_PLAN_REVIEWER_PERMISSION_MODE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_PLAN_WORKSPACE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PLATFORM` | active | — | — | Platform / CLI Hybrid | Override platform detection |
| `EVOLVE_PLUGIN_ROOT` | active | — | — | Core Infrastructure (never consolidate) | Read-only plugin install location |
| `EVOLVE_POLICY_BYPASS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PROFILES_DIR_OVERRIDE` | active | — | — | Override / Test Seams | Override profiles dir path |
| `EVOLVE_PROFILE_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PROFILE_OVERRIDE` | dead | — | — | Override / Test Seams | Override pre-built profile path [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_PROFILE_WORKTREE_AWARE` | dead | — | — | Worktree / Workspace | Mark profile as worktree-aware [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_PROJECT_ROOT` | active | — | — | Core Infrastructure (never consolidate) | Writable project directory (dual-root pattern) |
| `EVOLVE_PROMPTS_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PROMPT_BUDGET_ENFORCE` | dead | — | — | Observability / Prompt Tuning | Make prompt-over-cap a hard error [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_PROMPT_MAX_TOKENS` | active | — | — | Observability / Prompt Tuning | Soft prompt token cap for role-context-builder |
| `EVOLVE_PSMAS_SKIP` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_QUOTA_DANGER_PCT` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_QUOTA_RESET_AT` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_QUOTA_RESET_HOURS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_REFLECTION_JOURNAL` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_REINVOKE_CMD` | dead | — | — | Observability / Prompt Tuning | Stored reinvoke command for rate-limit recovery [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_RELEASE_REQUIRE_PREFLIGHT` | active | — | — | Observability / Prompt Tuning | Force release preflight gate |
| `EVOLVE_RELEASE_STRICT_PASS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_REQUIRE_INTENT` | active | — | — | Workflow Defaults | Force intent phase on every cycle |
| `EVOLVE_REQUIRE_TEAM_CONTEXT` | active | — | — | Workflow Defaults | Require team context before builder |
| `EVOLVE_RESEARCH_CACHE_ENABLED` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_RESEARCH_HOOK_DISABLED` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_RESET` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_RESOLVE_ROOTS_LOADED` | dead | — | — | Core Infrastructure (never consolidate) | Idempotency guard for resolve-roots.sh sourcing [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_RESUME` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_RESUME_ALLOW_HEAD_MOVED` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_RESUME_COMPLETED_PHASES` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_RESUME_MODE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_RESUME_PHASE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_RETROSPECTIVE_MODEL` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_RETRO_MODEL` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_RETRY_BACKOFF_BASE_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_REVIEW_GATE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ROUTER_CLI` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ROUTER_MODEL` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ROUTER_REPLAN` | active | enum | shadow | Advisor Maximization (ADR-0052) | Post-scout re-plan rollout dial (advisor-maximization WS2): `off` (no re-plan; the upfront plan stands) / `shadow` (re-plan computed + logged to replan-plan.json, the upfront clamped plan still drives — DEFAULT) / `advisory` (re-plan replaces the clamped plan after the integrity floor re-clamps it; opt-in, post-soak). Composition-root view set by applyEnv; the re-plan call site reads it (behavior wires in WS2-S3). Unknown → `off` + WARN (a typo never silently enables the re-plan). |
| `EVOLVE_ROUTING_JUDGE` | active | bool | 0 | Advisor Maximization (ADR-0052) | Opt-in LLM-as-judge route-quality scoring (advisor-maximization WS4), strictly off the build/critical path: `off` (DEFAULT — no judge call, byte-identical) / `on` (the fast-tier judge scores the emitted route + records the score for forensics; never gates ship, never alters the plan). A plain bool, not a stage — the judge cannot move behavior, so off/shadow/advisory would be indistinguishable. Composition-root view (RoutingConfig.RoutingJudge) set by applyEnv; the scoring call site reads it (behavior wires in WS4-S3). Unknown → off + WARN (a typo never silently enables the judge). |
| `EVOLVE_ROUTING_MODE` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Routing brain: `llm`/`dynamic`/`dynamic-llm` (LLM proposes, kernel clamps) / `static`/`static-preset`/`preset` (triggers + spine only, no LLM). Unknown → `llm` + WARN |
| `EVOLVE_RUN_TIMEOUT` | active | — | — | Observability / Prompt Tuning | Per-subagent run timeout |
| `EVOLVE_SANDBOX` | active | — | — | Sandbox Cluster | Enable outer sandbox-exec/bwrap wrapper |
| `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` | active | — | — | Sandbox Cluster | EPERM fallback for nested-Claude (Darwin 25.4+) |
| `EVOLVE_SCOUT_CLI` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_SCOUT_INTERACTIVE_POLICY` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_SCOUT_LATENCY_CEILING_S` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_SCOUT_MODEL` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_SCOUT_PERMISSION_MODE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_SCROLLBACK_LINES` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_SHIP_AUTO_CONFIRM` | active | — | — | Workflow Defaults | CI mode: skip interactive y/N in ship.sh |
| `EVOLVE_SHIP_RELEASE_NOTES` | active | — | — | Workflow Defaults | Create GitHub release on ship |
| `EVOLVE_SHIP_SCRIPT` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_SKIP_CYCLE_HEALTH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_SKIP_PREFLIGHT` | active | — | — | Readiness Gate (pre-batch) | Emergency hatch: skip the whole readiness gate (no checks, no boot) |
| `EVOLVE_SKIP_PREFLIGHT_BOOT` | active | — | — | Readiness Gate (pre-batch) | Run the cheap checks (structure/CLI/host) but skip the real bridge-boot probe — CI/offline (bridge-boot downgrades Halt→Warn) |
| `EVOLVE_SKIP_WORKTREE` | active | — | — | Worktree / Workspace | Emergency hatch: skip per-cycle worktree isolation |
| `EVOLVE_STATE_FILE_OVERRIDE` | dead | — | — | State File Cluster (cycle 7 consolidation) | Override state.json path [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_STATE_OVERRIDE` | dead | — | — | State File Cluster (cycle 7 consolidation) | Alias for `EVOLVE_STATE_FILE_OVERRIDE`; emits stderr WARN [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_STDOUT_FILTER` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_STRATEGY` | active | — | — | Workflow Defaults | Cycle strategy override |
| `EVOLVE_STRICT_AUDIT` | active | — | — | Workflow Defaults | WARN→FAIL promotion in ship.sh + failure-adapter blocking (v8.35+); single severity gate |
| `EVOLVE_STRICT_FAILURES` | dead | — | — | Workflow Defaults | Bridged to `EVOLVE_STRICT_AUDIT`; emits stderr WARN; removal target v8.61+ [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_SWARM_CONCURRENCY` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_SWARM_PLANNER` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_SWARM_PORT_BASE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_SWARM_STAGE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_SYSTEM_PROMPT` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_TASK_MODE` | active | — | — | Workflow Defaults | Profile tier selector (default/research/deep) |
| `EVOLVE_TDD_ENGINEER_CLI` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_TDD_ENGINEER_MODEL` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_TDD_ENGINEER_PERMISSION_MODE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_TDD_MODEL` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_TDD_PERMISSION_MODE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_TDD_PHASE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_TDD_PHASE_N` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_TDD_PLAN_INPUT` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_TESTING` | active | — | — | Core Infrastructure (never consolidate) | Test harness mode — disables real CLI calls |
| `EVOLVE_TEST_COST_GUARD_STRICT` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_TEST_COST_THRESHOLD` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_TEST_PHASE_ENABLED` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_TRACKER_TTL_DAYS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL` | active | — | — | Triage Cluster (cycle 7 trim) | Opt A: auto-skip Triage when 3 conditions hold (≤1 scout task AND empty carryoverTodos AND predicate-dependency-check.sh exit 0). Default on (=1); opt-out with =0. Writes a stub `triage-decision.{md,json}` with `auto_skip: true` so downstream phases see consistent inputs. |
| `EVOLVE_TRIAGE_CAP_GATE` | active | enum | enforce | Gates | R9.2 triage capacity clamp: off\|shadow\|enforce. Committed coverage floors above ceil(1.25·K observed throughput) reject triage into the correction ladder. |
| `EVOLVE_TRIAGE_DISABLE` | active | — | — | Triage Cluster (cycle 7 trim) | Opt-out of triage default-on (v8.59+) |
| `EVOLVE_TRIAGE_ENABLED` | dead | — | — | Triage Cluster (cycle 7 trim) | v8.56–v8.58 opt-in; replaced by `EVOLVE_TRIAGE_DISABLE`; removed from docs |
| `EVOLVE_TRIAGE_MODEL` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_TRIAGE_PERMISSION_MODE` | test-seam | — | — | — | Read only by _test.go files. |
| `EVOLVE_TRIAGE_TOP_N` | active | — | — | Triage Cluster (cycle 7 trim) | Override triage top_n selection count |
| `EVOLVE_USE_LEGACY_BASH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_USE_PHASE_REGISTRY` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Set `0` to skip reading `phase-registry.json` (built-in defaults only) |
| `EVOLVE_WORKTREE_BASE` | active | — | — | Worktree / Workspace | Per-cycle worktree base path |
| `EVOLVE_WORKTREE_PATH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_X` | test-seam | — | — | — | Read only by _test.go files. |

<!-- GENERATED:flag-index END -->
