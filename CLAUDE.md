# Project Instructions

> **Cross-CLI canonical instructions are in [AGENTS.md](AGENTS.md).** This file (CLAUDE.md) is the Claude Code-specific overlay — runtime hooks, plugin manifest details, version-specific notes, and incident history. Read AGENTS.md first for the universal pipeline contract; come here for Claude Code runtime specifics.

> **Per-version release notes index**: [docs/operations/release-notes/](docs/operations/release-notes/index.md) — quick navigation to specific version sections + complete chronology in [CHANGELOG.md](CHANGELOG.md).

> This file is read by AI coding assistants. Platform equivalents: `CLAUDE.md` (Claude Code), `GEMINI.md` (Gemini CLI), `AGENTS.md` (generic). Content is platform-agnostic.

## Autonomous Execution

If the user is in autonomous mode (bypass permissions / yolo mode / auto-approve), YOU MUST:

1. **Continue all cycles without pausing** — complete every requested cycle end-to-end without stopping to ask for user approval. Never output "should I continue?" or wait for confirmation between cycles.
2. **Run the FULL pipeline every cycle** — launch Scout, Builder, and Auditor agents for every cycle. No shortcuts, no inline edits that skip agents, no batch-claiming multiple cycles. Each cycle must produce workspace artifacts (scout-report.md, build-report.md, audit-report.md).
3. **Run phase-gate.sh at every transition** — the deterministic phase gate script (`scripts/lifecycle/phase-gate.sh`) must execute at every phase boundary. This is non-negotiable even in bypass mode. Bypass permissions means "don't ask the user" — it does NOT mean "skip integrity checks."
4. **Never fabricate cycles** — claiming cycle numbers without doing work is a CRITICAL violation. Every cycle number must correspond to real Scout → Build → Audit → Ship → Learn execution.
5. **Phase agents MUST be invoked via `scripts/dispatch/subagent-run.sh`** — the in-process `Agent` tool is forbidden in production cycles. v8.21.0 made this **structurally enforced**: `Agent` is denied in `orchestrator.json:disallowed_tools` AND blocked at the kernel layer by `phase-gate-precondition.sh` whenever `cycle-state.json` exists. There is no bypass. The runner enforces per-agent CLI permission profiles in `.evolve/profiles/` (least-privilege isolation), generates a per-invocation challenge token, and writes a tamper-evident ledger entry that `phase-gate.sh check_subagent_ledger_match` verifies against the on-disk artifact's SHA256.
6. **OS-level sandboxing wraps every claude subprocess.** When `EVOLVE_SANDBOX=1` (or profile `sandbox.enabled: true`), the runner wraps `claude -p` in `sandbox-exec` (macOS) or `bwrap` (Linux). Anthropic's Secure Deployment Guide states `--allowedTools` is "a permission gate, not a sandbox" — so kernel-layer enforcement is non-negotiable in production. Auditor and Evaluator profiles run with `read_only_repo: true` (read-only mount of the repo). **`EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` is REQUIRED for nested-claude scenarios** (the primary use case for `/evolve-loop`, where the slash command runs from inside Claude Code's own sandbox). On Darwin 25.4+, `sandbox_apply()` returns EPERM when the parent process is itself sandboxed — the flag instructs `claude.sh` to retry the inner subagent unsandboxed, while kernel hooks (role-gate, ship-gate, phase-gate-precondition) remain active and enforce trust boundaries structurally. **`scripts/dispatch/evolve-loop-dispatch.sh` auto-enables the flag** via `scripts/dispatch/detect-nested-claude.sh` whenever `CLAUDECODE` / `CLAUDE_CODE_*` env vars are present (defense-in-depth alongside `SKILL.md`). The v8.21 deprecation of this flag was reverted in v8.22.0 after evidence showed the worktree-provisioning fix and the EPERM fallback target *orthogonal* layers (builder writes vs sub-claude startup).
7. **Mutation testing pre-flight on every eval.** `phase-gate.sh gate_discover_to_build` runs `scripts/verification/mutate-eval.sh` against each new eval definition. If kill rate < 0.8, eval is flagged as tautological. Rollout phase 1: WARN-only logging. Rollout phase 2 (after one verification cycle): fail the gate.
8. **Adversarial Auditor mode is default-on.** The runner prepends an "ADVERSARIAL AUDIT MODE" framing to the auditor's prompt that requires positive evidence for PASS verdicts. Auditor model defaults to Opus (different family from Builder's Sonnet) to break same-model-judge sycophancy. Disable only with `ADVERSARIAL_AUDIT=0` for deliberately permissive sweeps.

