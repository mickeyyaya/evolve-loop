package triage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTriagePolicy(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInboxBatchesSection_AtEnforceTheLaneItemWithAConsoleCompanionIsNotSelectable(t *testing.T) {
	root := t.TempDir()
	writeTriagePolicy(t, root, `{"workflow": {"repair": {"fixability": {"stage": "enforce"}}}}`)
	writeInboxItem(t, root, "a.json", `{"id":"x-lane","weight":0.9,"route":"lane"}`)
	writeInboxItem(t, root, "b.json", `{"id":"x-console","weight":0.4,"route":"console-manual"}`)

	got := inboxBatchesSection(root, nil)

	if !strings.Contains(got, "x-lane: console_companion_pending: x-console") || strings.Contains(got, "inbox_batches") {
		t.Errorf("section =\n%s\nwant the lane item held with the companion reason and no selectable batch", got)
	}
}
