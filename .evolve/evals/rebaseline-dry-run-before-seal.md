---
score_cap:
  - criterion: "A rebaseline whose seal would not verify forward is refused and leaves ledger.jsonl byte-identical (durable package regression test)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -v -run '^TestRebaseline_RefusesWriteWhenForwardWalkWouldFail$' ./internal/adapters/ledger/ | grep -q -- '--- PASS: TestRebaseline_RefusesWriteWhenForwardWalkWouldFail'"
  - criterion: "Stale epoch-anchor sidecar: Rebaseline refuses, carries core.ErrLedgerChainBroken, and writes no ledger file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1756_001_StaleAnchorRebaselineRefusesAndWritesNothing$' ./acs/cycle1756 | grep -q -- '--- PASS: TestC1756_001_StaleAnchorRebaselineRefusesAndWritesNothing'"
  - criterion: "Unanchored sealed segment: the dry-run covers segment binding, so Rebaseline refuses with ledger.ErrSealResidue and writes nothing"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1756_002_UnanchoredSegmentRebaselineRefusesAndWritesNothing$' ./acs/cycle1756 | grep -q -- '--- PASS: TestC1756_002_UnanchoredSegmentRebaselineRefusesAndWritesNothing'"
  - criterion: "Ledgers one seal repairs (damaged prefix, foreign tail, break after a sidecar anchor, anchored segment) still get exactly one seal, verify, and move the tip"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1756_003_RepairableLedgersStillSealAndVerify$' ./acs/cycle1756 | grep -q -- '--- PASS: TestC1756_003_RepairableLedgersStillSealAndVerify'"
  - criterion: "A malformed ledger.tip is refused without writing anything"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1756_004_MalformedTipRebaselineRefusesAndWritesNothing$' ./acs/cycle1756 | grep -q -- '--- PASS: TestC1756_004_MalformedTipRebaselineRefusesAndWritesNothing'"
  - criterion: "The ledger package suite stays green, including the pre-existing Rebaseline tests"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestRebaseline_SealsDamagedPrefix$' ./internal/adapters/ledger/ | grep -q -- '--- PASS: TestRebaseline_SealsDamagedPrefix' && go test -count=1 ./internal/adapters/ledger/"
---

# Eval: Rebaseline dry-runs its seal before writing

> Pins the ordering contract of `FileLedger.Rebaseline`
> (`go/internal/adapters/ledger/rebaseline.go`): the method must decide whether
> its operator seal would verify forward (the chain walk from the prospective
> seal point plus the segment-binding check `VerifyDeep` applies) using the
> lines it already loaded, and append the seal only when that dry-run passes. A
> refusal must leave `ledger.jsonl`, `ledger.tip`, `ledger-anchor.json` and the
> segments byte-identical. Before the fix, `Rebaseline` appended first and
> verified afterward. A stale anchor sidecar or an unanchored segment therefore
> grew the ledger by one seal line and moved the tip, and then returned an
> error. Source: auditor prescription d9bc45f9 (first seen cycle 1433),
> selected from the `retro-failure-identity` sweep in cycle 1756 and reproduced
> by that cycle's bug-reproduction phase (ledger grew 552 -> 822 bytes on a
> refused call).

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test -count=1 -v -run '^TestRebaseline_RefusesWriteWhenForwardWalkWouldFail$' ./internal/adapters/ledger/ | grep -q -- '--- PASS: TestRebaseline_RefusesWriteWhenForwardWalkWouldFail'`
- `[code]` `cd go && go test -tags acs -count=1 -run '^TestC1756_00[1-4]_' ./acs/cycle1756`
- `[code]` `cd go && go test -count=1 ./internal/adapters/ledger/`

## Adversarial Cases
- Negative: a stale anchor sidecar and an unanchored segment must both be refused with nothing written (001, 002).
- Edge/OOD: a malformed `ledger.tip` must be refused with nothing written (004).
- Cheapest gaming fakes: an always-refuse `Rebaseline` fails 003. A walk-only dry-run that skips segment binding fails 002. A refusal that drops its `%w` cause fails 001 and 002. All three were mutation-probed in cycle 1756.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| durable-regression | package test pins refuse-without-writing | 8/10 | `go test -run '^TestRebaseline_RefusesWriteWhenForwardWalkWouldFail$' ./internal/adapters/ledger/` |
| walk-class-refusal | stale anchor sidecar refused, byte-identical, cause kept | 8/10 | `go test -tags acs -run TestC1756_001 ./acs/cycle1756` |
| binding-class-refusal | unanchored segment refused, byte-identical, ErrSealResidue kept | 7/10 | `go test -tags acs -run TestC1756_002 ./acs/cycle1756` |
| anti-no-op | repairable ledgers still seal, verify and move the tip | 7/10 | `go test -tags acs -run TestC1756_003 ./acs/cycle1756` |
| malformed-edge | malformed tip refused without writing | 4/10 | `go test -tags acs -run TestC1756_004 ./acs/cycle1756` |
| no-regression | ledger package suite green | 6/10 | `go test -count=1 ./internal/adapters/ledger/` |
