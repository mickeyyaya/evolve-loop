package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

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
