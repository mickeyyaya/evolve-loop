---
score_cap:
  - criterion: "The batch hook scope never plans or deletes a non-cycle branch"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2244_001' ./acs/cycle2244"
  - criterion: "The operator run deletes a landed non-cycle branch only after a verified refs.bundle holds its tip"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2244_004' ./acs/cycle2244"
  - criterion: "The operator run deletes an ended non-cycle branch only after a verified refs.bundle holds its tip"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2244_005' ./acs/cycle2244"
  - criterion: "A branch whose bundle cannot be written, a fresh unlanded branch and main are kept"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2244_006|TestC2244_007' ./acs/cycle2244"
  - criterion: "Every changed function of go/internal/gc is at 100% under the BranchSweep tests"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2244_008' ./acs/cycle2244"
---

# Eval: gc X6 branch sweep, hook and operator

> Pins component X6 of docs/plans/gc-closes-the-legacy-gap-2026-11.md (D2, D7, D8). The batch hook sweeps only cycle-* branches. The operator run also sweeps other local branches that are landed or ended, after `git bundle verify` passes on a refs.bundle in the operator-salvage store. Source: cycle 2244.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| hook-scope | the hook never deletes feat/x | 3/10 | `TestC2244_001` |
| bundle-first | delete only after a verified bundle | 4/10 | `TestC2244_004` |
| ended-rule | an ended branch goes after its bundle | 5/10 | `TestC2244_005` |
| keep-rules | no bundle, fresh or main stays | 3/10 | `TestC2244_006`, `TestC2244_007` |
| coverage | changed gc functions at 100% | 6/10 | `TestC2244_008` |
