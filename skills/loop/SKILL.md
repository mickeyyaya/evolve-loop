---
name: loop
description: Use when the user invokes /evolve-loop or asks to run autonomous improvement cycles, self-evolving development, compound discovery, or multi-cycle code improvement with research, build, audit, and learning phases
argument-hint: "[--budget-usd N | --cycles N | --resume] [strategy] [goal]"
---

# Evolve Loop v18.11

> Self-evolving development pipeline. Orchestrates 4 agents through 6 lean phases per cycle: Discover → Build → Audit → Ship → Learn → Meta-Cycle. This skill performs destructive operations (commits, pushes, version bumps) — only invoke when the user explicitly requests it via `/evolve-loop` or asks to run improvement cycles.

## Platform overlay (v8.15.0+)

Tool and command names in this file use **Claude Code conventions** (`Read`, `Bash`, `Skill`, `Agent`, etc.). If you are running this skill from a different CLI (Gemini, Codex, generic), read [reference/platform-detect.md](reference/platform-detect.md) FIRST — it tells you which translation overlay to load (`reference/<platform>-tools.md` for tool names, `reference/<platform>-runtime.md` for invocation patterns). As of v12.0.0 the runtime is the Go binary (`go/bin/evolve`); cross-CLI subagent execution is handled by `evolve subagent run` and the `evolve serve-phase <name>` phaseproto wire. See [docs/platform-compatibility.md](../../docs/platform-compatibility.md) for the support matrix.

> **v12.1 status:** All command examples in this skill use the native `evolve <subcommand>` CLI. The legacy bash dispatcher remains archived at `archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh` as the operator-only `EVOLVE_USE_LEGACY_BASH=1` rollback hatch — not for general use.

> **What this does in one paragraph:** Each `/evolve-loop` invocation runs one or more self-contained improvement cycles — Scout finds work, Builder implements it in an isolated worktree, Auditor reviews it, and `ship.sh` commits only what passes. A trust kernel of three shell hooks (`phase-gate-precondition.sh`, `role-gate.sh`, `ship-gate.sh`) enforces phase order and artifact integrity at the OS layer, not the prompt layer — so the pipeline's safety properties hold even in autonomous / bypass-permissions mode. Failures become structured lessons via the Retrospective agent; the loop gets smarter with each pass.

## STRICT MODE — Read this first (v11.5.0+)

When invoked via `/evolve-loop [args]`, you MUST execute exactly one bash command that runs the native Go binary's `evolve loop` subcommand. The binary lives at `$EVOLVE_GO_BIN` (operator override) or `<plugin_root>/go/bin/evolve` (default). **Your cwd is the user's project directory, NOT the plugin install** — let the resolver find the binary:

```bash
EVOLVE_REQUIRE_INTENT=1 EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 "${EVOLVE_GO_BIN:-$(find $HOME/.claude/plugins \( -path '*/marketplaces/evolve-loop/go/bin/evolve' -o -path '*/marketplaces/evolve-loop/go/evolve' -o -path '*/cache/evolve-loop/evolve-loop/*/go/bin/evolve' -o -path '*/cache/evolve-loop/evolve-loop/*/go/evolve' \) -type f 2>/dev/null | sort | tail -1)}" loop <args>
```

The `find` expression locates the Go binary in either install layout (marketplace or cache). `sort | tail -1` prefers the highest-version cache install. The whole `"${EVOLVE_GO_BIN:-$(find ...)}"` expression is **one** shell argument from bash's perspective.

**Quote `<args>` correctly.** When `<args>` contains an apostrophe (e.g., `doesn't`, `won't`, `it's`), the shell tokenizer breaks because `'` opens an unmatched quoted string. Always wrap the goal portion in double quotes:

