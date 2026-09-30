# Comment history: `acs/cycle555`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle555/predicates_test.go:3` — above `package cycle555`

```text
// Package cycle555 materialises the cycle-555 acceptance criteria for this
// fleet lane's SOLE `## top_n` task per triage-report.md:
//
//	coverage-gate-tag-parity — close the residual gap left by the already-landed
//	ciparity.CoverageTestArgs/CoverageTags SSOT (commit 78d73f08). The SSOT tag
//	set is "integration" only, but the repo puts real coverage behind BOTH
//	//go:build integration AND //go:build acs tags — four internal/** packages
//	(core, acssuite, phases/audit, evalgate) carry in-package `//go:build acs`
//	tests. A coverage run through the SSOT with the acs tag MISSING under-counts
//	them (R1: 47.0% plain vs 90.6% tagged for internal/phases/ship). Builder
//	adds `acs` to CoverageTags (→ "integration acs") + no other plain -cover
//	call site gates a tagged package.
//
// Per the AC-Materialization Contract (R9.3 "predicates bind ONLY to triage-
// committed work"), this package predicates ONLY that item. The cycle-555
// scout-report.md proposed THREE OTHER tasks (workspace-hygiene-s1/-s3,
// memo-phase-routing-broken); triage-report.md explicitly scoped them to OTHER
// fleet lanes ("not re-bucketed here"), so they get NO predicate here.
//
// Predicate strategy — behavioral-via-subprocess (the cycle-549/553 precedent,
// never a source grep): the SSOT under test (ciparity.CoverageTags,
// CoverageTestArgs) is exercised by the in-package behavioral tests the TDD
// engineer authored this cycle in go/internal/ciparity/coverage_tagparity_test.go
// — including a hermetic tag-gated fixture module whose acs-only function is
// measured through a REAL `go test -coverprofile` built from CoverageTestArgs.
// Each predicate drives `go test -run <TestName> ./internal/ciparity` as a
// subprocess over that real code and asserts (a) the targeted test actually ran
// (closes the cycle-85 "no tests to run" degenerate trap) and (b) it passed
// (exit 0, no `--- FAIL`). Before the Builder adds the acs tag, CoverageTags is
// "integration" only, so every gated test is RED for the right reason.
//
// In-package behavioral tests these predicates gate on:
//
//	internal/ciparity/coverage_tagparity_test.go
//	  TestCoverageTags_IncludesACSTag
//	  TestCoverageTestArgs_ThreadsBothTagsAndPreservesPkgOrder
//	  TestCoverageTestArgs_TagGatedFixtureMeasuresTaggedCoverage
//
// The Builder's role: change ciparity.CoverageTags from "integration" to
// "integration acs" (and record the audit-inventory proof in the cycle report).
// Builder must NOT modify the test files.
```

### `go/acs/cycle555/predicates_test.go:67` — above `func requireRanAndGreen(t *testing.T, out string, code, min int) {`

```text
// requireRanAndGreen fails the predicate unless the -run filter matched at least
// `min` tests (guards the cycle-85 "no tests to run" degenerate pass) and the
// package exited 0 with no `--- FAIL`.
```
