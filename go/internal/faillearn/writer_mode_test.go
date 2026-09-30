package faillearn

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

const publishedMode fs.FileMode = 0o644

func TestWriteArtifacts_PublishedArtifactsHaveMode0644(t *testing.T) {
	runDir, lessonsDir, inboxDir := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "inbox")

	if err := WriteArtifacts(remediationEvent(), runDir, lessonsDir, WithInbox(inboxDir, remediationItems())); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}

	paths := []string{filepath.Join(runDir, "retrospective-report.md")}
	for _, it := range remediationItems() {
		paths = append(paths, filepath.Join(inboxDir, it.ID+".json"))
	}
	lessons, err := os.ReadDir(lessonsDir)
	if err != nil {
		t.Fatalf("read lessons dir: %v", err)
	}
	for _, e := range lessons {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			paths = append(paths, filepath.Join(lessonsDir, e.Name()))
		}
	}
	const reportPlusTwoInboxItemsPlusLesson = 4
	if len(paths) != reportPlusTwoInboxItemsPlusLesson {
		t.Fatalf("expected 4 published artifacts to stat, got %d (%v)", len(paths), paths)
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("stat published artifact %s: %v", p, err)
			continue
		}
		if got := info.Mode().Perm(); got != publishedMode {
			t.Errorf("%s published with mode %04o, want %04o — os.CreateTemp yields 0600 and the publish path must Chmod to the atomicwrite contract before linking; a 0600 floor artifact is unreadable to the other fleet lanes and the operator that read it",
				filepath.Base(p), got, publishedMode)
		}
	}
}

func TestWriteArtifacts_ModeParityAlsoHoldsWithoutTheInboxOption(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()

	if err := WriteArtifacts(remediationEvent(), runDir, lessonsDir); err != nil {
		t.Fatalf("WriteArtifacts without options: %v", err)
	}
	info, err := os.Stat(filepath.Join(runDir, "retrospective-report.md"))
	if err != nil {
		t.Fatalf("stat retrospective: %v", err)
	}
	if got := info.Mode().Perm(); got != publishedMode {
		t.Errorf("option-free retrospective published with mode %04o, want %04o", got, publishedMode)
	}
}

func TestWriteArtifacts_ExistingArtifactModeIsNotRewritten(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()

	report := filepath.Join(runDir, "retrospective-report.md")
	if err := os.WriteFile(report, []byte("# richer LLM-authored retrospective\n"), 0o600); err != nil {
		t.Fatalf("seed existing retrospective: %v", err)
	}
	if err := os.Chmod(report, 0o600); err != nil {
		t.Fatalf("chmod seed: %v", err)
	}

	if err := WriteArtifacts(remediationEvent(), runDir, lessonsDir); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}

	info, err := os.Stat(report)
	if err != nil {
		t.Fatalf("stat preserved retrospective: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("pre-existing retrospective mode was rewritten to %04o — the skip path must leave a preserved artifact entirely untouched (content AND mode)", got)
	}
	if body, err := os.ReadFile(report); err != nil || string(body) != "# richer LLM-authored retrospective\n" {
		t.Errorf("pre-existing retrospective content was clobbered: %q (err=%v)", string(body), err)
	}
}
