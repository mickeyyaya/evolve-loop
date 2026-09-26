package phasespec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Add a phase here when its persona starts writing source.
// See ADR-0097.
func TestUserSpecs_SourceWritersDeclareWritesSource(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	for _, name := range []string{"bug-reproduction", "test-amplification"} {
		raw, err := os.ReadFile(filepath.Join(root, ".evolve", "phases", name, "phase.json"))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var spec PhaseSpec
		if err := json.Unmarshal(raw, &spec); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !spec.WritesSource {
			t.Errorf("%s authors files into the worktree (see its agent.md) and must declare \"writes_source\": true — the fence would otherwise revert its deliverable", name)
		}
	}
}
