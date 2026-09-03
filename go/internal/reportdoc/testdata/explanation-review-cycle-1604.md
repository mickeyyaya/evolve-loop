## Explanation Documentation
- Status: VERIFIED
- Build status: required
- Document: docs/explain/builds/cycle-1604-01m1fd688vm2a4y7kz2nqyt870.md
- Document SHA256: 0e8ab3b608dc77e9c54dfcb127d218d85633d21443e14aa53db0f231ee3da58c
- Evidence: `shasum -a 256` on the worktree copy reproduces the handoff SHA exactly; an in-package `-overlay` probe re-ran `explanationdocs.diffSHA256` against the base SHA and reproduced `ed578d570d40b523645aae16873ab430a3acb1cc113dca83e69f1c21c2afb9d8` with `material_paths=[go/internal/phaseio/digest.go go/internal/phases/tdd/tdd.go]`, matching `build-explanation.json:10-14` — the host whole-diff verification is sound.
- Evidence: docs/explain/builds/cycle-1604-01m1fd688vm2a4y7kz2nqyt870.md:22 states the degraded-before-generic ordering implemented at go/internal/phaseio/digest.go:29-31, and — unlike the cycle-1603 document that earned `dad80abb…` — correctly narrows the claim to "a large **generic** value", which is exactly the path probe C confirms and does not overstate the typed-plane gap in `M1`.
- Evidence: docs/explain/builds/cycle-1604-01m1fd688vm2a4y7kz2nqyt870.md:28 claims byte-identical inactive output, reproduced by `TestC1604_008` at go/acs/cycle1604/predicates_test.go:207-232.
- Evidence: docs/explain/builds/cycle-1604-01m1fd688vm2a4y7kz2nqyt870.md:14-19 lists six Changed Areas; every one resolves to a real path in the staged diff and no path is invented (the document itself is the seventh staged path).
- Note: the document is scoped to the digest mechanism and does not repeat the build report's closure framing, so `C1` is not an explanation-review defect.

