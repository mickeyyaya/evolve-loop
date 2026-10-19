---
score_cap:
  - criterion: "A ledger entry for an ask id that the request does not have is refused, and a sound ledger is still accepted"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1952_001_ALedgerEntryForAnUnknownAskIsRefused ./acs/cycle1952"
  - criterion: "A ledger whose request_digest differs from the digest of the request, or whose pair differs, is refused"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1952_002_ALedgerWithAStaleRequestDigestIsRefused ./acs/cycle1952"
  - criterion: "Each ask needs exactly one entry and a not_applicable entry needs a note"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1952_003_EachAskNeedsExactlyOneEntryAndANotApplicableEntryNeedsANote ./acs/cycle1952"
  - criterion: "./internal/handoff is at 100 in go/.cover-strict, enrolled in .apicover-enforce, and passes apicover -enforce"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1952_004_TheHandoffPackageIsAtOneHundredAndEnrolledInBothGates ./acs/cycle1952"
  - criterion: "The package note under docs/architecture/packages links docs/architecture/phase-handoff-contract.md and the link resolves"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1952_005_ThePackageNoteLinksTheSpecAndTheLinkResolves ./acs/cycle1952"
---

# Eval: Phase handoff H1, the handoff types and the ledger schema, parse and validate

> Pins the first component of ADR-0141: the new package `go/internal/handoff` with `Request`, `Ask`, `Ledger`, `Entry`, `Exit`, `Parse` and `Validate`. The refusal table is the contract: a ledger that names an unknown ask, binds a stale request digest, lacks an entry, doubles an entry or leaves a not_applicable entry without a note must not pass. The package is a leaf; its production callers arrive with H2 to H8. Source: cycle 1952, spec `docs/architecture/phase-handoff-contract.md` section 4.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| unknown-ask | an entry for a missing ask id is refused | 8/10 | `TestC1952_001` |
| stale-digest | a stale request_digest or a wrong pair is refused | 8/10 | `TestC1952_002` |
| one-entry-per-ask | missing, duplicate and note-less not_applicable entries are refused | 8/10 | `TestC1952_003` |
| gates | cover-strict 100, apicover-enforce enrollment | 7/10 | `TestC1952_004` |
| package-note | the note links the spec | 5/10 | `TestC1952_005` |
