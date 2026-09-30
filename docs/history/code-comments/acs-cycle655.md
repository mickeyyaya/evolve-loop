# Comment history: `acs/cycle655`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle655/predicates_test.go:3` — above `package cycle655`

```text
// Package cycle655 encodes the acceptance criteria for the COMPLETION of
// builder-task-binding-topn-gate (8th recurrence of the wrong-task-build
// disease: cycles 282, 310, 522, 575, 577, 599, 640, 645). Cycle-652 built the
// gate correctly (audit PASS 0.94, adversarial PASS, its 4 ACS predicates
// green) but FAILed ship on ONE missing chore: the new package
// go/internal/topngate was never graduated into go/.apicover-enforce with an
// apicover_named_test.go, so the repo-wide completeness regression
// (TestApicoverEnforce_CoversEveryInternalPackage) would go RED once the
// package landed. This is the 3rd recurrence of the new-package-graduation gap
// (cycle 575 binaryguard, cycle 587 ciwatch, cycle 652 topngate).
//
// These predicates are BEHAVIORAL: 001-004 shell `go test` against the real
// go/internal/topngate package (the system under test) and 005 shells the real
// repo-wide apicover completeness gate, rather than grepping source — so a
// predicate greens only when the gate actually blocks/approves the right builds
// AND the package is genuinely graduated, not when a magic string is present.
// The white-box unit suite the Builder carries forward (verbatim from the
// cycle-652 worktree) lives at
// go/internal/topngate/{gate_test.go,reviewer_test.go,builder_authority_test.go}.
```

### `go/acs/cycle655/predicates_test.go:80` — above `func TestC655_003_ReplayCycle640ShapeBlocksBeforeAudit(t *testing.T) {`

```text
// TestC655_003_ReplayCycle640ShapeBlocksBeforeAudit re-verifies AC-3: replaying
// the cycle-640 shape (triage=statefile task, build=token-resolver task) blocks
// at the build->audit transition instead of consuming audit/ship phases, with
// an abort_reason naming the wrong-task slug. Exercised by the real
// TestReplayCycle640Shape regression, which asserts Review returns
// Approve=false with the wrong slug in Reason.
```

### `go/acs/cycle655/predicates_test.go:109` — above `func TestC655_005_TopngateApicoverGraduated(t *testing.T) {`

```text
// TestC655_005_TopngateApicoverGraduated binds the apicover half of AC-4 — the
// REMAINING GAP this completion cycle closes. Three assertions, all must hold:
//
//	(a) go/.apicover-enforce lists ./internal/topngate. This is an inherent
//	    config-presence check (membership in the enforce SSOT), waived below.
//	(b) go/internal/topngate/apicover_named_test.go exists and names NewReviewer
//	    — the package's only exported symbol (scout-verified). The graduation
//	    artifact CI's "api-coverage enforce" step exercises for correctness.
//	(c) THE LOAD-BEARING BEHAVIORAL ASSERTION: the real repo-wide completeness
//	    gate TestApicoverEnforce_CoversEveryInternalPackage PASSES. This shells
//	    the actual gate that failed ship in cycle-652 — it enumerates every
//	    ./internal/... package via `go list` and fails if any (including a
//	    carried-forward-but-ungraduated topngate) is absent from the SSOT, or
//	    if a stale/typo line was added. (c) is what makes (a) meaningful: you
//	    cannot green this predicate by pasting a garbage line — the package must
//	    genuinely exist AND be enumerated AND match the SSOT.
//
// RED at TDD time because (a) and (b) are absent in the fresh worktree; GREEN
// once the Builder carries the package forward and adds both artifacts.
```
