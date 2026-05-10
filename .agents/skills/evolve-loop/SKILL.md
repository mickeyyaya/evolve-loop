---
name: evolve-loop
description: Use when the user invokes /evolve-loop or asks to run autonomous improvement cycles, self-evolving development, compound discovery, or multi-cycle code improvement with research, build, audit, and learning phases
argument-hint: "[cycles] [strategy] [goal]"
---

# Evolve Loop v9.0

> Self-evolving development pipeline. Orchestrates 4 agents through 6 lean phases per cycle: Discover → Build → Audit → Ship → Learn → Meta-Cycle. This skill performs destructive operations (commits, pushes, version bumps) — only invoke when the user explicitly requests it via `/evolve-loop` or asks to run improvement cycles.

## Platform overlay (v8.15.0+)

Tool and command names in this file use **Claude Code conventions** (`Read`, `Bash`, `Skill`, `Agent`, etc.). If you are running this skill from a different CLI (Gemini, Codex, generic), read [reference/platform-detect.md](reference/platform-detect.md) FIRST — it tells you which translation overlay to load (`reference/<platform>-tools.md` for tool names, `reference/<platform>-runtime.md` for invocation patterns). On non-Claude platforms the runtime falls back to the hybrid driver: shell scripts dispatch through `scripts/cli_adapters/<cli>.sh`, which delegates to the Claude binary for actual subagent execution. See [docs/platform-compatibility.md](../../docs/platform-compatibility.md) for the support matrix.

## STRICT MODE — Read this first (v8.13.7+)

When invoked via `/evolve-loop [args]`, you MUST execute exactly one bash command. **Your cwd is the user's project directory, NOT the plugin install** — relative paths like `bash scripts/...` will fail with rc=127. Use this exact one-liner which resolves the dispatcher's absolute path regardless of install layout:

```bash
EVOLVE_REQUIRE_INTENT=1 EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 bash "$(find $HOME/.claude/plugins \( -path '*/marketplaces/evolve-loop/scripts/dispatch/evolve-loop-dispatch.sh' -o -path '*/cache/evolve-loop/evolve-loop/*/scripts/dispatch/evolve-loop-dispatch.sh' \) -type f 2>/dev/null | sort | tail -1)" <args>
```

The `find` expression locates the dispatcher in either install layout (marketplace or cache). `sort | tail -1` prefers the highest-version cache install if both exist (cache install dirs sort by version since the dir name is the version number). The `bash "$(...)"` invocation is **one** command from the shell's perspective — the substitution happens before bash runs.

**Quote `<args>` correctly (v8.30.0).** When `<args>` contains an apostrophe (e.g., `doesn't`, `won't`, `it's`), the shell tokenizer breaks because `'` opens an unmatched quoted string. Always wrap the goal portion in double quotes:

```bash
# CYCLES STRATEGY then a SINGLE double-quoted goal:
... bash "$(find ...)" 3 balanced "make UI more elegant; user said it doesn't work"

# Or just CYCLES + double-quoted goal (strategy defaults to balanced):
... bash "$(find ...)" 5 "the goal isn't trivial — needs research"
```

Single quotes inside the goal are fine when the goal itself is double-quoted. Avoid passing apostrophe-containing goals as bare unquoted args — the shell parses `doesn't` as `doesn` + opening-`'t` and waits for a closing single-quote that never comes.

**DO NOT** invent paths like `<plugin_root>/skills/evolve-loop/scripts/...` — the dispatcher is at `<plugin_root>/scripts/`, NOT under `skills/`. The skill (this file) and the dispatcher live in sibling directories under the plugin root.

…and then read its summary. Nothing else. The dispatcher loops `run-cycle.sh` once per cycle and asserts each cycle produced Intent + Scout + Builder + Auditor ledger entries. Any cycle that bypasses the pipeline (orchestrator shortcut) makes the dispatcher exit with rc=2 and a CRITICAL diagnostic.

**Why `EVOLVE_REQUIRE_INTENT=1`** (v8.19.1+): the intent persona structures the user's goal into an `intent.md` artifact (8 fields + AwN classifier + ≥1 challenged premise) before Scout fires. This is the pre-Scout phase that prevents the "vague goal → wrong direction" failure mode the cycle-25 incident exposed. It's autonomy-preserving — no human checkpoint, no pause; the kernel verifies structure and the cycle continues. The user only invokes `/evolve-loop` and intent capture happens automatically as the first phase.

