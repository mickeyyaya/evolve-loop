package main

import (
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestWireOrchestratorDeps_TheIntentRunnerReadsDeltaModeFromThePolicy(t *testing.T) {
	for policyJSON, want := range map[string]string{"": "- mode: full", `{"workflow":{"intent_delta":true}}`: "- mode: delta"} {
		root, evolveDir := wiringRoot(t, policyJSON)
		d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
		composer, ok := d.Runners[core.PhaseIntent].(interface {
			ComposePrompt(string, core.PhaseRequest) string
		})
		if !ok {
			t.Fatal("the cycle root's intent runner must compose its prompt")
		}
		if got := composer.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: root}); !strings.Contains(got, want) {
			t.Errorf("policy %q: prompt = %q, want %q", policyJSON, got, want)
		}
		d.CloseSignals()
	}
}
