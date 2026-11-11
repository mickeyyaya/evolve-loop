package checkpoint

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestResumableReasonsAreValidCheckpointReasons(t *testing.T) {
	resumable := core.ResumableReasons()
	if len(resumable) == 0 {
		t.Fatal("core.ResumableReasons() is empty; resume discovery would accept no checkpoint")
	}
	for _, reason := range resumable {
		if !Reason(reason).IsValid() {
			t.Errorf("resumable reason %q is not a valid checkpoint reason", reason)
		}
		if !core.IsResumableReason(reason) {
			t.Errorf("core.ResumableReasons() lists %q but core.IsResumableReason rejects it", reason)
		}
	}
}

func TestSettledCheckpointReasonsAreNotResumable(t *testing.T) {
	for _, reason := range []Reason{ReasonPhaseComplete, ReasonRetired} {
		if core.IsResumableReason(string(reason)) {
			t.Errorf("settled reason %q must not be resumable", reason)
		}
	}
}
