# Control Flags Reference — `EVOLVE_*`

> **86 distinct flags** as of 2026-05-10. See cluster annotations for consolidation targets in cycles 8–10.
> Canonical source: `grep -rohE 'EVOLVE_[A-Z_]+' scripts/ agents/ skills/ | sort -u`

## Status Key

| Status | Meaning |
|--------|---------|
| ACTIVE | Read in production code; do not remove without a deprecation window |
| DEPRECATED | Still honored via bridge; emits stderr WARN; remove in a future cycle |
| OVERLAPPING | Aliases an ACTIVE flag under a different name; consolidation target |
| DEAD | No production reader; safe to remove from docs |
| INTERNAL | Set by the runner for subprocess injection; not operator-facing |

---

## Core Infrastructure (never consolidate)

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_PROJECT_ROOT` | ACTIVE | Writable project directory (dual-root pattern) |
| `EVOLVE_PLUGIN_ROOT` | ACTIVE | Read-only plugin install location |
| `EVOLVE_CYCLE_STATE_FILE` | ACTIVE | Override cycle-state.json path (test seam) |
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

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_MAX_BUDGET_USD` | ACTIVE | Per-subagent budget cap (operator override, highest priority; use for all ceiling needs) |
| `EVOLVE_BUDGET_CAP` | DEPRECATED | Bridged to `EVOLVE_MAX_BUDGET_USD` (v8.60+); emits stderr WARN; removal target v8.61+ |
| `EVOLVE_BUDGET_ENFORCE` | ACTIVE | Use profile-resolved per-phase caps (legacy strict mode) |
| `EVOLVE_BATCH_BUDGET_CAP` | ACTIVE | Cumulative batch budget ceiling (default $20, v8.58+) |
| `EVOLVE_BATCH_BUDGET_DISABLE` | ACTIVE | Disable batch budget tripwire |
| `EVOLVE_FANOUT_PER_WORKER_BUDGET_USD` | ACTIVE | Per-fanout-worker budget cap |
| `EVOLVE_BUILDER_COST_THRESHOLD` | ACTIVE | Builder cost-overrun guard threshold (default $2.00; v8.60+) |
| `EVOLVE_BUILDER_COST_GUARD_STRICT` | ACTIVE | Make builder cost-overrun a hard gate failure (default off; v8.60+) |

> **Cycle 9 CLOSED**: `EVOLVE_BUDGET_CAP` is now deprecated with a bridge to `EVOLVE_MAX_BUDGET_USD` (v8.60+).
> `EVOLVE_MAX_BUDGET_USD` is the single canonical per-subagent ceiling flag. When both are set, `EVOLVE_MAX_BUDGET_USD` wins.
> Builder cost-overrun guard (`_check_builder_cost_overrun` in `phase-gate.sh`) reads `builder-usage.json` against the threshold.
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

## Triage Cluster (cycle 7 trim)

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_TRIAGE_DISABLE` | ACTIVE | Opt-out of triage default-on (v8.59+) |
| `EVOLVE_TRIAGE_TOP_N` | ACTIVE | Override triage top_n selection count |
| `EVOLVE_TRIAGE_ENABLED` | DEAD | v8.56–v8.58 opt-in; replaced by `EVOLVE_TRIAGE_DISABLE`; removed from docs |

## Fan-out Cluster (intentionally separate — do not consolidate per-phase flags)

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_FANOUT_ENABLED` | ACTIVE | Master switch for fan-out |
| `EVOLVE_FANOUT_SCOUT` | ACTIVE | Enable fan-out for scout phase |
| `EVOLVE_FANOUT_AUDITOR` | ACTIVE | Enable fan-out for auditor phase |
| `EVOLVE_FANOUT_RETROSPECTIVE` | ACTIVE | Enable fan-out for retrospective phase |
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

## Workflow Defaults

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_STRICT_AUDIT` | ACTIVE (canonical) | WARN→FAIL promotion in ship.sh + failure-adapter blocking (v8.35+); single severity gate |
| `EVOLVE_STRICT_FAILURES` | DEPRECATED | Bridged to `EVOLVE_STRICT_AUDIT`; emits stderr WARN; removal target v8.61+ |
| `EVOLVE_TASK_MODE` | ACTIVE | Profile tier selector (default/research/deep) |
| `EVOLVE_REQUIRE_INTENT` | ACTIVE | Force intent phase on every cycle |
| `EVOLVE_REQUIRE_TEAM_CONTEXT` | ACTIVE | Require team context before builder |
| `EVOLVE_PLAN_REVIEW` | ACTIVE | Enable Sprint 2 plan-review phase (opt-in) |
| `EVOLVE_DISABLE_AUTO_RETROSPECTIVE` | ACTIVE | Opt-out of inline retrospective on FAIL/WARN |
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

## Observability / Prompt Tuning

| Flag | Status | Purpose |
|------|--------|---------|
| `EVOLVE_PROMPT_MAX_TOKENS` | ACTIVE | Soft prompt token cap for role-context-builder |
| `EVOLVE_PROMPT_BUDGET_ENFORCE` | ACTIVE | Make prompt-over-cap a hard error |
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
