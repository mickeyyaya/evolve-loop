package faillearn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func partialWriteItems() []InboxItem {
	return []InboxItem{
		{
			ID:         "retro-1292-first-item-reaches-disk",
			Title:      "First remediation item — written before the failure",
			Weight:     0.95,
			Kind:       "bug",
			Priority:   "H",
			Files:      []string{"go/internal/faillearn/inbox.go"},
			InjectedBy: "retrofile",
		},
		{
			ID:         "retro-1292-second-item-fails",
			Title:      "Second remediation item — the write that fails",
			Weight:     0.9,
			Kind:       "bug",
			Priority:   "H",
			Files:      []string{"go/internal/faillearn/writer.go"},
			InjectedBy: "retrofile",
		},
		{
			ID:         "retro-1292-third-item-never-attempted",
			Title:      "Third remediation item — never attempted",
			Weight:     0.85,
			Kind:       "bug",
			Priority:   "M",
			Files:      []string{"go/internal/faillearn/inbox.go"},
			InjectedBy: "retrofile",
		},
	}
}

func unqueuedSection(t *testing.T, body string) string {
	t.Helper()
	const heading = "still UNQUEUED"
	lines := strings.Split(body, "\n")
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "#") && strings.Contains(ln, heading) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("degraded artifact has no %q section — the list of items that reached no queue is the payload of this artifact:\n%s", heading, body)
	}
	var out []string
	for _, ln := range lines[start:] {
		if strings.HasPrefix(ln, "#") {
			break
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

func collidingInboxDir(t *testing.T, failIdx int) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("prepare inbox dir: %v", err)
	}
	victim := partialWriteItems()[failIdx]
	body, err := json.MarshalIndent(InboxItem{
		ID:         victim.ID,
		Title:      "a DIFFERENT item already filed under this id by another lane",
		Weight:     0.5,
		Kind:       "chore",
		Priority:   "L",
		InjectedBy: "other-lane",
	}, "", "  ")
	if err != nil {
		t.Fatalf("encode colliding item: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, victim.ID+".json"), body, 0o644); err != nil {
		t.Fatalf("write colliding item: %v", err)
	}
	return dir
}

func TestWriteArtifacts_PartialWriteNamesOnlyUnqueuedItems(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	items := partialWriteItems()
	inboxDir := collidingInboxDir(t, 1)

	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(inboxDir, items))

	if err == nil {
		t.Fatal("an id collision must still abort WriteArtifacts — preserving an accurate item list is an ADDITION to failing loudly")
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "retrospective-report.md")); statErr == nil {
		t.Error("retrospective-report.md was written while remediation items reached no queue — the 1255 abort ordering must stay unreversed")
	}
	queued := items[0]
	if _, statErr := os.Stat(filepath.Join(inboxDir, queued.ID+".json")); statErr != nil {
		t.Fatalf("fixture premise broken: item %q was expected to reach disk before the failing item: %v", queued.ID, statErr)
	}

	raw, readErr := os.ReadFile(filepath.Join(runDir, unqueuedRetroName))
	if readErr != nil {
		t.Fatalf("the diagnosis must still be preserved as %s: %v", unqueuedRetroName, readErr)
	}
	section := unqueuedSection(t, string(raw))

	if strings.Contains(section, queued.ID) {
		t.Errorf("%s lists %q as still UNQUEUED, but that item is on disk in the inbox — the degraded artifact overclaims which remediation was lost (cycle-1290 D2)\n--- UNQUEUED section ---\n%s", unqueuedRetroName, queued.ID, section)
	}
	for _, it := range items[1:] {
		if !strings.Contains(section, it.ID) {
			t.Errorf("%s omits %q from the UNQUEUED list — that item reached no queue and is the work that would otherwise be lost\n--- UNQUEUED section ---\n%s", unqueuedRetroName, it.ID, section)
		}
	}
	if !strings.Contains(string(raw), "UNQUEUED") {
		t.Errorf("%s must keep its explicit UNQUEUED marker", unqueuedRetroName)
	}
}

func TestWriteArtifacts_PartialWriteItemRejectionNamesOnlyUnqueuedItems(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	inboxDir := filepath.Join(t.TempDir(), "inbox")

	items := partialWriteItems()
	items[1].ID = ""

	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(inboxDir, items))
	if err == nil {
		t.Fatal("an item with no id must still be rejected loudly")
	}
	queued := items[0]
	if _, statErr := os.Stat(filepath.Join(inboxDir, queued.ID+".json")); statErr != nil {
		t.Fatalf("fixture premise broken: item %q was expected to reach disk before the rejected item: %v", queued.ID, statErr)
	}

	raw, readErr := os.ReadFile(filepath.Join(runDir, unqueuedRetroName))
	if readErr != nil {
		t.Fatalf("an item-level rejection must still preserve the diagnosis: %v", readErr)
	}
	section := unqueuedSection(t, string(raw))

	if strings.Contains(section, queued.ID) {
		t.Errorf("%s lists %q as still UNQUEUED although it reached the inbox before the rejected item\n--- UNQUEUED section ---\n%s", unqueuedRetroName, queued.ID, section)
	}
	if !strings.Contains(section, items[2].ID) {
		t.Errorf("%s omits %q — an item after the rejection point reached no queue and must still be named\n--- UNQUEUED section ---\n%s", unqueuedRetroName, items[2].ID, section)
	}
}

func TestWriteArtifacts_PartialWrite_TotalFailureNamesEveryItem(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	items := partialWriteItems()

	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(blockedInboxPath(t), items))
	if err == nil {
		t.Fatal("an unwritable inbox directory must still return an error")
	}
	raw, readErr := os.ReadFile(filepath.Join(runDir, unqueuedRetroName))
	if readErr != nil {
		t.Fatalf("the diagnosis must be preserved on a total inbox failure: %v", readErr)
	}
	section := unqueuedSection(t, string(raw))
	for _, it := range items {
		if !strings.Contains(section, it.ID) {
			t.Errorf("%s omits %q, but NOTHING reached the queue on this arm — under-claiming loses the work the artifact exists to preserve\n--- UNQUEUED section ---\n%s", unqueuedRetroName, it.ID, section)
		}
	}
}

func TestWriteArtifacts_PartialWrite_FirstItemFailsNamesEveryItem(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	items := partialWriteItems()

	err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(collidingInboxDir(t, 0), items))
	if err == nil {
		t.Fatal("a collision on the first item must still abort")
	}
	raw, readErr := os.ReadFile(filepath.Join(runDir, unqueuedRetroName))
	if readErr != nil {
		t.Fatalf("read %s: %v", unqueuedRetroName, readErr)
	}
	section := unqueuedSection(t, string(raw))
	for _, it := range items {
		if !strings.Contains(section, it.ID) {
			t.Errorf("%s omits %q although the very first write failed — no item reached the queue on this arm\n--- UNQUEUED section ---\n%s", unqueuedRetroName, it.ID, section)
		}
	}
}
