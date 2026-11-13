package audit

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestSecondaryArtifacts_AndThePromptAgreeOnWhetherDispositionsAreOwed(t *testing.T) {
	t.Parallel()
	const ancestor = 1425
	open := []any{map[string]any{"id": "d1", "text": "open defect", "status": "OPEN"}}
	settled := []any{map[string]any{"id": "d2", "text": "closed", "status": "FIXED", "evidence": "docs/x.md"}}
	cases := []struct {
		name     string
		manifest bool
		registry bool
		entries  []any
		ledger   bool
		want     bool
	}{
		{name: "no manifest", want: false},
		{name: "manifest with no ancestor ledger", manifest: true, want: false},
		{name: "ancestor with only settled rows", manifest: true, ledger: true, entries: settled, want: false},
		{name: "one open row", manifest: true, ledger: true, entries: open, want: true},
		{name: "registry fallback with one open row", registry: true, ledger: true, entries: open, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			ws := filepath.Join(root, ".evolve", "runs", "cycle-1431")
			ancestorWS := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(ancestor))
			for _, d := range []string{ws, ancestorWS} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			binding := map[string]any{"cycle": ancestor, "branch": "cycle-1425", "snapshot_sha": "deadbeef", "base_sha": "cafebabe"}
			if tc.manifest {
				writeJSON(t, filepath.Join(ws, continuation.ManifestName), binding)
			}
			if tc.registry {
				writeJSON(t, continuation.RegistryPath(root), map[string]any{"lane-x": binding})
				writeJSON(t, filepath.Join(ws, core.LaneScopeFile), map[string]any{"todo_ids": []string{"lane-x"}, "goal_hash": "g"})
			}
			if tc.ledger {
				writeJSON(t, filepath.Join(ancestorWS, ledgerFile), map[string]any{"origin_cycle": ancestor, "entries": tc.entries})
			}
			req := core.PhaseRequest{Cycle: 1431, Workspace: ws, ProjectRoot: root}
			got := (hooks{}).SecondaryArtifacts(req)
			if tc.want {
				if want := filepath.Join(ws, defectDispositionFile); len(got) != 1 || got[0] != want {
					t.Fatalf("owed dispositions must be the one secondary %s, got %v", want, got)
				}
			} else if got != nil {
				t.Fatalf("no owed dispositions must declare no secondary, got %v", got)
			}
			prompted := strings.Contains((hooks{}).ComposePrompt("body", req), "Inherited defect dispositions")
			if prompted != (got != nil) {
				t.Fatalf("the prompt asks for the file (%v) exactly when the wait requires it (%v)", prompted, got != nil)
			}
		})
	}
}
