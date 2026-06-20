# Control Flags Reference — `EVOLVE_*`

> **93+ distinct flags** as of 2026-05-27 (count approximate). See cluster annotations for consolidation targets in cycles 8–10.
> Canonical source — bash surface: `grep -rohE 'EVOLVE_[A-Z_]+' legacy/scripts/ agents/ skills/ | sort -u`.
> Go-native surface (NOT captured by the bash grep — e.g. the dynamic-routing family lives in `go/internal/`): `grep -rohE 'EVOLVE_[A-Z_]+' go/ | sort -u`.

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
| `EVOLVE_RESOLVE_ROOTS_LOADED` | DEAD | Idempotency guard for resolve-roots.sh sourcing [no reader on any surface as of 2026-06-11 inventory] |
| `EVOLVE_FAILURE_CLASSIFICATIONS_LOADED` | DEAD | Idempotency guard for failure-classifications.sh [no reader on any surface as of 2026-06-11 inventory] |

## Sandbox Cluster

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_SANDBOX` | ACTIVE | Enable outer sandbox-exec/bwrap wrapper |
| `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` | ACTIVE | EPERM fallback for nested-Claude (Darwin 25.4+) |

> **Cycle 7 retirement**: The two inner-sandbox flags were removed (no Go reader; the Go bridge controls inner-sandbox via `sandbox.ShouldWrap`). Use `EVOLVE_SANDBOX=on/off`.

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
| `workflow.phase_enables.build-planner` | MIGRATED (cycle-39) | Opt C build-planner phase. Set `on` in policy.json to enable (advisory mode). Replaces removed env flag. See ADR-0019. |

> **Token-budget cost gates removed**: the dollar-cost calculation and every gate
> that decided off it (loop stop, phase FAIL, checkpoint-by-cost, optional-phase
> skip) were removed because cost is model-dependent and unreliable. The `budget`
> package, the gemini price table, and claude.sh budget-tier resolution are gone.
> The flags above remain accepted-but-ignored for backward compatibility.
>
> **Cycle 10 CLOSED**: Workflow Defaults cluster — `EVOLVE_STRICT_*` (2 flags) consolidated.
> `EVOLVE_STRICT_FAILURES` bridged to `EVOLVE_STRICT_AUDIT` (canonical).
> Dispatch policy flags (`EVOLVE_DISPATCH_VERIFY`, `EVOLVE_DISPATCH_STOP_ON_FAIL`) moved to config-as-code (dispatch-cluster-28).
>
## State File Cluster (cycle 7 consolidation)

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_STATE_FILE_OVERRIDE` | ACTIVE (canonical) | Override state.json path |
| `EVOLVE_STATE_OVERRIDE` | DEPRECATED | Alias for `EVOLVE_STATE_FILE_OVERRIDE`; emits stderr WARN |

## Bypass / Emergency Hatches

Emergency bypasses are explicit CLI flags rather than environment variables:
`evolve guard <phase|role|ship> --bypass`, `evolve postedit-validate --bypass`,
`evolve commit-prefix-gate --bypass`, and `evolve ship --bypass-commit-gate`
or `--bypass-prefix-gate`.

## Triage Cluster (cycle 7 trim)

| Flag | Status | Purpose |
|------|--------|---------|
| `workflow.phase_enables.triage` | MIGRATED (cycle-39) | Triage opt-out: set `=off` in policy.json. Default-on (v8.59+). Replaces removed env flag. |
| `EVOLVE_TRIAGE_ENABLED` | DEAD | v8.56–v8.58 opt-in; replaced by env flag (now removed, cycle-39); use policy.json triage phase_enables. |


