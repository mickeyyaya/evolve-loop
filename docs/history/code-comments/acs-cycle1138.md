# Comment history: `acs/cycle1138`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1138/predicates_test.go:3` — above `package cycle1138`

```text
// Package cycle1138 materialises the cycle-1138 acceptance criteria for this
// fleet lane's single assigned item:
//
//   - warn-failed-verify-still-requires-real-report → regression-lock that a
//     bare FAIL/WARN verdict sentinel does NOT buy a phase out of the
//     deliverable contract's required-Sections check.
//
// The invariant. `deliverable.verifyMarkdown` (go/internal/deliverable/deliverable.go:134-158)
// runs the required-`Sections` loop unconditionally, ahead of and independent of
// the verdict-sentinel / failure-context block. Nothing between the two returns
// early. So "I am reporting FAIL, therefore I need not write the real report" is
// not, and must never become, a legal reading of the contract — the report body
// is owed on every verdict. Scout's read says the invariant already holds in
// code but is untested: no case pairs "sentinel declares FAIL/WARN" with
// "required sections absent". This cycle's deliverable is the executable lock.
//
// Predicate strategy — each predicate exercises the system under test, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban):
//
//   - 001 CRUX / behavioural: calls the real `deliverable.Verify` on a
//     build-report whose entire body is a FAIL sentinel (then a WARN sentinel)
//     and asserts the missing_section violation still fires. This is the
//     invariant itself, asserted against production code — it red-fails the
//     instant anyone adds a short-circuit that skips section-checking on
//     FAIL/WARN, whether or not the Builder's unit test survives.
//   - 002 NEGATIVE CONTROL / anti-no-op: the same FAIL-sentinel report WITH the
//     required `## Changes` section present must NOT report missing_section.
//     Without this, 001 would also pass against a hypothetical broken verifier
//     that flags missing_section unconditionally; together they pin the check to
//     the actual section content, not to the verdict.
//   - 003 DELIVERABLE: runs the Builder's new unit test as a real subprocess and
//     requires (a) exit 0 AND (b) a `--- PASS: TestVerify_WarnOrFailSentinel_StillRequiresSections`
//     line in the -v output. The second half is load-bearing: `go test -run` on a
//     name that matches nothing exits 0, so exit-code-alone would green on a
//     no-op that never wrote the test.
//   - 004 REGRESSION: the whole `internal/deliverable` package suite stays green,
//     so the new test cannot be paid for by breaking a neighbour.
//
// Predicates 001/002 are expected pre-existing GREEN (they assert an invariant
// scout read as already-true); 003/004 are the RED-on-arrival ones. Both kinds
// are load-bearing: the green pair is what actually guards the behaviour against
// future regression, which is the point of the todo.
```
