//go:build acs

// Package cycle1663 materializes the acceptance criteria of the ONE inbox item
// this fleet lane committed (lane-scope.json todo_ids ∩ triage-report.md
// ## top_n) — and nothing else (R9.3):
//
//	lost-ship-dossier-evidence  (high, weight 0.82, feature)
//
// The lane's second scoped id, dossier-producer-params-struct, was triage-
// DROPPED as already shipped (cycleDossierParams is live at
// go/internal/core/dossier_producer.go, inbox record consumed via ship
// 2026-09-13) and gets ZERO predicates here; its own contract still runs as
// the landed core tests 003 names.
//
// The defect. finalizeCycle stamps the landing-lost SystemFailureSignal onto
// CycleResult and downgrades the verdict to WARN (lost_landing_floor.go,
// PR #482) — but writeCycleDossier receives only the outcome string, so the
// committed knowledge-base/cycles/cycle-N.{json,md} carries a WARN
// indistinguishable from any other WARN and the operator cannot see WHY.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 landing-lost dossier carries the signal's category AND evidence,
//	    through the REAL writeCycleDossier path (both production callers)  → 001
//	AC2 the landed sibling with the same transient ship error carries none;
//	    an ordinary PASS dossier stays byte-clean                         → 002
//	AC3 acs/cycle1544 predicates 004-005 restored and green               → 004
//	house-rule floor: landed dossier/floor tests not weakened; the schema
//	    drift guard (Go struct ⇄ schemas/cycle-dossier.schema.json) stays
//	    green once the new field exists                                   → 003
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 002 — the
// cycle-1536 sibling recorded the SAME GIT_FLEET_REBASE_NEEDED and LANDED;
// a sink that keys on "ship-error present" instead of the floor's signal
// passes 001 and fails this. EDGE/OOD: 002's ordinary-PASS golden (nonexistent
// workspace ⇒ never shipped ⇒ nothing to lose; the bytes must equal the
// pre-change producer's — no `null`, no empty section), 001's abnormal-exit
// row (a signal stamped before an abort must survive the epilogue's own
// dossier) and its no-signal twin (nothing fabricated on the FAIL path).
// SEMANTIC: evidence durability (001), false-alarm immunity (002), schema +
// anti-weakening floor (003), historical-suite restoration (004) — four
// distinct behaviours.
//
// No grep-only predicates (cycle-85 ban): every seam here (completeCycle,
// abnormalEpilogue, writeCycleDossier, the dossier build) is unexported
// inside internal/core / internal/dossier, so each predicate drives it via
// the sanctioned behavioural-via-subprocess shape — a `-run`-narrowed,
// `-count=1`, single-named-package `go test -v` that must print
// `--- PASS: <name>` for every Builder-frozen binding test. Asserting on the
// PASS LINE, never exit 0, is load-bearing: a pattern matching NO test exits
// 0 with "no tests to run", so a still-missing binding would false-GREEN.
//
// Flaky-shape contract: ONE named package per invocation, never `/...`, never
// a whole-package run of the 40s+ core suite; no wall-clock bounds, no
// literal PIDs, no bare git.
//
// Reachability probe (cycle-644 rule): this package imports only
// pkg/acsassert and the standard library — a leaf. The frozen core test
// references `dossier.ParseJSON`/`dossier.VerdictWarn` from package core,
// which ALREADY imports internal/dossier (dossier_producer.go) — no new edge;
// and internal/dossier already imports internal/cyclestate (the signal type's
// home), so a dossier field typed on the signal is buildable without touching
// core→dossier→core. Compiler-probed at RED time: `go vet ./internal/core/`
// green with the frozen test in place.
package cycle1663

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	dossierPkg = "github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	// cycle1544Pkg is the historical ACS package whose 004-005 this cycle
	// restores (AC3). It is acs-tagged, so 004 shells it with -tags acs.
	cycle1544Pkg = "github.com/mickeyyaya/evolve-loop/go/acs/cycle1544"
)

