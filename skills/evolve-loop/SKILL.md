---
name: evolve-loop
description: Use when the user invokes /evolve-loop or asks to run autonomous improvement cycles, self-evolving development, compound discovery, or multi-cycle code improvement with research, build, audit, and learning phases
argument-hint: "[cycles] [strategy] [goal]"
---

# Evolve Loop v8.11

> Self-evolving development pipeline. Orchestrates 4 agents through 6 lean phases per cycle: Discover → Build → Audit → Ship → Learn → Meta-Cycle. This skill performs destructive operations (commits, pushes, version bumps) — only invoke when the user explicitly requests it via `/evolve-loop` or asks to run improvement cycles.

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
  "ledgerSummary": {"totalEntries": 0, "cycleRange": [0, 0], "scoutRuns": 0, "builderRuns": 0, "totalTasksShipped": 0, "totalTasksFailed": 0, "crystallizedSkills": 0, "avgTasksPerCycle": 0, "distilledFacts": []},
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
Phase 1: RESEARCH ── proactive research loop          → online-researcher.md
Utility:   SEARCH ─── intent-aware web search engine    → smart-web-search.md
Phase 2:   DISCOVER ── [Scout] scan + task selection    → phases.md
Phase 3:   BUILD ───── [Builder] implement (worktree)   → phase3-build.md
Phase 4:   AUDIT ───── [Auditor] review + eval gate     → phases.md
Phase 5:   SHIP ────── commit + push                    → phase5-ship.md
Phase 6:   LEARN ───── instinct extraction + feedback   → phase6-learn.md
Phase 7:   META ────── self-improvement (every 5 cycles) → phase7-meta.md
```

## Orchestrator Loop

For each cycle:
1. Claim cycle number (OCC protocol)
2. **`bash scripts/phase-gate.sh <gate> $CYCLE $WORKSPACE`** — MANDATORY at every phase transition
3. Scout → Builder → Auditor → phase-gate verification → Ship → Learn
4. Inline S-tasks directly; worktree M-tasks with `isolation: "worktree"`
5. Max 3 retries per task; WARN/FAIL blocks shipping
6. Output Discovery Briefing → continue immediately
7. **Never stop to ask. Never skip agents. Never fabricate cycles. Complete ALL requested cycles.**

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
