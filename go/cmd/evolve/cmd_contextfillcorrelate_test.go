package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriteReportAtomicRefusesASymlink is the D3 regression: --out must not
// follow a pre-planted symlink and truncate whatever it points at. The victim
// file's content is asserted intact, which is the part that actually matters.
func TestWriteReportAtomicRefusesASymlink(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim.txt")
	const secret = "do not truncate me\n"
	if err := os.WriteFile(victim, []byte(secret), 0o600); err != nil {
		t.Fatalf("seed victim: %v", err)
	}
	link := filepath.Join(dir, "report.md")
	if err := os.Symlink(victim, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := writeReportAtomic(link, "# report\n"); err == nil {
		t.Error("writeReportAtomic followed a symlink, want a refusal")
	}
	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("read victim: %v", err)
	}
	if string(got) != secret {
		t.Errorf("victim content = %q, want it untouched (%q)", got, secret)
	}
}

// TestWriteReportAtomicWritesAndReplaces pins the happy path and the replace
// path, and asserts the temp file does not survive either.
func TestWriteReportAtomicWritesAndReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.md")

	for _, want := range []string{"# first\n", "# second\n"} {
		if err := writeReportAtomic(path, want); err != nil {
			t.Fatalf("writeReportAtomic(%q): %v", want, err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		if string(got) != want {
			t.Errorf("content = %q, want %q", got, want)
		}
		if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
			t.Errorf("temp file survived the rename (stat err = %v)", err)
		}
	}
}