## Platform / CLI Hybrid

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_PLATFORM` | ACTIVE | Override platform detection |

## Worktree / Workspace

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_WORKTREE_BASE` | ACTIVE | Per-cycle worktree base path |
| `EVOLVE_DRY_RUN_PROVISION_WORKTREE` | DEAD | Dry-run worktree provisioning [no reader on any surface as of 2026-06-11 inventory] |

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
| `EVOLVE_STRICT_FAILURES` | DEAD | Bridged to `EVOLVE_STRICT_AUDIT`; emits stderr WARN; removal target v8.61+ [no reader on any surface as of 2026-06-11 inventory] |
| `workflow.phase_enables.intent` | MIGRATED (cycle-39) | Force intent phase on every cycle: set `=on` in policy.json. Replaces removed env flag. |
| `workflow.phase_enables.plan-review` | MIGRATED (cycle-39) | Enable Sprint 2 plan-review phase (opt-in): set `=on` in policy.json. Replaces removed env flag. |
| `EVOLVE_DISPATCH_STOP_ON_FAIL` | REMOVED (dispatch-cluster-28) | Former dispatch fail-fast switch; moved to `policy.json` `dispatch.policy` config |
| `EVOLVE_DISPATCH_VERIFY` | REMOVED (dispatch-cluster-28) | Former dispatch verify toggle; moved to `policy.json` `dispatch.policy` config |
| `EVOLVE_AUTO_PRUNE` | ACTIVE | Enable auto-prune of expired state entries |
| `EVOLVE_SHIP_AUTO_CONFIRM` | ACTIVE | CI mode: skip interactive y/N in ship.sh |
| `EVOLVE_DIFF_COMPLEXITY_DISABLE` | ACTIVE | Disable diff-complexity check in auditor |
| `EVOLVE_CONSENSUS_AUDIT` | ACTIVE | Enable consensus-dispatch for auditor |
| `EVOLVE_AUDITOR_TIER_OVERRIDE` | ACTIVE | Override auditor model tier |

## Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off)

> Read by the composition-root loader `go/internal/config/config.go` (the ONLY env site). Precedence: env > `docs/architecture/phase-registry.json` > built-in default. See [docs/architecture/dynamic-phase-routing.md](dynamic-phase-routing.md) and ADR-0024 (proposed PhaseAdvisor evolution).

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

## Observability / Prompt Tuning

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_PROMPT_MAX_TOKENS` | ACTIVE | Soft prompt token cap for role-context-builder |
| `EVOLVE_PROMPT_BUDGET_ENFORCE` | ACTIVE | Make prompt-over-cap a hard error |
| `EVOLVE_CACHE_PREFIX_V2` | ACTIVE (default `1`) | v8.61.0 Campaign A — static-first / dynamic-last prompt layering. When `1`: (Cycle A1) subagent-run.sh emits a small INVOCATION CONTEXT user prompt; (Cycle A2) claude.sh attaches the role-specific bedrock from `build-invocation-context.sh` via `--append-system-prompt` AND adds `--exclude-dynamic-system-prompt-sections` so per-machine sections move out of the cached system layer. Promoted to default=1 in cycle 43 (v10.6+), overdue since v8.62 target. Set `EVOLVE_CACHE_PREFIX_V2=0` to revert to legacy v1 ordering. |
| `EVOLVE_CONTEXT_DIGEST` | ACTIVE (default `1`) | v8.62.0 Campaign B (Tier 2 — digest layer). When `1`, role-context-builder.sh: (B1) lazy-builds `cycle-digest.json` via `build-cycle-digest.sh`; (B2) replaces full intent.md cat with a compact `## Intent (compact)` block (intent_anchor + acceptance_criteria from digest) for scout/triage/plan-review/tdd/builder phases — auditor + retrospective still get the full file. Real-world reduction: scout 84%, triage 40%, builder 43%. Promoted to default=1 in cycle 24 (v9.4.0). Set `EVOLVE_CONTEXT_DIGEST=0` to revert to legacy full-file mode. |
| `EVOLVE_ANCHOR_EXTRACT` | ACTIVE (default `1`) | v8.63.0 Campaign C (Tier 3 — anchored artifacts). When `1`, role-context-builder.sh extracts only named `<!-- ANCHOR:<name> -->` regions from prior phase artifacts instead of `cat`-ing whole files. Persona templates (scout/builder/auditor/retrospective) emit anchor markers around output sections. Backwards-compat: pre-v8.63 artifacts without anchors fall back to full-file emission once per file (no duplication regression). Auditor reads `diff_summary`+`test_results` from build-report and `proposed_tasks`+`acceptance_criteria` from scout-report; triage reads `proposed_tasks` only. Promoted to default=1 in cycle 24 (v9.4.0). Set `EVOLVE_ANCHOR_EXTRACT=0` to revert to legacy full-file mode. |
| `EVOLVE_INSTINCT_SUMMARY_CAP` | ACTIVE | Max instinct summaries in state.json |
| `EVOLVE_CARRYOVER_TODO_MAX_UNPICKED` | ACTIVE | Carryover todos threshold |
| `EVOLVE_RELEASE_REQUIRE_PREFLIGHT` | ACTIVE | Force release preflight gate |
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
| `EVOLVE_TRIAGE_ENABLED` | Comment-only; production now uses `workflow.phase_enables.triage` (policy.json) | Removed from CLAUDE.md in cycle 7; env opt-out also removed in cycle-39 |
| `EVOLVE_DIR_OVERRIDE` | Test-only conditional; no production reader | Leave in test; document as test-only |
| `EVOLVE_PROJECT_ROOT_OVERRIDE` | 1 occurrence, no reader | Document as dead |
| `EVOLVE_REPO_ROOT_OVERRIDE` | 1 occurrence, no reader | Document as dead |

