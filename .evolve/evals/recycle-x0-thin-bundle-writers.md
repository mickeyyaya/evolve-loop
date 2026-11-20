---
score_cap:
  - criterion: "A branch-sweep leaf and a salvage leaf of a branch one commit past origin/main hold a bundle under 1 MB that verifies"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2558_001|TestC2558_002' ./acs/cycle2558"
  - criterion: "A tip that origin/main holds gets a leaf HEAD with the tip and no bundle, and the release goes on"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2558_003|TestC2558_004' ./acs/cycle2558"
  - criterion: "A failed bundle create or verify step keeps the tree and the branch"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2558_005|TestC2558_006' ./acs/cycle2558"
  - criterion: "Without an origin/main ref the bundle still holds the branch and verifies"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2558_007' ./acs/cycle2558"
---

# Eval: Salvage and branch-sweep bundles hold only the commits that origin/main does not hold

> Pins the thin-bundle contract of `bundleVerified` (go/internal/gc/salvage.go). Before the fix the bundle ran without an `^origin/main` base, so each salvage leaf held the full branch history (3 MB for a one-commit branch in the fixture). Source incident: cycle 2558, bug-reproduction report.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| thin-leaf | One commit past origin/main gives a bundle under 1 MB that verifies | 4/10 | `go test -tags acs -run 'TestC2558_001\|TestC2558_002' ./acs/cycle2558` |
| landed-tip | Tip in origin/main gives HEAD and no bundle, release continues | 4/10 | `go test -tags acs -run 'TestC2558_003\|TestC2558_004' ./acs/cycle2558` |
| failed-step | A failed bundle step keeps tree and branch | 5/10 | `go test -tags acs -run 'TestC2558_005\|TestC2558_006' ./acs/cycle2558` |
| no-origin | Without origin/main the full bundle still verifies | 6/10 | `go test -tags acs -run TestC2558_007 ./acs/cycle2558` |
