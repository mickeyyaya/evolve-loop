# Comment history: `acs/cycle1269`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1269/predicates_test.go:3` — above `package cycle1269`

```text
// Package cycle1269 materialises the cycle-1269 acceptance criteria for the one
// fleet-scoped task triage committed to this lane:
//
//   - contextfill-telemetry-package → a new stdlib+cyclestate leaf package
//     go/internal/contextfill that DERIVES a context-window fill ratio from the
//     TokenUsage counts phasetiming/cyclestate already persist, plus a stubbed
//     model-tier context-window-size map.
//
// The two sibling proposals (wire-context-fill-stage, context-fill-hint-prompt-
// injection) are `## deferred` in triage-report.md, so this package authors ZERO
// predicates for them (R9.3: predicates bind only to triage-committed work).
//
// Predicate strategy — every load-bearing assertion CALLS the system under test
// and asserts on its return value (the cycle-85 degenerate-predicate ban). The
// package does not exist at RED, so this file does not compile: an ACS package
// that fails to compile is a HARD suite error, never a silent PASS, which is the
// correct RED signal for a greenfield package.
//
//   - 001 pins the occupancy SEMANTICS: all four TokenUsage fields count toward
//     the window, so a builder that sums only Input+Output fails.
//   - 002 is the negative predicate: a non-positive window must return
//     ErrInvalidWindow, never panic and never a silent NaN/Inf/0.
//   - 003 is the table-driven edge sweep: zero tokens, sub-threshold,
//     at-threshold, and over-window (ratio > 1.0, deliberately unclamped).
//   - 004 pins the hot-classification boundary against the exported threshold.
//   - 005 exercises the model-tier window map over modelcatalog's canonical
//     tier vocabulary, and pins unknown -> 0 (no invented default).
//   - 006 is the ADR-0069 new-package graduation check (config-check waiver).
//   - 007 runs the package's OWN unit suite as a subprocess — proof the builder
//     shipped the table-driven test file and that the package builds standalone.
//
// Import shape was compiler-probed at RED (per the reachability-probe
// obligation): a throwaway go/internal/probecf importing internal/cyclestate
// built clean (`go build ./internal/probecf` rc=0), so pinning
// `contextfill.FillRatio` over a `cyclestate.TokenUsage` argument does not
// commit the tree to an unbuildable import cycle. acs -> internal imports are
// precedented (acs/redteam imports internal/redteamcheck).
```

### `go/acs/cycle1269/predicates_test.go:199` — above `func TestC1269_006_NewPackageGraduatesIntoAPICover(t *testing.T) {`

```text
// TestC1269_006_NewPackageGraduatesIntoAPICover is the ADR-0069 new-package
// graduation obligation: a new go/internal/<pkg> must land BOTH halves in the
// same diff — the enrollment line in go/.apicover-enforce AND an
// apicover_named_test.go that names every export. Enrolled-but-unnamed fails
// the repo-wide gate; unenrolled aborts the build phase.
//
// acs-predicate: config-check — the enrollment half is inherently a
// config-presence assertion (a line in a gate manifest); the named-test half is
// EXERCISED for real by predicate 007, which runs the package suite.
```
