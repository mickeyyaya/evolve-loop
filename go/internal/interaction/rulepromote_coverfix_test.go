package interaction_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

const promotableRegex = "Rate this session before exiting"

func TestPromoteRule_InvalidRuleWritesNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	id, err := interaction.PromoteRule(dir, "y.*", "1,Enter", "too short", healthyCorpus)
	if err == nil || id != "" {
		t.Fatalf("PromoteRule = (%q, %v), want an empty id and a validation error", id, err)
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil || len(entries) != 0 {
		t.Errorf("a refused rule left %d entries in the registry (read error %v)", len(entries), readErr)
	}
}

func TestPromoteRule_UnreadableRegistryPathIsAnError(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	id, err := interaction.PromoteRule(blocker, promotableRegex, "1,Enter", "note", healthyCorpus)
	if err == nil || id != "" {
		t.Fatalf("PromoteRule under a file = (%q, %v), want an empty id and the stat error", id, err)
	}
}

func TestPromoteRule_WriteFailureIsReturned(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("chmod does not restrict root")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	id, err := interaction.PromoteRule(dir, promotableRegex, "1,Enter", "note", healthyCorpus)
	if err == nil || id != "" {
		t.Fatalf("PromoteRule into a read-only directory = (%q, %v), want an empty id and the write error", id, err)
	}
}
