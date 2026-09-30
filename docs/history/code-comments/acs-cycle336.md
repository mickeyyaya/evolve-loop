# Comment history: `acs/cycle336`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle336/predicates_test.go:3` — above `package cycle336`

```text
// Package cycle336 materializes the cycle-336 acceptance criteria for the single
// behavior-preserving DRY task committed to triage `## top_n`:
//
//	aggregator-dedup-helpers — three behavior-preserving simplifications inside
//	    go/internal/aggregator/aggregator.go:
//	      1. remove the dead write-only `anyWarn` variable from writeVerdict
//	         (suppressed today by `_ = anyWarn`; the verdict depends only on
//	         anyFail/allPass, so removal is a pure no-op);
//	      2. extract `scanFirstCapture(path string, re *regexp.Regexp) string`
//	         and rewrite extractVerdict + extractScore to delegate;
//	      3. extract `appendWorkerSections(b *strings.Builder, heading string,
//	         workers []string, readFile func(string)([]byte,error)) error` and
//	         rewrite the four write* functions (writeConcat / writeVerdict /
//	         writePlanReview / writeCrossCLIVote) to call it.
//
// Floor binding (R9.3 / cycle-280 lesson). Triage `## top_n` for cycle 336 holds
// exactly this one task; it is gated here. The nine carryoverTodos are
// triage-DROPPED historical infra/phase-delivery failures and get ZERO
// predicates (deferred-floor starvation, cycle-280). No coverage floor is
// committed, so no floor predicate.
//
// Predicate design (cycle-85 lesson — every load-bearing gate EXERCISES the
// system under test; structural source assertions are config-check WAIVED):
//
//   - C336_001 is BEHAVIORAL: it imports the real aggregator package and drives
//     the exported Aggregate() entry point end-to-end across the merge modes the
//     refactor touches. The three changes are behavior-PRESERVING, so this gate
//     is pre-existing GREEN today and acts as the anti-regression lock — a
//     builder edit that changed any observable consequence (a verdict, an exit
//     code, a worker-heading level) would flip it RED. It pins precisely the
//     surfaces the refactor risks: the WARN/default verdict paths (the anyWarn
//     removal must not change them) and both worker-heading levels ("## Worker:"
//     and "### Worker:" — appendWorkerSections must reproduce both).
//
//   - C336_002 / C336_003 / C336_004 / C336_005 are the structural
//     dedup-completion gates (config-check waived — see inline waivers). They
//     assert the SOURCE-STRUCTURE outcome of the code-reduction goal: the dead
//     variable is gone, the two helpers exist AND are delegated to (call sites,
//     not just a definition), and the file is strictly shorter than the 466-line
//     baseline. These carry the RED today (anyWarn present ×4, helpers absent,
//     466 lines). The behavioral weight is carried by C336_001.
//
// AC map (1:1 with scout-report "Acceptance Criteria Summary"):
//
//	AC1 anyWarn absent from aggregator.go                         → C336_002
//	AC2 scanFirstCapture defined; extractVerdict/extractScore delegate → C336_003 (def + ≥2 call sites)
//	AC3 appendWorkerSections defined and called (≥4 occurrences)  → C336_004
//	AC4 aggregator test suite green (behavior preserved)          → C336_001 (drives Aggregate end-to-end)
//	AC5 aggregator.go strictly < 466 lines                        → C336_005
//	AC6 cycle-336 ACS predicates pass                             → this package (meta)
```
