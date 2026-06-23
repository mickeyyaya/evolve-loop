package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAllowlist(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "allow.txt"),
		[]byte("# comment\n\ngo/a_test.go\n  go/b_test.go  \n# skip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadAllowlist(dir, "allow.txt", io.Discard)
	if len(got) != 2 || got[0] != "go/a_test.go" || got[1] != "go/b_test.go" {
		t.Errorf("got %v, want [go/a_test.go go/b_test.go]", got)
	}
	if loadAllowlist(dir, "nope.txt", io.Discard) != nil {
		t.Error("missing allowlist file should return nil")
	}
	if loadAllowlist(dir, "", io.Discard) != nil {
		t.Error("empty path should return nil")
	}
}
