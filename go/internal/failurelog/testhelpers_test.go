package failurelog

// Local test helpers — deliberately NOT test/fixtures: fixtures imports
// internal/core, and core imports failurelog (SealCycle failure floor),
// so using fixtures here would create an import cycle in this package's
// in-package test binary.

import (
	"os"
	"path/filepath"
	"testing"
)

// mustWrite writes content to path (creating parent dirs) and returns path.
func mustWrite(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// mustRead reads path or fails the test.
func mustRead(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}
