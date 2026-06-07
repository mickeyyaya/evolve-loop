package faillearn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteArtifacts_WritesReportAndLesson(t *testing.T) {
	t.Parallel()
	runDir := filepath.Join(t.TempDir(), "runs", "cycle-243")
	lessonsDir := filepath.Join(t.TempDir(), "instincts", "lessons")
	ev := fixtureEvent()

	if err := WriteArtifacts(ev, runDir, lessonsDir); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}

	report, err := os.ReadFile(filepath.Join(runDir, "retrospective-report.md"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if string(report) != string(RenderRetrospectiveMarkdown(ev)) {
		t.Error("report content differs from RenderRetrospectiveMarkdown output")
	}

	id, want := RenderLessonYAML(ev)
	lesson, err := os.ReadFile(filepath.Join(lessonsDir, id+".yaml"))
	if err != nil {
		t.Fatalf("read lesson: %v", err)
	}
	if string(lesson) != string(want) {
		t.Error("lesson content differs from RenderLessonYAML output")
	}
}

// The floor must never clobber a richer artifact: if the LLM retro (or a
// previous floor write) already produced the file, skip it.
func TestWriteArtifacts_DedupesByLessonID(t *testing.T) {
	t.Parallel()
	runDir := t.TempDir()
	lessonsDir := t.TempDir()
	ev := fixtureEvent()
	id, _ := RenderLessonYAML(ev)

	sentinel := "- id: " + id + "\n  description: LLM-authored, do not clobber\n"
	if err := os.WriteFile(filepath.Join(lessonsDir, id+".yaml"), []byte(sentinel), 0o644); err != nil {
		t.Fatalf("seed lesson: %v", err)
	}

	if err := WriteArtifacts(ev, runDir, lessonsDir); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(lessonsDir, id+".yaml"))
	if string(got) != sentinel {
		t.Error("existing lesson was overwritten; dedupe by id must skip")
	}
}

func TestWriteArtifacts_PreservesExistingReport(t *testing.T) {
	t.Parallel()
	runDir := t.TempDir()
	lessonsDir := t.TempDir()
	ev := fixtureEvent()

	sentinel := "# LLM-authored retrospective — richer than the floor\n"
	if err := os.WriteFile(filepath.Join(runDir, "retrospective-report.md"), []byte(sentinel), 0o644); err != nil {
		t.Fatalf("seed report: %v", err)
	}

	if err := WriteArtifacts(ev, runDir, lessonsDir); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(runDir, "retrospective-report.md"))
	if string(got) != sentinel {
		t.Error("existing retrospective-report.md was overwritten; floor must preserve richer artifacts")
	}
}

func TestWriteArtifacts_NoTmpResidue(t *testing.T) {
	t.Parallel()
	runDir := t.TempDir()
	lessonsDir := t.TempDir()

	if err := WriteArtifacts(fixtureEvent(), runDir, lessonsDir); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}

	for _, dir := range []string{runDir, lessonsDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("readdir: %v", err)
		}
		for _, e := range entries {
			if strings.Contains(e.Name(), ".tmp") {
				t.Errorf("tmp residue %s in %s", e.Name(), dir)
			}
		}
	}
}

func TestWriteArtifacts_ErrorOnUnwritableTarget(t *testing.T) {
	t.Parallel()
	// runDir path occupied by a FILE — mkdir must fail loudly.
	occupied := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(occupied, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := WriteArtifacts(fixtureEvent(), occupied, t.TempDir()); err == nil {
		t.Error("want error when runDir is not a directory, got nil")
	}
}

// Loop-scope fatals have no cycle workspace: an empty runDir writes the
// lesson only, skipping the report instead of inventing a path.
func TestWriteArtifacts_EmptyRunDirSkipsReport(t *testing.T) {
	t.Parallel()
	lessonsDir := t.TempDir()
	ev := fixtureEvent()
	if err := WriteArtifacts(ev, "", lessonsDir); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}
	id, _ := RenderLessonYAML(ev)
	if _, err := os.Stat(filepath.Join(lessonsDir, id+".yaml")); err != nil {
		t.Fatalf("lesson must be written with empty runDir: %v", err)
	}
	if _, err := os.Stat("retrospective-report.md"); err == nil {
		t.Fatal("report leaked into cwd with empty runDir")
	}
}
