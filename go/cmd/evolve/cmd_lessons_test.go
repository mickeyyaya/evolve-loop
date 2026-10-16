package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

func TestLessonsRecurrence_SortedByCountWithFixStatus(t *testing.T) {
	t.Parallel()
	led := recurrence.NewLedger()
	led.Entries["rare"] = &recurrence.Entry{Pattern: "rare", Count: 2}
	led.Entries["frequent"] = &recurrence.Entry{Pattern: "frequent", Count: 7, FixItemID: "frequent-fix"}
	led.Entries["mid"] = &recurrence.Entry{Pattern: "mid", Count: 4}

	var buf bytes.Buffer
	renderRecurrenceReport(&buf, led)
	out := buf.String()

	iFreq := strings.Index(out, "frequent")
	iMid := strings.Index(out, "mid")
	iRare := strings.Index(out, "rare")
	if iFreq < 0 || iMid < 0 || iRare < 0 {
		t.Fatalf("missing a pattern row:\n%s", out)
	}
	if !(iFreq < iMid && iMid < iRare) {
		t.Fatalf("rows not sorted by descending count:\n%s", out)
	}
	if !strings.Contains(out, "fix=frequent-fix") {
		t.Fatalf("fix-item status missing for linked pattern:\n%s", out)
	}
	if !strings.Contains(out, "fix=none") {
		t.Fatalf("fix=none status missing for unlinked pattern:\n%s", out)
	}
}

func TestLessonsBackfill_DryRunAndWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lessonsDir := filepath.Join(root, ".evolve", "instincts", "lessons")
	if err := os.MkdirAll(lessonsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lessonContent := "- id: cycle-1-test\n  pattern: test-defect\n  failureContext:\n    errorCategory: infrastructure\n"
	if err := os.WriteFile(filepath.Join(lessonsDir, "cycle-1-test.yaml"), []byte(lessonContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lessonsDir, "broken.yaml"), []byte("invalid yaml: ["), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := runLessonsBackfill([]string{"--project-root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("expected 0, got %d (stderr=%s)", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "1  test-defect") {
		t.Errorf("expected pattern in stdout:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "skipped: broken.yaml") {
		t.Errorf("expected skipped in stdout:\n%s", out.String())
	}
	targetPath := filepath.Join(root, ".evolve", "recurrence-ledger.json")
	if _, err := os.Stat(targetPath); err == nil {
		t.Errorf("dry-run should not create ledger")
	}

	out.Reset()
	errBuf.Reset()
	code = runLessonsBackfill([]string{"--write", "--project-root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("expected 0, got %d (stderr=%s)", code, errBuf.String())
	}
	led, err := recurrence.Load(targetPath)
	if err != nil {
		t.Fatalf("load saved ledger: %v", err)
	}
	if count := led.Count("test-defect"); count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}

func TestLessonsBackfill_MissingDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := runLessonsBackfill([]string{"--project-root", root}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
	if errBuf.Len() == 0 {
		t.Errorf("expected error message on stderr")
	}
}

func TestLessonsBackfill_BadArgs(t *testing.T) {
	t.Parallel()
	var out, errBuf bytes.Buffer
	code := runLessonsBackfill([]string{"--no-such-flag"}, &out, &errBuf)
	if code != 10 {
		t.Fatalf("expected 10 for bad flag, got %d", code)
	}

	out.Reset()
	errBuf.Reset()
	code = runLessonsBackfill([]string{"extra-arg"}, &out, &errBuf)
	if code != 10 {
		t.Fatalf("expected 10 for extra arg, got %d", code)
	}
}

func TestLessons_SubcommandsAndUsage(t *testing.T) {
	t.Parallel()
	var out, errBuf bytes.Buffer
	code := runLessons(nil, nil, &out, &errBuf)
	if code != 10 {
		t.Fatalf("expected 10 for empty args, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "backfill") {
		t.Errorf("expected usage to mention backfill:\n%s", errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	code = runLessons([]string{"unknown"}, nil, &out, &errBuf)
	if code != 10 {
		t.Fatalf("expected 10 for unknown subcommand, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "backfill") {
		t.Errorf("expected unknown to mention backfill:\n%s", errBuf.String())
	}
}
