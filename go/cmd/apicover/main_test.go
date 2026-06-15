package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_ReportsUncoveredAndIgnoredWarningOnly(t *testing.T) {
	var buf bytes.Buffer
	code, err := Run(Config{Dirs: []string{"testdata/sample"}}, &buf)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()

	// ExportedConst is never referenced by usage_test.go -> reported uncovered.
	if !strings.Contains(out, "ExportedConst") {
		t.Errorf("expected ExportedConst in the uncovered report; got:\n%s", out)
	}
	// LegacyShim carries //apicover:ignore -> printed in the ignore-list with reason.
	if !strings.Contains(out, "LegacyShim") || !strings.Contains(out, "legacy shim") {
		t.Errorf("expected LegacyShim ignore-list line with its reason; got:\n%s", out)
	}
	// Default run is warning-only.
	if code != 0 {
		t.Errorf("default run must be warning-only (exit 0); got %d", code)
	}
}

func TestRun_EnforceExitsNonZeroOnUncovered(t *testing.T) {
	var buf bytes.Buffer
	code, err := Run(Config{Dirs: []string{"testdata/sample"}, Enforce: true}, &buf)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code == 0 {
		t.Errorf("-enforce with uncovered symbols present must exit non-zero; got 0\n%s", buf.String())
	}
}

// TestRun_WithCoverFile_ClassifiesCovered exercises the two-signal JOIN end to
// end: a symbol named by a test AND >0% in the cover file is Covered (so it is
// NOT listed as a problem). Without this, the "Covered" path is never proven and
// a regression in the cover-key format would pass silently.
func TestRun_WithCoverFile_ClassifiesCovered(t *testing.T) {
	imp, err := packageImportPath("testdata/sample")
	if err != nil {
		t.Fatalf("packageImportPath: %v", err)
	}
	syms, err := Enumerate("testdata/sample")
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}
	var line int
	for _, s := range syms {
		if s.Name == "ExportedFunc" {
			line = s.Line
		}
	}
	if line == 0 {
		t.Fatal("ExportedFunc not enumerated")
	}

	cover := filepath.Join(t.TempDir(), "cover.func")
	content := fmt.Sprintf("%s/sample.go:%d:\tExportedFunc\t100.0%%\n", imp, line)
	if err := os.WriteFile(cover, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := Run(Config{Dirs: []string{"testdata/sample"}, CoverPath: cover}, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	// ExportedFunc is named + 100% -> Covered, and Covered symbols are not listed.
	if strings.Contains(out, "ExportedFunc") {
		t.Errorf("ExportedFunc should be Covered (named + 100%%) and not reported as a problem; got:\n%s", out)
	}
}
