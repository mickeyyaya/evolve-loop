package ship

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadShipJournal_SkipsBadLinesAndEmptySHAsAndTheLastEntryOfASHAWins(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := shipJournalPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"sha":"a","class":"cycle","ts":"2026-11-01T00:00:00Z","cycle":1}` + "\n" +
		"not json\n" +
		`{"sha":"","class":"cycle"}` + "\n" +
		`{"sha":"a","class":"manual","ts":"2026-11-01T00:00:05Z"}` + "\n" +
		`{"sha":"b","class":"cycle","ts":"bad","cycle":2}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readShipJournal(root)
	if len(got) != 2 || got["a"].Class != "manual" || got["b"].Cycle != 2 {
		t.Fatalf("journal = %+v", got)
	}
	if empty := readShipJournal(t.TempDir()); len(empty) != 0 {
		t.Fatalf("an absent journal reads as empty, got %+v", empty)
	}
}
