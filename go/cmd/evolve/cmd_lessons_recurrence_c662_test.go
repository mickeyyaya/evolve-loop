package main

// cmd_lessons_recurrence_c662_test.go — cycle-662 RED contract for
// chronicle-s1-recurrence-index AC5: `evolve lessons recurrence` must surface the
// backfilled NON-generic top patterns, not the operator-reset/loop-fatal noise
// floor that dominates raw counts. This pins the render seam: Generic entries are
// excluded from the report so a de-noised, actionable top-N is what an operator
// sees.
//
// Builder contract: renderRecurrenceReport must skip entries whose Generic flag
// is set (they are classification noise), while keeping specific-defect patterns.
//
// RED today: recurrence.Entry has no Generic field, so package main fails to
// compile. GREEN once Builder adds Entry.Generic and filters the render.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// TestC662_RenderExcludesGenericPatterns — AC5. A generic pattern with a huge
// count must NOT appear in the report; a lower-count specific defect must.
func TestC662_RenderExcludesGenericPatterns(t *testing.T) {
	led := recurrence.NewLedger()
	led.Entries["operator-reset"] = &recurrence.Entry{Pattern: "operator-reset", Count: 96, Generic: true}
	led.Entries["loop-fatal"] = &recurrence.Entry{Pattern: "loop-fatal", Count: 62, Generic: true}
	led.Entries["builder-out-of-lane-ships-red"] = &recurrence.Entry{
		Pattern: "builder-out-of-lane-ships-red", Count: 3, Generic: false,
	}

	var buf bytes.Buffer
	renderRecurrenceReport(&buf, led)
	out := buf.String()

	if !strings.Contains(out, "builder-out-of-lane-ships-red") {
		t.Errorf("report omits the non-generic top pattern:\n%s", out)
	}
	if strings.Contains(out, "operator-reset") {
		t.Errorf("report includes generic noise operator-reset (must be de-noised):\n%s", out)
	}
	if strings.Contains(out, "loop-fatal") {
		t.Errorf("report includes generic noise loop-fatal (must be de-noised):\n%s", out)
	}
}
