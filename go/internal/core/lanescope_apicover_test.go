package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// TestLaneScope_ExportedSchemaAndFilename names the exported LaneScope type
// and LaneScopeFile const, pinning that an env-scoped RunCycle materializes
// the pin at <workspace>/<LaneScopeFile> with a schema matching LaneScope.
func TestLaneScope_ExportedSchemaAndFilename(t *testing.T) {
	root := t.TempDir()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root,
		GoalHash:    "goal-apicover",
		Env:         map[string]string{ipcenv.FleetScopeKey: "todo-a,todo-b"},
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(RunWorkspacePath(root, 1), LaneScopeFile))
	if err != nil {
		t.Fatalf("read %s: %v", LaneScopeFile, err)
	}
	var ls LaneScope
	if err := json.Unmarshal(b, &ls); err != nil {
		t.Fatalf("unmarshal into LaneScope: %v\n%s", err, b)
	}
	if ls.GoalHash != "goal-apicover" {
		t.Errorf("LaneScope.GoalHash = %q, want goal-apicover", ls.GoalHash)
	}
	if len(ls.TodoIDs) != 2 || ls.TodoIDs[0] != "todo-a" || ls.TodoIDs[1] != "todo-b" {
		t.Errorf("LaneScope.TodoIDs = %v, want [todo-a todo-b]", ls.TodoIDs)
	}
}