```bash
# CYCLES STRATEGY then a SINGLE double-quoted goal:
... loop 3 balanced "make UI more elegant; user said it doesn't work"

# Or just CYCLES + double-quoted goal (strategy defaults to balanced):
... loop 5 "the goal isn't trivial — needs research"

# Or use the explicit --goal-text flag:
... loop --cycles 3 --strategy balanced --goal-text "make UI more elegant"
```

Single quotes inside the goal are fine when the goal itself is double-quoted. Avoid passing apostrophe-containing goals as bare unquoted args.

**Budget-driven dispatch:** Pass `--budget-usd N` (or `--budget N`) to run cycles until cumulative cost ≥ $N, rather than a fixed count. Example: `... loop --budget-usd 5 "improve test coverage"`. The cycle count becomes a safety upper bound (default 50). Passing both `--budget-usd N --cycles M` stops at whichever comes first.

**Resume after pause:** If a cycle was checkpointed (Claude Code subscription quota wall, batch cap near, or operator-requested), recover with `--resume`:
```bash
... loop --resume
```
The native dispatcher locates the most recent paused cycle, validates state (git HEAD unchanged, worktree exists), and re-runs the orchestrator from the paused phase boundary. Trust kernel is preserved — phase-gate, role-gate, ship-gate enforce the same invariants during resume. See [docs/architecture/checkpoint-resume.md](../../docs/architecture/checkpoint-resume.md) for the full protocol.

**Rollback hatch (v11.5.0+):** If the native dispatcher misbehaves, `EVOLVE_USE_LEGACY_BASH=1` exec's to the archived bash dispatcher at `archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh`. Same argv, same exit codes. Only the dispatch path differs.

**DO NOT** invent paths like `<plugin_root>/skills/loop/go/bin/...` — the binary lives at `<plugin_root>/go/bin/evolve`, NOT under `skills/`. The skill (this file) and the binary live in sibling directories under the plugin root.

…and then read the binary's summary. Nothing else. The dispatcher loops one cycle per iteration and asserts each cycle produced Intent + Scout + Builder + Auditor ledger entries. Any cycle that bypasses the pipeline (orchestrator shortcut) makes the dispatcher exit with rc=2 and a CRITICAL diagnostic.

**Why `EVOLVE_REQUIRE_INTENT=1`** (v8.19.1+): the intent persona structures the user's goal into an `intent.md` artifact (8 fields + AwN classifier + ≥1 challenged premise) before Scout fires. This is the pre-Scout phase that prevents the "vague goal → wrong direction" failure mode the cycle-25 incident exposed. It's autonomy-preserving — no human checkpoint, no pause; the kernel verifies structure and the cycle continues. The user only invokes `/evolve-loop` and intent capture happens automatically as the first phase.

**Why `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1`** (v8.20.1+): on macOS Darwin 25.4+, `sandbox-exec` cannot be nested. When the orchestrator subagent (already wrapped in sandbox) tries to spawn the builder subagent (which also wraps in sandbox), the inner `sandbox_apply()` returns `EPERM` because the outer sandbox forbids creating new sandbox profiles. This is a kernel-level macOS restriction, not a config bug. The flag tells the adapter: on EPERM, retry the inner subagent without sandbox-exec — preserving sandbox where the kernel allows it, degrading only where the kernel forces it. On Linux (`bwrap`) the flag is a no-op because nested namespaces work. Without this flag, every macOS `/evolve-loop` cycle that needs a builder fails at the orchestrator → builder hand-off with `rc=71 sandbox_apply: Operation not permitted`.

**Both flags are the documented default for the slash command path; do not omit them.**

