---
score_cap:
  - criterion: "cmd/evolve.parseLoopArgs is at or under the 50-line size-ratchet ceiling"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_001_ParseLoopArgsShrunkToAllowance ./acs/cycle1751"
  - criterion: "go/internal/sizeratchet/offenders.json no longer lists cmd/evolve.parseLoopArgs"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_004_OffendersJSONDropsParseLoopArgs ./acs/cycle1751"
  - criterion: "the full existing TestParseLoopArgs_* subtest family, including the negative TestParseLoopArgs_MalformedCLIFlagRejected case, still passes after extraction"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_007_ParseLoopArgsTestFamilyPasses ./acs/cycle1751"
  - criterion: "the module still builds and go vet ./cmd/evolve/... is clean after the extraction"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_010_BuildAndVetClean ./acs/cycle1751"
---

# Eval: Shrink cmd/evolve.parseLoopArgs below the size-ratchet ceiling

> Pins the cycle-1751 lane-writable size-ratchet shrink of `cmd/evolve.parseLoopArgs`
> (go/cmd/evolve/cmd_loop_args.go), one of three offenders selected by scout from
> `go/internal/sizeratchet/offenders.json`. The function must shrink to the module's
> 50-line ceiling via behavior-preserving extraction (no logic change) and its
> offenders.json entry must be removed, not resized — the ratchet is a ceiling per
> the wave-25 lesson already cited in this cycle's goal text. The full existing
> `TestParseLoopArgs_*` subtest family (19 subtests across 7 files), including the
> negative `TestParseLoopArgs_MalformedCLIFlagRejected` case, must still pass, and
the module must still build and vet clean.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| size-ceiling | Function shrinks to ≤50 lines | 8/10 | `TestC1751_001_ParseLoopArgsShrunkToAllowance` |
| ratchet-entry-removed | offenders.json entry removed (not resized) | 7/10 | `TestC1751_004_OffendersJSONDropsParseLoopArgs` |
| regression+negative | Full subtest family (incl. negative case) passes | 9/10 | `TestC1751_007_ParseLoopArgsTestFamilyPasses` |
| build-clean | `go build ./...` and `go vet ./cmd/evolve/...` exit 0 | 8/10 | `TestC1751_010_BuildAndVetClean` |
