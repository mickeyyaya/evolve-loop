package panewatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAll_AMalformedWorkspaceGlobIsAnErrorNotAnEmptyResult(t *testing.T) {
	snaps, err := ReadAll(filepath.Join(t.TempDir(), "[unclosed"))

	if err == nil || snaps != nil {
		t.Fatalf("snaps=%v err=%v: a pattern error must not look like a quiet workspace", snaps, err)
	}
}

func TestReadFile_AnUnreadableSnapshotIsAnErrorAndNeverAMissingOne(t *testing.T) {
	dir := t.TempDir()
	asDir := filepath.Join(dir, "scout"+fileSuffix)
	if err := os.Mkdir(asDir, 0o755); err != nil {
		t.Fatal(err)
	}

	s, ok, err := readFile(asDir)

	if err == nil || ok || s != (Snapshot{}) {
		t.Fatalf("s=%+v ok=%v err=%v", s, ok, err)
	}
	if _, ok, err := readFile(filepath.Join(dir, "absent"+fileSuffix)); ok || err != nil {
		t.Errorf("an absent snapshot is the clean not-found: ok=%v err=%v", ok, err)
	}
}

func TestReadAll_ReportsEveryUnreadableSnapshotAndStillReturnsTheGoodOnes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "a"+fileSuffix), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b"+fileSuffix), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c"+fileSuffix), []byte(`{"agent":"c"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	snaps, err := ReadAll(dir)

	if err == nil || !strings.Contains(err.Error(), "pane watch b") {
		t.Fatalf("err = %v", err)
	}
	if len(snaps) != 1 || snaps[0].Agent != "c" {
		t.Fatalf("snaps = %+v", snaps)
	}
}