**You MUST NOT, when activating this skill** (tool names below are Claude Code conventions; consult `reference/<platform>-tools.md` for your CLI's equivalents — the prohibitions apply to those equivalents too):
- Use TodoWrite (CC) / `write_todos` (Gemini) / your CLI's task-list tool to decompose the goal into sub-tasks (the goal is for the orchestrator subagent inside each cycle, not for you).
- Invoke Edit, Write, or Bash (or `replace`/`write_file`/`run_shell_command` on Gemini, etc.) for any task other than the dispatcher itself (and reading its output).
- Invoke the in-process Agent / Task / `activate_skill`-as-subagent dispatch to run Scout/Builder/Auditor. Phase agents are spawned by `evolve subagent run` from inside the native `evolve cycle run` orchestrator. They are profile-restricted; the in-process subagent dispatch is not.
- "Help out" by editing files between cycles. Builder edits files inside its worktree, gated by `role-gate.sh`. You are not Builder.

**Why this is strict and not advisory:** the 2026-04-29 flow audit (cycles 8201–8213) showed that prompt-driven orchestration routinely shortcuts. Most cycles in that window have an Auditor ledger entry but no Scout/Builder entries — meaning Scout and Builder were either skipped or run via the in-process Agent tool that bypasses the kernel hooks (`role-gate`, `phase-gate-precondition`, `ship-gate`). The dispatcher closes that gap structurally: every cycle goes through `evolve cycle run` (or `evolve loop` for batches), which spawns the orchestrator subagent under `.evolve/profiles/orchestrator.json` (Edit/Write/git ops blocked at the kernel layer); that orchestrator then invokes Scout, Builder, Auditor via `subagent-run.sh` (sequence enforced by `phase-gate-precondition.sh`).

**Reading the summary correctly:**

| Dispatcher exit | Meaning | Your follow-up |
|---|---|---|
| `0` | All cycles ran AND ledger verified clean | Report the summary; that's it |
| `1` | A cycle invocation failed (e.g., subagent crash, state.json unwritable) | Surface the specific cycle's stderr and stop — do NOT retry inline |
| `2` | INTEGRITY BREACH: orchestrator silently skipped Scout/Builder/Auditor (no orchestrator-report.md, or report doesn't disclose the gap) — CRITICAL | Quote the exact ledger counts to the user; recommend inspecting `.evolve/runs/cycle-N/` to investigate; STOP |
| `3` | Batch completed with one or more recoverable failures (infrastructure / audit-fail / build-fail). Failure modes recorded to `state.json:failedApproaches[]` for the next dispatch's orchestrator to read and adapt | Report which cycles had which classification; surface state.json:failedApproaches summary; offer to re-run with the same goal so subsequent cycles can adapt |
| `10` | Bad arguments | Re-prompt with valid args |

**Dispatch policy (`EVOLVE_DISPATCH_POLICY`):** See [docs/architecture/control-flags.md](docs/architecture/control-flags.md) § `EVOLVE_DISPATCH_POLICY` for flag details (`off` / `verify` / `stop`; default: `verify`).

The rest of this file (architecture, model routing, phase docs) is reference material for the **orchestrator subagent** that `evolve cycle run` spawns. You, the slash-command handler, do not consult it during a `/evolve-loop` invocation.

---

> **Trust boundary (v8.13.1, v12.1-updated)**: enforced by THREE PreToolUse kernel hooks running as native Go: `evolve guard ship` (only `evolve ship` can perform git commit/push/gh release), `evolve guard role` (Edit/Write must match the active phase's path allowlist), `evolve guard phase` (`evolve subagent run` invocations must follow Scout→Builder→Auditor sequence per `.evolve/cycle-state.json`). For automated cycles, prefer `evolve cycle run [--goal-text GOAL]` (or `evolve loop` for batches) — it spawns a profile-restricted orchestrator subagent that operates within these hooks. Legacy in-line orchestration remains supported but the hooks apply equally to it.

> **v8.13.2 / v12.0.0**: self-healing release pipeline. For version-bump releases use `evolve release <version>` (native Go). The pipeline runs pre-flight gating, auto-generates a CHANGELOG entry from conventional commits, atomically ships via `evolve ship`, polls the marketplace for up to 5 minutes, and auto-rolls-back on any post-push failure. Use `--dry-run` to simulate without mutations. See [docs/release-protocol.md](../../docs/release-protocol.md) for vocabulary (push ≠ tag ≠ release ≠ publish ≠ propagate).

## Shared Agent Values

The following JSON block is the canonical state initialization for the evolve-loop. Agents must use these field names when reading from or writing to `state.json`.

```json
{
  "lastUpdated": "2026-04-20T10:00:00Z",
  "lastCycleNumber": 0,
  "version": 1,
  "research": {
    "queries": []
  },
  "evaluatedTasks": [],
  "failedApproaches": [],
  "evalHistory": [],
  "instinctCount": 0,
  "operatorWarnings": [],
  "stagnation": {"nothingToDoCount": 0, "recentPatterns": []},
  "warnAfterCycles": 5,
  "tokenBudget": {"perTask": 80000, "perCycle": 200000, "researchPhase": 25000},
  "mastery": {"level": "novice", "consecutiveSuccesses": 0},
  "ledgerSummary": {"totalEntries": 0, "cycleRange": [0, 0], "scoutRuns": 0, "builderRuns": 0, "totalTasksShipped": 0, "totalTasksFailed": 0, "avgTasksPerCycle": 0},
  "instinctSummary": [],
  "projectBenchmark": {
    "lastCalibrated": null, "calibrationCycle": 0, "overall": 0,
    "dimensions": {
      "documentationCompleteness": {"automated": 0, "llm": 0, "composite": 0},
      "specificationConsistency": {"automated": 0, "llm": 0, "composite": 0},
      "defensiveDesign": {"automated": 0, "llm": 0, "composite": 0},
      "evalInfrastructure": {"automated": 0, "llm": 0, "composite": 0},
      "modularity": {"automated": 0, "llm": 0, "composite": 0},
      "schemaHygiene": {"automated": 0, "llm": 0, "composite": 0},
      "conventionAdherence": {"automated": 0, "llm": 0, "composite": 0},
      "featureCoverage": {"automated": 0, "llm": 0, "composite": 0}
    },
    "benchHist": [], "highWaterMarks": {}
  },
  "fitnessScore": 0.0,
  "fitnessHistory": [],
  "fitnessRegression": false,
  "discoveryVelocity": {
    "current": 0,
    "benchHist": [],
    "rolling3": 0.0
  },
  "proposals": [],
  "researchAgenda": {
    "lastUpdated": null,
    "items": [],
    "capsuleIndex": {
      "docComp": [],
      "specCons": [],
      "defDesign": [],
      "evalInfra": [],
      "modul": [],
      "schemaHyg": [],
      "convAdher": [],
      "featCov": []
    }
  },
  "researchLedger": {
    "triedConcepts": [],
    "diversityTracker": {
      "dimensionCoverage": {},
      "lastResearchedDims": []
    }
  },
  "promptVariants": []
}
```

**Usage:** `/evolve-loop [--budget-usd N | --cycles N] [strategy] [goal]`
(legacy bare positional integer = cycles; deprecation WARN emitted —
prefer `--budget-usd N` or `--cycles N` explicitly)

## Quick Start

Parse `$ARGUMENTS` (v9.0.5+ — budget-first guidance, both modes supported):

- **`--budget-usd N`** (alias `--budget N`) → cost-driven mode: run cycles
  until cumulative spend ≥ \$N, then stop with `stop_reason=budget`.
  CYCLES becomes a safety upper bound (default 50). This is the
  recommended mode — costs are predictable; cycle counts are not.
- **`--cycles N`** → cycle-driven mode: run exactly N cycles regardless
  of cost. `EVOLVE_BATCH_BUDGET_CAP` (default \$20) remains a hard system
  ceiling.
- **Strategy** (positional, after flags): `balanced` (default) | `innovate` |
  `harden` | `repair` | `ultrathink` | `autoresearch`.
- **Goal** (positional, after strategy): free-form text; quote it when it
  contains apostrophes or shell metacharacters.
- **Legacy bare integer** (`/evolve-loop 3 balanced "goal"`) still parses
  as cycles in v9.0.x with a deprecation WARN. The v10.0.0 candidate
  will consider flipping bare-positional to dollars; until then, use
  the explicit flag to be flip-safe.

| Strategy | Focus | Approach | Strictness |
|----------|-------|----------|------------|
| `balanced` | Broad discovery | Standard | MEDIUM+ blocks |
| `innovate` | New features, gaps | Additive | Relaxed style |
| `harden` | Stability, tests | Defensive | Strict all |
| `repair` | Bugs, broken tests | Fix-only, smallest diff | Strict regressions |
| `ultrathink` | Complex refactors | tier-1 forced | Strict + confidence |
| `autoresearch` | Hypothesis testing | Fixed metrics, embraces failure | Divergent, unpenalized |

## Quota Handling & Auto-Resume (v10.6.0+)

When the dispatcher exits with `DISPATCH_RC=5` it emits a structured marker
line:

```
QUOTA-PAUSE: cycle=N wake-at=ISO8601 source=X attempts=K/M
```

This is the Claude Code subscription quota wall: the cycle ran far enough
to consume ≥80% of `EVOLVE_BATCH_BUDGET_CAP`, then a phase failed with
empty stderr (the quota-likely signature). `subagent-run.sh` wrote a
checkpoint and `cycle-state.json` carries the wake-at timestamp.

**Auto-resume is always on.** When you (the model) see `DISPATCH_RC=5` /
`QUOTA-PAUSE:` in dispatcher output, do this:

1. **Parse** the `wake-at=ISO8601` value from the marker line.
2. **Compute delay**: `delaySeconds = max(60, min(3600, wake_at_epoch - now_epoch + 60))`.
   - Floor at 60s so a near-zero remaining window doesn't busy-loop.
   - Cap at 3600s because `ScheduleWakeup` clamps to [60, 3600]; if the
     window is longer, call ScheduleWakeup again on the next wake.
3. **Call `ScheduleWakeup`** with that delay and `prompt="/evolve-loop --resume"`.
   Provide a `reason` like `"waiting for quota reset at <wake-at>"`.
4. On each wake, if `now < wake_at`, repeat steps 2–3 (chained wake-ups).
5. When `now >= wake_at`, the prompt fires `/evolve-loop --resume` which
   in turn invokes the native `evolve loop --resume` path → it bumps the
   `autoResumeAttempts` counter and re-runs the paused cycle from its
   last clean phase boundary.
6. The `autoResumeAttempts` cap (default 3, see `attempts=K/M` in the
   marker) prevents infinite quota-resume-quota loops; once exhausted,
   `evolve loop --resume` exits rc=2 and leaves the marker for operator
   intervention.

**Operator interrupt window.** The QUOTA-PAUSE marker line includes the
exact `wake-at=ISO8601` timestamp so the operator can Ctrl+C any pending
ScheduleWakeup before it fires. There is no flag-based opt-out — the loud
log is the intervention surface.

**Fallback if `ScheduleWakeup` is unavailable.** If you cannot call
`ScheduleWakeup` (e.g., the tool isn't loaded in this session's surface),
log the QUOTA-PAUSE marker verbatim so the operator can resume manually:
`evolve loop --resume`. Never silently swallow the DISPATCH_RC=5 signal.

See `docs/architecture/auto-resume.md` for the full architectural
contract and env-var reference.

## Architecture

```
Phase 0:   CALIBRATE ─ benchmark (once per invocation) → phase0-calibrate.md
Phase 0b: INTENT ─── [Intent] structure user goal     → docs/architecture/intent-phase.md
                     (always-on for /evolve-loop slash command path; v8.19.1+)
Phase 1: RESEARCH ── proactive research loop          → online-researcher.md
Utility:   SEARCH ─── intent-aware web search engine    → smart-web-search.md
Phase 2:   DISCOVER ── [Scout] scan + task selection    → phases.md
Phase 2b: TRIAGE ─── [Triage] top_n scope decision    → agents/evolve-triage.md
                     (v8.56.0+ Layer C, opt-in via EVOLVE_TRIAGE_ENABLED=1)
Phase 3:   BUILD ───── [Builder] implement (worktree)   → phase3-build.md
Phase 4:   AUDIT ───── [Auditor] review + eval gate     → phases.md
Phase 5:   SHIP ────── publish via `evolve release` (or `evolve ship` for non-release commits) → phase5-ship.md
Phase 6:   LEARN ───── instinct extraction + feedback   → phase6-learn.md
Phase 7:   META ────── self-improvement (every 5 cycles) → phase7-meta.md
```

## Orchestrator Loop

For each cycle:
1. Claim cycle number (OCC protocol)
2. **Phase transitions are enforced by `evolve guard phase`** (PreToolUse hook in `.claude/settings.json`) plus the in-process Go orchestrator's state machine. Operators do not invoke a separate phase-gate command; the kernel layer enforces it automatically.
3. Intent (v8.19.1+, always for /evolve-loop) → Scout → Builder → Auditor → phase-gate verification → Ship → Learn
4. **Subagents are dispatched by the Go orchestrator** (`go/internal/core/orchestrator.go`) via the `bridge` adapter, which spawns the configured CLI (`claude -p`, `gemini`, or the agy/codex adapter) and feeds the per-phase agent prompt from `agents/<name>.md`. The kernel still enforces per-agent CLI permission profiles in `.evolve/profiles/` and writes tamper-evident ledger entries. The `evolve subagent run <agent> <cycle> <workspace>` CLI is available for manual single-phase dispatch (used by `evolve serve-phase` and the cross-CLI consensus harness). The bash `subagent-run.sh` path was removed in v12.0.0.
5. Max 3 retries per task; WARN/FAIL blocks shipping
6. Output Discovery Briefing → continue immediately
7. **Never stop to ask. Never skip agents. Never fabricate cycles. Complete ALL requested cycles.**

### Declarative cycle driver (v11.5.0+: native Go)

Instead of running the loop from inside this skill's prompt, the native binary's `evolve cycle run` subcommand drives a single cycle end-to-end:

1. Picks the next cycle number (or accepts `--cycle N`).
2. Initializes `.evolve/cycle-state.json` with `phase=calibrate`.
3. Spawns the orchestrator subagent (via the Go subagent runner under `.evolve/profiles/orchestrator.json`; Edit/Write/git ops still blocked at the kernel hook layer — hooks are unchanged).
4. Clears cycle-state on exit.

The orchestrator subagent (`agents/evolve-orchestrator.md`) advances phases via the native state machine; the `evolve guard phase` PreToolUse hook reads cycle-state to validate that the next subagent invocation matches the expected order.

Use `evolve loop` (multi-cycle batch) or `evolve cycle run` (single cycle) for autonomous runs. Pre-v11.5.0 operators who depend on the bash dispatch path can set `EVOLVE_USE_LEGACY_BASH=1` to exec the archived dispatcher at `archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh`.

## Agents

| Role | File | Tier | Output |
|------|------|------|--------|
| Scout | `agents/evolve-scout.md` | tier-2 | `scout-report.md` |
| Builder | `agents/evolve-builder.md` | tier-2 | `build-report.md` |
| Auditor | `agents/evolve-auditor.md` | tier-2 | `audit-report.md` |

## Model Routing

| Phase | Default | Upgrade → | Downgrade → |
|-------|---------|-----------|-------------|
| Scout | tier-2 | Cycle 1 / goal → tier-1 | Cycle 4+ → tier-3 |
| Builder | tier-2 | M+5 files / retry ≥ 2 → tier-1 | S + cache → tier-3 |
| Auditor | tier-2 | Security → tier-1 | Clean → tier-3 |
