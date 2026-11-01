package shipjournal

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEntry_TheStampOfTheWriterIsTheTimeOfTheReader(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 27, 4, 4, 2, 0, time.FixedZone("x", 8*3600))
	e := Entry{SHA: "4fc4588", Class: ClassCycle, Cycle: 2038}.Stamped(at)
	got, ok := e.Time()
	if e.TS != "2026-10-26T20:04:02Z" || TimeFormat != time.RFC3339 || !ok || !got.Equal(at) {
		t.Fatalf("stamp %q, time %v %v", e.TS, got, ok)
	}
	if _, ok := (Entry{SHA: "b", Class: ClassCycle}).Time(); ok {
		t.Error("an entry with no stamp has no time")
	}
}

func TestPath_IsTheJournalUnderTheEvolveDirectory(t *testing.T) {
	t.Parallel()
	if got := Path("/p/.evolve"); FileName != "ship-journal.jsonl" || got != filepath.Join("/p/.evolve", FileName) {
		t.Fatalf("Path = %s", got)
	}
}
