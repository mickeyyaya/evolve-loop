package continuation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestListRegistryEntries covers the read the operator surface (`evolve
// continuation list`) is built on: every binding, and an absent registry as an
// empty map rather than an error — no registry is the normal state of a healthy
// project, and an error there would make the command useless exactly when it is
// most often run.
func TestListRegistryEntries(t *testing.T) {
	t.Run("absent registry is empty, not an error", func(t *testing.T) {
		got, err := ListRegistryEntries(t.TempDir())
		if err != nil {
			t.Fatalf("ListRegistryEntries on a project with no registry: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("want no bindings, got %d: %v", len(got), got)
		}
	})

	t.Run("returns every bound scope", func(t *testing.T) {
		root := t.TempDir()
		a := Continuation{SnapshotSHA: "aaa1", Branch: "cycle-1484", BaseSHA: "b1", Cycle: 1484}
		b := Continuation{SnapshotSHA: "bbb2", Branch: "cycle-1490", BaseSHA: "b2", Cycle: 1490}
		if err := WriteRegistryEntry(root, "scope-a", a); err != nil {
			t.Fatalf("bind scope-a: %v", err)
		}
		if err := WriteRegistryEntry(root, "scope-b", b); err != nil {
			t.Fatalf("bind scope-b: %v", err)
		}
		got, err := ListRegistryEntries(root)
		if err != nil {
			t.Fatalf("ListRegistryEntries: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 bindings, got %d: %v", len(got), got)
		}
		if got["scope-a"].SnapshotSHA != a.SnapshotSHA || got["scope-a"].Cycle != a.Cycle {
			t.Errorf("scope-a round-trip lost data: %+v", got["scope-a"])
		}
		if got["scope-b"].SnapshotSHA != b.SnapshotSHA {
			t.Errorf("scope-b round-trip lost data: %+v", got["scope-b"])
		}
	})
}

// TestRedactHostPaths pins the M1 fix (audit cycle-1507): released pointers ride
// the ship commit into TRACKED .evolve/inbox items on a public remote, so the
// absolute host paths must collapse to "~" — while the refs salvage actually
// resumes from (snapshot, base, branch) stay byte-identical, since redacting
// those would turn a privacy fix into a data-loss bug.
func TestRedactHostPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		t.Skipf("no resolvable home dir in this environment: %v", err)
	}
	in := Continuation{
		Worktree:     filepath.Join(home, "ai", "claude", "evolve-loop-runtime", ".evolve", "worktrees", "cycle-1"),
		FindingsPath: filepath.Join(home, "runs", "audit-fail-reason.json"),
		Branch:       "cycle-1484",
		SnapshotSHA:  "9813bc621fe4aa0d",
		BaseSHA:      "d3c69cd2aa11bb22",
		Cycle:        1484,
	}
	got := RedactHostPaths(in)

	if strings.Contains(got.Worktree, home) {
		t.Errorf("Worktree still carries the host home path %q: %q", home, got.Worktree)
	}
	if strings.Contains(got.FindingsPath, home) {
		t.Errorf("FindingsPath still carries the host home path %q: %q", home, got.FindingsPath)
	}
	wantWorktree := "~" + string(filepath.Separator) + filepath.Join("ai", "claude", "evolve-loop-runtime", ".evolve", "worktrees", "cycle-1")
	if got.Worktree != wantWorktree {
		t.Errorf("Worktree = %q, want %q", got.Worktree, wantWorktree)
	}
	if got.SnapshotSHA != in.SnapshotSHA || got.BaseSHA != in.BaseSHA || got.Branch != in.Branch || got.Cycle != in.Cycle {
		t.Errorf("redaction altered a salvage ref: %+v (want snapshot/base/branch/cycle from %+v)", got, in)
	}

	t.Run("a path outside the home dir is untouched", func(t *testing.T) {
		out := RedactHostPaths(Continuation{Worktree: "/srv/shared/worktree", FindingsPath: "relative/findings.json"})
		if out.Worktree != "/srv/shared/worktree" || out.FindingsPath != "relative/findings.json" {
			t.Errorf("non-home paths were rewritten: %+v", out)
		}
	})
}