// assertSuiteTestsPass shells `go test [-tags T] -run '^(names)$' -count=1 -v
// pkg` against ONE named package and requires EVERY name to print a
// `--- PASS: <name>` line. -count=1 defeats the test cache so a stale cached
// result from an earlier phase can never stand in for a live run. The two
// argv shapes are spelled out as literal calls (not assembled) so the
// flaky-shape lint can see the -run narrowing one hop into this helper.
func assertSuiteTestsPass(t *testing.T, tags, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	var (
		stdout, stderr string
		code           int
		err            error
	)
	if tags == "" {
		stdout, stderr, code, err = acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	} else {
		stdout, stderr, code, err = acsassert.SubprocessOutput("go", "test", "-tags", tags, "-run", pattern, "-count=1", "-v", pkg)
	}
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

// TestC1663_001_LostLandingEvidenceReachesTheCommittedDossier — AC1. The
// binding tests drive cycle-1535's REAL ship artifacts (ship-error
// GIT_FLEET_REBASE_NEEDED, no ship-binding) through cycleRun.completeCycle —
// finalizeCycle fires the floor, writeCycleDossier commits the record — and
// assert the committed JSON carries a `system_failure` object whose
// `category` is "landing-lost" and whose `evidence` equals the floor's
// evidence VERBATIM, with the Markdown half naming both; and that the
// second production caller, cycleRun.abnormalEpilogue, threads a signal
// already stamped on the result the same way (and fabricates nothing when
// there is none). The verdict routing (WARN) is asserted unchanged: this is
// evidence added, not a re-classification.
func TestC1663_001_LostLandingEvidenceReachesTheCommittedDossier(t *testing.T) {
	assertSuiteTestsPass(t, "", corePkg,
		"TestDossierSystemFailure_LostLandingReachesTheCommittedDossier",
		"TestDossierSystemFailure_AbnormalEpilogueThreadsTheSignal",
	)
}

// TestC1663_002_LandedSiblingAndOrdinaryPassCarryNoEvidence — AC2. The same
// real closeout over cycle-1536's artifacts (same transient error + a
// binding for commit adcbddb2) must commit a PASS dossier with NO
// system-failure object and no leaked "landing-lost"/error-code text — and
// the binding's commit_sha, proving the producer read THAT workspace. And
// the ordinary-PASS golden (testdata/dossierparams/cycle-4243.golden.*,
// captured from the producer at b9c0df76 BEFORE any signal field existed)
// must still match byte for byte: a nil signal adds nothing to the record.
func TestC1663_002_LandedSiblingAndOrdinaryPassCarryNoEvidence(t *testing.T) {
	assertSuiteTestsPass(t, "", corePkg,
		"TestDossierSystemFailure_LandedSiblingCarriesNone",
		"TestDossierSystemFailure_OrdinaryPassStaysByteClean",
	)
}

// TestC1663_003_SchemaFloorAndLandedRegressionsNotWeakened is the
// ANTI-WEAKENING floor plus the schema half of the deliverable. The cheapest
// way to green 001 is to loosen what already pins this surface: relax the
// floor's negatives, drop the FAIL-golden byte pin (a nil signal must add
// nothing on the FAIL path either), or add a Go field without extending
// schemas/cycle-dossier.schema.json — which TestSchema_NoDrift rejects
// bidirectionally (every wire field in the schema, every definition
// registered in schemaObjects). Naming them makes each of those paths RED.
// Two invocations, each ONE named package with -run narrowing — never a
// sweep of the 40s+ core suite.
func TestC1663_003_SchemaFloorAndLandedRegressionsNotWeakened(t *testing.T) {
	assertSuiteTestsPass(t, "", dossierPkg,
		"TestSchema_NoDrift",
	)
	assertSuiteTestsPass(t, "", corePkg,
		"TestDetectLostLanding_RealCycle1535IsNotAPass",
		"TestDetectLostLanding_RealCycle1536Landed",
		"TestDetectLostLanding_CycleThatNeverShippedIsNotFlagged",
		"TestFinalizeCycle_LostLandingDowngradesTheVerdictAndRecordsTheSignal",
		"TestFinalizeCycle_LandedCycleIsUnchanged",
		"TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes",
		"TestWriteCycleDossier_ParamsAreKeyedAndOptional",
		"TestWriteCycleDossier_WritesValidArtifact",
		"TestWriteCycleDossier_FailOutcomeRecordsDefect",
		"TestWriteCycleDossier_LeavesCleanTree",
		"TestDossierFailure_FailCarriesIdentity",
		"TestDossierFailure_PassKeepsShape",
		"TestAbnormalEpilogue_WritesDossierDigestAndCoherentState",
		"TestAbnormalEpilogue_NoopAfterNormalCloseout",
	)
}

// TestC1663_004_Cycle1544PredicatesRestoredAndGreen — AC3, verbatim from the
// inbox record: "acs/cycle1544 predicates 004-005 restored and green". The
// 2026-08-23 console salvage excised 001-005 from that suite; 004-005 (the
// dossier-evidence half) are restored this cycle and must print their own
// PASS lines under the acs tag. RED today for the right reason: before this
// cycle the pattern matched no test ("no tests to run", exit 0) — the exact
// false-GREEN shape asserting on the PASS line exists to catch.
func TestC1663_004_Cycle1544PredicatesRestoredAndGreen(t *testing.T) {
	assertSuiteTestsPass(t, "acs", cycle1544Pkg,
		"TestC1544_004_LostLandingEvidenceReachesTheCommittedDossier",
		"TestC1544_005_LandedSiblingAndOrdinaryPassCarryNoEvidence",
	)
}
