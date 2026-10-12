package gc

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func sampleArchiveRecord(path string) DevArchiveRecord {
	return DevArchiveRecord{
		Path: path, Branch: "br-" + filepath.Base(path), Head: "0123456789abcdef0123456789abcdef01234567",
		Checkpoint: "refs/checkpoints/" + filepath.Base(path) + "/20261012T010203Z", IdleDays: 21.5,
		ArchivedAt: time.Date(2026, 10, 12, 1, 2, 3, 0, time.UTC),
	}
}

func TestDevArchive_AppendedRecordsReadBackInOrderAndExactly(t *testing.T) {
	evolve := filepath.Join(t.TempDir(), ".evolve")
	first, second := sampleArchiveRecord("/hub/dev/one"), sampleArchiveRecord("/hub/dev/two")
	second.Checkpoint = ""
	for _, rec := range []DevArchiveRecord{first, second} {
		if err := AppendDevArchive(evolve, rec); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadDevArchive(evolve)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []DevArchiveRecord{first, second}) {
		t.Fatalf("read back %+v, want %+v", got, []DevArchiveRecord{first, second})
	}
	raw, err := os.ReadFile(DevArchivePath(evolve))
	if err != nil || strings.Count(string(raw), "\n") != 2 {
		t.Fatalf("manifest = %q (err %v), want exactly two newline-terminated records", raw, err)
	}
	if filepath.Dir(DevArchivePath(evolve)) != filepath.Join(evolve, "archive") {
		t.Errorf("manifest at %s, want it under %s/archive", DevArchivePath(evolve), evolve)
	}
}

func TestReadDevArchive_AMissingManifestIsEmptyAndABadLineNamesItsNumber(t *testing.T) {
	evolve := t.TempDir()
	if recs, err := ReadDevArchive(evolve); err != nil || recs != nil {
		t.Fatalf("missing manifest: %v, %v; want nil, nil", recs, err)
	}
	if err := AppendDevArchive(evolve, sampleArchiveRecord("/hub/dev/ok")); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(DevArchivePath(evolve), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("\n{not json}\n")
	f.Close()
	_, err = ReadDevArchive(evolve)
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("err = %v, want a failure that names line 3 (a blank line 2 is skipped but counted)", err)
	}
}

func TestDevArchive_IOFailuresSurface(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendDevArchive(blocker, sampleArchiveRecord("/p")); err == nil {
		t.Error("append under a file: nil error, want the mkdir failure")
	}
	if _, err := ReadDevArchive(blocker); err == nil {
		t.Error("read under a file: nil error, want the open failure")
	}
	asDir := t.TempDir()
	if err := os.MkdirAll(DevArchivePath(asDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := AppendDevArchive(asDir, sampleArchiveRecord("/p")); err == nil {
		t.Error("append onto a directory: nil error, want the open failure")
	}
	if _, err := ReadDevArchive(asDir); err == nil {
		t.Error("read of a directory: nil error, want the read failure")
	}
	nan := sampleArchiveRecord("/p")
	nan.IdleDays = math.NaN()
	if err := AppendDevArchive(t.TempDir(), nan); err == nil {
		t.Error("append of an unencodable record: nil error, want the encode failure")
	}
}

func TestDevArchiveRecord_RestoreCommandsRebuildTheTreeThenTheWork(t *testing.T) {
	rec := sampleArchiveRecord("/hub/dev/cl-x")
	want := []string{
		"git worktree add /hub/dev/cl-x br-cl-x",
		"evolve checkpoint restore cl-x/20261012T010203Z --into /hub/dev/cl-x",
	}
	if got := rec.RestoreCommands(); !reflect.DeepEqual(got, want) {
		t.Errorf("RestoreCommands = %q, want %q", got, want)
	}
	rec.Checkpoint = ""
	if got := rec.RestoreCommands(); !reflect.DeepEqual(got, want[:1]) {
		t.Errorf("a clean archive: RestoreCommands = %q, want only the worktree add", got)
	}
}