**Why `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1`** (v8.20.1+): on macOS Darwin 25.4+, `sandbox-exec` cannot be nested. When the orchestrator subagent (already wrapped in sandbox) tries to spawn the builder subagent (which also wraps in sandbox), the inner `sandbox_apply()` returns `EPERM` because the outer sandbox forbids creating new sandbox profiles. This is a kernel-level macOS restriction, not a config bug. The flag tells the adapter: on EPERM, retry the inner subagent without sandbox-exec — preserving sandbox where the kernel allows it, degrading only where the kernel forces it. On Linux (`bwrap`) the flag is a no-op because nested namespaces work. Without this flag, every macOS `/evolve-loop` cycle that needs a builder fails at the orchestrator → builder hand-off with `rc=71 sandbox_apply: Operation not permitted`.

**Both flags are the documented default for the slash command path; do not omit them.**

**You MUST NOT, when activating this skill** (tool names below are Claude Code conventions; consult `reference/<platform>-tools.md` for your CLI's equivalents — the prohibitions apply to those equivalents too):
- Use TodoWrite (CC) / `write_todos` (Gemini) / your CLI's task-list tool to decompose the goal into sub-tasks (the goal is for the orchestrator subagent inside each cycle, not for you).
- Invoke Edit, Write, or Bash (or `replace`/`write_file`/`run_shell_command` on Gemini, etc.) for any task other than the dispatcher itself (and reading its output).
- Invoke the in-process Agent / Task / `activate_skill`-as-subagent dispatch to run Scout/Builder/Auditor. Phase agents are spawned by `subagent-run.sh` from inside `run-cycle.sh`. They are profile-restricted; the in-process subagent dispatch is not.
- "Help out" by editing files between cycles. Builder edits files inside its worktree, gated by `role-gate.sh`. You are not Builder.

**Why this is strict and not advisory:** the 2026-04-29 flow audit (cycles 8201–8213) showed that prompt-driven orchestration routinely shortcuts. Most cycles in that window have an Auditor ledger entry but no Scout/Builder entries — meaning Scout and Builder were either skipped or run via the in-process Agent tool that bypasses the kernel hooks (`role-gate`, `phase-gate-precondition`, `ship-gate`). The dispatcher closes that gap structurally: every cycle goes through `run-cycle.sh`, which spawns the orchestrator subagent under `.evolve/profiles/orchestrator.json` (Edit/Write/git ops blocked at the kernel layer); that orchestrator then invokes Scout, Builder, Auditor via `subagent-run.sh` (sequence enforced by `phase-gate-precondition.sh`).

**Reading the summary correctly:**

| Dispatcher exit | Meaning | Your follow-up |
|---|---|---|
| `0` | All cycles ran AND ledger verified clean | Report the summary; that's it |
| `1` | A `run-cycle.sh` invocation failed | Surface the specific cycle's stderr and stop — do NOT retry inline |
| `2` | INTEGRITY BREACH: orchestrator silently skipped Scout/Builder/Auditor (no orchestrator-report.md, or report doesn't disclose the gap) — CRITICAL | Quote the exact ledger counts to the user; recommend inspecting `.evolve/runs/cycle-N/` to investigate; STOP |
| `3` | Batch completed with one or more recoverable failures (infrastructure / audit-fail / build-fail). Failure modes recorded to `state.json:failedApproaches[]` for the next dispatch's orchestrator to read and adapt | Report which cycles had which classification; surface state.json:failedApproaches summary; offer to re-run with the same goal so subsequent cycles can adapt |
| `10` | Bad arguments | Re-prompt with valid args |

**Exit code 3 is the evolutionary path (v8.16.1+)**: when a cycle's `orchestrator-report.md` honestly declares "INFRASTRUCTURE FAILURE", an audit `Verdict: FAIL`, or a build failure, the dispatcher records the failure to `state.json:failedApproaches[]` and continues to the next cycle. The next cycle's orchestrator reads `failedApproaches` and adapts its approach (different scope, alternative path, or operator escalation). This is what "evolve-loop" means: learn from failure, try a different approach, move forward. STOP-class behavior (rc=2) is preserved for actual kernel breaches.

**Dispatch policy (v8.60+):** `EVOLVE_DISPATCH_POLICY=off|verify|stop` controls per-cycle pipeline verification. `verify` (default) checks scout/builder/auditor ledger entries and continues on recoverable failures. `stop` reverts to fail-fast (rc=2 on any failure; use for CI gates). `off` skips verification entirely (legacy debug only). The deprecated flags `EVOLVE_DISPATCH_VERIFY=0` and `EVOLVE_DISPATCH_STOP_ON_FAIL=1` bridge to `EVOLVE_DISPATCH_POLICY=off` and `=stop` respectively with a one-time stderr WARN.

The rest of this file (architecture, model routing, phase docs) is reference material for the **orchestrator subagent** that `run-cycle.sh` spawns. You, the slash-command handler, do not consult it during a `/evolve-loop` invocation.

---

> **v8.13.1**: trust boundary now enforced by THREE PreToolUse kernel hooks: `ship-gate.sh` (only `scripts/lifecycle/ship.sh` can perform git commit/push/gh release), `role-gate.sh` (Edit/Write must match the active phase's path allowlist), `phase-gate-precondition.sh` (`subagent-run.sh` invocations must follow Scout→Builder→Auditor sequence per `.evolve/cycle-state.json`). For automated cycles, prefer `bash scripts/dispatch/run-cycle.sh [GOAL]` — it spawns a profile-restricted orchestrator subagent that operates within these hooks. Legacy in-line orchestration (this skill's prompt-driven loop) remains supported but the hooks apply equally to it.

> **v8.13.2**: self-healing release pipeline. For version-bump releases, prefer `bash scripts/release-pipeline.sh <version>` over direct `ship.sh`. The pipeline runs pre-flight gating, auto-generates a CHANGELOG entry from conventional commits, atomically ships via `ship.sh`, polls the marketplace for up to 5 minutes, and auto-rolls-back on any post-push failure. Use `--dry-run` to simulate without mutations. See [docs/release-protocol.md](../../docs/release-protocol.md) for vocabulary (push ≠ tag ≠ release ≠ publish ≠ propagate).

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

**Usage:** `/evolve-loop [cycles] [strategy] [goal]`

## Quick Start

Parse `$ARGUMENTS`:
- First number → `cycles` (default: 2)
- `innovate|harden|repair|ultrathink` → `strategy` (default: `balanced`)
- Remaining → `goal` (default: null = autonomous)

| Strategy | Focus | Approach | Strictness |
|----------|-------|----------|------------|
| `balanced` | Broad discovery | Standard | MEDIUM+ blocks |
| `innovate` | New features, gaps | Additive | Relaxed style |
| `harden` | Stability, tests | Defensive | Strict all |
| `repair` | Bugs, broken tests | Fix-only, smallest diff | Strict regressions |
| `ultrathink` | Complex refactors | tier-1 forced | Strict + confidence |
| `autoresearch` | Hypothesis testing | Fixed metrics, embraces failure | Divergent, unpenalized |

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
Phase 5:   SHIP ────── publish via release-pipeline.sh   → phase5-ship.md (or scripts/lifecycle/ship.sh for non-release commits)
Phase 6:   LEARN ───── instinct extraction + feedback   → phase6-learn.md
Phase 7:   META ────── self-improvement (every 5 cycles) → phase7-meta.md
```

## Orchestrator Loop

For each cycle:
1. Claim cycle number (OCC protocol)
2. **`bash scripts/lifecycle/phase-gate.sh <gate> $CYCLE $WORKSPACE`** — MANDATORY at every phase transition
3. Intent (v8.19.1+, always for /evolve-loop) → Scout → Builder → Auditor → phase-gate verification → Ship → Learn
4. **Subagents MUST be launched via `bash scripts/dispatch/subagent-run.sh <agent> $CYCLE $WORKSPACE`** — never via the in-process `Agent` tool (or `activate_skill`-as-subagent on Gemini, or any equivalent same-session dispatch) in production. Builder gets its worktree via `WORKTREE_PATH` env var. The runner enforces per-agent CLI permission profiles in `.evolve/profiles/` and writes a tamper-evident ledger entry. On non-Claude CLIs, `subagent-run.sh` dispatches to the per-platform adapter at `scripts/cli_adapters/<cli>.sh`; the Gemini adapter uses the hybrid pattern (delegates to Claude binary). Legacy `LEGACY_AGENT_DISPATCH=1` fallback permitted for one A/B cycle only.
5. Max 3 retries per task; WARN/FAIL blocks shipping
6. Output Discovery Briefing → continue immediately
7. **Never stop to ask. Never skip agents. Never fabricate cycles. Complete ALL requested cycles.**

### v8.13.1 alternative: declarative cycle driver

Instead of running the loop from inside this skill's prompt, you may invoke `bash scripts/dispatch/run-cycle.sh [GOAL]`. The driver:

1. Picks the next cycle number (or accepts `--cycle N`).
2. Initializes `.evolve/cycle-state.json` with `phase=calibrate`.
3. Spawns the orchestrator subagent (`bash scripts/dispatch/subagent-run.sh orchestrator $CYCLE $WORKSPACE`) under the orchestrator profile (Edit/Write/git ops blocked at the kernel hook layer).
4. Clears cycle-state on exit.

The orchestrator subagent (`agents/evolve-orchestrator.md`) calls `bash scripts/lifecycle/cycle-state.sh advance <phase> <agent>` between phases; `phase-gate-precondition.sh` reads cycle-state to validate that the next subagent invocation matches the expected order.

Use this when you want every gate active (recommended for autonomous cycles). Use the in-line skill loop when you need tighter control or are debugging.

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
