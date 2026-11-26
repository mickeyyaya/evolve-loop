package ship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type fixShip struct {
	message string
	dryRun  bool
}

func shipManualFix(t *testing.T, c fixShip) ([]signalcenter.Event, string) {
	t.Helper()
	repo := makeRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".git", "info", "exclude"), []byte(".commit-gate/\n"), 0o644); err != nil {
		t.Fatalf("write exclude: %v", err)
	}
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "go", "test", "journeys", "journeys.json"), `[{"id":"live","journey":"A path works","test":"internal/x.TestOne","issue":"#1"}]`)
	mustWrite(t, filepath.Join(repo, "go", "internal", "x", "one_test.go"), "package x\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {}\n")
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nchange\n")
	runGit(t, repo, "add", "go")
	writeAttestation(t, repo, treeStateSHA(t, repo))
	center := signalcenter.New()
	res, err := runShip(t, repo, Options{Class: ClassManual, CommitMessage: c.message, AutoConfirm: "test", Signals: center, DryRun: c.dryRun})
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("manual ship must land: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	center.Flush()
	var fixProof []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Module == signalcenter.ModuleFixProof {
			fixProof = append(fixProof, e)
		}
	}
	landed := ""
	if !c.dryRun {
		landed = runGitOut(t, repo, "log", "-1", "--format=%B")
	}
	return fixProof, landed
}

func TestShipManual_AFixCommitWithNoJourneyTrailerEmitsFixProofMissingAndStillLands(t *testing.T) {
	t.Parallel()
	got, landed := shipManualFix(t, fixShip{message: "fix: a defect"})
	if len(got) != 1 || got[0].Code != signalcenter.CodeFixProofMissing || got[0].Fields["problem"] != "missing" || got[0].Severity != signalcenter.SeverityWarn {
		t.Fatalf("want one warn FIX_PROOF_MISSING with problem=missing, got %+v", got)
	}
	if !strings.HasPrefix(landed, "fix: a defect") {
		t.Errorf("the shadow signal must not stop the commit; landed:\n%s", landed)
	}
}

func TestShipManual_AFixCommitNamesEachProblemOfItsTrailer(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"fix: a defect\n\nFix-Journey: nope":                         "unknown_journey",
		"fix: a defect\n\nFix-Journey: none":                         "malformed",
		"fix: a defect\n\nFix-Journey: test:internal/x.TestAbsent":   "missing_test",
		"fix(scope)!: a defect\n\nFix-Journey: test:internal/x.Gone": "missing_test",
	}
	for message, problem := range tests {
		message, problem := message, problem
		t.Run(problem, func(t *testing.T) {
			t.Parallel()
			got, _ := shipManualFix(t, fixShip{message: message})
			if len(got) != 1 || got[0].Fields["problem"] != problem {
				t.Errorf("want one signal with problem=%s, got %+v", problem, got)
			}
		})
	}
}

func TestShipManual_AFixCommitWithAProofEmitsNothingEvenAfterShipAppendsItsFooter(t *testing.T) {
	t.Parallel()
	for _, message := range []string{
		"fix: a defect\n\nFix-Journey: live",
		"fix: a defect\n\nFix-Journey: test:internal/x.TestOne",
		"fix: a defect\n\nFix-Journey: none a rename that keeps behavior",
		"feat: a thing",
	} {
		got, landed := shipManualFix(t, fixShip{message: message})
		if len(got) != 0 {
			t.Errorf("%q must emit nothing, got %+v", message, got)
		}
		if !strings.Contains(landed, "Reviewed-by:") || !strings.Contains(landed, "## Actual diff") {
			t.Errorf("the landed message must carry ship's trailer and footer after the author's text:\n%s", landed)
		}
	}
}

func TestShipManual_ADryRunEmitsNoFixProofSignal(t *testing.T) {
	t.Parallel()
	if got, _ := shipManualFix(t, fixShip{message: "fix: a defect", dryRun: true}); len(got) != 0 {
		t.Errorf("a dry run makes no commit, so it counts nothing: %+v", got)
	}
}
