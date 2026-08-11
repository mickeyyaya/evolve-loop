---
score_cap:
  - criterion: "The live console-plane ledger (.evolve/ledger.jsonl) deep-verifies: `evolve ledger verify --deep` exits 0"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run TestC1435_001_ConsoleLedgerDeepVerifyGreen ./acs/cycle1435"
  - criterion: "The repair was an attributable operator rebaseline: an operator-role reset-seal-rebaseline entry citing the inbox item and cycle, bound to the .evolve/ledger-rebaseline.json evidence artifact"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1435_003_RebaselineSealBoundToEvidenceArtifact ./acs/cycle1435"
  - criterion: "The repair is append-only: the damaged prefix (first 114400 lines, containing the line-114368 break) is preserved byte-for-byte"
    max_if_missing: 2
    evidence: "cd go && go test -tags acs -count=1 -run TestC1435_004_LedgerPrefixPreservedByteForByte ./acs/cycle1435"
  - criterion: "`ledger verify --deep` still rejects a planted prev_hash break (the green in criterion 1 is not a neutered detector)"
    max_if_missing: 2
    evidence: "cd go && go test -tags acs -count=1 -run TestC1435_002_DeepVerifyStillDetectsPlantedBreak ./acs/cycle1435"
  - criterion: "`ledger rebaseline` refuses an empty --note and writes nothing (the operator gate on a bulk trust decision)"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC1435_006_RebaselineRefusesUnattributedNote ./acs/cycle1435"
---

# Eval: Live rebaseline of the console-plane ledger

> Pins the outcome of the `ledger-fleet-concurrency-chain` campaign's last open
> operational item. The root-cause code — flock-serialized `appendChained`,
> anchor-ambiguity rejection, and `evolve ledger rebaseline` — shipped in
> cycle-1433 and is unit-tested, but the real `.evolve/ledger.jsonl` was never
> repaired: at cycle-1435 authoring time `evolve ledger verify --deep` exited 2
> with `BROKEN: line 114368 prev_hash mismatch` on a 115,231-line file. This eval
> keeps the console plane green and, just as important, keeps the *shape* of the
> repair honest: append-only (the damaged prefix stays byte-for-byte on disk, per
> ADR-0048's preservation choice over a destructive rebuild) and attributable (a
> named operator note, per the `--note` gate in `rebaseline.go:47`). A future
> cycle that "greens" the ledger by truncating or hand-editing the damaged region
> fails criteria 2 and 3 even though criterion 1 would pass.
>
> Source incident: cycle-1435 (item `ledger-fleet-concurrency-chain`; the break
> had drifted 78729 → 114368 as appends continued against the unrepaired chain).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| chain-green | `verify --deep` exits 0 on the live ledger | 3/10 | `go test -tags acs -run TestC1435_001_ConsoleLedgerDeepVerifyGreen` |
| attributable-seal | operator `reset-seal-rebaseline` note cites item+cycle and matches the evidence artifact | 5/10 | `go test -tags acs -run TestC1435_003_RebaselineSealBoundToEvidenceArtifact` |
| append-only | first 114400 lines byte-identical (sha256 `b7088c02…`, 35510006 bytes) | 2/10 | `go test -tags acs -run TestC1435_004_LedgerPrefixPreservedByteForByte` |
| detector-live | planted prev_hash break still rejected | 2/10 | `go test -tags acs -run TestC1435_002_DeepVerifyStillDetectsPlantedBreak` |
| operator-gate | empty `--note` refused, ledger unmutated | 4/10 | `go test -tags acs -run TestC1435_006_RebaselineRefusesUnattributedNote` |