## Internal (subprocess injection — not operator-facing)

| Flag | Purpose |
|------|---------|
| `EVOLVE_PROJECT_WRITABLE` | Set by resolve-roots.sh after verification |

---

## Consolidation Roadmap

| Cycle | Cluster | Action |
|-------|---------|--------|
| 7 (done) | State-file | Deprecated `EVOLVE_STATE_OVERRIDE` → `EVOLVE_STATE_FILE_OVERRIDE` |
| 8 (done) | Sandbox | Deprecated inner-sandbox flags via bridge (v8.60); retired in cycle-7 |
| 9 (done) | Budget | Deprecated `EVOLVE_BUDGET_CAP` → `EVOLVE_MAX_BUDGET_USD` bridge (v8.60); added builder cost-overrun guard |
| 10 (done) | Workflow Defaults | Deprecated `EVOLVE_STRICT_FAILURES` → `EVOLVE_STRICT_AUDIT`; deprecated dispatch policy flags, moved to `policy.json` dispatch config (dispatch-cluster-28) |
<!-- GENERATED:flag-index BEGIN — do not edit by hand; run `evolve flags generate` -->

## Generated Flag Index

Complete flag index — generated from `go/internal/flagregistry` (SSOT). Edit the registry, then run `evolve flags generate`; do not edit this table by hand.

