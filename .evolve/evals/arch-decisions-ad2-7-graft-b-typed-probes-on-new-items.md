---
score_cap:
  - criterion: "Admit refuses an item whose probes already pass, and no override lifts the refusal"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestAdmit_RefusesAnItemWhoseProbesAlreadyPass$' ./internal/inboxcurator"
  - criterion: "Admit admits an item with one probe that still fails, and an item with no probes never asks the tree"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestAdmit_AdmitsAnItemWithOneProbeThatStillFails|TestAdmit_AnItemWithNoProbesNeverAsksTheTree)$' ./internal/inboxcurator"
  - criterion: "The intake refuses a probe of an unknown kind, with no argument, or on a path outside the repository"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestCheckIntake_RefusesAProbeOfAnUnknownKindOrNoArgumentOrAPathOutsideTheRepository$' ./internal/inboxcurator"
  - criterion: "A sweep quarantines a probed item whose declared paths are all gone"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestSweep_EveryDeclaredPathGoneIsAQuarantine$' ./internal/inboxcurator"
  - criterion: "A sweep consumes deterministically when every probe passes, and that proposal replaces the quarantine and the test-share consume"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestSweep_EveryProbePassingIsADeterministicConsumeThatReplacesTheTestShareAndTheQuarantine$' ./internal/inboxcurator"
  - criterion: "The boundary verb refuses a passing-probe item at add time and proposes consume and quarantine at the sweep"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2399_001|TestC2399_004|TestC2399_005)' ./acs/cycle2399"
---

# Eval: Typed probes on new inbox items

> Pins AD2-7 (Graft B of docs/plans/architecture-decisions-2026-11.md). An item declares typed probes: facts that are true when its work is done. The intake refuses an item whose probes already pass, because the work is done. The boundary sweep consumes an item when every probe passes, and proposes a quarantine when every declared path is gone. Source incident: the stale-item audit of 2026-11, where merged work kept ranking in the top 12 of the inbox.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| intake-refuses-done | Probes that already pass refuse the item | 7/10 | `go test -run TestAdmit_RefusesAnItemWhoseProbesAlreadyPass ./internal/inboxcurator` |
| intake-admits-todo | One failing probe admits; no probes never asks the tree | 6/10 | `go test -run TestAdmit_AdmitsAnItemWithOneProbeThatStillFails ./internal/inboxcurator` |
| probe-typing | An unknown kind or a bad path is refused | 6/10 | `go test -run TestCheckIntake_RefusesAProbe ./internal/inboxcurator` |
| sweep-quarantine | Every declared path gone proposes a quarantine | 7/10 | `go test -run TestSweep_EveryDeclaredPathGoneIsAQuarantine ./internal/inboxcurator` |
| sweep-consume | Every probe passing proposes a deterministic consume | 7/10 | `go test -run TestSweep_EveryProbePassingIsADeterministicConsume ./internal/inboxcurator` |
| verb-wiring | The real verbs reach the seams | 7/10 | `go test -tags acs -run TestC2399 ./acs/cycle2399` |
