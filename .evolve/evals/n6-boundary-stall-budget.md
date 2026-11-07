---
score_cap:
  - criterion: "A stalled boundary family is cut at the budget to a skip and no later family starts"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/cliupdate -run '^(TestUpdate_AStalledFamilyIsCutAtTheBudgetAndNoLaterFamilyStarts|TestUpdate_ABudgetCutAfterTheUpdaterIsNeverASmokeFailure)$'"
  - criterion: "The default budget admits one full family and cuts the two-family worst case; a non-positive budget is the default, not an expired deadline"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/cliupdate -run '^(TestDefaultBoundaryBudget_AdmitsOneFullFamilyAndCutsTheTwoFamilyWorstCase|TestUpdate_ANonPositiveBudgetIsTheDefaultDeadlineNotAnExpiredOne)$'"
---

# Eval: N6 boundary stall budget

> Pins criterion 4 of inbox item model-currency-c5-launch-model-verification-remainder: the boundary CLI update has a hard budget (`DefaultBoundaryBudget`, 20 minutes) that turns a stall into a skip. Source incident: cycles 2145 and 2195 landed the budget, and the 2195 push lost to a diverged origin/main.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| budget-cut | stalled family becomes skipped, later families do not start | 5/10 | `go test ./internal/cliupdate -run TestUpdate_AStalledFamily` |
| budget-default | default budget size and non-positive handling | 6/10 | `go test ./internal/cliupdate -run DefaultBoundaryBudget` |
