//go:build acs

// Package cycle459 materialises the cycle-459 acceptance criteria for the
// single triage-committed task `triagecap-prose-counter-defect` (inbox
// 2026-07-02, post-mortem of cycles 448/449): the triage-cap gate defect
// chain F1–F5 in go/internal/triagecap (floors.go, reviewer.go, demotion.go).
//
// AC map (1:1, R9.3 floor-binding; predicates for the one ## top_n task only —
// the fleet-policy tasks were deferred by triage and get ZERO predicates):
//
//	AC1  F1 golden: the EXACT cycle-449 report counts 3, not 7   → C459_001 (golden, direct call)
//	AC2  F1 semantic: evidence citations never count as floors    → C459_002 (negative, direct call)
//	AC3  F1 anti-weakening: true multi-floor commitments still
//	     count fully (cycle-448 golden = 4; cycle-283 stays 12)   → C459_003 (edge, direct call)
//	AC4  F2: reject reason states the declaration escape
//	     (triage-decision.json committed_floors[])                → C459_004 (semantic, named unit test)
//	AC5  F5: reject reason lists the counted packages             → C459_005 (semantic, named unit test)
//	AC6  F4: reset-sealed cycle is a transparent gap — the
//	     448/449 pair demotes cycle 451                           → C459_006 (positive, named unit test)
//	AC7  F4: relief stays one cycle; stale pairs outside the
//	     window keep enforcing                                    → C459_007 (negative, named unit tests)
//	AC8  F3: floor-bearing report without committed_floors
//	     declaration WARNs (and only then)                        → C459_008 (negative+edge, named unit test)
//	AC9  F3: the triage prompt instructs emitting the companion   → C459_009 (config-check, pre-existing GREEN pin)
//	AC10 regression: triagecap vet + -race suite green            → C459_010 (regression)
//
// 1:1 enforcement: 10 predicates + 0 manual + 0 removed = 10 ACs, each AC
// exactly one disposition, none double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"): C459_001/002
// fail directly on the unfixed counter (7 and 4 instead of 3 and 1).
// C459_004..008 shell to this cycle's named RED unit tests in
// internal/triagecap (gate_defect_chain_test.go) guarded by requireTestsRan,
// so an unwritten or renamed test can never green them. C459_010 fails while
// any triagecap unit test is red. C459_003 and C459_009 are pre-existing
// GREEN pins: C459_003 is the anti-weakening bound that keeps the gate's
// purpose intact (real overpacking must still count), and C459_009 pins the
// prompt instruction the F3 producer check depends on.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C459_002 (evidence prose must NOT count), C459_007 (stale
//	            pair must NOT demote; relief must NOT extend past one cycle),
//	            C459_008's silent subtests (warning must NOT fire with a
//	            declaration or without floors)
//	Edge/OOD:   C459_003 (multi-target item = boundary of "scoped counting"),
//	            C459_006 (missing middle cycle = the reset hole)
//	Semantic:   C459_004/005 (actionable corrective is distinct behavior from
//	            rejecting), C459_001 vs C459_003 (3-not-7 AND 4-stays-4 are
//	            only jointly satisfiable by target-scoped counting)
package cycle459

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const triagecapPkg = "github.com/mickeyyaya/evolve-loop/go/internal/triagecap"

// goldenVocab mirrors the production package vocabulary relevant to the
// cycle-448/449 goldens: the true floor targets plus the phantom sources the
// defective counter attributed (scout, sysexec) and distractors. Must match
// gapGoldenPkgs in internal/triagecap/gate_defect_chain_test.go.
var goldenVocab = []string{
	"core", "bridge", "audit",
	"scout", "sysexec",
	"config", "router", "llmroute", "recovery", "evidence", "paths",
}

func golden(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "triagecap", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	return string(data)
}

