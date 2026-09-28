package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

// initDossierRepo makes root a git working tree so writeCycleDossier's commit
// (dossier.Write(..., true)) has a repo to add+commit into. Production always
// runs against the git main tree; the tests mirror that precondition.
func initDossierRepo(t *testing.T) string {
	t.Helper()
	return gittest.Fixture(t).Dir
}

func TestDossierVerdict_MapsCycleOutcomes(t *testing.T) {
	cases := map[string]string{
		VerdictPASS:                      dossier.VerdictPass,
		CycleOutcomeShippedViaBuild:      dossier.VerdictPass,
		VerdictFAIL:                      dossier.VerdictFail,
		VerdictWARN:                      dossier.VerdictWarn,
		VerdictSKIPPED:                   dossier.VerdictWarn,
		CycleOutcomeSkippedAuditAdvisory: dossier.VerdictWarn,
		CycleOutcomeSkippedUnknown:       dossier.VerdictWarn,
		"":                               dossier.VerdictWarn,
		"BOGUS":                          dossier.VerdictWarn,
	}
	for outcome, want := range cases {
		if got := dossierVerdict(outcome); got != want {
			t.Errorf("dossierVerdict(%q) = %q, want %q", outcome, got, want)
		}
	}
}

func TestWriteCycleDossier_WritesValidArtifact(t *testing.T) {
	root := initDossierRepo(t)
	ws := t.TempDir()
	if err := writeCycleDossier(nil, cycleDossierParams{ProjectRoot: root, WorkspacePath: ws, Cycle: 7, Goal: "improve X", RunID: "run-ulid", Outcome: CycleOutcomeShippedViaBuild}); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "knowledge-base", "cycles", "cycle-7.json"))
	if err != nil {
		t.Fatalf("dossier not written: %v", err)
	}
	d, err := dossier.ParseJSON(data)
	if err != nil {
		t.Fatalf("written dossier unparseable: %v", err)
	}
	if err := d.Validate(); err != nil {
		t.Errorf("written dossier invalid: %v", err)
	}
	if d.Cycle != 7 || d.Goal != "improve X" || d.RunID != "run-ulid" {
		t.Errorf("dossier fields wrong: cycle=%d goal=%q run=%q", d.Cycle, d.Goal, d.RunID)
	}
	if d.FinalVerdict != dossier.VerdictPass {
		t.Errorf("FinalVerdict = %q, want PASS (SHIPPED_VIA_BUILD)", d.FinalVerdict)
	}
}

func TestWriteCycleDossier_FailOutcomeRecordsDefect(t *testing.T) {
	root := initDossierRepo(t)
	if err := writeCycleDossier(nil, cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 8, Goal: "fix Y", RunID: "run2", Outcome: VerdictFAIL}); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "knowledge-base", "cycles", "cycle-8.json"))
	d, err := dossier.ParseJSON(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if d.FinalVerdict != dossier.VerdictFail || len(d.Defects) == 0 {
		t.Errorf("FAIL cycle must record FAIL + defects; got verdict=%q defects=%d", d.FinalVerdict, len(d.Defects))
	}
}

// Regression: the tree-diff guard trips on any untracked
// knowledge-base/cycles/* pair left behind.
func TestWriteCycleDossier_LeavesCleanTree(t *testing.T) {
	root := initDossierRepo(t)
	if err := writeCycleDossier(nil, cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 537, Goal: "closeout", RunID: "run3", Outcome: CycleOutcomeShippedViaBuild}); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	cmd := exec.Command("git", "status", "--porcelain", "-uall")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		t.Fatalf("writeCycleDossier left the tree dirty (guard would trip):\n%s", s)
	}
}
