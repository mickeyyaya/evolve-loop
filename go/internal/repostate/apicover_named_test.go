package repostate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestTrackedSet_BindsTrackedFilesThroughRealGit binds the package's two
// exported functions to their real behavior end-to-end: TrackedFiles is the
// underlying enumerator and TrackedSet its basename projection — one fixture
// repo proves both carry git's actual index state (a staged file appears in
// each; an on-disk-only file appears in neither), rather than the names
// merely existing. Graduates internal/repostate to the public-API DoD
// (go/.apicover-enforce).
func TestTrackedSet_BindsTrackedFilesThroughRealGit(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"tracked.json", "minted.json"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", ".evolve/profiles/tracked.json")

	files, err := TrackedFiles(root, ".evolve/profiles")
	if err != nil {
		t.Fatalf("TrackedFiles: %v", err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "tracked.json" {
		t.Errorf("TrackedFiles = %v, want exactly the staged tracked.json", files)
	}

	set, err := TrackedSet(root, ".evolve/profiles", ".json")
	if err != nil {
		t.Fatalf("TrackedSet: %v", err)
	}
	if !set["tracked"] || set["minted"] || len(set) != 1 {
		t.Errorf("TrackedSet = %v, want {tracked:true} only", set)
	}
}
