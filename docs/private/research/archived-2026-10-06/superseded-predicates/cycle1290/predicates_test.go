//go:build acs

package cycle1290

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"os"
	"path/filepath"
	"testing"
)

func TestC1290_001_FloorArtifactsPublishAtTheAtomicwriteMode(t *testing.T) {
	runDir, lessonsDir, inboxDir := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "inbox")

	if err := faillearn.WriteArtifacts(failureEvent(), runDir, lessonsDir, faillearn.WithInbox(inboxDir, remediationItems())); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}

	paths := []string{filepath.Join(runDir, "retrospective-report.md")}
	for _, it := range remediationItems() {
		paths = append(paths, filepath.Join(inboxDir, it.ID+".json"))
	}
	ents, err := os.ReadDir(lessonsDir)
	if err != nil {
		t.Fatalf("read lessons dir: %v", err)
	}
	for _, e := range ents {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			paths = append(paths, filepath.Join(lessonsDir, e.Name()))
		}
	}
	if len(paths) != 4 {
		t.Fatalf("expected 4 published artifacts (report + 2 inbox items + 1 lesson), got %d: %v", len(paths), paths)
	}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("stat %s: %v", p, err)
			continue
		}
		if got := info.Mode().Perm(); got != 0o644 {
			t.Errorf("%s published with mode %04o, want 0644 (atomicwrite contract)", filepath.Base(p), got)
		}
	}
}
