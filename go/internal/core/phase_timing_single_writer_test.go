package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhaseTimings_SingleWriter(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
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
