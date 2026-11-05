package shipjournal

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJournal(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRead_ReturnsEachEntryInFileOrderAndSkipsBlankLines(t *testing.T) {
	t.Parallel()
	path := writeJournal(t, `{"sha":"a","class":"cycle","ts":"2026-11-01T00:00:00Z","cycle":1}`+"\n\n"+`{"sha":"b","class":"manual","ts":"2026-11-01T00:00:01Z"}`+"\n")
	j, err := Read(path)
	if err != nil || j.Path != path || len(j.Entries) != 2 || j.Entries[0].SHA != "a" || j.Entries[0].Cycle != 1 || j.Entries[1].Class != "manual" || len(j.Malformed) != 0 {
		t.Fatalf("journal %+v, err %v", j, err)
	}
	if j.Strict() != nil {
		t.Errorf("a clean journal is strict-clean, got %v", j.Strict())
	}
}

func TestRead_AMalformedLineIsKeptApartWithItsLineNumberAndTheGoodLinesStillRead(t *testing.T) {
	t.Parallel()
	path := writeJournal(t, `{"sha":"a"}`+"\nnot json\n"+`{"sha":"c"}`+"\n")
	j, err := Read(path)
	if err != nil || len(j.Entries) != 2 || len(j.Malformed) != 1 || j.Malformed[0].Line != 2 || j.Malformed[0].Err == nil {
		t.Fatalf("journal %+v, err %v", j, err)
	}
	strict := j.Strict()
	if strict == nil || !strings.Contains(strict.Error(), path+" line 2:") {
		t.Fatalf("strict = %v, want the path and line 2", strict)
	}
	if !errors.Is(strict, j.Malformed[0].Err) {
		t.Errorf("strict must wrap the cause of the line: %v", strict)
	}
}

func TestRead_AnAbsentJournalIsEmptyAndAnUnreadableOneIsAnError(t *testing.T) {
	t.Parallel()
	j, err := Read(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(j.Entries) != 0 || len(j.Malformed) != 0 {
		t.Fatalf("absent journal: %+v, %v", j, err)
	}
	dir := t.TempDir()
	if _, err := Read(dir); err == nil || errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), dir) {
		t.Fatalf("a directory is unreadable and names its path, got %v", err)
	}
}

func TestJournal_HoldsItsPathAndALineErrorForEachMalformedLine(t *testing.T) {
	t.Parallel()
	path := writeJournal(t, "not json\n")
	var got Journal
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read = %v", err)
	}
	if got.Path != path || len(got.Malformed) != 1 {
		t.Fatalf("journal = %+v, want its path and one malformed line", got)
	}
	var first LineError = got.Malformed[0]
	if first.Line != 1 || first.Err == nil {
		t.Errorf("line error = %+v, want line 1 with its error", first)
	}
}
