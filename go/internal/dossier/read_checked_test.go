package dossier

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkedDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeCorpusDossier(t, root, 11, nil)
	return CyclesDir(root)
}

func denyAccess(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
	if f, err := os.Open(path); err == nil {
		_ = f.Close()
		t.Fatalf("%s is still readable after chmod 0: the fixture cannot prove a read failure as this user", path)
	}
}

func wantReadFailure(t *testing.T, failures []ReadFailure, path string) {
	t.Helper()
	if len(failures) != 1 || failures[0].Path != path || failures[0].Err == nil || !strings.Contains(failures[0].Error(), path) {
		t.Fatalf("failures = %+v, want exactly one at %s that names the path in its text", failures, path)
	}
}

func TestReadDirChecked_ReportsACorruptFileAndKeepsTheGoodOnes(t *testing.T) {
	t.Parallel()
	dir := checkedDir(t)
	bad := filepath.Join(dir, "cycle-12.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, failures := ReadDirChecked(dir, 1, 0)

	wantReadFailure(t, failures, bad)
	if len(got) != 1 || got[0].Cycle != 11 {
		t.Errorf("dossiers = %+v, want only cycle 11", got)
	}
}

func TestReadDirChecked_ReportsAnUnreadableFile(t *testing.T) {
	t.Parallel()
	dir := checkedDir(t)
	file := filepath.Join(dir, "cycle-11.json")
	denyAccess(t, file)

	got, failures := ReadDirChecked(dir, 1, 0)

	wantReadFailure(t, failures, file)
	if len(got) != 0 {
		t.Errorf("dossiers = %+v, want none", got)
	}
}

func TestReadDirChecked_ReportsAnUnreadableDirectory(t *testing.T) {
	t.Parallel()
	dir := checkedDir(t)
	denyAccess(t, dir)

	_, failures := ReadDirChecked(dir, 1, 0)

	wantReadFailure(t, failures, dir)
}

func TestReadDirChecked_ReportsNothingForAnAbsentDirectoryOrAHealthyOne(t *testing.T) {
	t.Parallel()
	dir := checkedDir(t)

	got, failures := ReadDirChecked(dir, 1, 0)
	_, absent := ReadDirChecked(filepath.Join(dir, "nope"), 1, 0)

	if len(failures) != 0 || len(got) != 1 {
		t.Errorf("healthy dir: dossiers %d failures %+v, want 1 and none", len(got), failures)
	}
	if len(absent) != 0 {
		t.Errorf("absent dir: failures %+v, want none", absent)
	}
}

func TestReadDir_StillSkipsEachFailure(t *testing.T) {
	t.Parallel()
	dir := checkedDir(t)
	if err := os.WriteFile(filepath.Join(dir, "cycle-12.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ReadDir(dir, 1)

	if len(got) != 1 || got[0].Cycle != 11 {
		t.Errorf("ReadDir = %+v, want only cycle 11", got)
	}
}

func TestReadDirChecked_ReturnsCyclesInNumericOrderNotNameOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCorpusDossier(t, root, 9, nil)
	writeCorpusDossier(t, root, 10, nil)

	got, failures := ReadDirChecked(CyclesDir(root), 1, 0)

	if len(failures) != 0 {
		t.Fatalf("failures = %+v, want none", failures)
	}
	if len(got) != 2 || got[0].Cycle != 9 || got[1].Cycle != 10 {
		t.Errorf("dossiers = %+v, want cycle 9 and then cycle 10: the name cycle-10.json sorts before cycle-9.json", got)
	}
}

func TestReadDirChecked_ACeilingExcludesTheCorruptFileAboveIt(t *testing.T) {
	t.Parallel()
	dir := checkedDir(t)
	above := filepath.Join(dir, "cycle-90.json")
	if err := os.WriteFile(above, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, failures := ReadDirChecked(dir, 1, 20)

	if len(failures) != 0 {
		t.Errorf("failures = %+v, want none: cycle 90 is above the ceiling 20", failures)
	}
	if len(got) != 1 || got[0].Cycle != 11 {
		t.Errorf("dossiers = %+v, want only cycle 11", got)
	}
}

func TestReadDirChecked_ACeilingStillReportsACorruptFileInsideIt(t *testing.T) {
	t.Parallel()
	dir := checkedDir(t)
	inside := filepath.Join(dir, "cycle-12.json")
	if err := os.WriteFile(inside, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, failures := ReadDirChecked(dir, 1, 12)

	wantReadFailure(t, failures, inside)
}
