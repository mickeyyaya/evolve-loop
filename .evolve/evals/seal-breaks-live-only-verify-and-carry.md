---
score_cap:
  - criterion: "Plain Verify passes on a sealed, untampered ledger (one seal, a keep-one seal, two seals with appends between)"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1827_001_' ./acs/cycle1827"
  - criterion: "Plain Verify still fails on a sealed ledger whose live tail was edited or lost its first line (the seam to the newest segment is checked)"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1827_002_' ./acs/cycle1827"
  - criterion: "A byte-identical carry re-proves through carrySatisfied on a sealed ledger and is declined when the sealed ledger's live tail lost a line"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestCarrySatisfied_(ReProvesOnASealedLedger|DeclinesWhenASealedLedgersLiveTailLostALine)$' ./internal/phases/ship"
  - criterion: "VerifyDeep queued behind Seal's truncate-then-anchor window reports no ErrSealResidue for the seal in progress"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1827_004_' ./acs/cycle1827"
  - criterion: "A seal that truncated but never anchored, with no seal running, still reports ErrSealResidue and re-running Seal completes it"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1827_005_' ./acs/cycle1827"
  - criterion: "docs/plans/ledger-restructure-2026-10.md lists this item as a hard prerequisite of C12 and C13"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1827_006_' ./acs/cycle1827"
---

# Eval: seal-breaks-live-only-verify-and-carry

> Pins that sealing the ledger no longer breaks live-only `Verify`, and so no longer disables every
> byte-identical carry: after `Seal` the first live line chains from a line in the newest segment, and
> `Verify` must start its strict walk from that segment's tail while still rejecting an edited or
> truncated live tail. It also pins that a `VerifyDeep` queued behind `Seal` never sees the
> truncated-but-unanchored window as residue, while a real interrupted seal still does. Source
> incident: the 2026-10-07 re-review of the carry lane (finding A4); reproduced in cycle 1825's
> bug-reproduction phase (`line 0 chained-genesis prev_hash mismatch` after `Seal`). Cycle 1825's
> audited change failed only at ship (tree drift from another lane); cycle 1827 re-binds the contract.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| sealed-verify-positive | plain Verify passes after one or two seals | 3/10 | `go test -tags acs -run TestC1827_001_ ./acs/cycle1827` |
| sealed-verify-negative | edited / truncated live tail still fails | 3/10 | `go test -tags acs -run TestC1827_002_ ./acs/cycle1827` |
| carry-on-sealed-ledger | carrySatisfied re-proves, declines a truncated tail | 3/10 | `go test -run TestCarrySatisfied_... ./internal/phases/ship` |
| seal-gap-no-residue | VerifyDeep behind Seal sees no residue | 4/10 | `go test -tags acs -run TestC1827_004_ ./acs/cycle1827` |
| residue-still-detected | an interrupted seal still reports ErrSealResidue | 5/10 | `go test -tags acs -run TestC1827_005_ ./acs/cycle1827` |
| plan-prerequisite | C12 and C13 rows name this item | 8/10 | `go test -tags acs -run TestC1827_006_ ./acs/cycle1827` |

## Graders
- [code] `cd go && go test -count=1 ./internal/adapters/ledger/... ./internal/phases/ship/...` exits 0, including tests: seal then plain Verify passes; tampered live line still fails Verify; seal then carrySatisfied passes; VerifyDeep inside Seal gap reports no ErrSealResidue. Run it with the `EVOLVE_FLEET*` variables unset: inside a fleet lane `TestWorktreeShipIntegrate_GoldenArgvLogsAndErrors`, `TestUserContract` and `TestNewlyAddedRedViaRunNative` fail on the unmodified base as well.
- [code] negative: tampered live line after seal makes Verify return an error (`TestC1827_002_`, three tamperings).
- [code] negative: a seal that truncated but never anchored, with no seal running, still reports ErrSealResidue (`TestC1827_005_`).
- [code] `grep -q 'seal-breaks-live-only-verify-and-carry' docs/plans/ledger-restructure-2026-10.md`
