# Environment variable reference

> **Authoritative lookup table** for every evolve-loop environment variable and
> the small set of non-env knobs (CLI flags, JSON fields, output labels) that
> share the same "current behavior" contract. Migrated verbatim from the
> `CLAUDE.md` "Current behavior" table; defaults reflect production posture as of
> v13.0.0 (env-var defaults baselined at v10.8.0, amended per-row).
>
> Related: [bridge & adapters architecture](../architecture/bridge-and-adapters.md) ·
> [glossary](../00-overview/glossary.md) ·
> [CLI capability matrix](./cli-capability-matrix.md)

**Reading the tables.** "Default" is the production default when the var is
unset. "Effect" is what the var does. "Rollback / notes" is how to revert to
prior behavior plus the ADR/doc that owns the detail. A `(default-on)` var is
already enforced; an `(opt-in)` var does nothing until set.

---

## Trust / EGPS gate

| Env var | Default | Effect | Rollback / notes |
|---|---|---|---|
| `acs-verdict.json` (gate, not env) | enforced | Cycle ships only if `red_count == 0`. WARN level removed in v10.0.0. | No rollback — hard gate. See [egps-v10](../../docs/architecture/egps-v10.md). |
| `EVOLVE_TEST_PHASE_ENABLED` | `1` (default-on) | `1`: TDD-Engineer writes behavioral predicates before Builder; Tester validates after. `0`: Builder writes own predicates (v10.1 fallback, degrades quality). | `=0` reverts to v10.1 self-predicate fallback. Default flipped cycle-86 (predicate-quality Layer 4). |
| `EVOLVE_TRIAGE_DISABLE` | `0` (Triage on) | Runs Layer C Triage on every cycle. | `=1` skips Triage; soft WARN if skipped without override. |
| `workflow.strict_audit` (`.evolve/policy.json`) | `false` (fluent) | WARN ships by default (v8.28.0+). | `true` blocks on WARN. Migrated from the `EVOLVE_STRICT_AUDIT` env dial (flag-reduction, ADR-0064). Note: WARN removed in v10.0.0 — applies to pre-EGPS bootstrap cycles only. |
| `ADVERSARIAL_AUDIT` | `1` (default-on) | Runner prepends "ADVERSARIAL AUDIT MODE" framing requiring positive evidence for PASS; Auditor defaults to Opus (different family from Builder's Sonnet) to break same-model-judge sycophancy. | `=0` disables adversarial framing. |

## Phase routing & lifecycle

| Env var | Default | Effect | Rollback / notes |
|---|---|---|---|
| `EVOLVE_BUILD_PLANNER` | `1` (advisory, v12.3+) | Opt-C build-planner phase (wired v10.19; cycle-103). `0` = shadow (no-op); `1` = advisory (`build-plan.md` produced). | `=0` shadow. 3-cycle rollout shadow→advisory→enforce. See [ADR-0019](../../docs/architecture/adr/0019-build-planner-phase.md). |
| `EVOLVE_REFLECTION_JOURNAL` | `1` (advisory, v10.20) | Per-phase Reflection Journal: each phase agent appends `## Reflection` + `<phase>-reflection.yaml` sidecar. Reflector (every cycle) + Retrospective/Memo consume YAML. v10.20 advisory (missing YAML → WARN); v10.21 enforce (phase-gate blocks missing YAML). | `=0` disables. See [reflection-journal](../../docs/architecture/reflection-journal.md), [learn-phase](../../docs/architecture/learn-phase.md). |
| `EVOLVE_DISABLE_AUTO_RETROSPECTIVE` | `0` (on) | FAIL/WARN cycles auto-run retrospective + merge-lesson. Cost ~$0.30–0.50/cycle. | `=1` disables auto-retro. |
| `EVOLVE_PLAN_REVIEW` | `0` (off) | Four-lens review (CEO/Eng/Design/Security) between discover and build; verdict gates via `gate_plan_review_to_tdd`. | `=1` to enable. |
| `EVOLVE_INTENT_DELTA` | `0` (opt-in) | `1`: `intent-batch-resolve.sh` runs before intent phase to compute `INTENT_MODE=full\|delta` by comparing `GOAL_HASH` against `state.json:currentBatch.goalHash`. Delta mode emits `intent-delta.md` (patch) or `[intent-unchanged]`; `gate_intent_to_research` accepts both. Requires `EVOLVE_REQUIRE_INTENT=1`. | `=0` reverts to full intent every cycle. See [incremental-intent](../../docs/architecture/incremental-intent.md). |
| `EVOLVE_DYNAMIC_ROUTING` | `off` (static state machine drives, v13.0.0/PR #4) | Rollout stage for the Go routing kernel (`go/internal/router`, `core.PhaseAdvisor`): `off`/`0` = legacy static sequence drives (byte-identical to pre-routing); `shadow` = router logs would-have-routed plan, static still drives; `advisory`/`enforce` = advisor DRIVES (ADR-0024 §1 floor LIVE as of PR-5): the advisor's whole-cycle plan is clamped by `ClampPlanToFloor` and drives run/skip for every non-mandatory phase; `enforceNext` (active at Advisory+, re-validated by `CanTransition` + `SpineSatisfiedUpTo`) overrides the static successor. Two guarantees: configurable never-skip set is `EVOLVE_MANDATORY_PHASES`; non-configurable integrity floor is `ship ⇒ build ∧ audit ∧ (tdd unless trivial)`. Planner failure ⇒ nil plan ⇒ fall back to spine (fail-safe to static). | `=off`/`0` reverts to static. First live advisory cycle was cycle-108. See [dynamic-phase-routing](../../docs/architecture/dynamic-phase-routing.md), ADR-0024. |
| `EVOLVE_ROUTING_MODE` | `llm` | `llm` = LLM proposes + kernel clamps; `static` = triggers + spine only, no advisor plan. | Companion to `EVOLVE_DYNAMIC_ROUTING`. |
| `EVOLVE_MANDATORY_PHASES` | `scout,build,audit,ship` | CSV never-skip set. Omitting `audit`/`ship` emits a `weak-spine` WARN (backstopped by the integrity floor). | Precedence: env > registry file > built-in default. |
| `EVOLVE_CONDITIONAL_MANDATORY` | `tdd:cycle_size!=trivial` | `phase:expr` conditional-mandatory rule. | — |
| `EVOLVE_MAX_OPTIONAL_INSERTIONS` | `4` | Cap on optional-phase insertions. NOT applied to plan-driven inserts (the plan is the advisor's clamped whole-cycle selection). | — |
| `EVOLVE_USE_PHASE_REGISTRY` | (on) | `0` ignores `docs/architecture/phase-registry.json`. | Precedence: env > registry file > built-in default. |

## Bridge / CLI dispatch & interaction

| Env var | Default | Effect | Rollback / notes |
|---|---|---|---|
| `EVOLVE_GO_BIN` | unset | Path to the Go binary — the sole runtime entrypoint for `evolve cycle run`/`loop`/`doctor`/`guard`/`ledger`/`acs`/`ship`. When unset, resolves `<project_root>/go/bin/evolve`. | Set to a cross-compiled artifact (e.g. `~/.local/bin/evolve-darwin-arm64`) for system-wide install. The bash dispatcher and the `EVOLVE_USE_LEGACY_BASH` rollback hatch were removed in the Go-only consolidation — there is no bash fallback. History: [migration-from-bash](../../docs/migration-from-bash.md). |
| `EVOLVE_INTERACTIVE_POLICY` | `recommended_or_first` (default-on, v12.1+) | Bridge prepends a deterministic policy block to every phase prompt so subagents self-resolve `AskUserQuestion`/y/N without hanging. Values: `recommended_or_first` (pick "(Recommended)" or first), `escalate` (no block, fail loudly — legacy), `auto_yes` (binary y/N → yes; multi-option → recommended-or-first). Unknown values silently default to `recommended_or_first`. Block < 200 tokens, deterministic (preserves prompt-prefix cache). | `=escalate` for legacy fail-loud posture. Impl: `go/internal/adapters/bridge/bridge.go:injectPolicyPrefix`. |
| `EVOLVE_<AGENT>_INTERACTIVE_POLICY` | unset | Per-agent override. `<AGENT>` = agent name upcased, hyphens→underscores (`scout` → `EVOLVE_SCOUT_INTERACTIVE_POLICY`, `tdd-engineer` → `EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY`). | Precedence: `req.Env` per-agent > process env per-agent > `req.Env` global > process env global > default. Pin one phase (e.g. `EVOLVE_AUDITOR_INTERACTIVE_POLICY=escalate`). |
| `EVOLVE_<PHASE>_PERMISSION_MODE` | unset (v12.1+) | Sets the Claude Code `--permission-mode` flag for one phase invocation. Values: `plan`, `acceptEdits`, `default`. Wired into all 6 phase runners (intent, scout, triage, tdd, build, audit) via `go/internal/phaseflags/`. Only Claude drivers honor it (claude-p, claude-tmux); Codex/Gemini refuse gracefully (`plan_mode_degraded` ledger entry). | Precedence: `req.Env[EVOLVE_<PHASE>_PERMISSION_MODE]` > `profile.permission_mode` (JSON in `.evolve/profiles/<phase>.json`) > unset. `profile.extra_flags` pass through unconditionally. |
| `EVOLVE_<AGENT>_CLI` / `EVOLVE_CLI` | unset | Per-agent / global CLI selection. Precedence for primary CLI: `EVOLVE_<AGENT>_CLI` > `EVOLVE_CLI` > `profile.CLI` > `claude-tmux` (final default). | Launch flag `--cli <agent>=<cli>` is sugar over the per-agent env. See [ADR-0029](../../docs/architecture/adr/0029-cli-fallback-chain-and-per-agent-overrides.md), [cli-capability-matrix](./cli-capability-matrix.md). |
| `EVOLVE_<AGENT>_MODEL` | unset | Per-agent model override. `--model <agent>=<model>` launch flag is sugar over it. | Today `--model` keys use phase names; `--cli` keys use agent names (ADR-0029 §G2 note). |
| `evolve bridge send` (CLI, not env) | n/a | Facet A live injection: queue a live command for a running `*-tmux` agent — `evolve bridge send --workspace=DIR --agent=NAME [--kind=command\|interrupt\|nudge\|system_rule\|keystroke] [--source=cli] <body>`. Driver drains its inbox from the artifact-wait poll loop: `command`/`nudge`/`system_rule` idle-gated (wait for prompt marker), `interrupt` sends ESC first, `keystroke` → raw `tmux send-keys`. Cursor seeks EOF on launch (no backlog replay). Go-tmux-only. | See [ADR-0023](../../docs/architecture/adr/0023-live-injection-and-launch-rules.md). |
| `EVOLVE_SYSTEM_PROMPT` / `EVOLVE_<AGENT>_SYSTEM_PROMPT` | unset | Facet B: per-agent system-level rules prepended as a `## Rules` block to the prompt at launch (CLI-agnostic). | Precedence: `EVOLVE_<AGENT>_SYSTEM_PROMPT` > `EVOLVE_SYSTEM_PROMPT` > profile `system_prompt` > read(profile `system_prompt_file`) > none. Prompt order: rules < policy < body. See ADR-0023. |
| `EVOLVE_BRIDGE_RECIPE_DIR` | embedded | Override directory for interaction recipes (`go/internal/bridge/recipe/`); else uses embedded recipes. | See [ADR-0031](../../docs/architecture/adr/0031-recipe-engine-and-capability-catalog.md). |
| `EVOLVE_BRIDGE_CATALOG_DIR` | `.evolve/bridge-catalogs` | Override directory for the per-CLI capability catalog (`go/internal/bridge/capabilities/`); consulted before the embedded set. | See ADR-0031, [cli-capability-matrix](./cli-capability-matrix.md). |
| `EVOLVE_AGY_REQUIRE_FULL` | `0` | `1`: `agy.sh` exits 99 if neither `agy` nor `claude` binary found (mirrors `EVOLVE_GEMINI_REQUIRE_FULL`). | Default: graceful degradation. |
| `EVOLVE_AGY_BINARY` | unset | Testing seam: override the `agy` binary path. Honored only when `EVOLVE_TESTING=1`. | Used by ACS predicates to force NATIVE/DEGRADED. |
| `EVOLVE_GEMINI_BINARY` / `EVOLVE_CODEX_BINARY` | unset | Testing seam (gated by `EVOLVE_TESTING=1`): override PATH-detected binary for gemini/codex adapters. Empty value simulates "no binary found". | See [ADR-0003](../../docs/architecture/adr/0003-true-native-cli-invocation.md). |
| `EVOLVE_ANTHROPIC_BASE_URL` | unset | When set, exported as `ANTHROPIC_BASE_URL` before every `claude -p`. Proxy-agnostic: target must speak Anthropic Messages API (`POST /v1/messages`). NOT required for subscription auth (`claude -p` reads `~/.claude.json` OAuth natively). Use only for custom endpoints (LiteLLM, corporate gateway). | Example: `http://127.0.0.1:4000/v1` (LiteLLM). `hermes proxy start` does not exist — do not use. Run `evolve doctor` to detect auth mode. |

## Budget & cost

| Env var / flag | Default | Effect | Rollback / notes |
|---|---|---|---|
| `EVOLVE_MAX_BUDGET_USD` | `999999` | Per-invocation budget override. Highest priority over `budget_tiers` and profile defaults. | — |
| `EVOLVE_BATCH_BUDGET_CAP` | `20.00` | Cumulative USD ceiling across cycles in a single dispatcher invocation. | Trips with `DISPATCH_RC=4`. |
| `--budget-usd N` (CLI flag) | unset | Primary stop condition. | Exits with `stop_reason=budget`. |
| `EVOLVE_BUILDER_COST_THRESHOLD` | `2.00` | `gate_build_to_audit` appends an audit defect on overrun. | `EVOLVE_BUILDER_COST_GUARD_STRICT=1` for fail-fast. |
| `EVOLVE_CHECKPOINT_AT_PCT` | `95` | Pre-emptive checkpoint at cumulative cost %. `--resume` reads it. | See [checkpoint-resume](../../docs/architecture/checkpoint-resume.md). |

## Observability & stall detection

| Env var | Default | Effect | Rollback / notes |
|---|---|---|---|
| `EVOLVE_TRACKER_ENABLED` | `0` (opt-in) | `1`: replays NDJSON via `tracker-writer.sh` post-phase. | v10.6+ candidate flips default-on. See [phase-tracker](../../docs/architecture/phase-tracker.md). |
| `EVOLVE_OBSERVER_AUTOSPAWN` | `1` (default-on, cycle-122 Fix 3 / ADR-0030) | `1` (or unset): `evolve loop`'s orchestrator wires `core.WithObserver(observer.NewCoreAdapter())` so a per-phase stall detector spawns from `RunCycle` automatically — restoring pre-v12 bash-dispatcher behavior dropped between v12.0.0 and cycle-122. | `=0` falls back to `noopObserver{}` (byte-identical to pre-fix; rollback hatch). See [ADR-0030](../../docs/architecture/adr/0030-phase-observer-autospawn-in-evolve-loop.md). |
| `EVOLVE_OBSERVER_ENFORCE` | `1` (default-on since v10.18.0) | `1`: the standalone `evolve phase-observer` subcommand kills its watched subagent (SIGTERM) on stall. Consulted by the spawned observer for kill-vs-log behavior. | `=0` log-only. **CORRECTION (cycle-122):** the auto-spawn path for `evolve loop` is `EVOLVE_OBSERVER_AUTOSPAWN`, not this var (false from v12.0.0–cycle-122). |
| `EVOLVE_OBSERVER_STALL_S` | `600` | Primary stall threshold for phase-observer (manual subcommand AND auto-spawned adapter per ADR-0030). | Bridges from `EVOLVE_INACTIVITY_THRESHOLD_S` (DEPRECATED). |
| `EVOLVE_OBSERVER_POLL_S` | `5` | Stat-poll interval (s) for the auto-spawned observer's stdout-log growth check. | — |
| `EVOLVE_OBSERVER_NUDGE_S` | `0` (opt-in) | `>0` and `NUDGE_S <= idle < STALL_S`: phase-observer appends ONE `nudge` envelope to the agent inbox (`<workspace>/.bridge-inbox/<agent>.ndjson`) before the hard SIGTERM; a draining `*-tmux` driver injects it to prompt summarize+continue or finalize. Inert for headless drivers. | Body overridable via `EVOLVE_OBSERVER_NUDGE_BODY`. See ADR-0023. |
| `EVOLVE_STDOUT_FILTER` | `on` (v12.2+) | After each phase the runner writes `<phase>-stdout.clean.txt` next to raw `<phase>-stdout.log` — human-readable trace (~8–20% of raw): `stream_event.*` redraw noise dropped, hook envelopes one-lined, tool_result middle-truncated, `assistant.thinking.signature` stripped. Raw file BYTE-FOR-BYTE UNCHANGED (cyclecost.go + phaseobserver read raw). Best-effort; failures WARN and never block. | `off` to skip. Impl: `go/internal/logfilter/`. Measured cycle-106: 200 KB → 15 KB (7.6%). |
| `FinalVerdict` (orchestrator output field, not env) | `PASS\|FAIL\|WARN\|SHIPPED_VIA_BUILD\|SKIPPED_AUDIT_ADVISORY\|SKIPPED_UNKNOWN` (v12.2+) | Per-cycle JSON outcome label. Disambiguates the old bare `SKIPPED`: HEAD moved during cycle → `SHIPPED_VIA_BUILD`; retro decision text contains `would-have-blocked` → `SKIPPED_AUDIT_ADVISORY`; else `SKIPPED_UNKNOWN`. PASS/FAIL/WARN unchanged. | Impl: `Orchestrator.finalizeOutcome` in `go/internal/core/orchestrator.go`. Source incident: cycle-107. |
| `cycle_label` (LedgerEntry field, not env) | unset | Manual operator entries (release audits, ad-hoc events) MUST use `"cycle": 0` + `"cycle_label": "<semantic>"` (e.g. `"manual-release-v10.16.0"`). Numeric `cycle` is exclusively the integer cycle sequence. | Legacy `"cycle": "<string>"` entries absorbed by a defensive unmarshaler in `go/internal/core/ports.go` (string cycles → `CycleLabel`, `Cycle=0`). On-disk ledger never rewritten (preserves SHA256 hash chain). |

## Context window

| Env var | Default | Effect | Rollback / notes |
|---|---|---|---|
| `EVOLVE_CONTEXT_AUTOTRIM` | `0` (opt-in) | `1`: head-60% / tail-35% prompt trim above `EVOLVE_PROMPT_MAX_TOKENS=30000`. | `=0` disables. See [context-window-control](../../docs/architecture/context-window-control.md). |
| `EVOLVE_PROMPT_MAX_TOKENS` | `30000` | Token threshold above which autotrim activates. | Only meaningful with `EVOLVE_CONTEXT_AUTOTRIM=1`. |

## Fan-out / parallelism

| Env var | Default | Effect | Rollback / notes |
|---|---|---|---|
| `EVOLVE_FANOUT_ENABLED` | `0` (off) | Parallel sub-personas. | `=1` to enable. Builder/Intent/Orchestrator/TDD excluded by `parallel_eligible` (single-writer invariant). See [sequential-write-discipline](../../docs/architecture/sequential-write-discipline.md). |
| `EVOLVE_FANOUT_CONCURRENCY` | `2` | Concurrency cap for fan-out workers. | — |
| `EVOLVE_FANOUT_PER_WORKER_BUDGET_USD` | `0.20` | Per-worker budget cap. | — |

## Sandbox & worktree

| Env var | Default | Effect | Rollback / notes |
|---|---|---|---|
| `EVOLVE_SANDBOX` | (see note) | `1`: runner wraps `claude -p` in `sandbox-exec` (macOS) / `bwrap` (Linux). Auditor/Evaluator profiles run `read_only_repo: true`. | — |
| `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` | (auto for nested) | `1`: REQUIRED for nested-claude; auto-enabled by `evolve-loop-dispatch.sh` via `detect-nested-claude.sh` when the bash rollback hatch fires. | — |
| `EVOLVE_INNER_SANDBOX` | auto | `false` when nested-Claude detected; force enable/disable with `1`/`0`. | `EVOLVE_FORCE_INNER_SANDBOX` deprecated. |
| `EVOLVE_WORKTREE_BASE` | auto | Resolved by `preflight-environment.sh`: `.evolve/worktrees/` > `$TMPDIR/evolve-loop/<hash>`. | Never auto-enable `EVOLVE_SKIP_WORKTREE=1` (operator-only emergency hatch). |
| `EVOLVE_SKIP_WORKTREE` | `0` | Operator-only emergency hatch to skip per-cycle worktree provisioning. | Do not auto-enable. |

## Ship / commit gate

| Env var | Default | Effect | Rollback / notes |
|---|---|---|---|
| `evolve ship` (native, no flag) | always native | The ship phase runs native Go (`go/internal/phases/ship/`): self-SHA TOFU, audit-binding, EGPS gate, atomic commit+ff-merge+push, gh release. CLI: `evolve ship [--class cycle\|manual\|release\|trivial] [--dry-run] "<msg>"`. | The `EVOLVE_NATIVE_SHIP=0` shell-out to a bash `ship.sh` was removed in the Go-only consolidation; ship is native-only. Parity history: `go/internal/phases/ship/native_test.go` pins commit-message footers, exit codes, and ledger semantics. |
| `EVOLVE_BYPASS_COMMIT_GATE` | `0` (gate on, v13.0.0+) | `evolve ship --class manual` is the single chokepoint for interactive commits (bare `git commit` ship-gate-denied); requires a fresh commit-gate attestation `.commit-gate/attestation.json` whose `tree_state_sha == sha256(git diff HEAD)`. Produce with `/commit`. Missing/malformed/stale → ship refuses (`IntegrityError`). `--dry-run` exempt. | `=1` skips the check (routine use is a CLAUDE.md violation). Impl: gate `evolve commit-gate run` (`go/internal/commitgate/`); enforcement `go/internal/phases/ship/commitgate.go`. See [commit skill](../../skills/commit/SKILL.md). |
| `EVOLVE_SHIP_AUTO_CONFIRM` | unset | `1`: CI mode for `--class manual` — skips the interactive y/N confirm. | — |
| `EVOLVE_BYPASS_SHIP_VERIFY` | `0` | `1`: permanent compatibility bridge that bypasses ship-gate verification (bare `git push origin main` is denied since v8.13.0). | Emits deprecation WARN — prefer `--class manual`. |
| `EVOLVE_FORCE_FRESH` | `0` (guard on) | A fresh `evolve loop` REFUSES (exit 2, `stop_reason=unfinished_cycle`) when an unfinished cycle is detected (`cycle-state.json` ahead of `lastCycleNumber`, OR unreadable). Prints the resume∥reset fork. Resolve via `evolve loop --resume` or `evolve cycle reset` (seals: archives workspace + snapshot + `reset-manifest.json` to `.evolve/runs/cycle-<N>.reset-<ts>/`, advances `lastCycleNumber`, ledger entry). | `=1` restores prior silent-clobber (history NOT sealed). Impl: `unfinishedCycle` + guard in `go/cmd/evolve/cmd_loop.go`; `core.SealCycle` in `go/internal/core/reset.go`. |

## Research tool

| Env var | Default | Effect | Rollback / notes |
|---|---|---|---|
| `EVOLVE_RESEARCH_CACHE_ENABLED` | `0` (opt-in) | Adds `state.json:researchCache.entries[<sha>]`. Cache hit when fingerprint matches + `cycle - produced_at_cycle ≤ EVOLVE_RESEARCH_CACHE_MAX_AGE`. | — |
| `EVOLVE_RESEARCH_CACHE_MAX_AGE` | `5` | Max cycle age for a research-cache hit. | Only with `EVOLVE_RESEARCH_CACHE_ENABLED=1`. |
| `EVOLVE_ALLOW_DEEP_RESEARCH` | `0` | `1`: lifts per-agent quota cap; records `deep_overrides` counter. Does not disable hook telemetry. | See [research-tool](../../docs/architecture/research-tool.md). |
| `EVOLVE_RESEARCH_QUOTA_SOFT` | *(planned)* | Soft quota: allows over-quota web calls but emits WARN in guards.log. | Not yet implemented in `research-quota-gate.sh` as of cycle-89. |
| `EVOLVE_RESEARCH_HOOK_DISABLED` | `0` | `1`: `research-quota-gate.sh` is a no-op but counters still increment (telemetry-only). | — |
| `EVOLVE_KB_SEARCH_PATHS` | `knowledge-base/research/:.evolve/instincts/lessons/:docs/research/` | Colon-separated roots for the native knowledge-base search (`go/internal/research`). | — |

## Non-env reference (paths, output labels, classes)

| Item | Value / contract | Notes |
|---|---|---|
| Runtime location | `go/internal/...` (Go-only) | The runtime is the Go binary; the bash `legacy/scripts/` tree was removed in the Go-only consolidation. A few shell helpers remain only as test fixtures + the commit-gate runner. |
| Ship class `cycle` (default) | Full audit-binding: recent PASS, SHA match, HEAD/tree bound, `acs-verdict.json` red_count==0 | `/evo:loop` cycle commits. |
| Ship class `manual` | Skips audit-binding; requires fresh commit-gate attestation (bypass `EVOLVE_BYPASS_COMMIT_GATE=1`); interactive y/N (CI: `EVOLVE_SHIP_AUTO_CONFIRM=1`) | Operator-driven manual commits. |
| Ship class `release` | Skips audit (version-bump mutates files post-audit); logs RELEASE class loudly | `evolve release` pipeline only. |

### Operator read-only commands

- `evolve guard list-audit-fails [--evolve-dir DIR] [--json]` — enumerate `state.json:failedApproaches[]` with `classification=code-audit-fail` still within the 30-day retention window. Pure read.
- `evolve eval diversity-check <evalsDir> [slug]` — suite-level adversarial-diversity check (v13.0.0+). Exit `0` PASS, `1` WARN, `2` HALT, `10` bad args. Companions: `evolve eval quality-check <eval.md>` (Level-0 tautology), `evolve eval verify <eval.md> <workspace>` (independent re-execution). Read-only.
- `evolve setup detect [--json]` — onboarding digest (read-only): per-CLI binary/auth-mode/capability-tier/verdict + per-phase routing + envelope/cross-family/allowed_clis constraints, with `.evolve/policy.json` pins overlaid and any floor breach reported as `pin_violation`. (Step 9b removed `evolve setup validate` + `llm_config.json`; the clamp now lives in `policy.ValidatePin`, surfaced by `detect` and hard-enforced at dispatch.) See [setup-onboarding](../../docs/architecture/setup-onboarding.md), ADR-0027.

### Operator verdict-writing commands

- `evolve acs suite --cycle N [--root .] [--evolve-dir .evolve]` — runs the bash EGPS predicate suite (cycle-N + regression-suite + red-team), writes `<evolve-dir>/runs/cycle-N/acs-verdict.json`; exit `2` on any RED, `0` all green. `evolve acs run --cycle N <pkg>` runs `go test -json` on a package into the same verdict file.

> **Session cost isolation (v10.8.0+):** `claude -p` subagent invocations bill to the OAuth session that launched the dispatcher (the parent Claude Code session), not the batch budget meter. Run `/clear` before starting a new evolve-loop batch to isolate costs. The batch meter (`state.json:currentBatch.cycleAccruedCostUSD`) tracks per-cycle accumulation but cannot capture OAuth session charges.
