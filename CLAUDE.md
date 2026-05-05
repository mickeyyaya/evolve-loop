# Project Instructions

> This file is read by AI coding assistants. Platform equivalents: `CLAUDE.md` (Claude Code), `GEMINI.md` (Gemini CLI), `AGENTS.md` (generic). Content is platform-agnostic.

## Autonomous Execution

If the user is in autonomous mode (bypass permissions / yolo mode / auto-approve), YOU MUST:

1. **Continue all cycles without pausing** — complete every requested cycle end-to-end without stopping to ask for user approval. Never output "should I continue?" or wait for confirmation between cycles.
2. **Run the FULL pipeline every cycle** — launch Scout, Builder, and Auditor agents for every cycle. No shortcuts, no inline edits that skip agents, no batch-claiming multiple cycles. Each cycle must produce workspace artifacts (scout-report.md, build-report.md, audit-report.md).
3. **Run phase-gate.sh at every transition** — the deterministic phase gate script (`scripts/phase-gate.sh`) must execute at every phase boundary. This is non-negotiable even in bypass mode. Bypass permissions means "don't ask the user" — it does NOT mean "skip integrity checks."
4. **Never fabricate cycles** — claiming cycle numbers without doing work is a CRITICAL violation. Every cycle number must correspond to real Scout → Build → Audit → Ship → Learn execution.
5. **Phase agents MUST be invoked via `scripts/subagent-run.sh`** — the in-process `Agent` tool is forbidden in production cycles. v8.21.0 made this **structurally enforced**: `Agent` is denied in `orchestrator.json:disallowed_tools` AND blocked at the kernel layer by `phase-gate-precondition.sh` whenever `cycle-state.json` exists. There is no bypass. The runner enforces per-agent CLI permission profiles in `.evolve/profiles/` (least-privilege isolation), generates a per-invocation challenge token, and writes a tamper-evident ledger entry that `phase-gate.sh check_subagent_ledger_match` verifies against the on-disk artifact's SHA256.
6. **OS-level sandboxing wraps every claude subprocess.** When `EVOLVE_SANDBOX=1` (or profile `sandbox.enabled: true`), the runner wraps `claude -p` in `sandbox-exec` (macOS) or `bwrap` (Linux). Anthropic's Secure Deployment Guide states `--allowedTools` is "a permission gate, not a sandbox" — so kernel-layer enforcement is non-negotiable in production. Auditor and Evaluator profiles run with `read_only_repo: true` (read-only mount of the repo). The `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` flag (sandbox bypass on Darwin nested-sandbox EPERM) is **DEPRECATED in v8.21 and will be REMOVED in v8.22.0** — its purpose is obviated by the v8.21.0 worktree provisioning fix. Engaging it now emits loud WARN logs.
7. **Mutation testing pre-flight on every eval.** `phase-gate.sh gate_discover_to_build` runs `scripts/mutate-eval.sh` against each new eval definition. If kill rate < 0.8, eval is flagged as tautological. Rollout phase 1: WARN-only logging. Rollout phase 2 (after one verification cycle): fail the gate.
8. **Adversarial Auditor mode is default-on.** The runner prepends an "ADVERSARIAL AUDIT MODE" framing to the auditor's prompt that requires positive evidence for PASS verdicts. Auditor model defaults to Opus (different family from Builder's Sonnet) to break same-model-judge sycophancy. Disable only with `ADVERSARIAL_AUDIT=0` for deliberately permissive sweeps.

**The rule is: maximum velocity, zero shortcuts.** Go fast by being efficient, not by skipping steps.

### Worktree Provisioning Contract (v8.21.0+)

