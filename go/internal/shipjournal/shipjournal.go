package shipjournal

import (
	"path/filepath"
	"time"
)

const (
	FileName   = "ship-journal.jsonl"
	ClassCycle = "cycle"
	TimeFormat = time.RFC3339
)

type Entry struct {
	SHA   string `json:"sha"`
	Class string `json:"class"`
	TS    string `json:"ts"`
	Cycle int    `json:"cycle,omitempty"`
}

func Path(evolveDir string) string { return filepath.Join(evolveDir, FileName) }

func (e Entry) Stamped(at time.Time) Entry {
	e.TS = at.UTC().Format(TimeFormat)
	return e
}

func (e Entry) Time() (time.Time, bool) {
	at, err := time.Parse(TimeFormat, e.TS)
	return at, err == nil
}
