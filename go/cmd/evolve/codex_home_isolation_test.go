package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexHome_InTestsIsAScratchDirUnderTheTempDir(t *testing.T) {
	t.Parallel()
	got := os.Getenv("CODEX_HOME")
	if got == "" || !strings.HasPrefix(got, filepath.Clean(os.TempDir())) {
		t.Fatalf("CODEX_HOME = %q in tests: a codex launch would write the operator's ~/.codex/config.toml", got)
	}
}
