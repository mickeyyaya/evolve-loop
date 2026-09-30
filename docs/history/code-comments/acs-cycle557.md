# Comment history: `acs/cycle557`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle557/predicates_test.go:3` — above `package cycle557`

```text
// Package cycle557 materialises the cycle-557 acceptance criteria for this
// fleet lane's SOLE `## top_n` task per triage-report.md:
//
//	fuzz-parser-surfaces (slice 1) — add Go-native fuzz harnesses for
//	clihealth.ParseResetHint and bridge.ClassifyExhausted, seeded from the
//	existing golden fixtures (TestParseResetHintTable,
//	TestClassifyExhausted_RealManifests), asserting never-panic + sane
//	invariants (no negative durations, no far-future resets).
//
// Per the AC-Materialization Contract (R9.3 "predicates bind ONLY to triage-
// committed work"), this package predicates ONLY that item. Slices (2)-(5) of
// the inbox item (panestream pane-rule fuzz, quotastate parse fuzz,
// manifest+inbox loader fuzz, fleet.Partition rapid tests) are explicitly out
// of scope this cycle per triage-report.md's rationale and get NO predicate
// here.
//
// Scope correction (read-first, AGENTS.md rule 8): the inbox item and
// triage-report.md both label FuzzClassifyExhausted a "clihealth" fuzz target
// alongside FuzzParseResetHint. The real exported ClassifyExhausted lives in
// go/internal/bridge (usageclassify.go), not go/internal/clihealth — clihealth
// has no exhausted-classification function at all. Materializing a
// FuzzClassifyExhausted against a nonexistent clihealth API would invent an
// API (banned); the fuzz harness is authored against the real function in its
// actual package instead, which is what the acceptance criterion's plain-
// language function names ("FuzzParseResetHint + FuzzClassifyExhausted")
// substantively require.
//
// Predicate strategy (behavioral-via-subprocess, the cycle-549/553/555
// precedent — never a source grep): each predicate drives `go test -list`
// then `go test -run -fuzz -fuzztime` as subprocesses over the REAL compiled
// fuzz targets the TDD engineer authored this cycle:
//
//	go/internal/clihealth/resetparse_fuzz_test.go   FuzzParseResetHint
//	go/internal/bridge/usageclassify_fuzz_test.go   FuzzClassifyExhausted
//
// Before this cycle, `go test -list '^Fuzz'` on either package printed
// nothing (zero Fuzz funcs existed anywhere in the repo — the inbox's own
// "0 fuzz tests today" claim, independently confirmed by TDD-engineer grep).
// The -list assertion is RED for the right reason until the harness exists;
// this is a test-infrastructure-authoring task, so the harnesses ARE the
// deliverable and are pre-existing GREEN once authored (documented in
// test-report.md) — the underlying ParseResetHint/ClassifyExhausted
// implementations were already correct going in (both -fuzztime=8s local runs
// found zero crashers, "new interesting" corpus growth only). Builder's job
// is to extend fuzz time / wire a CI lane and fix anything a longer run
// surfaces, per the inbox's own "bounded fuzztime in CI" framing.
```

### `go/acs/cycle557/predicates_test.go:86` — above `func requireFuzzGreen(t *testing.T, pkg, funcName string, minSeeds int) {`

```text
// requireFuzzGreen drives a real bounded fuzz run (not just the seed corpus)
// over the compiled target and requires a clean exit: no crash, no failed
// seed, and at least minSeeds distinct seed subtests actually ran (closes the
// cycle-85 "no tests to run" degenerate trap — a fuzz func with zero seeds and
// a pattern that matches nothing would otherwise pass vacuously).
```
