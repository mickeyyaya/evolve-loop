package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

func TestC662_EscalateRetroReasonIgnoresGenericPatterns(t *testing.T) {
	const reason = "proceed: no failures requiring adaptation"

	genericLed := recurrence.NewLedger()
	genericLed.Entries["operator-reset"] = &recurrence.Entry{
		Pattern: "operator-reset", Cycles: []int{1, 2}, Count: 2, Generic: true,
	}
	if got := escalateRetroReason(reason, "operator-reset", genericLed); got != reason {
		t.Errorf("generic pattern at count=2 was escalated:\n got  %q\n want %q (unchanged — generic noise must not escalate)", got, reason)
	}

	realLed := recurrence.NewLedger()
	realLed.Entries["builder-out-of-lane-ships-red"] = &recurrence.Entry{
		Pattern: "builder-out-of-lane-ships-red", Cycles: []int{1, 2}, Count: 2, Generic: false,
	}
	got := escalateRetroReason(reason, "builder-out-of-lane-ships-red", realLed)
	if !strings.HasPrefix(got, "adapt:") {
		t.Errorf("non-generic pattern at count=2 did not escalate:\n got %q\n want an 'adapt:' escalation", got)
	}
}
