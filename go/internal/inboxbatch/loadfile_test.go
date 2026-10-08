package inboxbatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestLoadFile_NoticeNamesTruncationApartFromControlCharacters(t *testing.T) {
	t.Parallel()
	control := "\\" + "u0007"
	cases := []struct {
		name, body, want, notWant string
	}{
		{"truncated-title", `{"title":"` + strings.Repeat("t", maxFieldLen+1) + `"}`, ": truncated overlength title to bound (item loaded)", "control"},
		{"truncated-files-acceptance", `{"files":["` + strings.Repeat("f", maxFieldLen+1) + `"],"acceptance":["` + strings.Repeat("a", maxAcceptanceLen+1) + `"]}`, ": truncated overlength files, acceptance to bound (item loaded)", "control"},
		{"control-only", `{"title":"one` + control + `two"}`, ": sanitized control characters in title", "truncat"},
		{"both", `{"title":"one` + control + `two","acceptance":["` + strings.Repeat("a", maxAcceptanceLen+1) + `"]}`, ": sanitized control characters in title; truncated overlength acceptance to bound (item loaded)", "skipped"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := filepath.Join(t.TempDir(), tc.name+".json")
			if err := os.WriteFile(p, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, warnings, err := LoadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(warnings) != 1 || warnings[0] != tc.name+".json"+tc.want || strings.Contains(warnings[0], tc.notWant) {
				t.Errorf("notice = %q, want %q without %q", warnings, tc.name+".json"+tc.want, tc.notWant)
			}
		})
	}
}
