---
score_cap:
  - criterion: "A run without --keep leaves its tagged rows in .evolve/agent-runs/llm-calls.ndjson"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2135_001' ./acs/cycle2135"
  - criterion: "A second import of the same call adds no row"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2135_003' ./acs/cycle2135"
  - criterion: "An import failure warns on stderr and keeps the run exit code"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2135_004|TestC2135_005' ./acs/cycle2135"
  - criterion: "A run with no model-attempt rows creates no durable ledger and no warning"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2135_002' ./acs/cycle2135"
---

# Eval: evolve agent run keeps its llm-calls rows in a durable ledger

> Pins the T6 component of docs/plans/token-telemetry-2026-10.md. Cycle 2135 found that `evolve agent run` removes its workspace and the workspace llm-calls.ndjson rows with it, so the token spend of a standalone run is lost. The predicates drive the real `agentcmd.New` entry point with a fake bridge that writes rows into the run workspace.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| rows-kept-and-tagged | Rows carry the run label and the profile | 5/10 | `TestC2135_001` |
| idempotent-import | A repeated call identity adds no row | 6/10 | `TestC2135_003` |
| failure-isolated | A blocked ledger warns and keeps the exit code | 6/10 | `TestC2135_004`, `TestC2135_005` |
| no-rows-no-noise | An empty run creates no ledger and no warning | 8/10 | `TestC2135_002` |
