package retrofile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParsePreventiveActions_UnterminatedFence(t *testing.T) {
	bad := "## Recommended preventive actions\n\n```json\n[{\"id\":\"x\"}]\n"
	if _, err := ParsePreventiveActions([]byte(bad)); err == nil {
		t.Fatal("expected error on unterminated ```json block, got nil")
	}
}

func TestParsePreventiveActions_MalformedJSON(t *testing.T) {
	bad := "## Recommended preventive actions\n\n```json\n{not json}\n```\n"
	if _, err := ParsePreventiveActions([]byte(bad)); err == nil {
		t.Fatal("expected error on malformed JSON payload, got nil")
	}
}

func TestParsePreventiveActions_HeadingButNoFence(t *testing.T) {
	doc := "## Recommended preventive actions\n- just a prose bullet\n"
	actions, err := ParsePreventiveActions([]byte(doc))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("got %d actions, want 0 for a prose-only section", len(actions))
	}
}

func TestFileActions_EmptyIDSkipped(t *testing.T) {
	inbox := t.TempDir()
	written, err := FileActions(inbox, 640, []PreventiveAction{{ID: "", Title: "no id"}}, 0.75, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("FileActions: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("wrote %d items, want 0 — an empty-id action must be skipped", len(written))
	}
}

func TestFileActions_MkdirFailsWhenInboxIsFile(t *testing.T) {
	base := t.TempDir()
	inboxPath := filepath.Join(base, "inbox")
	if err := os.WriteFile(inboxPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := FileActions(inboxPath, 640, []PreventiveAction{{ID: "x", Title: "t", PriorityClass: "correctness"}}, 0.75, time.Unix(0, 0).UTC()); err == nil {
		t.Fatal("expected error when inboxDir is a regular file, got nil")
	}
}

func TestFileActions_NonItemJSONIgnoredDuringDedup(t *testing.T) {
	inbox := t.TempDir()
	if err := os.WriteFile(filepath.Join(inbox, "manifest.json"), []byte("[1,2,3]"), 0o644); err != nil {
		t.Fatalf("seed non-item json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "noid.json"), []byte(`{"note":"x"}`), 0o644); err != nil {
		t.Fatalf("seed noid json: %v", err)
	}
	written, err := FileActions(inbox, 640, []PreventiveAction{{ID: "fresh", Title: "t", PriorityClass: "correctness"}}, 0.75, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("FileActions: %v", err)
	}
	if len(written) != 1 {
		t.Errorf("wrote %d items, want 1 — non-item JSON must not block a new action", len(written))
	}
}
