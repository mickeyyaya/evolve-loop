# Comment history: `acs/cycle1313`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1313/predicates_test.go:3` — above `package cycle1313`

```text
// Package cycle1313 materializes the cycle-1313 acceptance criteria for this
// fleet lane's sole committed inbox item,
// triage-commit-time-protected-surface-admission (per R9.3 no predicates bind
// to any other lane's items).
//
// AC map (1:1, from scout-report.md Selected Tasks verifiableBy +
// Acceptance Criteria Summary):
//
//	AC1 Classify FAILs any triage artifact whose top_n card `files=` (both
//	    the brace-delimited and bare encodings actually observed in real
//	    triage-report.md output) intersects guards.IsProtectedSurface, citing
//	    the offending id and path in the diagnostic, and correctly
//	    disambiguates the offending card among several.
//	    → C1313_001 runs the three Reject* unit tests as a subprocess and
//	      requires each named "--- PASS:" marker (exit-0 alone could hide a
//	      renamed/skipped test).
//	AC2 A non-protected top_n card is unaffected (byte-identical PASS
//	    behavior preserved), including a card with no files= segment at all.
//	    → C1313_002 same subprocess pattern over the two regression/edge
//	      unit tests.
//	AC3 Existing console_routed_prompt_test.go / triage_test.go suites remain
//	    green (no behavior change to the prompt-composition route).
//	    → C1313_003 runs the full triage package suite (excluding the acs
//	      tag) and asserts a clean exit.
//	AC4 A permanent regression entry
//	    (.evolve/evals/triage-commit-time-protected-surface-admission.md)
//	    exists and passes the SSOT quality checker with a non-empty,
//	    real-command evidence set.
//	    → C1313_004 runs internal/evalqualitycheck — the exact code behind
//	      `evolve eval quality-check` — and requires Overall==PASS over ≥2
//	      commands, closing the vacuous-empty-eval hole.
//
// Adversarial axes: negative (Reject* tests assert FAIL on a protected
// path, both files= encodings), edge (no files= segment at all must not
// false-positive; multi-card batch must not misattribute the offender),
// semantic (PASS-unaffected vs FAIL-on-protected are distinct behaviors, not
// one behavior restated). No source-grep predicates (cycle-85 rule): every
// predicate here executes the system under test as a subprocess or runs the
// SSOT checker — none asserts on source-file text alone.
```