**The rule is: maximum velocity, zero shortcuts.** Go fast by being efficient, not by skipping steps.

**Runtime constraints (current behavior):** Per-cycle git worktrees are provisioned by `run-cycle.sh` (recorded in `cycle-state.json:active_worktree`). Orchestrator and all phase agents MAY NOT call `git worktree add/remove` (denied by profiles). The failure-adapter (`scripts/failure/failure-adapter.sh`) computes a deterministic PROCEED/RETRY/BLOCK decision from `state.json:failedApproaches[]` — orchestrator reads this JSON and follows it verbatim. Ledger entries include `prev_hash` + `entry_seq` for tamper-evident hash-chaining; verify with `bash scripts/observability/verify-ledger-chain.sh`. Audit `Verdict: WARN` ships by default (set `EVOLVE_STRICT_AUDIT=1` to block). `ship.sh` advances `state.json:lastCycleNumber` after every successful cycle ship.

> For implementation details and version history (v8.21–v8.37), see [docs/operations/release-archive.md](docs/operations/release-archive.md).

## Auto-Retrospective on FAIL/WARN (v8.45.0+)

Reverses the pre-v8.45 "batched per v8.12.3" design where Retrospective never fired automatically — failures got recorded as raw `state.json:failedApproaches[]` entries (single-loop) but the structured lesson YAML (double-loop, per Argyris & Schon 1978) never got produced.

Post-v8.45.0 orchestrator flow:

| Verdict | Pre-v8.45 | Post-v8.45 |
|---|---|---|
| PASS | ship → learn | ship → learn (unchanged) |
| WARN (fluent default) | record + ship | record + ship + **retrospective + merge-lesson** |
| FAIL | record-only | record + **retrospective + merge-lesson** |

The retrospective subagent runs inline. Cost: ~$0.30-0.50 per FAIL/WARN cycle (Sonnet model per `.evolve/profiles/retrospective.json`). Lesson YAML is written to `.evolve/instincts/lessons/<id>.yaml` and merged into `state.json:instinctSummary[]` so the next cycle's Scout/Builder/Auditor see it.

**Operator opt-out**: `EVOLVE_DISABLE_AUTO_RETROSPECTIVE=1` reverts to pre-v8.45 record-only behavior. Useful for tight cost-control deployments.

**Why this matters**: completes the Reflexion-style verbal-RL loop (Shinn et al. 2023). Pre-v8.45 the failure-recording side worked but the reflection side never fired. Post-v8.45 the loop is structurally complete: failure → reflection → instinct → next-cycle-input.

**Kernel changes**: `scripts/lifecycle/phase-gate.sh` gained `gate_audit_to_retrospective`; `scripts/lifecycle/cycle-state.sh` recognizes `retrospective` as a valid phase; orchestrator profile gained `Bash(merge-lesson-into-state.sh:*)`. `scripts/guards/phase-gate-precondition.sh` already permitted `retrospective` agent during audit/ship phases — v8.45 just wires the orchestrator persona to use it.

## Checkpoint-Resume + Context-Window Control (v9.1.0+)

