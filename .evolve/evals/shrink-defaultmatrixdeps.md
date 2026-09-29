---
score_cap:
  - criterion: "cmd/evolve.defaultMatrixDeps is at or under the 50-line size-ratchet ceiling"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_002_DefaultMatrixDepsShrunkToAllowance ./acs/cycle1751"
  - criterion: "go/internal/sizeratchet/offenders.json no longer lists cmd/evolve.defaultMatrixDeps"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_005_OffendersJSONDropsDefaultMatrixDeps ./acs/cycle1751"
  - criterion: "the full existing TestVerifyReleaseCLIMatrix_* subtest family, including the negative TestVerifyReleaseCLIMatrix_AllFailVisible case, still passes after extraction"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_008_DefaultMatrixDepsTestFamilyPasses ./acs/cycle1751"
  - criterion: "the module still builds and go vet ./cmd/evolve/... is clean after the extraction"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_010_BuildAndVetClean ./acs/cycle1751"
---

# Eval: Shrink cmd/evolve.defaultMatrixDeps below the size-ratchet ceiling

> Pins the cycle-1751 lane-writable size-ratchet shrink of `cmd/evolve.defaultMatrixDeps`
> (go/cmd/evolve/cmd_release_verify_clis.go), the smallest and lowest-risk of the three
> offenders scout selected from `go/internal/sizeratchet/offenders.json`. The static
> per-CLI dependency table constructor must shrink to the module's 50-line ceiling via
> behavior-preserving extraction (no logic change), splitting per-CLI-family table
> construction from assembly, and its offenders.json entry must be removed, not
> resized. The full existing `TestVerifyReleaseCLIMatrix_*` subtest family, including
> the negative `TestVerifyReleaseCLIMatrix_AllFailVisible` case, must still pass, and
> the module must still build and vet clean.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| size-ceiling | Function shrinks to ≤50 lines | 8/10 | `TestC1751_002_DefaultMatrixDepsShrunkToAllowance` |
| ratchet-entry-removed | offenders.json entry removed (not resized) | 7/10 | `TestC1751_005_OffendersJSONDropsDefaultMatrixDeps` |
| regression+negative | Full subtest family (incl. negative case) passes | 9/10 | `TestC1751_008_DefaultMatrixDepsTestFamilyPasses` |
| build-clean | `go build ./...` and `go vet ./cmd/evolve/...` exit 0 | 8/10 | `TestC1751_010_BuildAndVetClean` |
