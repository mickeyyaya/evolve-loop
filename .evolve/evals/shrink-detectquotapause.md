---
score_cap:
  - criterion: "cmd/evolve.detectQuotaPause is at or under the 50-line size-ratchet ceiling"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_003_DetectQuotaPauseShrunkToAllowance ./acs/cycle1751"
  - criterion: "go/internal/sizeratchet/offenders.json no longer lists cmd/evolve.detectQuotaPause"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_006_OffendersJSONDropsDetectQuotaPause ./acs/cycle1751"
  - criterion: "the full existing TestDetectQuotaPause_* subtest family, including the edge-case TestDetectQuotaPause_EmptySourceReadsAsUnknown case, still passes after extraction"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_009_DetectQuotaPauseTestFamilyPasses ./acs/cycle1751"
  - criterion: "the module still builds and go vet ./cmd/evolve/... is clean after the extraction"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1751_010_BuildAndVetClean ./acs/cycle1751"
---

# Eval: Shrink cmd/evolve.detectQuotaPause below the size-ratchet ceiling

> Pins the cycle-1751 lane-writable size-ratchet shrink of `cmd/evolve.detectQuotaPause`
> (go/cmd/evolve/cmd_loop_control.go), one of three offenders scout selected from
> `go/internal/sizeratchet/offenders.json`. The quota-pause marker detector must
> shrink to the module's 50-line ceiling via behavior-preserving extraction (no logic
> change), and its offenders.json entry must be removed, not resized. The full
> existing `TestDetectQuotaPause_*` subtest family, including the edge-case
> `TestDetectQuotaPause_EmptySourceReadsAsUnknown` case (empty-source-as-unknown
> fallback), must still pass, and the module must still build and vet clean.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| size-ceiling | Function shrinks to ≤50 lines | 8/10 | `TestC1751_003_DetectQuotaPauseShrunkToAllowance` |
| ratchet-entry-removed | offenders.json entry removed (not resized) | 7/10 | `TestC1751_006_OffendersJSONDropsDetectQuotaPause` |
| regression+edge | Full subtest family (incl. edge case) passes | 9/10 | `TestC1751_009_DetectQuotaPauseTestFamilyPasses` |
| build-clean | `go build ./...` and `go vet ./cmd/evolve/...` exit 0 | 8/10 | `TestC1751_010_BuildAndVetClean` |
