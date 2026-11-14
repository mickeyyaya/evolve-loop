package inboxmover

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyCycleOutcome_RefusesForeignCycleHeldItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	ownDir := filepath.Join(inbox, "processing", "cycle-10")
	otherDir := filepath.Join(inbox, "processing", "cycle-11")
	if err := os.MkdirAll(ownDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ownDir, "own.json"), []byte(`{"id":"own","title":"own"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	foreignPath := filepath.Join(otherDir, "foreign.json")
	if err := os.WriteFile(foreignPath, []byte(`{"id":"foreign","title":"foreign"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	res, err := ApplyCycleOutcome(Options{ProjectRoot: root, Stderr: &stderr}, CycleOutcome{
		Cycle:        10,
		Passed:       true,
		CommittedIDs: []string{"own", "foreign"},
	})
	if err != nil {
		t.Fatalf("ApplyCycleOutcome: %v", err)
	}
	if len(res.Promoted) != 1 || res.Promoted[0] != "own" {
		t.Fatalf("Promoted = %v, want [own]", res.Promoted)
	}
	if _, err := os.Stat(foreignPath); err != nil {
		t.Fatalf("foreign item left its place: %v", err)
	}
	if !strings.Contains(stderr.String(), "foreign") {
		t.Fatalf("stderr did not name refused foreign item: %s", stderr.String())
	}
}

func TestFilterPromotionScope_GuardsForeignAndUnpinnedItems(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	for _, dir := range []string{
		inbox,
		filepath.Join(inbox, "processing", "cycle-20"),
		filepath.Join(inbox, "processing", "cycle-21"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeItem := func(dir, id string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(`{"id":"`+id+`","title":"`+id+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeItem(filepath.Join(inbox, "processing", "cycle-20"), "held")
	writeItem(filepath.Join(inbox, "processing", "cycle-21"), "foreign")
	writeItem(inbox, "unheld")

	if emptyAllowed, emptyWarnings := FilterPromotionScope(Options{ProjectRoot: root}, 20, []string{"pinned"}, nil); len(emptyAllowed) != 0 || len(emptyWarnings) != 0 {
		t.Fatalf("empty FilterPromotionScope = (%v, %v), want nil", emptyAllowed, emptyWarnings)
	}

	allowed, warnings := FilterPromotionScope(Options{ProjectRoot: root}, 20, []string{"pinned"}, []string{"pinned", "held", "foreign", "unheld"})
	if len(allowed) != 2 || allowed[0] != "pinned" || allowed[1] != "held" {
		t.Fatalf("allowed = %v, want [pinned held]", allowed)
	}
	if len(warnings) != 2 || !strings.Contains(warnings[0], "foreign") || !strings.Contains(warnings[1], "unheld") {
		t.Fatalf("warnings = %v, want refusals for foreign and unheld", warnings)
	}
}
