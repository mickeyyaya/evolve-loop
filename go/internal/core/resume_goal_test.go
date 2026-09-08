package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreResumeGoal_LegacyIdentityPrecedesCallerGoal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		fleet        bool
		checkpoint   string
		requested    string
		wantHash     string
		wantConflict bool
	}{
		{name: "singleton restores batch", wantHash: "original"},
		{name: "singleton accepts original", requested: "original", wantHash: "original"},
		{name: "singleton rejects replacement", requested: "replacement", wantConflict: true},
		{name: "checkpoint wins over batch", checkpoint: "checkpoint", requested: "checkpoint", wantHash: "checkpoint"},
		{name: "fleet never borrows batch", fleet: true},
		{name: "fleet accepts explicit missing goal", fleet: true, requested: "provided", wantHash: "provided"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			state := State{}
			state.CurrentBatch.GoalHash = "original"
			req := CycleRequest{ProjectRoot: root, GoalHash: tc.requested}
			cs := CycleState{CycleID: 7, GoalHash: tc.checkpoint, WorkspacePath: filepath.Join(root, "workspace")}

			got, err := restoreResumeGoal(req, cs, state, tc.fleet)

			if tc.wantConflict {
				if err == nil || !strings.Contains(err.Error(), "resume identity mismatch") {
					t.Fatalf("replacement error=%v, want resume identity mismatch", err)
				}
				return
			}
			if err != nil || got.GoalHash != tc.wantHash {
				t.Fatalf("restored hash=%q err=%v, want %q and nil", got.GoalHash, err, tc.wantHash)
			}
			if tc.wantHash == "" && !strings.Contains(got.Context["goal"], "legacy checkpoint omitted original goal") {
				t.Fatalf("unknown fleet goal must be explicit, got %q", got.Context["goal"])
			}
		})
	}
}
