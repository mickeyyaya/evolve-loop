package inboxcurator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenItems_ReadsTheRootAndEveryClaimAndNamesAnUnreadableFile(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	writeFile(t, filepath.Join(inbox, "a-root.json"), `{"id":"root"}`)
	writeFile(t, filepath.Join(inbox, "processing", "cycle-7", "b-claimed.json"), `{"id":"claimed"}`)
	writeFile(t, filepath.Join(inbox, "consumed", "c-done.json"), `{"id":"done"}`)
	writeFile(t, filepath.Join(inbox, "d-torn.json"), `{"id":`)

	items, warnings, err := OpenItems(inbox)

	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	slices.Sort(ids)
	if err != nil || !slices.Equal(ids, []string{"claimed", "root"}) {
		t.Fatalf("OpenItems = %v, %v; want root and claimed", ids, err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].String(), "d-torn.json") {
		t.Errorf("warnings = %+v, want one naming d-torn.json", warnings)
	}
}

func TestOpenItems_AnUnreadableInboxIsAnError(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	writeFile(t, inbox, "a file")

	if _, _, err := OpenItems(inbox); err == nil {
		t.Fatal("OpenItems of a file = nil error, want an error")
	}
}

func TestOpenItems_AnUnreadableClaimDirIsAnError(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	writeFile(t, filepath.Join(inbox, "a-root.json"), `{"id":"root"}`)
	claim := filepath.Join(inbox, "processing", "cycle-7")
	if err := os.MkdirAll(claim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(claim, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(claim, 0o755) })

	if _, _, err := OpenItems(inbox); err == nil {
		t.Fatal("OpenItems with an unreadable claim dir = nil error, want an error")
	}
}
