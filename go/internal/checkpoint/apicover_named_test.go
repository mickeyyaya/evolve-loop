package checkpoint

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestDefaultAutoResumeAttempts_IsComposedDefault pins DefaultAutoResumeAttempts
// to its real consumer: Compose stamps it onto Checkpoint.AutoResumeMaxAttempts.
// If the const drifts from what Compose writes, the cap the auto-resume layer
// reads back would silently change — this asserts they stay the same value
// (the load-bearing contract) and that the default is a positive attempt budget.
func TestDefaultAutoResumeAttempts_IsComposedDefault(t *testing.T) {
	cp := Compose(core.CycleState{CycleID: 1, Phase: "build"}, ReasonBatchCapNear, 0, "", fixedTime())
	if cp.AutoResumeMaxAttempts != DefaultAutoResumeAttempts {
		t.Errorf("Compose AutoResumeMaxAttempts=%d, want DefaultAutoResumeAttempts=%d",
			cp.AutoResumeMaxAttempts, DefaultAutoResumeAttempts)
	}
	if DefaultAutoResumeAttempts <= 0 {
		t.Errorf("DefaultAutoResumeAttempts=%d must be a positive attempt budget", DefaultAutoResumeAttempts)
	}
}
