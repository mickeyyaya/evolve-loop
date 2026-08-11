---
score_cap:
  - criterion: "evolve ledger anchor refuses a seq carried by more than one distinct line, writing no anchor file"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestAnchor_RejectsAmbiguousSeq' ./internal/adapters/ledger/"
  - criterion: "--line-sha disambiguates an ambiguous seq and binds that exact line, and is rejected when it names no line or a line carrying a different seq"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestAnchor_LineSHA' ./internal/adapters/ledger/"
  - criterion: "anchoring an unambiguous seq with no flag keeps its pre-existing behavior (regression floor)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestAnchor_RecordsLineSHA' ./internal/adapters/ledger/"
---

# Eval: ledger anchor rejects a non-unique entry_seq

> `FileLedger.Anchor` (go/internal/adapters/ledger/anchor.go:157) binds the epoch
> anchor to the FIRST line whose `entry_seq` matches, and its own comment at
> line 173 acknowledges that siblings share a seq. Pre-CA.1 concurrent appends
> produced real sibling runs in the production ledger, so `evolve ledger anchor
> <seq>` could bind the EARLIER of two lines and move the trusted epoch anchor
> BACKWARD — silently re-exposing history the operator believed was sealed. That
> is a trust decision the operator never made. Source incident: inbox item
> `ledger-fleet-concurrency-chain` root-cause (b), cycle-1433; the backward-bind
> hazard was first recorded as a standing gotcha in the zero_ship_hardening
> campaign notes ("`ledger anchor <seq>` can bind BACKWARD — sibling seqs share
> numbers").
>
> This eval pins the fix permanently: ambiguity must be REFUSED (not resolved by
> arbitrary first-match), the disambiguation path must bind the exact named line,
> and the ordinary unambiguous path must not regress.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ambiguity-refused | A seq carried by >1 distinct line is refused with no anchor file written | 8/10 | `go test -run TestAnchor_RejectsAmbiguousSeq ./internal/adapters/ledger/` |
| line-sha-disambiguation | `--line-sha` binds the exact line; unknown/mismatched SHAs are refused | 7/10 | `go test -run TestAnchor_LineSHA ./internal/adapters/ledger/` |
| unambiguous-regression | The single-carrier path still anchors as before | 6/10 | `go test -run TestAnchor_RecordsLineSHA ./internal/adapters/ledger/` |
