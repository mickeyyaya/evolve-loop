# Control Flags Reference — `EVOLVE_*`

> The authoritative flag inventory is the **[Generated Flag Index](#generated-flag-index)** below — projected from `go/internal/flagregistry` (the SSOT) and regenerated via `evolve flags generate`. As of **v20.2.0** the operator registry is **35 rows** (flag-campaign-8 banked 48→35; trust the generated table below over any hand-written count).
>
> The historical hand-maintained cluster tables were removed in the flag-consolidation campaign: they duplicated the registry and drifted stale (a `never_duplicate` violation). The campaign retires the scattered `os.Getenv` override surface — capabilities move into typed `.evolve/policy.json` config, profile SSOT, DI, or documented subprocess protocol. Background: `knowledge-base/research/flag-consolidation-campaign-2026-06-19.md`.

## Status Key

| Status | Meaning |
|--------|---------|
| active | Read in production code; do not remove without a deprecation window |
| internal | Set by the runner for subprocess injection (IPC/bootstrap); not operator-facing |
| deprecated | Still honored via a bridge; emits a stderr WARN; remove in a future cycle |

<!-- GENERATED:flag-index BEGIN — do not edit by hand; run `evolve flags generate` -->

## Generated Flag Index

Complete flag index — generated from `go/internal/flagregistry` (SSOT). Edit the registry, then run `evolve flags generate`; do not edit this table by hand.

| Flag | Status | Kind | Default | Cluster | Purpose |
|------|--------|------|---------|---------|----------|
| `EVOLVE_ACS_GO_TIMEOUT_S` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ADAPTERS_DIR_OVERRIDE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CLI` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CLI_HEALTH` | active | — | — | Readiness Gate (pre-batch) | The one dial for the CLI-health bench layer (cycle-283: a quota-walled codex re-burned its boot on every dispatch all night because nothing remembered the wall). `0` disables ALL of it: the runner's bench-writer (exit-85 + classified `rate_limit` escalation → bench the CLI FAMILY in `.evolve/cli-health.json`, `benched_until` from the wall's own reset hint else a strike-doubled cooldown), the dispatch-chain demotion (benched families start at their fallback; bench is advice — all-benched dispatches least-recently-benched with a loud WARN; policy pins bypass entirely), the loop's per-cycle canary (one `bridge.LiveSmokeTest` per EXPIRED bench: recovered → cleared, walled again → strikes+1), and the advisor's environmental "CLI health" prompt section. Preflight's `cli-health` check (WARN-only) and `evolve doctor live <driver>` (the probe that can SEE a quota wall — boot smoke cannot, walls appear only after work is submitted) remain readable surfaces. |
| `EVOLVE_CLI_MAX_CONCURRENT_CODEX` | active | — | — | Concurrency / Sibling-Worktree (ADR-0054) | Per-CLI cross-process admission cap for the sibling-worktree concurrent-loop model (ADR-0054 Slice 4). Pattern: EVOLVE_CLI_MAX_CONCURRENT_<UPPERCASE_CLI_NAME> (e.g. EVOLVE_CLI_MAX_CONCURRENT_CODEX=2, EVOLVE_CLI_MAX_CONCURRENT_CLAUDE=3, EVOLVE_CLI_MAX_CONCURRENT_AGY=1). Default 0 = unbounded (byte-identical to pre-concurrency behavior — safe default, no behavior change). A failed acquire degrades to uncapped + WARN; admission control never blocks a phase outright. Read by internal/bridge/driver_tmux_repl.go via internal/cliadmit.Acquire. Holder-set JSON under $XDG_RUNTIME_DIR/evolve/cli-<name>.slots; stale holders auto-pruned by TTL (lease-as-liveness). See ADR-0054. |
| `EVOLVE_COMMIT_EVIDENCE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_CONDITIONAL_MANDATORY` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | `phase:expr` conditional-mandatory predicate; op ∈ `!= == >= <= > <` |
| `EVOLVE_DYNAMIC_ROUTING` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Rollout stage: `off`/`0` (static state machine drives — operator escape hatch) / `shadow` (router computes + logs, static drives) / `advisory` (router drives optional surface, spine static; DEFAULT) / `enforce` (router drives, kernel-clamped). Unknown value → `off` + WARN |
| `EVOLVE_FLEET` | active | bool | 0 | Fleet Cluster (Track C concurrency) | Fleet mode (CB.2+): bridges refuse the process-cwd fallback when no worktree is designated (typed ExitBadFlags, never CLI-fallback). Set by the `evolve fleet` supervisor (CE.2); single-driver runs leave it unset and keep the loud-WARN fallback. |
| `EVOLVE_FLEET_SCOPE` | active | string | — | Fleet Cluster (Track C concurrency) | Comma-joined todo IDs assigned to this fleet cycle (ADR-0049 E); the launched cycle's triage selects only its disjoint subset. Empty/unset ⇒ the cycle works the whole backlog. Reader: go/internal/core/cyclerun.go (set by the `evolve fleet` supervisor, fleet/fleet.go) |
| `EVOLVE_GO_BIN` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_INTENT_DELTA` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_KB_SEARCH_PATHS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_LEDGER_OVERRIDE` | active | — | — | Override / Test Seams | Override ledger.jsonl path |
| `EVOLVE_MANDATORY_PHASES` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | CSV ordered mandatory spine. Omitting `audit` or `ship` emits a `weak-spine` WARN |
| `EVOLVE_MAX_OPTIONAL_INSERTIONS` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Cap on optional phases the router may insert |
| `EVOLVE_MODEL_CATALOG_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PERSONA_OVERRIDE` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PHASE_IO` | active | — | — | Phase I/O (ADR-0050) | ADR-0050 Phase-3 unified-phase-I/O rollout dial. FULL off→shadow→advisory→enforce ladder (4-value, unlike the 3-value gate dials). off = dormant legacy dispatch, byte-identical (the rollback escape hatch); shadow = typed envelope assembled + compared against legacy disk reads (mismatch → ledger phaseio_shadow_mismatch); advisory = envelope populated + read alongside legacy (legacy still wins); enforce = the typed envelope is AUTHORITATIVE — phase readers consume it and the audit/ship verdict parse is sentinel-mandatory. DEFAULT enforce as of the 3.10 cutover (was off through 18.14.0); set EVOLVE_PHASE_IO=off to roll back. A typo falls back to off (fail-safe, never leaves the dial in an unintended state). |
| `EVOLVE_PHASE_RECOVERY` | active | — | — | Phase Recovery (ADR-0044, Go-native — one dial for the whole program) | The one dial for BOTH the ADR-0044 phase-recovery program (terminal-state recovery) AND the ADR-0045 corrective-interaction program (repair a live/just-completed phase through bounded interaction). ADR-0044: fatal-pane fast-fail at the stop-review checkpoint, the observer's chain-backed StallPolicy (subprocess injects it ONLY at explicit `enforce`), and the orchestrator's escalate→advise→promote hook (`config.RolloutStages.PhaseRecovery` view). ADR-0045 corrective ACTIONS ride the SAME stage: the graduated correction ladder (salvage→live-fix→re-dispatch), the pre-85 AskBroker rung, promoted-rule enforcement, and the live bidirectional channel (ADR-0037, folded in at I6 — `enforce` implies the channel). **Telemetry is EXEMPT**: ADR-0045 I1 interaction telemetry (`<phase>-interactions.ndjson` + `interaction-summary.json`) records at EVERY stage including `off` — only ACTIONS gate. Stages: `off` (detectors not consulted, no corrective actions; byte-identical legacy — telemetry still records) / `shadow` (DEFAULT — detect + log the would-act for every rung, legacy behavior decides; byte-identical) / `enforce` (fatal-pane preempt with `stop`; salvage relocates a misplaced deliverable; the kernel answers a blocking question pre-85; promoted enforce-stage rules fire; exit 81 hands the phase to the runner's CLI fallback chain). Unknown value → `off` (a typo never enables a kill-path). A Busy pane is never preempted/interrupted regardless of stage |
| `EVOLVE_PHASE_ROOTS` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PLUGIN_ROOT` | active | — | — | Core Infrastructure (never consolidate) | Read-only plugin install location |
| `EVOLVE_POLICY_BYPASS` | deprecated | — | — | — | Replaced by PhaseRequest.BypassPolicy bool field (cycle-4 flag-reduction). Bridged from env in cmd_cycle.go for backward compat. flag-campaign-8: --bypass-policy CLI-flag conversion deferred (operator bypass capability preserved via this bridge until then). |
| `EVOLVE_PROFILES_DIR_OVERRIDE` | active | — | — | Override / Test Seams | Override profiles dir path |
| `EVOLVE_PROFILE_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_PROJECT_ROOT` | active | — | — | Core Infrastructure (never consolidate) | Writable project directory (dual-root pattern) |
| `EVOLVE_PROMPTS_DIR` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_REFLECTION_JOURNAL` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_ROUTING_MODE` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Routing brain: `llm`/`dynamic`/`dynamic-llm` (LLM proposes, kernel clamps) / `static`/`static-preset`/`preset` (triggers + spine only, no LLM). Unknown → `llm` + WARN |
| `EVOLVE_SANDBOX` | active | — | — | Sandbox Cluster | Enable outer sandbox-exec/bwrap wrapper |
| `EVOLVE_STRICT_AUDIT` | active | — | — | Workflow Defaults | WARN→FAIL promotion in ship.sh + failure-adapter blocking (v8.35+); single severity gate |
| `EVOLVE_SYSTEM_PROMPT` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_USE_PHASE_REGISTRY` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Set `0` to skip reading `phase-registry.json` (built-in defaults only) |
| `EVOLVE_WORKTREE_BASE` | active | — | — | Worktree / Workspace | Per-cycle worktree base path |
| `EVOLVE_WORKTREE_ROOT` | active | — | — | Worktree / Workspace | SOURCE root for generated-doc predicates (dual-root pattern); ACS suite exports the cycle's worktree so `flags check`/`skills check` validate the worktree artifact (cycle-355). |

<!-- GENERATED:flag-index END -->
