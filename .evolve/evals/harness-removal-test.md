---
score_cap:
  - criterion: "The removal test design names the component, the window, the measure and the stop rule"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRemovalDesign_TheDesignNamesTheComponentTheWindowTheMeasureAndTheStopRule$' ./cmd/evolve"
  - criterion: "The removal test design waits for the M2 baseline of ADR-0133"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRemovalDesign_TheDesignWaitsForTheM2BaselineOfADR0133$' ./cmd/evolve"
  - criterion: "The design passes ste-lint strict through the docs entry point"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRemovalDesign_TheDocsEntryPointLintsTheDesignInSTE$' ./cmd/evolve"
---

# Eval: harness removal test design

> Pins the design of the harness removal test (plan R9 of docs/plans/harness-practices-2026-11.md, research F13.1). The design is a document, so the tests read it through the docs entry point. Source: cycle 2549, inbox item harness-removal-test.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| four-parts | Component, window, measure and stop rule are named | 6/10 | `go test -run TestRemovalDesign_TheDesignNames...` |
| m2-gate | The test waits for the M2 baseline | 7/10 | `go test -run TestRemovalDesign_TheDesignWaitsForTheM2BaselineOfADR0133` |
| ste | The design passes STE lint | 6/10 | `go test -run TestRemovalDesign_TheDocsEntryPointLintsTheDesignInSTE` |
