---
score_cap:
  - criterion: "A recurrence reweight goes through inboxmover and appends one inbox-lifecycle edit line to the project's ledger"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1932_001_RecurrenceReweightAppendsAnInboxLifecycleEditLine ./acs/cycle1932"
  - criterion: "A recurrence reweight passes the curator weight rule: a target above one is refused and the item keeps its weight"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1932_002_RecurrenceReweightRefusesAWeightTheCuratorRuleRejects ./acs/cycle1932"
  - criterion: "A reweight that would not raise the weight leaves the item bytes and the ledger untouched"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1932_003_RecurrenceReweightNeverLowersAndLeavesTheItemUntouched ./acs/cycle1932"
  - criterion: "No inbox item write outside internal/inboxmover: the write-site scan passes with no recurrence enrollment"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestInboxWriters_EveryAutofiledItemCarriesAKnownClass/every_production_write_into_the_inbox_is_enrolled' ./cmd/evolve && ! grep -q '\"internal/recurrence' cmd/evolve/inbox_writers_contract_test.go"
  - criterion: "internal/recurrence delegates the item write to the injected ApplyOptions.Reweight seam and writes neither the item nor a ledger line itself"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1932_00[567]' ./acs/cycle1932"
  - criterion: "The loop boundary caller applyEscalationBoundary wires the seam to inboxmover.Edit"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestApplyEscalationBoundary_ReweightsAnOpenItemThroughInboxmover$' ./cmd/evolve"
---

# Eval: route the recurrence reweight through inboxmover

> Pins the fix for inbox item `recurrence-reweight-bypasses-the-item-writer`.
> `recurrence.setItemWeight` rewrote the curator-owned `weight` field with `atomicwrite.JSON`.
> That was a second item writer outside `internal/inboxmover` (ADR-0116 owner table).
> It skipped the edit validation and the `inbox-lifecycle` ledger line, and under ADR-0112 its write is not `IsMoverWritten`.
> Source incident: cycle 1932 bug reproduction (`ApplyBoundary` bumped the item, the ledger kept only its seed line).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ledger-line | reweight appends an inbox-lifecycle edit line | 8/10 | `TestC1932_001_...` |
| weight-rule | target above one is refused | 7/10 | `TestC1932_002_...` |
| preservation | a non-raising reweight writes nothing | 5/10 | `TestC1932_003_...` |
| single-writer | write-site scan passes without a recurrence enrollment | 7/10 | `TestInboxWriters_...` + grep |
| seam-only | recurrence writes nothing itself; a nil or refusing seam bumps nothing | 8/10 | `TestC1932_005..007` |
| caller-proof | the loop boundary reweights through inboxmover | 8/10 | `TestApplyEscalationBoundary_ReweightsAnOpenItemThroughInboxmover` |

Cycle 1932 audit round 1 (H1) motivated the seam rows.
The first build kept `atomicwrite.JSON` and a copied ledger chain writer in `internal/recurrence`, and the write-site scan missed them.
