package audit

// secondary_artifacts_test.go — pins the arming condition of the audit
// SecondaryArtifacts hook (Phase B; adversarial-review HIGH: an untested
// conditional lets a degenerate implementation — inverted stat check or an
// unconditional return — pass the suite while either holding EVERY ordinary
// audit to its artifact timeout or silently regressing the 1397-1429
// continuation cutoff).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestSecondaryArtifacts_ArmsOnlyOnContinuationWorkspaces(t *testing.T) {
	ws := t.TempDir()
	req := core.PhaseRequest{Workspace: ws}

	if got := (hooks{}).SecondaryArtifacts(req); got != nil {
		t.Fatalf("ordinary (non-continuation) audit must declare NO secondaries — got %v; an unconditional declaration would hold every audit to its artifact timeout", got)
	}

	if err := os.WriteFile(filepath.Join(ws, "continuation-manifest.json"), []byte(`{"cycle":1426}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := (hooks{}).SecondaryArtifacts(req)
	want := filepath.Join(ws, "defect-dispositions.json")
	if len(got) != 1 || got[0] != want {
		t.Errorf("continuation audit must declare exactly the dispositions file, got %v want [%s] — a nil here regresses the 1397-1429 write-one-artifact-and-die class", got, want)
	}
}
