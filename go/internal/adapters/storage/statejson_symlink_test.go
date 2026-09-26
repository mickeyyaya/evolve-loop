package storage

// cycle-999 sever regression (atomicwrite-linked-state-sweep, cycle 1694):
// core/worktree.go linkGuardDeps symlinks a cycle worktree's
// .evolve/state.json and .evolve/cycle-state.json at their canonical files. A
// tmp+rename onto the UNRESOLVED path replaces the link with a regular file, so
// every later write strands in a detached copy. Each writer below must keep the
// link intact and land its bytes on the canonical target — for an absolute
// link, a relative link, and a link that dangles until the first write.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// linkedEvolveDir lays out <root>/canon/.evolve/<name> (seeded with seed
// unless shape is "dangling") and a worktree view <root>/wt/.evolve/<name>
// linking to it. It returns the worktree evolve dir, the link, its literal
// target, and the canonical path.
func linkedEvolveDir(t *testing.T, shape, name, seed string) (evolveDir, link, target, canonical string) {
	t.Helper()
	root := t.TempDir()
	canonDir := filepath.Join(root, "canon", ".evolve")
	evolveDir = filepath.Join(root, "wt", ".evolve")
	for _, d := range []string{canonDir, evolveDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	canonical = filepath.Join(canonDir, name)
	if shape != "dangling" {
		if err := os.WriteFile(canonical, []byte(seed), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	target = canonical
	if shape == "relative" {
		target = filepath.Join("..", "..", "canon", ".evolve", name)
	}
	link = filepath.Join(evolveDir, name)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	return evolveDir, link, target, canonical
}

// requireLinkIntact fails unless link is still a symlink pointing at target.
func requireLinkIntact(t *testing.T, link, target string) {
	t.Helper()
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat %s: %v", link, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is no longer a symlink (mode %v) — the write severed the link (cycle-999)", link, fi.Mode())
	}
	if got, err := os.Readlink(link); err != nil || got != target {
		t.Fatalf("%s points at %q (err=%v), want %q", link, got, err, target)
	}
}

var linkShapes = []string{"absolute", "relative", "dangling"}

func TestWriteState_WritesThroughSymlinkedStatePath(t *testing.T) {
	for _, shape := range linkShapes {
		t.Run(shape, func(t *testing.T) {
			evolveDir, link, target, canonical := linkedEvolveDir(t, shape, "state.json", `{"lastCycleNumber":1}`)

			if err := New(evolveDir).WriteState(context.Background(), core.State{LastCycleNumber: 1694}); err != nil {
				t.Fatalf("WriteState: %v", err)
			}
			requireLinkIntact(t, link, target)
			if got := readStateMap(t, canonical); got["lastCycleNumber"] != float64(1694) {
				t.Errorf("canonical %s = %v, want lastCycleNumber 1694", canonical, got)
			}
		})
	}
}

func TestUpdateState_WritesThroughSymlinkedStatePath(t *testing.T) {
	for _, shape := range linkShapes {
		t.Run(shape, func(t *testing.T) {
			evolveDir, link, target, canonical := linkedEvolveDir(t, shape, "state.json",
				`{"lastCycleNumber":5,"stateRevision":7,"operatorOwnedKey":"keep"}`)

			if _, err := New(evolveDir).UpdateState(context.Background(), func(s *core.State) {
				s.LastCycleNumber = 1694
			}); err != nil {
				t.Fatalf("UpdateState: %v", err)
			}
			requireLinkIntact(t, link, target)
			got := readStateMap(t, canonical)
			wantRev := float64(8) // bumped from the canonical value read through the link
			if shape == "dangling" {
				wantRev = 1
			}
			if got["lastCycleNumber"] != float64(1694) || got["stateRevision"] != wantRev {
				t.Errorf("canonical %s = %v, want lastCycleNumber 1694 stateRevision %v", canonical, got, wantRev)
			}
			if shape != "dangling" && got["operatorOwnedKey"] != "keep" {
				t.Errorf("unmodelled key lost through the link: %v", got)
			}
		})
	}
}

func TestWriteCycleState_WritesThroughSymlinkedCycleStatePath(t *testing.T) {
	for _, shape := range linkShapes {
		t.Run(shape, func(t *testing.T) {
			t.Setenv(ipcenv.CycleStateFileKey, "") // never write the live lane's cycle-state
			evolveDir, link, target, canonical := linkedEvolveDir(t, shape, "cycle-state.json",
				`{"cycle_id":1694,"phase":"tdd","checkpoint":{"marker":"keep"}}`)

			cs := core.CycleState{CycleID: 1694, Phase: "build"}
			if err := New(evolveDir).WriteCycleState(context.Background(), cs); err != nil {
				t.Fatalf("WriteCycleState: %v", err)
			}
			requireLinkIntact(t, link, target)
			got := readStateMap(t, canonical)
			if got["phase"] != "build" {
				t.Errorf("canonical %s = %v, want phase build", canonical, got)
			}
			if cp, _ := got["checkpoint"].(map[string]any); shape != "dangling" && (cp == nil || cp["marker"] != "keep") {
				t.Errorf("checkpoint block not spliced through the link: %v", got["checkpoint"])
			}
		})
	}
}
