---
score_cap:
  - criterion: "TestMain clears every inherited cell marker before any cmd/evolve test runs"
    max_if_missing: 6
    evidence: "cd go && env EVOLVE_PHASE=build EVOLVE_FLEET=1 EVOLVE_DISPATCH_DEPTH=1 EVOLVE_PROJECT_ROOT=$PWD go test -count=1 -run '^TestMain_ClearsInheritedCellMarkers$' ./cmd/evolve"
  - criterion: "The inbox and ship verb tests pass when the parent environment carries the cell markers"
    max_if_missing: 7
    evidence: "cd go && env EVOLVE_PHASE=build EVOLVE_FLEET=1 EVOLVE_DISPATCH_DEPTH=1 go test -count=1 -run '^(TestCmd_Inbox(Edit|Route|Verify|Withdraw|Curation)|TestRunShipCmd_AutoConfirmIsAnAuditedBypass|TestInboxClaim_EachRefusalHasItsExitCode)' ./cmd/evolve"
  - criterion: "The refusal tests that set a marker on purpose still refuse"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestQueuedVerbs_InsideAPhasePaneStillRefuse|TestCLIRoutingWrite_IsRefusedInsideAPhase|TestCodeRefresh_InsideAPhaseItRefuses|TestRatchet_TightenIsRefusedInsideAPhase|TestRunShipCmd_AutoConfirmIsRefusedInsideACell)' ./cmd/evolve"
---

# Eval: cmd/evolve tests are hermetic inside an LLM cell

> Pins the contract that the `go test ./cmd/evolve` process drops the cell markers it inherits from the bridge. Without it, the inbox curation verbs and `evolve ship --auto-confirm` refuse as a phase agent and 26 tests fail inside a cell. Source incident: cycle 2130, inbox item cmd-evolve-tests-hermetic-inside-a-cell.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| marker-guard | TestMain leaves no cell marker set | 6/10 | `TestMain_ClearsInheritedCellMarkers` under the cell environment |
| symptom | Inbox and ship verb tests pass with markers inherited | 7/10 | the curation test regexp under the cell environment |
| preservation | Deliberate-marker refusal tests still refuse | 7/10 | the refusal test regexp |
