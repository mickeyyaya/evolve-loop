# Comment history: `acs/cycle1130`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1130/predicates_test.go:3` — above `package cycle1130`

```text
// Package cycle1130 materialises the cycle-1130 acceptance criteria for the
// single fleet-scoped task `surface-verdict-conflict-in-audit-classify` (inbox
// item `verdict-coherence-auditor-vs-egps`, weight 0.92, kind
// pipeline-integrity, 4th recurrence of the cycle-87 / cycle-352 / cycle-456
// family; live cases 1107 / 1116 / 1117).
//
// STATE AT HEAD — read this before treating a green run as a no-op. HEAD is
// 27059076, the ADR-0076 continuation-on-fail salvage snapshot, which carries a
// PRIOR attempt's implementation of this exact task: hooks.Classify already
// captures the auditor's narrative verdict and emits an error-severity
// `verdict-conflict:` record across all ten override paths, and the cycle1127
// predicates for the same family are green. The task's own predicates are
// therefore PRE-EXISTING GREEN at HEAD by inheritance, not by no-op.
//
// Non-degeneracy was proven by counterfactual, not by assertion: with
// go/internal/phases/audit/audit.go reverted to the pre-salvage parent
// (bc2e3236) the producer predicate below FAILS on both EGPS branches
// ("no error-severity diagnostic carries verdict-conflict / PASS / WARN"),
// while the anti-noise predicate stays green — exactly the split a correct
// guard should show. Evidence is pasted in the cycle-1130 test-report.md.
//
// What cycle 1130 adds over the inherited cycle1124/cycle1127 suites: those pin
// that a conflict record EXISTS, is verbatim, is per-gate distinguishable, and
// is silent when coherent. None of them pin the scout report's actual
// verifiableBy — that ONE Classify call hands the operator BOTH halves of the
// forensic pair (the auditor's declared verdict AND the gate's red_count /
// normalized red identity), both at Severity=="error" so both ride
// cyclestate.ErrorMessages → AuditFailReasons → <phase>-fail-reason.json → the
// dossier's SubstantiveError. A refactor that keeps the record but drops the
// gate facts beside it — or demotes either to warning — leaves the operator
// with the same half-picture cycles 1107/1116/1117 already had, and is caught
// here.
//
// Predicate strategy — BEHAVIORAL, never a source-grep (the cycle-85
// degenerate-predicate ban). `hooks` and `Classify` are unexported, so each
// predicate runs the REAL in-package test binary as a subprocess against THIS
// worktree and asserts on its exit code. No predicate here asserts that a
// source file contains a string.
//
//   - 001 producer (AC-1): both halves of the pair arrive together, at error
//     severity, on the red_count>0 and ship_eligible=false branches.
//   - 002 anti-no-op (AC-2/AC-3): the gate's evidence survives on the COHERENT
//     path where no conflict record is emitted, and the clean-PASS path emits
//     neither half. An implementation that only ever emits facts alongside a
//     conflict record greens 001 and fails here.
//   - 003 no-regression under -race (AC-4): the AC names `-race` explicitly,
//     and the inherited cycle1127 predicates run without it — this is the only
//     predicate in the family that would catch a data race introduced in the
//     conflict path.
```

### `go/acs/cycle1130/predicates_test.go:71` — above `func runGoTest(t *testing.T, pkg, pattern string, race bool) (ok bool, out string) {`

```text
// runGoTest runs `go test -C <worktree>/go [-race] -run ^(pattern)$ <pkg>` and
// reports whether it exited 0. A compile failure in the target package (a
// legitimate RED signal) surfaces as a non-zero exit, NOT as a launch failure:
// SubprocessOutput returns a non-nil err for any non-zero exit, so only
// code < 0 is a genuine "could not launch" (cycle-574 lesson).
```
