package continuation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadManifest_AnUnreadableManifestIsAnErrorNotAFreshStart(t *testing.T) {
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, ManifestName), 0o755); err != nil {
		t.Fatal(err)
	}

	c, ok, err := ReadManifest(ws)

	if err == nil || ok || !strings.Contains(err.Error(), "continuation: read manifest") || c != (Continuation{}) {
		t.Fatalf("c=%+v ok=%v err=%v", c, ok, err)
	}
	if _, ok, err := ReadManifest(t.TempDir()); ok || err != nil {
		t.Errorf("an absent manifest is a clean not-found: ok=%v err=%v", ok, err)
	}
}

func TestExpandHostPaths_WithoutAHomeDirectoryThePathsStayRedacted(t *testing.T) {
	t.Setenv("HOME", "")
	c := Continuation{Worktree: "~/work/tree", FindingsPath: "~/work/findings.json", SnapshotSHA: "abc", Cycle: 3}

	got := ExpandHostPaths(c)

	if got != c {
		t.Fatalf("got %+v, want the input unchanged: %+v", got, c)
	}
}

func TestExpandHostPaths_ExpandsOnlyTheTildePrefixAgainstHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := ExpandHostPaths(Continuation{Worktree: "~/work/tree", FindingsPath: "/abs/findings.json"})

	if got.Worktree != filepath.Join(home, "work", "tree") || got.FindingsPath != "/abs/findings.json" {
		t.Fatalf("got %+v", got)
	}
	if again := ExpandHostPaths(got); again != got {
		t.Errorf("expansion is idempotent: %+v vs %+v", again, got)
	}
}
