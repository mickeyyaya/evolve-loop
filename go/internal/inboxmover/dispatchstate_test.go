package inboxmover

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTask drops a minimal task JSON into dir (created as needed).
func writeTask(t *testing.T, dir, id string, deps []string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"id":"` + id + `"`
	if len(deps) > 0 {
		body += `,"deps":["` + deps[0] + `"]`
	}
	body += `}`
	if err := os.WriteFile(filepath.Join(dir, "2026-07-13T00-00-00Z-"+id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestResolveDispatchState pins the lifecycle classification the fleet
// dispatch freshness gate builds on (cycle 767, dispatch-freshness-gate):
// each lifecycle dir maps to its state, pending carries the declared deps,
// processing names the owning cycle, and no evidence anywhere is StateUnknown
// (the fail-open posture — a bad or absent inbox must never false-skip a lane).
func TestResolveDispatchState(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeTask(t, inbox, "task-pending", []string{"dep-a"})
	writeTask(t, filepath.Join(inbox, "processing", "cycle-748"), "task-inflight", nil)
	writeTask(t, filepath.Join(inbox, "processed"), "task-done", nil)
	writeTask(t, filepath.Join(inbox, "rejected"), "task-nope", nil)
	writeTask(t, filepath.Join(inbox, "retry"), "task-again", nil)
	opts := Options{ProjectRoot: root}

	cases := []struct {
		id         string
		wantState  string
		wantDetail string
		wantDep    string
	}{
		{"task-pending", StatePending, "", "dep-a"},
		{"task-inflight", StateProcessing, "cycle-748", ""},
		{"task-done", StateProcessed, "", ""},
		{"task-nope", StateRejected, "", ""},
		{"task-again", StateRetry, "", ""},
		{"task-never-seen", StateUnknown, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			var got DispatchState = ResolveDispatchState(opts, tc.id)
			if got.State != tc.wantState || got.Detail != tc.wantDetail {
				t.Errorf("ResolveDispatchState(%q) = %+v, want state=%q detail=%q", tc.id, got, tc.wantState, tc.wantDetail)
			}
			if tc.wantDep != "" && (len(got.Deps) != 1 || got.Deps[0] != tc.wantDep) {
				t.Errorf("pending task must carry declared deps, got %v want [%s]", got.Deps, tc.wantDep)
			}
			if tc.wantDep == "" && len(got.Deps) != 0 {
				t.Errorf("non-pending states carry no deps, got %v", got.Deps)
			}
		})
	}
}
