---
name: evolve-loop/memory-protocol
description: "Shared memory protocol — defines all persistent state layers (JSONL ledger, workspace files, state.json, evals, instincts) and concurrency rules for the evolve-loop pipeline."
---

# Evolve Loop — Shared Memory Protocol

## Layer 0: Shared Values (Core Rules)

The `sharedValues` block in [SKILL.md](SKILL.md) is the canonical team constitution.

```json
{
  "sharedValues": {
    "behavioralRules": ["immutability", "scope-discipline"],
    "qualityThresholds": {"maxLines": 800}
  }
}
```

## Layer 1: JSONL Ledger (`.evolve/ledger.jsonl`)

Append-only log for cross-run traceability.

```jsonl
{"ts":"2026-04-20T10:00:00Z","cycle":1,"role":"scout","type":"discovery","data":{}}
```

## Layer 2: Markdown Workspace (`$WORKSPACE_PATH`)

Agent-owned files in the run-scoped workspace.

## Layer 3: State Manifest (Persistent State)

The `state.json` file is the **State Manifest**.

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
  "ledgerSummary": {"totalEntries": 0, "cycleRange": [0, 0], "scoutRuns": 0, "builderRuns": 0, "totalTasksShipped": 0, "totalTasksFailed": 0, "avgTasksPerCycle": 0, "distilledFacts": []},
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
      "dimCoverage": {},
      "lastResearchedDims": []
    }
  },
  "promptVariants": []
}
```

## Layer 4: Eval State

## Layer 5: Instincts

## Layer 6: Experiment Journal

