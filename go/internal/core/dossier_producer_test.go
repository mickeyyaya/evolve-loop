package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// initDossierRepo makes root a git working tree so writeCycleDossier's commit
// (dossier.Write(..., true)) has a repo to add+commit into. Production always
// runs against the git main tree; the tests mirror that precondition.
func initDossierRepo(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// TestDossierVerdict_MapsCycleOutcomes pins the CycleOutcome → dossier verdict
// mapping. Only a clean ship is PASS; an explicit FAIL is FAIL; every other
// terminal (WARN/SKIPPED/advisory/unknown) is WARN — a non-PASS record that
// captures experience without fabricating a pass or demanding defects.
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

// TestWriteCycleDossier_WritesValidArtifact is the core of the ADR-0055 fix: a
// completed cycle writes knowledge-base/cycles/cycle-N.json and it is valid.
func TestWriteCycleDossier_WritesValidArtifact(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	ws := t.TempDir()
	if err := writeCycleDossier(nil, root, ws, 7, "improve X", "run-ulid", CycleOutcomeShippedViaBuild, nil); err != nil {
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

// TestWriteCycleDossier_FailOutcomeRecordsDefect proves a failed cycle's record
// is truthful (FAIL + a defect), not an always-PASS skeleton.
func TestWriteCycleDossier_FailOutcomeRecordsDefect(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	if err := writeCycleDossier(nil, root, t.TempDir(), 8, "fix Y", "run2", VerdictFAIL, nil); err != nil {
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

// TestWriteCycleDossier_LeavesCleanTree is the regression for the recurring
// tree-diff-guard P0: after writeCycleDossier, the main tree must carry NO
// untracked knowledge-base/cycles/* pair for a later phase's guard to flag.
func TestWriteCycleDossier_LeavesCleanTree(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	if err := writeCycleDossier(nil, root, t.TempDir(), 537, "closeout", "run3", CycleOutcomeShippedViaBuild, nil); err != nil {
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
