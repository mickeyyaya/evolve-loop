//go:build acs

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
package cycle555

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const ciparityPkg = "github.com/mickeyyaya/evolve-loop/go/internal/ciparity"

// runCiparityTest drives `go test -run <filter> ./internal/ciparity` over the
// REAL compiled SSOT + the in-package behavioral tests, returning combined
// output + exit code. A subprocess (not a source read) is the load-bearing
// assertion, per the predicate-quality rule: it exercises the actual
// CoverageTags / CoverageTestArgs code.
func runCiparityTest(t *testing.T, runFilter string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, ciparityPkg)
	return stdout + "\n" + stderr, code
}

// requireRanAndGreen fails the predicate unless the -run filter matched at least
// `min` tests (guards the cycle-85 "no tests to run" degenerate pass) and the
// package exited 0 with no `--- FAIL`.
func requireRanAndGreen(t *testing.T, out string, code, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — the SSOT's behavioral tests are unwritten or renamed:\n%s", out)
		return
	}
	ran := strings.Count(out, "--- PASS") + strings.Count(out, "--- FAIL")
	if ran < min {
		t.Errorf("only %d test(s) ran, need >= %d (or internal/ciparity failed to build):\n%s", ran, min, out)
		return
	}
	if code != 0 || strings.Contains(out, "--- FAIL") {
		t.Errorf("internal/ciparity SSOT tests failed (exit=%d):\n%s", code, out)
	}
}

// TestC555_001_CoverageTagsIncludeACS (AC1): the SSOT tag set names BOTH the
// integration and acs tiers, so a coverage run through it measures the
// //go:build acs in-package tests (core/acssuite/phases/audit/evalgate) instead
// of under-counting them — and STILL keeps integration (no no-op-that-regresses).
func TestC555_001_CoverageTagsIncludeACS(t *testing.T) {
	out, code := runCiparityTest(t, "^TestCoverageTags_IncludesACSTag$")
	requireRanAndGreen(t, out, code, 1)
}

// TestC555_002_CoverageArgsThreadBothTags (AC2): the SSOT command builder
// threads the full tag set into one `-tags` value AND preserves the scoped
// package list verbatim & in order as trailing args — so a scoped run measures
// exactly the intended packages under both tiers.
func TestC555_002_CoverageArgsThreadBothTags(t *testing.T) {
	out, code := runCiparityTest(t, "^TestCoverageTestArgs_ThreadsBothTagsAndPreservesPkgOrder$")
	requireRanAndGreen(t, out, code, 1)
}

// TestC555_003_TagGatedFixtureMeasuredThroughSSOT (AC3, the load-bearing
// regression): a hermetic fixture module whose function is covered ONLY by an
// //go:build acs test is measured as COVERED when its coverage profile is built
// through ciparity.CoverageTestArgs — proving the acs tag actually flows through
// the SSOT to a real `go test -coverprofile`. Non-gameable: only really
// threading the acs tag makes the acs-gated code count; a magic-string edit can
// not satisfy it.
func TestC555_003_TagGatedFixtureMeasuredThroughSSOT(t *testing.T) {
	out, code := runCiparityTest(t, "^TestCoverageTestArgs_TagGatedFixtureMeasuresTaggedCoverage$")
	requireRanAndGreen(t, out, code, 1)
}
