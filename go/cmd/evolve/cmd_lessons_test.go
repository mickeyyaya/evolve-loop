package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// TestLessonsRecurrence_SortedByCountWithFixStatus (AC4): `evolve lessons
// recurrence` lists patterns sorted by descending count, each row carrying its
// fix-item status.
func TestLessonsRecurrence_SortedByCountWithFixStatus(t *testing.T) {
	led := recurrence.NewLedger()
	led.Entries["rare"] = &recurrence.Entry{Pattern: "rare", Count: 2}
	led.Entries["frequent"] = &recurrence.Entry{Pattern: "frequent", Count: 7, FixItemID: "frequent-fix"}
	led.Entries["mid"] = &recurrence.Entry{Pattern: "mid", Count: 4}

	var buf bytes.Buffer
	renderRecurrenceReport(&buf, led)
	out := buf.String()

	iFreq := strings.Index(out, "frequent")
	iMid := strings.Index(out, "mid")
	iRare := strings.Index(out, "rare")
	if iFreq < 0 || iMid < 0 || iRare < 0 {
		t.Fatalf("missing a pattern row:\n%s", out)
	}
	if !(iFreq < iMid && iMid < iRare) {
		t.Fatalf("rows not sorted by descending count:\n%s", out)
	}
	if !strings.Contains(out, "fix=frequent-fix") {
		t.Fatalf("fix-item status missing for linked pattern:\n%s", out)
	}
	if !strings.Contains(out, "fix=none") {
		t.Fatalf("fix=none status missing for unlinked pattern:\n%s", out)
	}
}
