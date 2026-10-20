package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func TestProvisionedWorktree_ItsTrackedInboxCopyRefusesTheMover(t *testing.T) {
	t.Parallel()
	plane := t.TempDir()
	wt := filepath.Join(t.TempDir(), "cycle-5")
	item := []byte(`{"id":"task-1","payload":"x"}`)
	for _, root := range []string{plane, wt} {
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "task-1.json"), item, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(plane, ".evolve", "state.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	core.LinkGuardDepsForTesting(wt, plane, 5)

	_, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: wt}, "task-1", "5")
	if err == nil || !strings.Contains(err.Error(), "EVOLVE_PROJECT_ROOT="+plane) {
		t.Fatalf("err = %v, want a refusal that names the plane root %s", err, plane)
	}
	if _, err := os.Stat(filepath.Join(wt, ".evolve", "inbox", "task-1.json")); err != nil {
		t.Errorf("the worktree copy moved: %v", err)
	}
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: plane}, "task-1", "5"); err != nil {
		t.Errorf("the plane claim must pass: %v", err)
	}
}
