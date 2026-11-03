package llmcalls

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImportTagged_MalformedSourceLinesAreCountedAndNotImported(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.ndjson")
	destination := filepath.Join(dir, "destination.ndjson")
	writeLines(t, source, `not json`, `{"call_id":"ok","phase":"build"}`)

	result, err := ImportTagged(destination, source, "run-x", "prof")

	if err != nil {
		t.Fatalf("ImportTagged: %v", err)
	}
	if result != (ImportResult{Imported: 1, Skipped: 1}) {
		t.Fatalf("result = %+v, want one imported and one skipped", result)
	}
}

func TestImportWith_ARewriteFailureStopsTheImportAndIsReturned(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.ndjson")
	destination := filepath.Join(dir, "destination.ndjson")
	writeLines(t, source, `{"call_id":"a","phase":"build"}`, `{"call_id":"b","phase":"build"}`)
	boom := errors.New("rewrite boom")

	result, err := importWith(destination, source, func(Record, []byte) ([]byte, error) { return nil, boom })

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the rewrite error", err)
	}
	if result.Imported != 0 {
		t.Fatalf("imported %d rows despite the rewrite failure", result.Imported)
	}
	if _, statErr := os.Stat(destination); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination exists after a failed rewrite (err=%v)", statErr)
	}
}

func TestImportTagged_AMissingSourceIsAnError(t *testing.T) {
	dir := t.TempDir()

	_, err := ImportTagged(filepath.Join(dir, "d.ndjson"), filepath.Join(dir, "missing.ndjson"), "r", "p")

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want a not-exist error", err)
	}
}

func TestImportTagged_AnUnreadableDestinationIsAnError(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.ndjson")
	destination := filepath.Join(dir, "destination.ndjson")
	writeLines(t, source, `{"call_id":"a","phase":"build"}`)
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := ImportTagged(destination, source, "r", "p"); err == nil {
		t.Fatal("a directory in place of the ledger must fail the import")
	}
}