| Flag | Status | Kind | Default | Cluster | Purpose |
|------|--------|------|---------|---------|----------|
| `EVOLVE_ACS_GO_TIMEOUT_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ADAPTERS_DIR_OVERRIDE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ADVISOR_DEPTH` | active | int | — | Advisor Maximization (ADR-0052) | Recursion-depth stamp for the PhaseAdvisor (advisor-maximization WS1-S2), DEFENSE-IN-DEPTH only: when ≥1 the advisor refuses to dispatch and the cycle degrades to the static path, so a brain can never nest another brain. The PRIMARY recursion guard is the mint denylist (mintConfigsFrom drops any minted phase named router/advisor); this env stamp catches the otherwise-unreachable case where such a phase were dispatched anyway. Unset/0/non-numeric = no guard (byte-identical normal path). |
| `EVOLVE_ANTHROPIC_BASE_URL` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CACHE_PREFIX_V2` | active | — | — | Observability / Prompt Tuning | v8.61.0 Campaign A — static-first / dynamic-last prompt layering. When `1`: (Cycle A1) subagent-run.sh emits a small INVOCATION CONTEXT user prompt; (Cycle A2) claude.sh attaches the role-specific bedrock from `build-invocation-context.sh` via `--append-system-prompt` AND adds `--exclude-dynamic-system-prompt-sections` so per-machine sections move out of the cached system layer. Promoted to default=1 in cycle 43 (v10.6+), overdue since v8.62 target. Set `EVOLVE_CACHE_PREFIX_V2=0` to revert to legacy v1 ordering. |
| `EVOLVE_CLI` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CLI_HEALTH` | active | — | — | Readiness Gate (pre-batch) | The one dial for the CLI-health bench layer (cycle-283: a quota-walled codex re-burned its boot on every dispatch all night because nothing remembered the wall). `0` disables ALL of it: the runner's bench-writer (exit-85 + classified `rate_limit` escalation → bench the CLI FAMILY in `.evolve/cli-health.json`, `benched_until` from the wall's own reset hint else a strike-doubled cooldown), the dispatch-chain demotion (benched families start at their fallback; bench is advice — all-benched dispatches least-recently-benched with a loud WARN; policy pins bypass entirely), the loop's per-cycle canary (one `bridge.LiveSmokeTest` per EXPIRED bench: recovered → cleared, walled again → strikes+1), and the advisor's environmental "CLI health" prompt section. Preflight's `cli-health` check (WARN-only) and `evolve doctor live <driver>` (the probe that can SEE a quota wall — boot smoke cannot, walls appear only after work is submitted) remain readable surfaces. |
| `EVOLVE_CLI_MAX_CONCURRENT_CODEX` | active | — | — | Concurrency / Sibling-Worktree (ADR-0054) | Per-CLI cross-process admission cap for the sibling-worktree concurrent-loop model (ADR-0054 Slice 4). Pattern: EVOLVE_CLI_MAX_CONCURRENT_<UPPERCASE_CLI_NAME> (e.g. EVOLVE_CLI_MAX_CONCURRENT_CODEX=2, EVOLVE_CLI_MAX_CONCURRENT_CLAUDE=3, EVOLVE_CLI_MAX_CONCURRENT_AGY=1). Default 0 = unbounded (byte-identical to pre-concurrency behavior — safe default, no behavior change). A failed acquire degrades to uncapped + WARN; admission control never blocks a phase outright. Read by internal/bridge/driver_tmux_repl.go via internal/cliadmit.Acquire. Holder-set JSON under $XDG_RUNTIME_DIR/evolve/cli-<name>.slots; stale holders auto-pruned by TTL (lease-as-liveness). See ADR-0054. |
| `EVOLVE_CODEX_CONFIG_PATH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CODEX_VERSION_PATH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_COMMIT_EVIDENCE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_COMPACT_PROMPTS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_COMPOSE_PHASES` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CONDITIONAL_MANDATORY` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | `phase:expr` conditional-mandatory predicate; op ∈ `!= == >= <= > <` |
| `EVOLVE_DISABLE_WORKSPACE_GUARD` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_DYNAMIC_ROUTING` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Rollout stage: `off`/`0` (static state machine drives — operator escape hatch) / `shadow` (router computes + logs, static drives) / `advisory` (router drives optional surface, spine static; DEFAULT) / `enforce` (router drives, kernel-clamped). Unknown value → `off` + WARN |
| `EVOLVE_FLEET` | active | bool | 0 | Fleet Cluster (Track C concurrency) | Fleet mode (CB.2+): bridges refuse the process-cwd fallback when no worktree is designated (typed ExitBadFlags, never CLI-fallback). Set by the `evolve fleet` supervisor (CE.2); single-driver runs leave it unset and keep the loud-WARN fallback. |
| `EVOLVE_FLEET_SCOPE` | active | string | — | Fleet Cluster (Track C concurrency) | Comma-joined todo IDs assigned to this fleet cycle (ADR-0049 E); the launched cycle's triage selects only its disjoint subset. Empty/unset ⇒ the cycle works the whole backlog. Reader: go/internal/core/cyclerun.go (set by the `evolve fleet` supervisor, fleet/fleet.go) |
| `EVOLVE_FORCE_FRESH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_GO_BIN` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_GO_BIN_TEST` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_GUARDS_LOG` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_HANG_CLASSIFIER` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_INTENT_DELTA` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_KB_SEARCH_PATHS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_LANE` | active | — | — | Concurrency / Fleet (ADR-0049) | Operator-pinned human-readable lane name for a worktree (e.g. EVOLVE_LANE=campaign), overriding the hash-of-root default (runscope.EnvLane). Readability only — correctness never depends on the override; the hash default is collision-safe across distinct roots. Introduced in concurrency-arch-slices Slice 1. |
| `EVOLVE_LEDGER_OVERRIDE` | active | — | — | Override / Test Seams | Override ledger.jsonl path |
| `EVOLVE_MANDATORY_PHASES` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | CSV ordered mandatory spine. Omitting `audit` or `ship` emits a `weak-spine` WARN |
| `EVOLVE_MARKETPLACE_DIR` | active | — | — | Observability / Prompt Tuning | Override marketplace dir (test/release seam) |
| `EVOLVE_MAX_OPTIONAL_INSERTIONS` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Cap on optional phases the router may insert |
| `EVOLVE_MODELCATALOG_AUTOREFRESH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_MODELCATALOG_CLASSIFIER_CLI` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_MODEL_CATALOG_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_OLLAMA_BASE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PERSONA_OVERRIDE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PHASE_IO` | active | — | — | Phase I/O (ADR-0050) | ADR-0050 Phase-3 unified-phase-I/O rollout dial. FULL off→shadow→advisory→enforce ladder (4-value, unlike the 3-value gate dials). off = dormant legacy dispatch, byte-identical (the rollback escape hatch); shadow = typed envelope assembled + compared against legacy disk reads (mismatch → ledger phaseio_shadow_mismatch); advisory = envelope populated + read alongside legacy (legacy still wins); enforce = the typed envelope is AUTHORITATIVE — phase readers consume it and the audit/ship verdict parse is sentinel-mandatory. DEFAULT enforce as of the 3.10 cutover (was off through 18.14.0); set EVOLVE_PHASE_IO=off to roll back. A typo falls back to off (fail-safe, never leaves the dial in an unintended state). |
| `EVOLVE_PHASE_RECOVERY` | active | — | — | Phase Recovery (ADR-0044, Go-native — one dial for the whole program) | The one dial for BOTH the ADR-0044 phase-recovery program (terminal-state recovery) AND the ADR-0045 corrective-interaction program (repair a live/just-completed phase through bounded interaction). ADR-0044: fatal-pane fast-fail at the stop-review checkpoint, the observer's chain-backed StallPolicy (subprocess injects it ONLY at explicit `enforce`), and the orchestrator's escalate→advise→promote hook (`config.RolloutStages.PhaseRecovery` view). ADR-0045 corrective ACTIONS ride the SAME stage: the graduated correction ladder (salvage→live-fix→re-dispatch), the pre-85 AskBroker rung, promoted-rule enforcement, and the live bidirectional channel (ADR-0037, folded in at I6 — `enforce` implies the channel). **Telemetry is EXEMPT**: ADR-0045 I1 interaction telemetry (`<phase>-interactions.ndjson` + `interaction-summary.json`) records at EVERY stage including `off` — only ACTIONS gate. Stages: `off` (detectors not consulted, no corrective actions; byte-identical legacy — telemetry still records) / `shadow` (DEFAULT — detect + log the would-act for every rung, legacy behavior decides; byte-identical) / `enforce` (fatal-pane preempt with `stop`; salvage relocates a misplaced deliverable; the kernel answers a blocking question pre-85; promoted enforce-stage rules fire; exit 81 hands the phase to the runner's CLI fallback chain). Unknown value → `off` (a typo never enables a kill-path). A Busy pane is never preempted/interrupted regardless of stage |
| `EVOLVE_PHASE_ROOTS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PLAN_WORKSPACE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PLATFORM` | active | — | — | Platform / CLI Hybrid | Override platform detection |
| `EVOLVE_PLUGIN_ROOT` | active | — | — | Core Infrastructure (never consolidate) | Read-only plugin install location |
| `EVOLVE_POLICY_BYPASS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PROFILES_DIR_OVERRIDE` | active | — | — | Override / Test Seams | Override profiles dir path |
| `EVOLVE_PROFILE_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PROJECT_ROOT` | active | — | — | Core Infrastructure (never consolidate) | Writable project directory (dual-root pattern) |
| `EVOLVE_PROMPTS_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PROMPT_MAX_TOKENS` | active | — | — | Observability / Prompt Tuning | Soft prompt token cap for role-context-builder |
| `EVOLVE_REFLECTION_JOURNAL` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_RELEASE_REQUIRE_PREFLIGHT` | active | — | — | Observability / Prompt Tuning | Force release preflight gate |
| `EVOLVE_RELEASE_STRICT_PASS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_RETRO_MODEL` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ROUTING_MODE` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Routing brain: `llm`/`dynamic`/`dynamic-llm` (LLM proposes, kernel clamps) / `static`/`static-preset`/`preset` (triggers + spine only, no LLM). Unknown → `llm` + WARN |
| `EVOLVE_SANDBOX` | active | — | — | Sandbox Cluster | Enable outer sandbox-exec/bwrap wrapper |
| `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` | active | — | — | Sandbox Cluster | EPERM fallback for nested-Claude (Darwin 25.4+) |
| `EVOLVE_SHIP_AUTO_CONFIRM` | active | — | — | Workflow Defaults | CI mode: skip interactive y/N in ship.sh |
| `EVOLVE_SKIP_PREFLIGHT` | active | — | — | Readiness Gate (pre-batch) | Emergency hatch: skip the whole readiness gate (no checks, no boot) |
| `EVOLVE_SKIP_PREFLIGHT_BOOT` | active | — | — | Readiness Gate (pre-batch) | Run the cheap checks (structure/CLI/host) but skip the real bridge-boot probe — CI/offline (bridge-boot downgrades Halt→Warn) |
| `EVOLVE_STDOUT_FILTER` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_STRICT_AUDIT` | active | — | — | Workflow Defaults | WARN→FAIL promotion in ship.sh + failure-adapter blocking (v8.35+); single severity gate |
| `EVOLVE_SYSTEM_PROMPT` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_TESTING` | active | — | — | Core Infrastructure (never consolidate) | Test harness mode — disables real CLI calls |
| `EVOLVE_USE_PHASE_REGISTRY` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Set `0` to skip reading `phase-registry.json` (built-in defaults only) |
| `EVOLVE_WORKTREE_BASE` | active | — | — | Worktree / Workspace | Per-cycle worktree base path |
| `EVOLVE_WORKTREE_PATH` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_WORKTREE_ROOT` | active | — | — | Worktree / Workspace | SOURCE root for generated-doc predicates (dual-root pattern); ACS suite exports the cycle's worktree so `flags check`/`skills check` validate the worktree artifact (cycle-355). |

<!-- GENERATED:flag-index END -->
