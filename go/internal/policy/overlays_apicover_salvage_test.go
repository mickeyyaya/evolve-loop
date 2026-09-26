package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLaunchOverlaysFailOpen_MissingAndMalformedPolicyDegrade(t *testing.T) {
	t.Parallel()
	missing := t.TempDir()
	got := ResolveLaunchOverlaysFailOpen(missing, "advisor", "claude-tmux", "deep")
	want := (Policy{}).ResolveOverlays(DispatchFromPhaseRequest("advisor", "claude-tmux", "deep", "deep"))
	if len(got) != len(want) {
		t.Fatalf("missing policy must degrade to compiled-default overlays: got %v want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("missing-policy overlays diverged at %d: got %v want %v", i, got, want)
		}
	}

	malformed := t.TempDir()
	if err := os.MkdirAll(filepath.Join(malformed, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(malformed, ".evolve", "policy.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got2 := ResolveLaunchOverlaysFailOpen(malformed, "advisor", "claude-tmux", "deep")
	if len(got2) != len(want) {
		t.Fatalf("malformed policy must degrade identically: got %v want %v", got2, want)
	}
}
