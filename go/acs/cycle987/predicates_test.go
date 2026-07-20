//go:build acs

// Package cycle987 materialises the cycle-987 acceptance criteria for the
// fleet-scoped inbox item `gate-wiring-binding-tests`, which triage split into
// two committed tasks:
//   - ship-gate-stale-attestation-binding-test
//   - qualitygate-reviewer-wiring-binding-test
//
// Both are TEST-ONLY tasks: the deliverable IS a set of DEFAULT-SUITE binding
// tests that catch a severed gate wire. The pre-existing coverage is the exact
// blind spot:
//
//   - internal/phases/ship/commitgate_test.go's stale/missing/valid attestation
//     tests (TestCommitGate_Manual*Attestation_*) sit behind //go:build
//     integration, so they DON'T run in `go test ./...` — a deleted or severed
//     verifyCommitGateAttestation wire is invisible to normal CI.
//   - internal/evalgate/gates_test.go tests qualityGate{}.check() DIRECTLY but
//     never through NewReviewer(...).Review(...), so deleting qualityGate{} from
//     reviewer.go:39's composition slice passes 100% of the existing suite and
//     silently re-admits tautological evals.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-563 precedent). Each
// predicate shells `go test -run '^Name$' -v` over the DEFAULT build suite (NO
// -tags integration) for the binding tests Builder must author, and requires
// that a `--- PASS: <name>` line appears. This genuinely exercises the
// system-under-test (the newly-authored binding tests run against the real
// enforcers):
//
//   - RED now: the named tests do not exist, so no PASS line → predicate fails.
//   - It ALSO fails (correctly) if Builder hides a binding test behind a build
//     tag the default suite skips — the very mistake this task exists to catch.
//   - GREEN only when every binding test exists AND passes in the default suite.
//
// A source-grep (acsassert.FileContains) predicate is deliberately AVOIDED: it
// would pass the moment the magic test name appears in a file, even behind
// //go:build integration — i.e. it could not distinguish the fix from the bug
// (the cycle-85 degenerate-predicate ban).
package cycle987

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	shipPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	evalgatePkg = "github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
)

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -v -count=1 pkg`
// in the DEFAULT build suite (no -tags) and requires EVERY name to have printed
// a `--- PASS: <name>` line. -count=1 defeats the test cache so the predicate
// always exercises current source. Asserting on the PASS line (not merely a
// zero exit code) is essential: `go test -run` on a pattern that matches no
// test exits 0 with "no tests to run", so a still-missing binding test would
// otherwise false-GREEN.
func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		// -1 means the subprocess never launched (toolchain/module resolution
		// failure) — a genuine harness error, not a test verdict.
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("default-suite binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag the default suite skips). exit=%d\n"+
				"combined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

// TestC987_001_ShipGateStaleAttestationBoundInDefaultSuite — AC1. The ship-gate
// commit-attestation triangle (stale → block, fresh → pass, missing → block)
// must be bound by tests that run in the DEFAULT suite, so a severed
// verifyCommitGateAttestation wire is caught by plain `go test ./...`. The
// existing TestCommitGate_Manual*Attestation_* tests sit behind //go:build
// integration and therefore do NOT satisfy this. Builder authors these three
// exact top-level tests, UNGUARDED (default suite), in internal/phases/ship,
// each driving verifyCommitGateAttestation against a real commit-gate
// attestation fixture.
func TestC987_001_ShipGateStaleAttestationBoundInDefaultSuite(t *testing.T) {
	assertDefaultSuiteTestsPass(t, shipPkg,
		"TestShipGate_StaleAttestationBlocked",   // stale tree_state_sha → CodeCommitGateStale
		"TestShipGate_FreshAttestationPasses",    // matching tree_state_sha → no error
		"TestShipGate_MissingAttestationBlocked", // absent attestation → CodeCommitGateMissing
	)
}

// TestC987_002_QualityGateWiredIntoReviewerBound — AC2. Deleting qualityGate{}
// from NewReviewer's gates slice (reviewer.go:39) currently passes the whole
// suite. These two DEFAULT-SUITE binding tests close that gap:
//
//   - TestQualityGate_WiredIntoReviewer asserts a gate named "predicate-quality"
//     is present in NewReviewer's composed gate list (mirrors the established
//     TestFloorBindingGate_WiredIntoReviewer pattern).
//   - TestNewReviewer_TautologyEvalBlocksAtEnforce drives the real
//     NewReviewer(config.StageEnforce).Review() END-TO-END against a tautology
//     (":") eval and asserts Approve==false — proving the wire, not just the
//     gate's own .check().
func TestC987_002_QualityGateWiredIntoReviewerBound(t *testing.T) {
	assertDefaultSuiteTestsPass(t, evalgatePkg,
		"TestQualityGate_WiredIntoReviewer",
		"TestNewReviewer_TautologyEvalBlocksAtEnforce",
	)
}