func runGoTest(t *testing.T, runFilter string, race bool, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-v"}
	if race {
		args = append(args, "-race")
	}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with
// no matching test exits 0 with "no tests to run", which would green a
// predicate on unwritten (or renamed) work.
func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

// TestC459_001_GoldenCycle449CountsThreeFloors (AC1, golden, behavioral —
// invokes the counter directly): the EXACT preserved cycle-449 triage report
// committed three coverage floors (core 85.0 / bridge 94.5 / audit 96.0, one
// per top_n item); the gate counted 7 because evidence citations named other
// packages with percentages. RED today: CountCommittedFloors returns 7.
func TestC459_001_GoldenCycle449CountsThreeFloors(t *testing.T) {
	got := triagecap.CountCommittedFloors(golden(t, "triage-cycle449-golden.md"), goldenVocab)
	if got != 3 {
		t.Errorf("cycle-449 golden committed floors = %d, want 3 — evidence citations must not count as floor commitments (F1)", got)
	}
}

// TestC459_002_EvidenceCitationsDoNotCount (AC2, negative, behavioral): a
// single-target floor item whose contract-mandated evidence names other
// packages with percentages ("bridge 93.5%; ... in audit", "scout fresh
// cover-func") counts exactly 1. RED today: counts 4.
func TestC459_002_EvidenceCitationsDoNotCount(t *testing.T) {
	artifact := "## top_n (commit to THIS cycle)\n" +
		"- salvage-core-coverage: raise core coverage floor to ≥85.0% — priority=H, evidence=scout fresh cover-func (bridge 93.5%; matchExhausted 66.7% in audit), source=scout\n"
	got := triagecap.CountCommittedFloors(artifact, goldenVocab)
	if got != 1 {
		t.Errorf("evidence-citation item floors = %d, want 1 (core is the only floor TARGET)", got)
	}
}

// TestC459_003_TrueMultiFloorCommitmentStillCountsFour (AC3, edge,
// anti-weakening pin — pre-existing GREEN): cycle 448 genuinely committed
// four floor targets ("coverage floors core ≥85.0%, audit ≥96.0%, bridge
// ≥94.5%" + "core total coverage ... ≥86.0%"). The F1 fix must not collapse
// true multi-package commitments — the gate still guards real overpacking
// (cycles 280/282/283). Must count 4 before AND after the fix.
func TestC459_003_TrueMultiFloorCommitmentStillCountsFour(t *testing.T) {
	got := triagecap.CountCommittedFloors(golden(t, "triage-cycle448-golden.md"), goldenVocab)
	if got != 4 {
		t.Errorf("cycle-448 golden committed floors = %d, want 4 — target-scoped counting must keep true multi-floor commitments fully counted", got)
	}
}

// TestC459_004_RejectReasonStatesDeclarationEscape (AC4/F2, semantic): the
// named RED unit test pinning that a reject reason states the
// declaration-primary escape (triage-decision.json committed_floors[]) must
// exist and pass. RED today: reviewer.go's reason names neither.
func TestC459_004_RejectReasonStatesDeclarationEscape(t *testing.T) {
	out, code := runGoTest(t, "TestCapReviewer_RejectReasonStatesDeclarationEscape", true, triagecapPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("reject-reason declaration-escape contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC459_005_RejectReasonListsCountedPackages (AC5/F5, semantic): the named
// RED unit test pinning that the reject reason lists WHICH packages the
// counter attributed must exist and pass. RED today: the reason shows only
// the count.
func TestC459_005_RejectReasonListsCountedPackages(t *testing.T) {
	out, code := runGoTest(t, "TestCapReviewer_RejectReasonListsCountedPackages", true, triagecapPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("reject-reason counted-package listing contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC459_006_ResetSealedGapStillDemotes (AC6/F4, positive): the incident
// replay — same-template rejections recorded at 448 and 449, cycle 450
// reset-sealed without a record, review at 451 must demote to shadow and
// auto-file the defect. RED today: ShouldDemote demands records at
// currentCycle-1 AND -2.
func TestC459_006_ResetSealedGapStillDemotes(t *testing.T) {
	out, code := runGoTest(t, "TestCapReviewer_ResetSealedGapStillDemotes", true, triagecapPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("reset-sealed-gap demotion contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC459_007_DemotionReliefStaysBounded (AC7/F4, negative): relief is one
// cycle (452 enforces again after 451 consumed the pair) and a stale pair
// outside the demotion window grants nothing — the anti-overcorrection
// bounds on gap transparency. Both named tests must run and pass.
func TestC459_007_DemotionReliefStaysBounded(t *testing.T) {
	out, code := runGoTest(t, "TestCapReviewer_ReliefIsOneCycleThenEnforces|TestCapReviewer_StaleRejectionPairOutsideWindowEnforces", true, triagecapPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("bounded-relief contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC459_008_MissingDeclarationWarns (AC8/F3, negative+edge): the producer
// check — a floor-bearing report without a committed_floors declaration
// WARNs (naming triage-decision.json + committed_floors); reports WITH the
// declaration or WITHOUT floors stay silent. RED today: no such check
// exists anywhere.
func TestC459_008_MissingDeclarationWarns(t *testing.T) {
	out, code := runGoTest(t, "TestCapReviewer_FloorBearingReportWithoutDeclarationWarns", true, triagecapPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("missing-declaration warning contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC459_009_TriagePromptInstructsCompanionDeclaration (AC9/F3,
// pre-existing GREEN pin): the F3 producer check is only satisfiable if the
// triage prompt keeps instructing agents to emit the companion — pin the
// instruction so a prompt regression cannot silently re-open the
// no-producer hole.
// acs-predicate: config-check — the prompt text IS the config surface under
// test; the behavioral half of F3 is C459_008.
func TestC459_009_TriagePromptInstructsCompanionDeclaration(t *testing.T) {
	prompt := filepath.Join(acsassert.RepoRoot(t), "agents", "evolve-triage.md")
	acsassert.FileContains(t, prompt, "triage-decision.json")
	acsassert.FileContains(t, prompt, "committed_floors")
}

// TestC459_010_TriagecapRegressionVetAndRace (AC10, regression): the touched
// package must be vet-clean and fully -race green — including every
// pre-existing replay pin (cycle-283 stays 12, cycle-301 stays 2, adjacent-
// pair demotion still fires) and this cycle's contract tests. RED today
// while the F1–F5 unit tests are red; GREEN only when the whole package is.
func TestC459_010_TriagecapRegressionVetAndRace(t *testing.T) {
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "vet", triagecapPkg)
	if code != 0 {
		t.Errorf("go vet %s exit=%d\n%s%s", triagecapPkg, code, stdout, stderr)
	}
	out, code := runGoTest(t, "", true, triagecapPkg)
	if code != 0 {
		t.Errorf("triagecap -race suite exit=%d\n%s", code, out)
	}
}