Two paired capabilities that handle resource exhaustion gracefully instead of losing in-flight work. Pre-v9.1.0, a cycle that hit a Claude Code subscription quota (GitHub #29579 signature: rc=1 + empty stderr after substantial work) lost the worktree, the cycle-state, and all Builder edits. Same for context exhaustion — operator had to start over.

### Checkpoint-Resume

| Layer | What it does | When it fires |
|---|---|---|
| Reactive (Cycle 3) | `_quota_likely` heuristic in `subagent-run.sh` writes a checkpoint when phase rc=1 + stderr empty + cost ≥80% of `BATCH_CAP` | Per-phase failure |
| Pre-emptive (Cycle 2) | Dispatcher exports `EVOLVE_CHECKPOINT_REQUEST=1` when cumulative cost crosses 95% (`EVOLVE_CHECKPOINT_AT_PCT`) | Per-cycle boundary |
| Operator-requested | `cycle-state.sh checkpoint operator-requested` | Manual |

When a checkpoint fires: `run-cycle.sh`'s EXIT trap preserves worktree + cycle-state + workspace artifacts. Operator types `/evolve-loop --resume` to pick up at the paused phase boundary. Trust kernel (phase-gate, role-gate, ship-gate, ledger SHA-chain) is unchanged — resume goes through the same enforcement.

### Context-Window Control

| Layer | What it does | Default |
|---|---|---|
| Per-phase autotrim | When `EVOLVE_CONTEXT_AUTOTRIM=1` AND prompt > `EVOLVE_PROMPT_MAX_TOKENS` (30k), trim head-60% + tail-35% with marker | opt-in |
| Per-cycle monitor JSON | `.evolve/runs/cycle-N/context-monitor.json` per-phase input_tokens + cumulative | always-on |
| Observability | `scripts/observability/show-context-monitor.sh <cycle>` (tabular, `--watch`, `--json`) | n/a |
| Threshold integration | At ≥95% cumulative context, sets `EVOLVE_CHECKPOINT_REQUEST=1` — same channel as cost-based pre-emption | n/a |

### v9.1.0 env-var reference

| Variable | Default | Purpose |
|---|---|---|
| `EVOLVE_CHECKPOINT_AT_PCT` | `95` | Pre-emptive trigger % (cost) |
| `EVOLVE_CHECKPOINT_WARN_AT_PCT` | `80` | Advisory WARN % (cost + context) |
| `EVOLVE_CHECKPOINT_DISABLE` | `0` | Set 1 to disable all checkpoint thresholds |
| `EVOLVE_QUOTA_DANGER_PCT` | `80` | Cost % below which reactive classification skips empty-stderr rc=1 |
| `EVOLVE_RESUME_ALLOW_HEAD_MOVED` | `0` | Set 1 to bypass HEAD-drift guard on resume |
| `EVOLVE_CONTEXT_AUTOTRIM` | `0` | Opt-in head/tail-preserving prompt trim |
| `EVOLVE_PROMPT_MAX_TOKENS` | `30000` | Per-phase prompt cap (unchanged from v8.56) |

See [docs/architecture/checkpoint-resume.md](docs/architecture/checkpoint-resume.md) and [docs/architecture/context-window-control.md](docs/architecture/context-window-control.md) for the full protocols.

## Three-Tier Strictness Model (v8.24.0+, refined v8.25.0)

evolve-loop's strictness is layered. The user-facing pain in pre-v8.24.0 came from conflating layers; v8.24.0 made the layers explicit; v8.25.0 replaced "skip the worktree" with "relocate the worktree" so isolation is preserved even in nested-Claude.

| Tier | Mechanism | Default | Auto-adapt? | What it catches |
|---|---|---|---|---|
| **1 — Structural integrity** | phase-gate, ledger SHA, role-gate, ship-gate (`scripts/guards/`) | Always on | NEVER | Reward hacking, phase-skipping, integrity breach (cycle 102–111, cycle 132–141 incidents) |
| **2 — OS isolation** | `sandbox-exec`/`bwrap`, per-cycle worktree | On (always) | Worktree path auto-selected per environment; sandbox falls back when nested | Compromised builder writing outside its sandbox; one cycle's edits leaking into another's workspace |
| **3 — Workflow defaults** | intent capture, fan-out, mutation testing, adversarial audit | Opt-in via env flags | N/A — already opt-in | Vague goals, sycophantic audits, tautological evals |

**The governing rule:** Tier 1 is non-negotiable and runs in privileged shell context (no sandbox dependency). Tier 2 *adapts to* the environment instead of *degrading* — per-cycle worktrees always exist; only the path moves. Tier 3 is operator-controlled per-run.

### Capability detection (v8.25.0+)

`scripts/dispatch/preflight-environment.sh` emits `.evolve/environment.json` (schema v3). The dispatcher reads `auto_config`:

| Field | Values | Decision rule |
|---|---|---|
| `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` | `"0"` / `"1"` | `1` when nested-Claude detected |
| `worktree_base` | absolute path | `EVOLVE_WORKTREE_BASE` > `.evolve/worktrees/` > `$TMPDIR/evolve-loop/<hash>` |
| `inner_sandbox` | `true` / `false` | `false` when nested-Claude or sandbox broken |

Operator overrides: `EVOLVE_WORKTREE_BASE=/path`, `EVOLVE_INNER_SANDBOX=1` (force-enable), `EVOLVE_INNER_SANDBOX=0` (force-disable). `EVOLVE_FORCE_INNER_SANDBOX=1` is deprecated (v8.60+); use `EVOLVE_INNER_SANDBOX=1`. **Do not auto-enable `EVOLVE_SKIP_WORKTREE=1`** — it abandons per-cycle isolation and is an operator-only emergency hatch with a loud WARN.

### Ship commit classifiers (v8.25.0+)

`scripts/lifecycle/ship.sh` requires an explicit class via `--class`:

| Class | Use case | Verification |
|---|---|---|
| `cycle` (default) | `/evolve-loop` cycle commits | Full audit-binding (must have recent PASS, SHA matches, HEAD/tree bound) |
| `manual` | Operator-driven manual commits | Skips audit; requires interactive y/N confirmation. CI mode via `EVOLVE_SHIP_AUTO_CONFIRM=1`. |
| `release` | Internal — `scripts/release-pipeline.sh` only | Skips audit (version-bump.sh mutates files post-audit); logs RELEASE class loudly |

Scripts/cron jobs using `EVOLVE_BYPASS_SHIP_VERIFY=1` continue to work (permanent compatibility bridge) but emit a deprecation warning pointing to `--class manual`.

## Release & Publish Workflow (v8.13.2+)

**"publish" ≠ "push".** See [docs/guides/publishing-releases.md](docs/guides/publishing-releases.md) for the canonical vocabulary (push, tag, release, propagate, publish, ship). When the user asks to "publish vX.Y.Z", use the self-healing pipeline:

```bash
bash scripts/release-pipeline.sh X.Y.Z              # full publish
bash scripts/release-pipeline.sh X.Y.Z --dry-run    # simulate, no mutations
bash scripts/release-pipeline.sh X.Y.Z --skip-tests # hot-fix path (CI-pre-verified)
```

The pipeline owns the entire lifecycle: pre-flight gate → version bump → auto-changelog from conventional commits → consistency check → atomic ship via `scripts/lifecycle/ship.sh` → marketplace propagation polling (5 min) → cache refresh → auto-rollback on any post-push failure.

**Bare `git push origin main` is denied by ship-gate** (since v8.13.0). Direct commits and pushes go through `scripts/lifecycle/ship.sh`. The release pipeline calls ship.sh internally; it does not bypass the gate.

### Required version markers (all auto-bumped by `version-bump.sh`)

1. `.claude-plugin/plugin.json` — canonical version (source of truth)
2. `.claude-plugin/marketplace.json` — `.plugins[0].version`
3. `skills/evolve-loop/SKILL.md` — heading `# Evolve Loop vX.Y` (only if major.minor changed)
4. `README.md` — "Current (vX.Y)" + version history row (only if major.minor changed)
5. `CHANGELOG.md` — `## [X.Y.Z]` block (auto-generated by `changelog-gen.sh`)

`scripts/utility/release.sh` is now a **consistency verifier** invoked by the pipeline. Run standalone for diagnostics: `bash scripts/utility/release.sh <version>`.

### Conventional commits

Auto-changelog buckets commits by type prefix:
- `feat:` / `feat(scope):` → `### Added`
- `fix:` → `### Fixed`
- `refactor:` / `perf:` / `stability:` → `### Changed`
- `docs:` → `### Documentation`
- `chore:` / `ci:` / `test:` / `build:` / `revert:` / `release:` → skipped
- no prefix → `### Other` (audit found ~40% of historical commits)

## Subagent Budget Controls (v8.13.4 / v8.13.5)

evolve-loop subagents have three budget-control mechanisms (highest priority first):

1. **`EVOLVE_MAX_BUDGET_USD`** (v8.13.4) — operator-controlled per-invocation override. Overrides all else.
2. **`EVOLVE_TASK_MODE` + `budget_tiers`** (v8.13.5) — declarative profile tier select. Used when the profile has a `budget_tiers` map AND the env var matches a key.
3. **`max_budget_usd`** (v8.12.x baseline) — static profile default. Always present.

Default is `--max-budget-usd 999999` (effectively unlimited, since v8.26.0). For hard cost ceilings: `EVOLVE_MAX_BUDGET_USD=<value>` (single ceiling, highest priority) or `EVOLVE_BUDGET_ENFORCE=1` (use profile-resolved per-phase caps). `EVOLVE_BUDGET_CAP` is **deprecated** (v8.60+) — it bridges to `EVOLVE_MAX_BUDGET_USD` with a stderr WARN; removal target v8.61+. The Scout profile ships with `default` / `research` / `deep` budget tiers as the canonical `EVOLVE_TASK_MODE` example.

Builder cost-overrun guard (v8.60+): `phase-gate.sh gate_build_to_audit` reads `builder-usage.json` against `EVOLVE_MAX_BUDGET_USD` (or `EVOLVE_BUILDER_COST_THRESHOLD`, default `$2.00`) and appends an audit-visible defect on overrun. Set `EVOLVE_BUILDER_COST_GUARD_STRICT=1` to fail-fast instead of WARN.

### Per-batch cumulative cap (v8.58.0+, Layer B)

Per-cycle caps remain unlimited by default (preserving v8.26's friction-free reasoning), but the dispatcher now tracks a **cumulative batch total** with a tripwire. After each cycle's cost is logged via `show-cycle-cost.sh --json`, the dispatcher accumulates `total.cost_usd` into `BATCH_TOTAL_COST` and checks it against `EVOLVE_BATCH_BUDGET_CAP` (default `20.00`). When exceeded, the batch breaks early with `DISPATCH_RC=4` — completed cycles are reported, remaining cycles skipped.

| Variable | Default | Effect |
|---|---|---|
| `EVOLVE_BATCH_BUDGET_CAP` | `20.00` | USD ceiling for cumulative cycle costs in a single dispatcher invocation |
| `EVOLVE_BATCH_BUDGET_DISABLE` | `0` | Set `1` to disable the tripwire (preserve pre-v8.58 unlimited-batch posture) |

The summary line emits `batch_total_cost=$X.XX / cap=$Y.YY` so post-run forensics is one grep away. Cycle telemetry that fails to produce `show-cycle-cost.sh` JSON contributes `$0` to the running total (best-effort accumulation; never blocks a legitimate cycle on a missing sidecar).

**User-stated budget (v8.60.0+, Layer 1):** Pass `--budget-usd N` (or `--budget N`) to use dollar spend as the primary stop condition. The dispatcher runs cycles until cumulative cost ≥ $N, then exits with `stop_reason=budget`. `EVOLVE_BATCH_BUDGET_CAP` remains the hard system ceiling — if `--budget-usd 50` is passed but `EVOLVE_BATCH_BUDGET_CAP=10`, the effective cap is $10 (`stop_reason=batch_cap`).

**Cycle→cost migration status (v9.0.5):**

| Surface | State |
|---|---|
| `--budget-usd N` / `--budget N` / `--cycles N` dispatcher flags | ✅ shipped v8.60.0 |
| `stop_reason=budget` cumulative-cost tripwire | ✅ shipped v8.60.0 |
| SKILL.md Quick Start budget-first framing | ✅ v9.0.5 |
| SKILL.md `argument-hint` advertises both modes | ✅ shipped v8.60.0 |
| Positional integer (bare `/evolve-loop 3 ...`) | ⚠️ still parses as **cycles** with deprecation WARN. v10.0.0 candidate will consider flipping to dollars (breaking change — warrants a major-version-bump signal). Prefer the explicit flag (`--cycles N` or `--budget-usd N`) to be flip-safe. |

> For detailed usage examples and forward-compatibility notes, see [docs/operations/release-archive.md](docs/operations/release-archive.md).

## Verification Before Claiming Done (v8.13.3+)

Three patterns the /insights audit identified as recurring friction. Apply ALL of them before reporting a task complete:

1. **Probe before declaring a CLI unavailable.** Never say "no `<tool>` command" without first running:
   ```bash
   bash scripts/utility/probe-tool.sh <tool>      # canonical helper, checks PATH + common install dirs
   # or directly:
   command -v <tool> 2>/dev/null || which <tool> 2>/dev/null || ls /usr/local/bin/<tool> ~/.local/bin/<tool> 2>/dev/null
   ```
   The audit caught one instance where Claude said "no gws command" when `gws` was installed at `~/.local/bin/`. List what you checked in your response.

2. **Read actual exports before implementing against a module.** When working in a worktree or generating code that imports from `module X`, run `Read` on `X` first and list its real exports. Do not invent function names from context. The audit caught Builder agents shipping code against imagined APIs that didn't match `enhancedAdaptive.ts`'s actual exports — requiring full rewrites.

3. **Run tests after multi-file refactors and report pass/fail counts.** A claim of "tests pass" without explicit numbers is unverified. Format: `bash scripts/<suite>.sh — N/N PASS, no regression`.

If any of the three doesn't apply (e.g., no test infra exists), say so explicitly rather than skipping.

## Shell & Environment Conventions (v8.13.3+)

This project's shell scripts target **bash 3.2** (macOS default) for portability. Multiple regressions in /insights traced to bash-4-only features.

### Banned in shell scripts:

- `declare -A` (associative arrays — bash 4+)
- `mapfile` / `readarray` (bash 4+)
- `${var^^}` / `${var,,}` case modifications (bash 4+)
- GNU-only sed flags: `sed -i ''` is BSD-incompatible — write to a `.tmp.$$` file and `mv` instead
- GNU-only `date -d` — use `date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$ts" +%s` on macOS, fallback chain `gdate || date -d || date -j -f` for portability

### Required for shell scripts:

- `set -uo pipefail` (NOT `set -e` for orchestrator scripts that need to capture sub-script exit codes — `set -e` interacts badly with `if !cmd; then; rc=$?` patterns where `rc` ends up 0)
- Atomic writes via mv-of-temp: `printf ... > "${file}.tmp.$$" && mv -f "${file}.tmp.$$" "$file"`
- `git diff HEAD` to capture tree-state SHAs (untracked files don't count — match the audit-binding model)
- Since v8.42.0, `skills/<name>/` directories are **symlinks** to `../.agents/skills/<name>/`. Git tracks content changes at the `.agents/` canonical path — this is expected. Auditors verifying SKILL.md edits should diff `git diff HEAD -- .agents/skills/evolve-loop/SKILL.md`, not the symlink path.

### SSE / streaming endpoints (when you encounter them):

- Disable middleware buffering explicitly
- Add explicit timeouts
- Provide a cancel-UI button — don't rely on browser-side timeout alone

The audit caught the wiki Add Topics feature where SSE buffering blocked article generation; resolution required all three.

## Confirm Direction Before Multi-Cycle Work (v8.13.3+)

For ambiguous requests like "research X and integrate into Y" or "design a system for Z", produce a **3-bullet plan with success criteria** before invoking any tools beyond Read/Grep. Wait for user confirmation before proceeding.

Multi-cycle evolve-loop runs (≥5 cycles) MUST do this. The audit identified the "giant useless circle" force-graph as a case where Claude built for 25 cycles in the wrong direction; a 3-bullet pre-check would have caught it in <5 minutes.

Format:

```
Direction:
- <approach in one sentence>
- <key decision and tradeoff>
- <what success looks like, measurably>

Proceed? (yes/redirect)
```

This rule does NOT apply to:
- Single-cycle bug fixes
- Tasks where the user has already specified file paths and behavior
- Tasks invoked from a previously-approved plan (the plan is the confirmation)

## Cross-Platform Behavior (v8.15.0+)

evolve-loop ships as a Claude Code plugin but is invocable from Gemini CLI via a hybrid driver. The skill content is platform-neutral; the runtime requires the `claude` binary regardless of which CLI initiated the conversation.

### How dispatch works per CLI

| Caller | Skill activation | `cli` field in profile | Adapter | Engine |
|---|---|---|---|---|
| Claude Code | `Skill` tool | `claude` | `scripts/cli_adapters/claude.sh` | `claude -p` (sandboxed) |
| Gemini CLI | `activate_skill` | `gemini` | `scripts/cli_adapters/gemini.sh` (hybrid shim) | delegates to `claude.sh` → `claude -p` |
| Codex CLI | hybrid (v8.51.0+) | `codex` | `scripts/cli_adapters/codex.sh` | delegates to `claude.sh` when claude on PATH; DEGRADED same-session mode otherwise |

The hybrid pattern exists because Gemini CLI lacks non-interactive prompt mode, `--max-budget-usd`, and subagent dispatch — so trust-boundary hooks can't structurally enforce phase isolation. Delegating to `claude.sh` gives Gemini-driven cycles full kernel-hook protection. See [docs/incidents/gemini-forgery.md](docs/incidents/gemini-forgery.md).

### Where the platform-specific knowledge lives

- `docs/architecture/platform-compatibility.md` — top-level support matrix and adapter contract
- `skills/evolve-loop/reference/platform-detect.md` — env-var probe table for runtime detection
- `skills/evolve-loop/reference/<platform>-tools.md` — tool name translation (`Read` → `read_file`, etc.)
- `skills/evolve-loop/reference/<platform>-runtime.md` — invocation patterns per CLI
- `scripts/dispatch/detect-cli.sh` — shell helper that returns one of `claude | gemini | codex | unknown`
- `scripts/cli_adapters/<cli>.sh` — runtime adapter; receives env-var contract from `subagent-run.sh`

Mirror the hybrid pattern (delegate to `claude.sh`) before attempting a native adapter. The native-adapter path requires: non-interactive prompt mode, profile-scoped permissions, and either a budget cap flag or external cost tracking.

## Swarm Architecture (v8.16+)

evolve-loop is structured around three composable layers (see [docs/architecture/tri-layer.md](docs/architecture/tri-layer.md) for the formal spec):

| Layer | Files | Purpose |
|---|---|---|
| **Skill** | `skills/<name>/SKILL.md` | Workflow + steps + exit criteria — the *how* |
| **Persona** | `agents/<role>.md` | One role, one perspective, one output format — the *who* |
| **Command** | `.claude-plugin/commands/<name>.md` | User-facing entry point — the *when* (orchestration) |

**The governing rule:** the user (or a slash command) is the orchestrator. **Personas do not invoke other personas.** Claude Code enforces this at runtime: subagents cannot spawn subagents.

### Fan-out (Sprint 1, Pattern-3)

Scout, Auditor, Retrospective, Plan-reviewer, Evaluator, and Inspirer can fan out into parallel sub-personas. Builder, Intent, Orchestrator, and TDD-engineer are **excluded** — single-writer invariant on shared state. The exclusion is structurally enforced (v8.55.0+) via `parallel_eligible` in profile JSON; `cmd_dispatch_parallel` rejects with exit 2 otherwise. See [docs/architecture/sequential-write-discipline.md](docs/architecture/sequential-write-discipline.md) for the full rule, role taxonomy, and the why.

> **Production posture (v8.55.0+):** Keep `EVOLVE_FANOUT_ENABLED=0` (the default) until v8.56 lean-cycle reduces baseline cost. The discipline + concurrency cap + per-worker budget cap rails ship in v8.55.0 so the feature is *defensibly disable-able*; operators who want speed at extra cost may opt in with the flags below, but the canonical operational mode remains sequential single-writer. Run the cycle-55 verification protocol (see `docs/architecture/sequential-write-discipline.md` "Operational posture") before flipping any flag in production.

| Flag | Default | Effect |
|---|---|---|
| `EVOLVE_FANOUT_ENABLED` | `0` | Master switch |
| `EVOLVE_FANOUT_SCOUT` / `_AUDITOR` / `_RETROSPECTIVE` | `0` | Enable fan-out per phase |
| `EVOLVE_FANOUT_CONCURRENCY` | `2` (was `4` pre-v8.55) | Max parallel workers in flight; lowered to halve peak token-burn rate so subscription quotas survive multi-hour `/loop` runs. Operators on API plans bump to `4`+ explicitly. |
| `EVOLVE_FANOUT_PER_WORKER_BUDGET_USD` | `0.20` (v8.55.0+) | Per-worker $ cap auto-injected as `EVOLVE_MAX_BUDGET_USD` when operator hasn't set one. Total fan-out spend ≤ `concurrency × cap × ceil(subtasks/concurrency)`. Operator-set `EVOLVE_MAX_BUDGET_USD` always wins. |
| `EVOLVE_FANOUT_CANCEL_ON_CONSENSUS` | `0` | Cancel remaining workers when K agree on FAIL |
| `EVOLVE_FANOUT_CACHE_PREFIX` | `1` | Write shared cache-prefix.md for prompt-cache hit on siblings (~47% token reduction) |

### Triage default-on (v8.59.0+)

The cycle-scope Triage phase (Layer C) now runs on **every** cycle unless the operator opts out with `EVOLVE_TRIAGE_DISABLE=1`. Promotion path follows the v8.55 default-off→verify→default-on→enforce ladder:

| Version | Default | Enforcement |
|---|---|---|
| v8.59.0 | **on** (this version) | Soft WARN in `gate_discover_to_build` if Triage skipped without `EVOLVE_TRIAGE_DISABLE=1` |
| v8.60+ candidate | on | Promote WARN to FAIL after one verification cycle confirms orchestrator follows |

Operator overrides:
- `EVOLVE_TRIAGE_DISABLE=1` — opt out (e.g., for tight cost-control runs where the extra ~$0.50/cycle is unacceptable)
- The `cycle_size_estimate=large` block in `gate_triage_to_plan_review` remains active when Triage is on — operators must split before re-entering.

### Plan review (Sprint 2)

`EVOLVE_PLAN_REVIEW=1` enables a four-lens review (CEO/Eng/Design/Security) between discover and build. Phase gate `gate_plan_review_to_tdd` enforces verdict. Default-off.

| Verdict | Trigger | Action |
|---|---|---|
| PROCEED | Avg ≥ 7 AND no lens < 5 | Advance to build |
| REVISE | Avg ≥ 5 AND any lens < 5 | Re-run Scout (max 2 retries) |
| ABORT | Any explicit ABORT OR avg < 5 | End cycle |

### Composable skill catalog (Sprint 3)

| Skill | Slash command | Pattern |
|---|---|---|
| `evolve-spec` | `/scout` | Pattern 3 fan-out |
| `evolve-plan-review` | `/plan-review` | Pattern 3 fan-out |
| `evolve-tdd` | `/tdd` | Pattern 1/2 |
| `evolve-build` | `/build` | Pattern 1/2 (single-writer) |
| `evolve-audit` | `/audit` | Pattern 3 fan-out |
| `evolve-ship` | `/ship` | Pattern 1/2 (atomic) |
| `evolve-retro` | `/retro` | Pattern 3 fan-out |
| `evolve-loop` (existing macro) | `/loop` | **Pattern 5** auto-orchestrated under trust kernel |

### Worker name pattern (`<role>-worker-<subtask>`)

Fan-out workers invoke `subagent-run.sh <role>-worker-<name>`. `cmd_run` strips the `-worker-<name>` suffix to find the parent profile. `phase-gate-precondition.sh` derives the parent role for sequence checking — workers inherit phase eligibility from their parent role.

### Verifying the swarm architecture

Run `bash scripts/tests/swarm-architecture-test.sh` to verify all three layers wire correctly (40 assertions covering plugin.json registrations, skill files, slash commands, persona files, profile parallel_subtasks, state machine, phase gate, aggregator merge modes, dispatch-parallel command, and end-to-end smoke test).

## Evolve Loop Task Priority

When selecting tasks for `/evolve-loop` cycles, follow this priority order:

1. **New features** — Building new functionality is the top priority
2. **Bug fixes** — Fixing potential bugs is second priority
3. **Security issues** — Fixing security vulnerabilities is last priority
