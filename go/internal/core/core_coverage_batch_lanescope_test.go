package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
)

func TestCoreCoverageBatch_MaterializeLaneScope(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "nested", "ws")
	materializeLaneScope(ws, " a , b,c ", "h1")
	got := loadLaneScope(ws)
	if got == nil {
		t.Fatalf("materializeLaneScope wrote no readable %s under %s", LaneScopeFile, ws)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got.TodoIDs, want) || got.GoalHash != "h1" {
		t.Errorf("lane scope = %+v, want ids %v and goal hash h1", *got, want)
	}
}

func TestCoreCoverageBatch_MaterializeLaneScopeFailures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	blocker := filepath.Join(root, "file")
	writeBatchFile(t, blocker, "x")
	scopeDir := filepath.Join(root, "scope-is-dir")
	if err := os.MkdirAll(filepath.Join(scopeDir, LaneScopeFile), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		workspace string
	}{
		{"empty workspace", ""},
		{"workspace under a regular file", filepath.Join(blocker, "ws")},
		{"lane scope path is a directory", scopeDir},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			materializeLaneScope(tc.workspace, "a", "h")
			if got := loadLaneScope(tc.workspace); got != nil {
				t.Errorf("loadLaneScope(%q) = %+v, want nil after a failed materialize", tc.workspace, *got)
			}
		})
	}
}

func TestCoreCoverageBatch_ScoutReportGoalHash(t *testing.T) {
	t.Parallel()
	present := t.TempDir()
	writeBatchFile(t, filepath.Join(present, scoutArtifactName),
		"# Scout\n```json\n{\"goal_hash\": \"first\"}\n```\n```json\n{\"goal_hash\": \"last\"}\n```\n")
	cases := []struct {
		name      string
		workspace string
		want      string
	}{
		{"missing scout report", t.TempDir(), ""},
		{"last fenced goal hash wins", present, "last"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := scoutReportGoalHash(tc.workspace); got != tc.want {
				t.Errorf("scoutReportGoalHash = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCoreCoverageBatch_BuildBudgetScale(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		missing bool
		want    float64
	}{
		{"missing workspace is unscaled", true, 1.0},
		{"large triage size scales", false, 1.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := filepath.Join(t.TempDir(), "ws")
			if !tc.missing {
				if err := os.MkdirAll(ws, 0o755); err != nil {
					t.Fatal(err)
				}
				writeBatchFile(t, filepath.Join(ws, "triage-report.md"), "cycle_size_estimate: large\n")
			}
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
			cr := &cycleRun{o: o, cs: CycleState{WorkspacePath: ws, CompletedPhases: []string{"triage"}}}
			if got := cr.buildBudgetScale(); got != tc.want {
				t.Errorf("buildBudgetScale() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCoreCoverageBatch_SeedDispositionSkeleton(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeBatchLedger(t, root, 7, defectledger.Doc{OriginCycle: 7, Entries: []defectledger.Entry{
		{ID: "d1", Text: "open defect", Status: defectledger.StatusOpen},
		{ID: "d2", Text: "fixed defect", Status: defectledger.StatusFixed},
	}})
	ws := t.TempDir()
	SeedDispositionSkeleton(ws, root, 7)
	raw, err := os.ReadFile(filepath.Join(ws, defectledger.DispositionsFile))
	if err != nil {
		t.Fatalf("no seeded %s: %v", defectledger.DispositionsFile, err)
	}
	var doc map[string][]seededDisposition
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("seeded file unparseable: %v", err)
	}
	want := []seededDisposition{{ID: "d1", Status: defectledger.StatusOpen, Text: "open defect"}}
	if !reflect.DeepEqual(doc["dispositions"], want) {
		t.Errorf("dispositions = %+v, want %+v", doc["dispositions"], want)
	}
}

func TestCoreCoverageBatch_SeedDispositionSkeletonSkips(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	open := defectledger.Doc{Entries: []defectledger.Entry{{ID: "d1", Text: "t", Status: defectledger.StatusOpen}}}
	writeBatchLedger(t, root, 1, open)
	writeBatchLedger(t, root, 2, defectledger.Doc{Entries: []defectledger.Entry{{ID: "d2", Status: defectledger.StatusFixed}}})
	writeBatchLedger(t, root, 3, defectledger.Doc{})
	writeBatchFile(t, filepath.Join(RunWorkspacePath(root, 3), defectledger.LedgerFile), "{not json")
	existing := t.TempDir()
	writeBatchFile(t, filepath.Join(existing, defectledger.DispositionsFile), "keep")
	fileWorkspace := filepath.Join(t.TempDir(), "ws-file")
	writeBatchFile(t, fileWorkspace, "x")
	cases := []struct {
		name      string
		workspace string
		root      string
		ancestor  int
		keep      string
	}{
		{"empty workspace", "", root, 1, ""},
		{"empty project root", t.TempDir(), "", 1, ""},
		{"target already present", existing, root, 1, "keep"},
		{"missing ancestor ledger", t.TempDir(), root, 9, ""},
		{"malformed ancestor ledger", t.TempDir(), root, 3, ""},
		{"ancestor ledger without open entries", t.TempDir(), root, 2, ""},
		{"seed write fails", fileWorkspace, root, 1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			SeedDispositionSkeleton(tc.workspace, tc.root, tc.ancestor)
			if tc.workspace == "" {
				return
			}
			raw, err := os.ReadFile(filepath.Join(tc.workspace, defectledger.DispositionsFile))
			if tc.keep == "" && err == nil {
				t.Errorf("seeded %s = %q, want no file", defectledger.DispositionsFile, raw)
			}
			if tc.keep != "" && string(raw) != tc.keep {
				t.Errorf("existing %s rewritten to %q, want %q", defectledger.DispositionsFile, raw, tc.keep)
			}
		})
	}
}

func writeBatchLedger(t *testing.T, root string, cycle int, doc defectledger.Doc) {
	t.Helper()
	dir := RunWorkspacePath(root, cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := defectledger.Write(dir, doc); err != nil {
		t.Fatal(err)
	}
}
