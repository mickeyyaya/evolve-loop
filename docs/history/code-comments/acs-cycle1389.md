# Comment history: `acs/cycle1389`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1389/predicates_test.go:3` — above `package cycle1389`

```text
// Package cycle1389 materializes the acceptance criteria of this lane's sole
// fleet-scoped inbox item `schema-aligned-salvage-layer` (weight 0.9,
// scout-report.md Task 1 "instrument-bad-verdict-classification" + Task 2
// "wire-bad-verdict-baseline-docs").
//
// Scope, per the inbox item's own text ("FIRST deliverable = instrumentation
// only"): a log-only classifier that inspects the exact bytes a CodeBadVerdict
// Result was computed from (deliverable.Result.Content — the single-read seam,
// deliverable.go:44-64) and recognizes three SAP-cited (docs/research/
// deliverable-alignment-2026-08/README.md §3.3) recoverable-malformed shapes —
// a fenced-JSON-wrapped sentinel, a trailing-comma sentinel payload, and a
// bare/displaced (unwrapped) verdict object — vs. a genuinely absent verdict.
// NO extraction/coercion logic ships this cycle; Result.OK/Violations must be
// byte-identical before/after (predicate 005).
//
// New production surface this cycle's Builder implements (contract fixed by
// this test file, cycle-644 lesson honored — no package-qualified pin without
// a compiler-probed shape; every symbol below is a NEW exported name inside
// the ALREADY-enrolled go/internal/deliverable package, not a new package, so
// no go/.apicover-enforce edit is required — house rule 1 n/a):
//
//	// go/internal/deliverable/salvage_instrument.go
//	type SalvagePattern string
//	const (
//		SalvagePatternNone          SalvagePattern = ""
//		SalvagePatternFencedJSON    SalvagePattern = "fenced-json"
//		SalvagePatternTrailingComma SalvagePattern = "trailing-comma"
//		SalvagePatternDisplaced     SalvagePattern = "displaced-line"
//	)
//	type BadVerdictClassification struct {
//		Recoverable bool
//		Pattern     SalvagePattern
//		Reason      string
//	}
//	func ClassifyBadVerdict(content string) BadVerdictClassification
//
// Predicates 001-004 drive that function directly (a real behavioral call,
// never a source-grep — the cycle-85 degenerate-predicate ban). Predicate 005
// is the zero-mutation regression guard (scout's Acceptance Criteria Summary:
// "full regression: go test ./go/internal/deliverable/..."). Predicate 006 is
// the REACHABILITY / wiring proof (house rule 2): it drives the real
// production caller — deliverable.NewReviewer(...).Review, the host contract
// gate reviewer.go:102 wires behind core.DeliverableReviewer — not
// ClassifyBadVerdict directly, so a classifier with no production caller stays
// RED. It expects Task 2's Builder to append one JSONL record per bad_verdict
// to <ProjectRoot>/.evolve/bad-verdict-baseline.jsonl (reusing the existing
// log.SidecarWriter/EmitAbnormal pattern, log/events.go) from inside
// Reviewer.Review, strictly AFTER the res.OK branch (so the block/approve
// decision itself is untouched — instrumentation only). Predicates 007/008
// materialize Task 2's doc AC (README.md gains a real, non-fabricated §7
// baseline section + a wiring-proof excerpt) — 007 is the structural half
// (predicate-testable: the heading and a fenced code excerpt naming
// ClassifyBadVerdict exist); the "numbers are real, not fabricated" half is
// NOT mechanically verifiable and is dispositioned manual+checklist in
// test-report.md instead of faked here as a predicate.
```
