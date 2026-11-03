package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurefp"
)

func TestFailureIdentity_BreakerAndEscalationUseTheOneSharedNormalizer(t *testing.T) {
	t.Parallel()

	for _, reason := range []string{measuredReasonCycle2071, measuredReasonCycle2049, measuredReasonCycle2073, "  padded narrative=FAIL \n"} {
		want := failurefp.Normalize(reason)
		if got := newContractBlockIdentity(reason).normalized; got != want {
			t.Errorf("contract identity normalized %q, want %q", got, want)
		}
		if fingerprint("build", "gate-block", []string{reason}) != fingerprint("build", "gate-block", []string{want}) {
			t.Errorf("breaker fingerprint of %q differs from the fingerprint of its shared normal form", reason)
		}
	}
}
