package triagecap

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestNewReviewer_ReturnsReviewer(t *testing.T) {
	got := NewReviewer(config.StageEnforce)
	if got == nil {
		t.Fatal("NewReviewer returned nil")
	}
	if _, ok := got.(*CapReviewer); !ok {
		t.Fatalf("NewReviewer returned %T, want *CapReviewer", got)
	}
}

func TestReadFailedApproaches_MissingFile_ReturnsNil(t *testing.T) {
	if got := readFailedApproaches(t.TempDir()); got != nil {
		t.Fatalf("missing state.json = %+v, want nil", got)
	}
}

func TestReadFailedApproaches_ParsesGateEntries(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, `{
		"failedApproaches": [
			{"cycle": 301, "summary": "cycle 301 failed during triage: triage overpacked: 8 committed coverage floors exceed cap"}
		]
	}`)

	got := readFailedApproaches(root)
	want := []FailEntry{{
		Cycle:   301,
		Summary: "cycle 301 failed during triage: triage overpacked: 8 committed coverage floors exceed cap",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readFailedApproaches = %+v, want %+v", got, want)
	}
}

func TestReadFailedApproaches_CorruptJSON_ReturnsNil(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, `{not json`)
	if got := readFailedApproaches(root); got != nil {
		t.Fatalf("corrupt state.json = %+v, want nil", got)
	}
}

func TestCommittedFloorPackages_Declaration(t *testing.T) {
	dir := t.TempDir()
	companion := filepath.Join(dir, TriageDecisionName())
	if err := os.WriteFile(companion, []byte(`{"committed_floors":["bridge"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := CommittedFloorPackages("prose intentionally ignored", companion, []string{"bridge", "recovery"})
	want := []string{"bridge"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CommittedFloorPackages declaration = %v, want %v", got, want)
	}
}

func TestCommittedFloorPackages_FallsBackToProse(t *testing.T) {
	artifact := "## top_n\n- coverage-bridge: raise bridge coverage to 95%\n"
	missingCompanion := filepath.Join(t.TempDir(), TriageDecisionName())

	got := CommittedFloorPackages(artifact, missingCompanion, []string{"bridge", "recovery"})
	want := []string{"bridge"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CommittedFloorPackages prose fallback = %v, want %v", got, want)
	}
}

func TestReadWindow_MissingState_ReturnsNil(t *testing.T) {
	if got := readWindow(t.TempDir()); got != nil {
		t.Fatalf("missing state.json = %+v, want nil", got)
	}
}

func TestReadWindow_ParsesToThroughput(t *testing.T) {
	root := t.TempDir()
	writeStateJSON(t, root, `{"triageThroughput":[{"cycle":290,"floors":3}]}`)

	got := readWindow(root)
	if len(got) != 1 || got[0].Cycle != 290 || got[0].Floors != 3 {
		t.Fatalf("readWindow = %+v, want cycle 290 floors 3", got)
	}
}

func writeStateJSON(t *testing.T, root, body string) {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
