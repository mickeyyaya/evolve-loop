package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPhaseTimings_SingleWriter is a source-scan guard, deliberately modelled
// on cycleoutcome's TestLaneScopeProjection_SingleWireShapeDeclaration — the
// guard that caught this diff's own SSOT violation.
//
// The defect it protects has now regressed twice. `phase-timing.json` must be
// composed and written through ONE path (cycleRun.flushPhaseTimings, which
// caches so the composition happens exactly once per cycle). A second raw
// caller re-appends this invocation's entries to a log that already contains
// them, so the durable log and the dossier built from it disagree — silently,
// and only on the resume path, which is exactly where nobody looks.
//
// Unit 01 (ADR-0103) moved the writer to the exported
// (*outcome.Recorder).WritePhaseTimings, which any package could construct
// and call, so the scan covers the WHOLE module (every non-test .go file
// outside the unit that defines it), not just this directory: Go's visibility
// no longer makes "one writer" structurally true, this guard does.
//
// If you are adding a legitimate writer: route it through flushPhaseTimings on
// the cycle's OWN cycleRun. If you truly need another, this test is the place
// to argue for it.
func TestPhaseTimings_SingleWriter(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", "..")) // internal/core → the go/ module
	if err != nil {
		t.Fatal(err)
	}
	const onlyWriter = "internal/core/failure_learning.go" // cycleRun.flushPhaseTimings
	const definer = "internal/core/outcome/"
	var offenders []string
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bin" || entry.Name() == "testdata" || (strings.HasPrefix(entry.Name(), ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, definer) {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(body), ".WritePhaseTimings(") && rel != onlyWriter {
			offenders = append(offenders, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("phase-timing.json must have ONE writer path (cycleRun.flushPhaseTimings in %s); "+
			"these non-test files call outcome.WritePhaseTimings directly: %v — a second raw caller "+
			"re-appends entries the log already holds, so the durable log and the dossier disagree", onlyWriter, offenders)
	}
}
