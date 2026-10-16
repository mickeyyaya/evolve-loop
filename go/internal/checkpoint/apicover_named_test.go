package checkpoint

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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
