---
score_cap:
  - criterion: "A successful `evolve ledger verify` over a break → eligible operator seal → valid tail states the sealed prefix it trusted, naming that anchor's own identity (entry_seq or line SHA) rather than a fixed string"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1678_006_SealedPrefixVerifiesAndNamesItsAnchor|TestC1678_008_AnchorProvenanceIsDerivedFromTheLedger' ./acs/cycle1678"
  - criterion: "The real repository ledger verifies, states the scope it verified, and is not mutated by the read"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1678_009_LiveLedgerVerifiesReportsItsScopeAndIsNotMutated ./acs/cycle1678"
  - criterion: "A break AFTER the last eligible seal still returns a chain-broken error (exit 2) — a seal covers the prefix behind it, never the tail ahead of it"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1678_007_BreakAfterTheLastSealStillReportsBroken ./acs/cycle1678"
---

# Eval: ledger verify states the sealed prefix it trusted

> Pins the observable contract of `evolve ledger verify` once epoch-anchor
> resolution is in play. The resolver itself (`effectiveAnchorSHA` in
> `go/internal/adapters/ledger/anchor.go`, plus `walkChain`'s strict resume)
> landed in cycle-1191 and is correct; what this eval protects is the operator's
> ability to tell the two very different successes apart. Before cycle-1677,
> `runLedgerVerify` printed `[ledger] OK: chain intact (<dir>/ledger.jsonl)`
> whether it had validated every byte from genesis or had deliberately trusted a
> 136k-line adjudicated prefix — one string for two claims, which is exactly the
> shape that let the ledger-1740 damage stay invisible for as long as it did
> (cycle-1191: unit-green, live-red).
>
> The cap is written against BEHAVIOUR, not prose: the provenance must carry the
> anchor's own identity (its `entry_seq` or its bound line SHA), so a literal in
> the success path cannot satisfy two ledgers with different seals. The refusal
> criterion is capped hardest because it is the safety half — a seal that could
> be read as covering a later forged tail would turn a preservation remedy into a
> chain-integrity bypass.
>
> Provenance note (cycle-1678): the behaviour was authored in cycle-1677, whose
> ship failed on an unrelated importer red; the work reached main through the
> cycle-1678 continuation, so the evidence commands name that cycle's predicate
> package — the suite the shipping cycle's gate actually ran. The criteria are
> unchanged from the cycle-1677 authoring.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| sealed-prefix-provenance | Success names the epoch anchor it started from, derived from the ledger | 6/10 | `go test -tags acs -run 'TestC1678_006…\|TestC1678_008…' ./acs/cycle1678` |
| live-corpus-and-read-only | The real 140k-line ledger verifies, reports its scope, and is never rewritten by a read | 7/10 | `go test -tags acs -run TestC1678_009… ./acs/cycle1678` |
| post-seal-break-refused | A forged line one past the anchor still exits 2 with a chain-broken error | 8/10 | `go test -tags acs -run TestC1678_007… ./acs/cycle1678` |
