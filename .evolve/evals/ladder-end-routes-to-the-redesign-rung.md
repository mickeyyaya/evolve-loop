---
score_cap:
  - criterion: "askLadder routes the end of the ladder (R3 or a spec stop) to the redesign rung with a LADDER_REDESIGN signal and a redesign mark"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run 'TestAskLadder_TheEndOfTheLadderRoutesToTheRedesign|TestAskLadder_ASpecStopRoutesToTheRedesign|TestRunCycle_AFailedLadderRoutesOnceToTheRedesignRung' ./internal/core"
  - criterion: "LADDER_REDESIGN is registered under the ladder module"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestAskLadder_TheCodeOfTheRedesignRouteIsRegisteredUnderTheLadderModule ./internal/core"
  - criterion: "No redesign before the end of the ladder and no second automatic redesign"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestAskLadder_NoRedesignRouteBeforeTheEndOfTheLadder|TestAskLadder_ASecondEndOfTheLadderGoesToTheOperatorWithNoSecondRedesign|TestRunCycle_APassingBuildNeverRoutesToTheRedesignRung' ./internal/core"
  - criterion: "ADR-0146 item 9 and the code agree on the redesign route"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2229_003|TestC2229_004' ./acs/cycle2229"
---

# Eval: A failed ladder routes to the redesign rung

> Pins ADR-0164 ruling 1 for the retry ladder. Cycle 2211 found that `askLadder` ended a failed ladder in a silent stop, as ADR-0146 item 9 said. Cycle 2229 continues that work after a ship divergence. The operator ruled on 2026-11-06 that the redesign rung of ADR-0126 runs first, and that the operator gets the subject only on a second end.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| end-routes | R3 and spec stops emit LADDER_REDESIGN and mark the state | 3/10 | `go test -run TestAskLadder_TheEndOfTheLadderRoutesToTheRedesign ./internal/core` |
| registered | the code is registered under the ladder module | 6/10 | `go test -run TestAskLadder_TheCodeOfTheRedesignRouteIsRegisteredUnderTheLadderModule ./internal/core` |
| bounds | no early redesign, no second automatic redesign | 6/10 | `go test -run TestAskLadder_NoRedesignRouteBeforeTheEndOfTheLadder ./internal/core` |
| record | ADR-0146 item 9 names the redesign route | 6/10 | `go test -tags acs -run TestC2229_003 ./acs/cycle2229` |
