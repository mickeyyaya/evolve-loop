package audit

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const (
	dispositionPreflightMissing    = "disposition-preflight: MISSING"
	dispositionPreflightIncomplete = "disposition-preflight: INCOMPLETE"
)

func TestClassify_DispositionPreflightMissingFileIsNamed(t *testing.T) {
	_, req := continuationFixture(t, 1330, 1342, []string{
		"boundary refresh does not repin the short sha",
		"symlinked test-suffix bypasses probe quarantine",
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Fatalf("fixture is wrong: a continuation with two unaccounted inherited defects must not PASS")
	}
	text := diagsText(diags)
	if !strings.Contains(text, dispositionPreflightMissing) {
		t.Errorf("AC1 unmet — diagnostics must name the structural pre-flight finding %q when defect-dispositions.json is entirely absent on a continuation cycle, distinct from the per-id \"(no disposition)\" switch text. diagnostics:\n%s", dispositionPreflightMissing, text)
	}
	if !strings.Contains(text, "2") {
		t.Errorf("AC1 unmet — the MISSING pre-flight message must count the inherited defects (2) so an operator knows the size of the gap without counting per-id lines. diagnostics:\n%s", text)
	}
}

func TestClassify_DispositionPreflightIncompleteFileIsNamed(t *testing.T) {
	ws, req := continuationFixture(t, 1330, 1342, []string{
		"boundary refresh does not repin the short sha",
		"symlinked test-suffix bypasses probe quarantine",
	})
	cite := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite, "reason": "landed"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Fatalf("fixture is wrong: d2 is unaccounted for and must block PASS")
	}
	text := diagsText(diags)
	if !strings.Contains(text, dispositionPreflightIncomplete) {
		t.Errorf("AC2 unmet — diagnostics must name the structural pre-flight finding %q when defect-dispositions.json covers fewer ids than the ancestor ledger enumerates. diagnostics:\n%s", dispositionPreflightIncomplete, text)
	}
	if !strings.Contains(text, "d2") {
		t.Errorf("AC2 unmet — the INCOMPLETE pre-flight message must name which id(s) are uncovered (d2), not merely a count. diagnostics:\n%s", text)
	}
	if strings.Contains(text, dispositionPreflightMissing) {
		t.Errorf("a partially-covered file must be reported INCOMPLETE, never MISSING — the two markers describe different operator actions. diagnostics:\n%s", text)
	}
}

func TestClassify_DispositionPreflightCompleteFileNoFalsePositive(t *testing.T) {
	ws, req := continuationFixture(t, 1330, 1342, []string{
		"boundary refresh does not repin the short sha",
		"symlinked test-suffix bypasses probe quarantine",
	})
	cite1 := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	cite2 := evidenceFile(t, req.ProjectRoot, "go/internal/core/cyclerun.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite1, "reason": "landed"},
			map[string]any{"id": "d2", "status": "FIXED", "evidence": cite2, "reason": "landed"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Errorf("a fully-dispositioned continuation must PASS; verdict = %q\ndiagnostics:\n%s", verdict, diagsText(diags))
	}
	text := diagsText(diags)
	if strings.Contains(text, dispositionPreflightMissing) || strings.Contains(text, dispositionPreflightIncomplete) {
		t.Errorf("a complete defect-dispositions.json must trip NEITHER new pre-flight marker — a pre-flight that always fires proves nothing. diagnostics:\n%s", text)
	}
}

func TestClassify_DispositionPreflightNoAncestorNoOp(t *testing.T) {
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)
	req := core.PhaseRequest{Cycle: 1342, Workspace: ws, ProjectRoot: t.TempDir()}

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Errorf("an ordinary, non-continuation cycle must PASS; verdict = %q\ndiagnostics:\n%s", verdict, diagsText(diags))
	}
	text := diagsText(diags)
	if strings.Contains(text, dispositionPreflightMissing) || strings.Contains(text, dispositionPreflightIncomplete) {
		t.Errorf("an ordinary cycle with no ancestor ledger must never trip the disposition pre-flight — it has nothing inherited to be incomplete about. diagnostics:\n%s", text)
	}
}