Per-cycle git worktrees are provisioned by `scripts/run-cycle.sh` (privileged shell context, structurally outside any LLM agent's reach) at `$EVOLVE_PROJECT_ROOT/.evolve/worktrees/cycle-N` on branch `evolve/cycle-N`. The path is recorded in `cycle-state.json:active_worktree`, exported as `WORKTREE_PATH`, and torn down via the EXIT trap on every exit path (success, failure, signal).

The trust-boundary invariant: **the orchestrator and all phase agents may NOT call `git worktree add` or `git worktree remove`.** Both are denied in `orchestrator.json` and in every phase profile that has a deny list. `cycle-state.sh set-worktree` is also privileged-shell-only (denied in orchestrator profile).

This closes the architectural gap that made v8.13.x — v8.20.2 require `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` to function on macOS Darwin 25.4: previously no component provisioned the build worktree, leaving `cycle-state.active_worktree` null, the builder's sandbox profile expanding `{worktree_path}` to empty, and all source writes EPERM.

If you ever need the worktree path from agent context, read it via `cycle-state.sh get active_worktree`. Never compute it yourself — the dispatcher is the canonical source.

## Release & Publish Workflow (v8.13.2+)

**"publish" ≠ "push".** See [docs/release-protocol.md](docs/release-protocol.md) for the canonical vocabulary (push, tag, release, propagate, publish, ship). When the user asks to "publish vX.Y.Z", use the self-healing pipeline:

```bash
bash scripts/release-pipeline.sh X.Y.Z              # full publish
bash scripts/release-pipeline.sh X.Y.Z --dry-run    # simulate, no mutations
bash scripts/release-pipeline.sh X.Y.Z --skip-tests # hot-fix path (CI-pre-verified)
```

The pipeline owns the entire lifecycle: pre-flight gate → version bump → auto-changelog from conventional commits → consistency check → atomic ship via `scripts/ship.sh` → marketplace propagation polling (5 min) → cache refresh → auto-rollback on any post-push failure.

**Bare `git push origin main` is denied by ship-gate** (since v8.13.0). Direct commits and pushes go through `scripts/ship.sh`. The release pipeline calls ship.sh internally; it does not bypass the gate.

### Required version markers (all auto-bumped by `version-bump.sh`)

1. `.claude-plugin/plugin.json` — canonical version (source of truth)
2. `.claude-plugin/marketplace.json` — `.plugins[0].version`
3. `skills/evolve-loop/SKILL.md` — heading `# Evolve Loop vX.Y` (only if major.minor changed)
4. `README.md` — "Current (vX.Y)" + version history row (only if major.minor changed)
5. `CHANGELOG.md` — `## [X.Y.Z]` block (auto-generated by `changelog-gen.sh`)

`scripts/release.sh` is now a **consistency verifier** invoked by the pipeline. Run standalone for diagnostics: `bash scripts/release.sh <version>`.

### Conventional commits

Auto-changelog buckets commits by type prefix:
- `feat:` / `feat(scope):` → `### Added`
- `fix:` → `### Fixed`
- `refactor:` / `perf:` / `stability:` → `### Changed`
- `docs:` → `### Documentation`
- `chore:` / `ci:` / `test:` / `build:` / `revert:` / `release:` → skipped
- no prefix → `### Other` (audit found ~40% of historical commits)

## Subagent Budget Controls (v8.13.4 / v8.13.5)

evolve-loop subagents have **three** budget-control mechanisms, evaluated in priority order:

### Precedence (highest priority first)

1. **`EVOLVE_MAX_BUDGET_USD`** (v8.13.4) — operator-controlled per-invocation override. Overrides all else.
2. **`EVOLVE_TASK_MODE` + `budget_tiers`** (v8.13.5) — declarative profile tier select. Used when the profile has a `budget_tiers` map AND the env var matches a key.
3. **`max_budget_usd`** (v8.12.x baseline) — static profile default. Always present.

### v8.13.4: per-invocation override

When a subagent task is unusually research-heavy or long-running and the static `max_budget_usd` in `.evolve/profiles/<agent>.json` is too tight, override per-invocation:

```bash
EVOLVE_MAX_BUDGET_USD=1.50 bash scripts/subagent-run.sh scout <cycle> <workspace>
```

The adapter logs the override loudly (`[claude-adapter] override max-budget-usd: ... (was ... from profile)`). Empty/malformed values → WARN + profile fallback. Negative values → rejected.

**When to use**: one-offs where the structured tier doesn't fit. Routine bypassing = CLAUDE.md violation; if your agent consistently needs more budget, declare a tier instead.

### v8.13.5: declarative task-mode tiers

For agents with structurally-different workloads (e.g., Scout doing codebase-scan vs Scout doing research-heavy web search), declare named tiers in the profile:

```json
{
  "max_budget_usd": 0.50,
  "budget_tiers": {
    "default": 0.50,
    "research": 1.50,
    "deep": 2.50
  }
}
```

Then select via `EVOLVE_TASK_MODE`:

```bash
EVOLVE_TASK_MODE=research bash scripts/subagent-run.sh scout <cycle> <workspace>
```

Adapter logs: `[claude-adapter] task-mode tier: research → $1.50 (was 0.50 from profile scout.json)`. Mode key absent from `budget_tiers` → WARN + profile fallback. No `budget_tiers` in profile + `EVOLVE_TASK_MODE` set → WARN.

**When to use**: agents whose workloads naturally cluster into 2-3 budget classes. The Scout profile (`.evolve/profiles/scout.json`) ships with `default` / `research` / `deep` tiers as the canonical example.

**Combining both**: `EVOLVE_TASK_MODE=research EVOLVE_MAX_BUDGET_USD=3.00` runs Scout with $3.00 cap; the explicit override wins, but the tier-resolution log line still appears for observability.

### Forward compatibility

These mechanisms complement (don't replace) Anthropic's `task_budget` (model-self-pacing). Once Claude Code adds `task_budget` support (currently API-only — see [Anthropic docs](https://platform.claude.com/docs/en/build-with-claude/task-budgets)), evolve-loop will integrate it as a fourth tier in the precedence chain. Hard $$ caps and declarative tiers remain useful even with model-self-pacing.

## Verification Before Claiming Done (v8.13.3+)

Three patterns the /insights audit identified as recurring friction. Apply ALL of them before reporting a task complete:

1. **Probe before declaring a CLI unavailable.** Never say "no `<tool>` command" without first running:
   ```bash
   bash scripts/probe-tool.sh <tool>      # canonical helper, checks PATH + common install dirs
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
| Codex CLI | (stub) | `codex` | `scripts/cli_adapters/codex.sh` | exits 99 — unsupported |

The hybrid pattern exists because Gemini CLI lacks non-interactive prompt mode (`gemini -p`), `--max-budget-usd`, and subagent dispatch. Without these, the trust boundary (`role-gate`, `ship-gate`, `phase-gate-precondition`) can't structurally enforce phase isolation. By delegating to `claude.sh`, Gemini-driven cycles inherit the full Claude Code kernel-hook protection. See [docs/incidents/gemini-forgery.md](docs/incidents/gemini-forgery.md) for why this matters.

### Where the platform-specific knowledge lives

- `docs/platform-compatibility.md` — top-level support matrix and adapter contract
- `skills/evolve-loop/reference/platform-detect.md` — env-var probe table for runtime detection
- `skills/evolve-loop/reference/<platform>-tools.md` — tool name translation (`Read` → `read_file`, etc.)
- `skills/evolve-loop/reference/<platform>-runtime.md` — invocation patterns per CLI
- `scripts/detect-cli.sh` — shell helper that returns one of `claude | gemini | codex | unknown`
- `scripts/cli_adapters/<cli>.sh` — runtime adapter; receives env-var contract from `subagent-run.sh`

### When implementing for a new CLI

Mirror the hybrid pattern (delegate to `claude.sh`) before attempting a native adapter. The native-adapter path requires verifying the new CLI has: non-interactive prompt mode, profile-scoped permissions, and either a budget cap flag or external cost tracking. Until those are confirmed, the hybrid path keeps the trust boundary intact.

## Swarm Architecture (v8.16+)

evolve-loop is structured around three composable layers (see [docs/architecture/tri-layer.md](docs/architecture/tri-layer.md) for the formal spec):

| Layer | Files | Purpose |
|---|---|---|
| **Skill** | `skills/<name>/SKILL.md` | Workflow + steps + exit criteria — the *how* |
| **Persona** | `agents/<role>.md` | One role, one perspective, one output format — the *who* |
| **Command** | `.claude-plugin/commands/<name>.md` | User-facing entry point — the *when* (orchestration) |

**The governing rule:** the user (or a slash command) is the orchestrator. **Personas do not invoke other personas.** Claude Code enforces this at runtime: subagents cannot spawn subagents.

### Sprint 1 — Pattern 3 fan-out (parallel sub-personas + aggregator)

Scout, Auditor, and Retrospective each fan out into specialized parallel sub-personas, then `scripts/aggregator.sh` merges them into the canonical phase artifact:

```bash
EVOLVE_FANOUT_ENABLED=1 EVOLVE_FANOUT_SCOUT=1 \
  bash scripts/subagent-run.sh dispatch-parallel scout <cycle> <workspace>
```

| Phase | Sub-personas | Default-off env flag |
|---|---|---|
| Scout | scout-codebase, scout-research, scout-evals | `EVOLVE_FANOUT_SCOUT` |
| Auditor | audit-eval-replay, audit-lint, audit-regression, audit-build-quality | `EVOLVE_FANOUT_AUDITOR` |
| Retrospective | retro-instinct, retro-gene, retro-failure | `EVOLVE_FANOUT_RETROSPECTIVE` |

Master switch: `EVOLVE_FANOUT_ENABLED=1`. Concurrency cap: `EVOLVE_FANOUT_CONCURRENCY` (default 4). Per-worker timeout: `EVOLVE_FANOUT_TIMEOUT` (default 600s). Builder is **excluded** from fan-out — single-writer invariant on the worktree.

### Sprint 2 — Multi-lens plan review (gstack `/autoplan` inspired)

A new `plan-review` phase between `discover` and `tdd` runs four lens reviewers (CEO/Eng/Design/Security) in parallel against `scout-report.md`. Aggregator computes verdict:

| Verdict | Trigger | Orchestrator action |
|---|---|---|
| PROCEED | Avg ≥ 7 AND no lens < 5 | Advance to TDD |
| REVISE | Avg ≥ 5 AND any lens < 5 | Re-run Scout (max 2 retries) |
| ABORT | Any explicit ABORT, OR avg < 5 | End cycle |

Default-off via `EVOLVE_PLAN_REVIEW=0`. Phase gate `gate_plan_review_to_tdd` enforces verdict at the kernel layer.

### Sprint 3 — Tri-layer composable skill catalog

7 new composable skills (`skills/evolve-{spec,plan-review,tdd,build,audit,ship,retro}/`) compose with the existing macro:

| Skill | Maps to slash command | Pattern |
|---|---|---|
| `evolve-spec` | `/scout` (codebase sub-scout) | Pattern 3 fan-out |
| `evolve-plan-review` | `/plan-review` | Pattern 3 fan-out |
| `evolve-tdd` | `/tdd` | Pattern 1/2 (single persona) |
| `evolve-build` | `/build` | Pattern 1/2 (single, single-writer) |
| `evolve-audit` | `/audit` | Pattern 3 fan-out |
| `evolve-ship` | `/ship` | Pattern 1/2 (atomic) |
| `evolve-retro` | `/retro` | Pattern 3 fan-out |
| `evolve-loop` (existing macro) | `/loop` | **Pattern 5** auto-orchestrated under trust kernel |

**Pattern 5 is specific to evolve-loop** because the trust kernel (sandbox + ledger SHA + phase-gate) substitutes for the human checkpoints addyosmani's framework relies on — see [docs/architecture/tri-layer.md](docs/architecture/tri-layer.md) for justification.

### Worker name pattern (`<role>-worker-<subtask>`)

Fan-out workers invoke `subagent-run.sh <role>-worker-<name>`. Examples:
- `scout-worker-codebase`, `scout-worker-research`, `scout-worker-evals`
- `auditor-worker-eval-replay`, `auditor-worker-lint`, `auditor-worker-regression`, `auditor-worker-build-quality`

`cmd_run` strips the `-worker-<name>` suffix to find the parent profile (`scout.json`), but writes to `<workspace>/workers/<full-agent>.md`. `phase-gate-precondition.sh` derives the parent role for sequence checking — workers inherit phase eligibility from their parent role's expected-agent set.

### Verifying the swarm architecture

Run `bash scripts/swarm-architecture-test.sh` to verify all three layers wire correctly (40 assertions covering plugin.json registrations, skill files, slash commands, persona files, profile parallel_subtasks, state machine, phase gate, aggregator merge modes, dispatch-parallel command, and end-to-end smoke test).

## Evolve Loop Task Priority

When selecting tasks for `/evolve-loop` cycles, follow this priority order:

1. **New features** — Building new functionality is the top priority
2. **Bug fixes** — Fixing potential bugs is second priority
3. **Security issues** — Fixing security vulnerabilities is last priority
