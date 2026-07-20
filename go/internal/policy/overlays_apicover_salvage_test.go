package policy

// overlays_apicover_salvage_test.go — apicover Phase-5 naming coverage for the
// salvaged cycle-943/950 export (false-RED salvage, post-v22.4.2): NAMES +
// EXERCISES ResolveLaunchOverlaysFailOpen. Behavioral (Rule 9): the fail-open
// contract is asserted — a missing/malformed policy.json degrades to the
// compiled-default overlays instead of aborting the launch.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLaunchOverlaysFailOpen_MissingAndMalformedPolicyDegrade(t *testing.T) {
	t.Parallel()
	// Missing policy.json → fail-open: same result as the compiled defaults.
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

	// Malformed policy.json → same fail-open degrade, never a panic/abort.
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
