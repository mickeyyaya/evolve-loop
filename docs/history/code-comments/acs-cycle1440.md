# Comment history: `acs/cycle1440`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1440/predicates_test.go:3` — above `package cycle1440`

```text
// Package cycle1440 materialises the cycle-1440 acceptance criteria for the
// three tasks triage committed to this lane:
//
//	carryover-pass-retirement            — PASS-closeout deletion path so a
//	                                       committed/fingerprint-matched carryover
//	                                       id retires instead of persisting forever.
//	deterministic-stage-refusal-router   — two-strikes-same-pathspec rule so a
//	                                       consecutive identical GIT_STAGE_FAILED
//	                                       refusal classifies deterministic instead
//	                                       of burning the whole retry budget
//	                                       (cycle-1365).
//	fingerprint-normalizer-path-variance — path/attempt-denominator normalization
//	                                       so one recurring defect fingerprints as
//	                                       ONE for the 3-strike breaker.
//
// Predicate strategy. Every predicate EXERCISES the production seam by running
// the named RED tests — never a source-grep of production text (the cycle-85
// degenerate-predicate ban): a predicate asserting "failure_digest.go now
// contains a path regex" would pass on a cosmetic edit that normalizes nothing.
// Each run is narrowed with `-run` (never a whole ./internal/core sweep — the
// flaky-predicate rule, cycles 1173/1175/1178) and demands an explicit
// `--- PASS: <name>` line per test, so a rename or a skip can never satisfy a
// predicate on exit code alone.
//
// Both NEGATIVE halves are load-bearing and are named here explicitly:
// AC2's `DifferentPathspecStaysTransient` refutes a rule that merely counts
// refusals, and AC3's `DistinctDefectsStayDistinct` refutes an over-broad
// normalizer that collapses two different defects into one fingerprint.
```
