//go:build acs

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

func TestC459_001_GoldenCycle449CountsThreeFloors(t *testing.T) {
	got := triagecap.CountCommittedFloors(golden(t, "triage-cycle449-golden.md"), goldenVocab)
	if got != 3 {
		t.Errorf("cycle-449 golden committed floors = %d, want 3 — evidence citations must not count as floor commitments (F1)", got)
	}
}

func TestC459_002_EvidenceCitationsDoNotCount(t *testing.T) {
	artifact := "## top_n (commit to THIS cycle)\n" +
		"- salvage-core-coverage: raise core coverage floor to ≥85.0% — priority=H, evidence=scout fresh cover-func (bridge 93.5%; matchExhausted 66.7% in audit), source=scout\n"
	got := triagecap.CountCommittedFloors(artifact, goldenVocab)
	if got != 1 {
		t.Errorf("evidence-citation item floors = %d, want 1 (core is the only floor TARGET)", got)
	}
}

func TestC459_003_TrueMultiFloorCommitmentStillCountsFour(t *testing.T) {
	got := triagecap.CountCommittedFloors(golden(t, "triage-cycle448-golden.md"), goldenVocab)
	if got != 4 {
		t.Errorf("cycle-448 golden committed floors = %d, want 4 — target-scoped counting must keep true multi-floor commitments fully counted", got)
	}
}

func TestC459_004_RejectReasonStatesDeclarationEscape(t *testing.T) {
	out, code := runGoTest(t, "TestCapReviewer_RejectReasonStatesDeclarationEscape", true, triagecapPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("reject-reason declaration-escape contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC459_005_RejectReasonListsCountedPackages(t *testing.T) {
	out, code := runGoTest(t, "TestCapReviewer_RejectReasonListsCountedPackages", true, triagecapPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("reject-reason counted-package listing contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC459_006_ResetSealedGapStillDemotes(t *testing.T) {
	out, code := runGoTest(t, "TestCapReviewer_ResetSealedGapStillDemotes", true, triagecapPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("reset-sealed-gap demotion contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC459_007_DemotionReliefStaysBounded(t *testing.T) {
	out, code := runGoTest(t, "TestCapReviewer_ReliefIsOneCycleThenEnforces|TestCapReviewer_StaleRejectionPairOutsideWindowEnforces", true, triagecapPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("bounded-relief contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC459_008_MissingDeclarationWarns(t *testing.T) {
	out, code := runGoTest(t, "TestCapReviewer_FloorBearingReportWithoutDeclarationWarns", true, triagecapPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("missing-declaration warning contract is red (exit=%d)\n%s", code, out)
	}
}

// acs-predicate: config-check — the prompt text IS the config surface under
func TestC459_009_TriagePromptInstructsCompanionDeclaration(t *testing.T) {
	prompt := filepath.Join(acsassert.RepoRoot(t), "agents", "evolve-triage.md")
	acsassert.FileContains(t, prompt, "triage-decision.json")
	acsassert.FileContains(t, prompt, "committed_floors")
}

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
