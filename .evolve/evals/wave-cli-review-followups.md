---
score_cap:
  - criterion: "The json flag of wave next comes from the parser: a note with the text --json does not add an envelope"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestWaveNext_TheJSONDecisionComesFromTheParser$' ./cmd/evolve"
  - criterion: "The wave record save retries once and stops after two failures"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestWaveNext_TheRecordSave(FailsOnceThenPasses|StopsAfterTwoFailures)$' ./cmd/evolve"
  - criterion: "The old wave closes only after the new record is saved"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestWaveNext_TheOldWaveClosesOnlyAfterTheNewRecordIsSaved$' ./cmd/evolve"
  - criterion: "An unknown process start time writes one WARN"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestWaveOwnsPID_AnUnknownStartTimeWarnsOnce$' ./cmd/evolve"
  - criterion: "The ledger seal test builds its own fixture and does not skip"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -v -run '^TestSeal_RealLedgerCopy$' ./internal/adapters/ledger | grep -q -- '--- PASS: TestSeal_RealLedgerCopy'"
---

# Eval: evolve wave review follow-ups

> Pins five LOW items from the round-2 review of `evolve wave` (inbox record
> `2026-10-08T22-00-00Z-wave-cli-review-followups`, source delta review of lane
> cl-wave-cli). Cycle 1949 turned each item into a test.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| one json source | parser and envelope agree on `--json` | 6/10 | `go test -run TestWaveNext_TheJSONDecisionComesFromTheParser` |
| tested retry | save retried, bounded at two attempts | 7/10 | `go test -run TestWaveNext_TheRecordSave...` |
| close order | old wave closes after the new save | 6/10 | `go test -run TestWaveNext_TheOldWaveClosesOnlyAfterTheNewRecordIsSaved` |
| start time | one WARN for an unknown start time | 6/10 | `go test -run TestWaveOwnsPID_AnUnknownStartTimeWarnsOnce` |
| hermetic ledger | no dependency on the worktree ledger | 5/10 | `go test -run TestSeal_RealLedgerCopy` |
