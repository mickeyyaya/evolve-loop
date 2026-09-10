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
// If you are adding a legitimate writer: route it through flushPhaseTimings on
// the cycle's OWN cycleRun. If you truly need another, this test is the place
// to argue for it.
func TestPhaseTimings_SingleWriter(t *testing.T) {
	dir, err := os.Getwd() // internal/core
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if strings.Contains(string(body), "writePhaseTimings(") && name != "failure_learning.go" {
			offenders = append(offenders, name)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("phase-timing.json must have ONE writer path (cycleRun.flushPhaseTimings); "+
			"these non-test files call writePhaseTimings directly: %v — a second raw caller "+
			"re-appends entries the log already holds, so the durable log and the dossier disagree", offenders)
	}
}
