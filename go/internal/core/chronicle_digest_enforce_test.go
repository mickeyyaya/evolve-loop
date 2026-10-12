package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestSeedChronicleDigest_EnforceWithNoHistoryInjectsNothing(t *testing.T) {
	t.Parallel()

	cfg := policy.Policy{}.ChronicleConfig()
	cfg.Digest = "enforce"
	ctxSnap := map[string]string{}
	var sink injectedStderr

	seedChronicleDigest(t.TempDir(), CycleState{WorkspacePath: t.TempDir()}, State{}, cfg, ctxSnap, &sink)

	if _, injected := ctxSnap["recent_outcomes"]; injected {
		t.Fatalf("an empty history writes no digest, so enforce has nothing to carry into the phases; got %v", ctxSnap)
	}
	if got := sink.String(); got != "" {
		t.Fatalf("an empty history is not a failure and must stay silent; stderr was:\n%q", got)
	}
}
