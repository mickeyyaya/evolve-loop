package cycleoutcome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApicoverNamed_LaneScopeIDs(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if got := LaneScopeIDs(ws); got != nil {
		t.Errorf("absent pin must yield nil, got %v", got)
	}
	if err := os.WriteFile(filepath.Join(ws, "lane-scope.json"), []byte(`{"todo_ids":["a","b"],"goal_hash":"h"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LaneScopeIDs(ws)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("LaneScopeIDs = %v, want [a b]", got)
	}
}
