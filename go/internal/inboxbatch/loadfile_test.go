package inboxbatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadFile_ParsesAcceptanceAndSanitises — the single-file loader the Task
// Contract projects from: acceptance is carried verbatim, control characters
// are stripped and overlength criteria bounded (a prompt surface), the id
// falls back to the filename stem exactly as LoadDir does.
func TestLoadFile_ParsesAcceptanceAndSanitises(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "task-a.json")
	long := strings.Repeat("x", maxAcceptanceLen+50)
	control := "\\" + "u0001" // the JSON escape for U+0001 (raw control bytes are invalid JSON)
	body := `{"title":"T","acceptance":["first criterion","bad` + control + `char","` + long + `"]}`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	it, warnings, err := LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if it.ID != "task-a" || it.Path != "task-a.json" || it.Title != "T" {
		t.Fatalf("identity: %+v", it)
	}
	if len(it.Acceptance) != 3 || it.Acceptance[0] != "first criterion" || it.Acceptance[1] != "bad char" || len(it.Acceptance[2]) != maxAcceptanceLen {
		t.Fatalf("acceptance = %q", it.Acceptance)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "sanitized") {
		t.Fatalf("sanitisation must be reported: %v", warnings)
	}
	if _, _, err := LoadFile(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("missing file must be an error")
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadFile(filepath.Join(dir, "bad.json")); err == nil {
		t.Fatal("malformed JSON must be an error")
	}
	// LoadDir sees the same record the same way (one parser).
	items, _, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range items {
		if d.ID == "task-a" && (len(d.Acceptance) != 3 || d.Acceptance[1] != "bad char") {
			t.Fatalf("LoadDir must parse acceptance identically: %+v", d)
		}
	}
}

// TestLoadFile_TitleIsAPromptSurfaceToo — the title renders as the Task
// Contract heading, so it gets the same control-character strip and bound.
func TestLoadFile_TitleIsAPromptSurfaceToo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "task-t.json")
	control := "\\" + "u0001"
	body := `{"title":"Line one` + control + `line two ` + strings.Repeat("y", maxFieldLen) + `","acceptance":["a"]}`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	it, warnings, err := LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(it.Title, 1) || len(it.Title) != maxFieldLen || !strings.HasPrefix(it.Title, "Line one line two") {
		t.Fatalf("title must be stripped and bounded: %q", it.Title)
	}
	if len(warnings) != 1 {
		t.Fatalf("sanitisation must be reported: %v", warnings)
	}
}
