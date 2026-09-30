# Comment history: `acs/cycle1522`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1522/predicates_test.go:3` — above `package cycle1522`

```text
// Package cycle1522 materializes the cycle-1522 acceptance criteria for the
// sole committed task of this fleet lane, `cap-audit-report-length`
// (scout-report.md ## Selected Tasks Task 1; triage-report.md ## top_n).
// fleet_scope pins this lane to that one todo-id, so per R9.3 no predicate here
// binds to any other lane's item, nor to the deferred
// `build-report-length-cap` sibling.
//
// Task: audit-report.md has no upper bound on total size. The ## Issues table
// grows one row per finding, ship re-reads and SHA-binds the whole file
// (go/internal/phases/ship/audit.go:83), and the next cycle's handoff carries
// the prior audit — so an unbounded report compounds token cost on every read.
// The fix follows defect_ledger.go:56-61's established idiom: bound it, RECORD
// the overflow, never silently drop or truncate.
//
// AC map (1:1 with .evolve/evals/cap-audit-report-length.md):
//
//	AC1 "auditReportMaxBytes const exists with a sane budget"
//	    → C1522_001 (the frozen unit contract's cap_value_sane sub-test; the
//	      const is also a compile dependency of that file, so an absent const
//	      is a build failure, not a silent skip).
//	AC2 "over-cap emits exactly one warning-severity diagnostic naming size
//	     and cap"
//	    → C1522_001 (over_cap_warns_once).
//	AC3 "under-cap is silent; the exact boundary is explicit (== cap silent,
//	     > cap warns)"
//	    → C1522_001 (under_cap_silent, exact_boundary_silent) — the negative /
//	      edge axis: an unconditional warner fails these.
//	AC4 "diagnostic-only: the verdict never flips on size, the on-disk
//	     artifact is never mutated (ship SHA-binds it)"
//	    → C1522_001 (over_cap_does_not_flip_verdict,
//	      over_cap_does_not_mutate_artifact) + C1522_003 (regression axis: the
//	      existing verdict-conflict semantics stay green alongside the cap).
//	AC5 "the documented cap matches the code cap — no prompt/gate drift"
//	    → C1522_002 (parses the value out of the Go const and requires the
//	      auditor reference doc to carry the SAME number).
//
// Predicate-quality posture (cycle-85 rule): C1522_001/003 EXECUTE the system
// under test — each shells the audit package's own tests, which call the real
// production seam hooks.Classify (the one runner.BaseRunner.Run invokes at
// runner.go:1117) — and they count NAMED "--- PASS:" markers rather than
// trusting exit 0, so a renamed, deleted, or skipped sub-test cannot green
// them. C1522_002 is a declared config/doc-sync check (waiver below): it is
// not a magic-string grep, because the expected string is DERIVED from the
// code's own constant, so it fails on drift in either direction.
//
// Reliability posture: every subprocess names ONE package and narrows with
// -run (the whole audit package is a 22s suite under no load — excluded
// deliberately); no wall-clock bounds, no literal PIDs, no bare-git, no
// unreaped load generators.
```
