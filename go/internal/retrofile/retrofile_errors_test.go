package retrofile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestParsePreventiveActions_UnterminatedFence — a heading with an opening
// ```json but no closing fence is a malformed contract the caller must see.
func TestParsePreventiveActions_UnterminatedFence(t *testing.T) {
	bad := "## Recommended preventive actions\n\n```json\n[{\"id\":\"x\"}]\n"
	if _, err := ParsePreventiveActions([]byte(bad)); err == nil {
		t.Fatal("expected error on unterminated ```json block, got nil")
	}
}

// TestParsePreventiveActions_MalformedJSON — a well-fenced but invalid JSON
// payload is surfaced, not silently dropped.
func TestParsePreventiveActions_MalformedJSON(t *testing.T) {
	bad := "## Recommended preventive actions\n\n```json\n{not json}\n```\n"
	if _, err := ParsePreventiveActions([]byte(bad)); err == nil {
		t.Fatal("expected error on malformed JSON payload, got nil")
	}
}

// TestParsePreventiveActions_HeadingButNoFence — a heading with no ```json
// block yields no actions and no error (prose-only preventive actions).
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

// TestFileActions_EmptyIDSkipped — an action with an empty ID is skipped
// (it has no dedup key and no valid slug), never filed.
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

// TestFileActions_MkdirFailsWhenInboxIsFile — a non-directory at inboxDir is a
// real error surfaced to the caller (best-effort callers can ignore it, but the
// function must not silently succeed).
func TestFileActions_MkdirFailsWhenInboxIsFile(t *testing.T) {
	base := t.TempDir()
	inboxPath := filepath.Join(base, "inbox")
	if err := os.WriteFile(inboxPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := FileActions(inboxPath, 640, []PreventiveAction{{ID: "x", Title: "t"}}, 0.75, time.Unix(0, 0).UTC()); err == nil {
		t.Fatal("expected error when inboxDir is a regular file, got nil")
	}
}

// TestFileActions_NonItemJSONIgnoredDuringDedup — a non-item JSON file under
// the inbox (an object with no id, or a JSON array) must not abort dedup nor
// suppress a legitimately new action.
func TestFileActions_NonItemJSONIgnoredDuringDedup(t *testing.T) {
	inbox := t.TempDir()
	if err := os.WriteFile(filepath.Join(inbox, "manifest.json"), []byte("[1,2,3]"), 0o644); err != nil {
		t.Fatalf("seed non-item json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "noid.json"), []byte(`{"note":"x"}`), 0o644); err != nil {
		t.Fatalf("seed noid json: %v", err)
	}
	written, err := FileActions(inbox, 640, []PreventiveAction{{ID: "fresh", Title: "t"}}, 0.75, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("FileActions: %v", err)
	}
	if len(written) != 1 {
		t.Errorf("wrote %d items, want 1 — non-item JSON must not block a new action", len(written))
	}
}
