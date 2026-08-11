---
score_cap:
  - criterion: "knowledge/architecture/state-and-ledger.md documents that Verify's expected tip is walkChain's re-derived lastSha from effectiveAnchorSHA forward, not a raw ledger.tip sidecar read"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1435_005_TipWitnessDocumented ./acs/cycle1435"
  - criterion: "The doc names the concrete code path (a ledger.go: line citation) so the claim can be re-verified against source rather than trusted"
    max_if_missing: 8
    evidence: "grep -q 'ledger.go:' knowledge/architecture/state-and-ledger.md"
---

# Eval: Tip-witness resolution documented in the ledger architecture doc

> Pins the documentation of a non-obvious integrity property. `Verify`'s `want`
> tip is **not** read from the `ledger.tip` sidecar — it is the `lastSha`
> re-derived by `walkChain` replaying every line forward from
> `effectiveAnchorSHA` (`go/internal/adapters/ledger/ledger.go:187`, `:292`,
> `:304-314`); the on-disk `ledger.tip` is only ever *compared against* that
> re-derived value, never trusted as the source of truth. That distinction is why
> a stale or forged `ledger.tip` cannot green a broken chain, and at cycle-1435 it
> existed only in code comments: `state-and-ledger.md` (235 lines) documented
> `Append`'s tip rewrite and contained zero occurrences of `effectiveAnchorSHA`
> or `walkChain`. An operator reading only the doc would reasonably conclude the
> sidecar is authoritative — the exact misreading this eval prevents from
> returning.
>
> Source incident: cycle-1435 (item `ledger-fleet-concurrency-chain`, Task 2).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| tip-witness-documented | doc names `effectiveAnchorSHA`, `walkChain`, `ledger.tip` and is git-tracked | 6/10 | `go test -tags acs -run TestC1435_005_TipWitnessDocumented` |
| source-citation | doc cites a concrete `ledger.go:` line so the claim is re-verifiable | 8/10 | `grep -q 'ledger.go:' knowledge/architecture/state-and-ledger.md` |
