---
score_cap:
  - criterion: "the Locate port that production wires never returns the cycle of an earlier life of the same todo"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestLaneLocator_ANewLifeOfATodoNeverTakesTheCycleOfItsEarlierLife$' ./cmd/evolve"
  - criterion: "the floor locator is race-free for the seal and the oversight callers"
    max_if_missing: 6
    evidence: "cd go && go test -race -count=1 -run '^TestRunsLocator_ASealAndTheOversightMayLocateAtOnce$' ./internal/orchestrator"
  - criterion: "the package note names RunsLocator as the Locate port production wires"
    max_if_missing: 7
    evidence: "grep -q 'RunsLocator' docs/architecture/packages/internal-orchestrator.md && ! grep -q 'Locate. port (.orchestrator.LaneCycle' docs/architecture/packages/internal-orchestrator.md"
---

# Eval: production lane locator earlier-life floor

> Production wired `orchestrator.LaneCycle` (floor 0) as the Locate port, so a relaunched todo could take the cycle of its earlier life. `RunsLocator` has the floor but only tests called it. Both `Pool.recover` and the oversight call Locate, so the wired port must be race-free. Source: cycle 2452 inbox item.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| production-seam | new life never takes an earlier life cycle | 4/10 | `TestLaneLocator_ANewLifeOfATodoNeverTakesTheCycleOfItsEarlierLife` |
| concurrent-callers | floor locator is race-free | 6/10 | `TestRunsLocator_ASealAndTheOversightMayLocateAtOnce` with `-race` |
| note | package note names the wired port | 7/10 | grep of the package note |
