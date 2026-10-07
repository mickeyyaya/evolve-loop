//go:build acs

package cycle1292

import (
	"encoding/json"
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"os"
	"path/filepath"
	"testing"
)

func TestC1292_001_PartialInboxWriteNamesOnlyUnqueuedItems(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	inboxDir := filepath.Join(t.TempDir(), "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("prepare inbox dir: %v", err)
	}
	items := ledgerItems()
	collision, err := json.MarshalIndent(faillearn.InboxItem{ID: items[1].ID, Title: "filed by another lane", Weight: 0.5, Kind: "chore", Priority: "L", InjectedBy: "other-lane"}, "", "  ")
	if err != nil {
		t.Fatalf("encode colliding item: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, items[1].ID+".json"), collision, 0o644); err != nil {
		t.Fatalf("write colliding item: %v", err)
	}

	writeErr := faillearn.WriteArtifacts(failureEvent(), runDir, lessonsDir, faillearn.WithInbox(inboxDir, items))
	if writeErr == nil {
		t.Fatal("an id collision must still abort WriteArtifacts — an accurate item list is an ADDITION to failing loudly, never a replacement")
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "retrospective-report.md")); statErr == nil {
		t.Error("retrospective-report.md was written while remediation reached no queue — the 1255 abort ordering must stay unreversed")
	}
	if _, statErr := os.Stat(filepath.Join(inboxDir, items[0].ID+".json")); statErr != nil {
		t.Fatalf("fixture premise broken: %q was expected on disk before the failing item: %v", items[0].ID, statErr)
	}

	raw, readErr := os.ReadFile(filepath.Join(runDir, "retrospective-unqueued.md"))
	if readErr != nil {
		t.Fatalf("the diagnosis must still be preserved as retrospective-unqueued.md: %v", readErr)
	}
	section := unqueuedSection(t, string(raw))
	if strings.Contains(section, items[0].ID) {
		t.Errorf("retrospective-unqueued.md lists %q as still UNQUEUED although it is on disk in the inbox — cycle-1290 D2, the overclaim this cycle closes\n--- UNQUEUED section ---\n%s", items[0].ID, section)
	}
	for _, it := range items[1:] {
		if !strings.Contains(section, it.ID) {
			t.Errorf("retrospective-unqueued.md omits %q, which reached no queue — under-claiming loses the work the artifact exists to preserve\n--- UNQUEUED section ---\n%s", it.ID, section)
		}
	}
}
