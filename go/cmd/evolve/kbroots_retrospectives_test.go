package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/research"
)

const retrospectiveLessonYAML = `- id: retro-c6-through-the-cycle-seam
  pattern: wavecanary-recall-root
  description: recovery retrospective wavecanary lesson that the cycle recall must surface
  confidence: 0.9
  source: fixture
  type: failure-lesson
  category: episodic
  preventiveAction: read the retrospective lesson before the next recovery
  failureContext:
    failedStep: build
    errorCategory: wavecanary-recall-root
    auditVerdict: FAIL
`

func TestKBRootsAbs_RecallsALessonFromDocsRetrospectives(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "retrospectives")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c6.yaml"), []byte(retrospectiveLessonYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := research.NewFileKB(kbRootsAbs(root)).Lookup(context.Background(), research.Query{Keywords: []string{"retrospective", "wavecanary"}})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	if len(got) != 1 || got[0].ID != "retro-c6-through-the-cycle-seam" {
		t.Fatalf("kbRootsAbs default roots recalled %v, want the docs/retrospectives lesson", got)
	}
}
