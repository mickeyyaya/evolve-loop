package audit

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Test 44 — the tail constructor accepts functional options (ADR-0103 unit
// 09): WithSignals reaches Config.Signals; zero options is today's phase. The
// two exports Option and WithSignals are named here for the apicover gate.
func TestNewDefaultWithStageCompactSpec_AcceptsOptions(t *testing.T) {
	var applied Config
	capture := Option(func(c *Config) { applied = *c })
	acc := func() *signalcenter.Center { return signalcenter.New() }
	if phase := NewDefaultWithStageCompactSpec(&fakeBridge{writeArtifact: "x"}, fakePromptsFS("body"), config.StageOff, true, nil, WithSignals(acc), capture); phase == nil || phase.BaseRunner == nil {
		t.Fatal("the phase is built with options")
	}
	if applied.Signals == nil || applied.Signals() == nil || !applied.CompactPrompts || applied.GenerateVerdict == nil {
		t.Fatalf("WithSignals reaches Config.Signals beside the production defaults: %+v", applied)
	}
	if phase := NewDefaultWithStageCompactSpec(&fakeBridge{writeArtifact: "x"}, fakePromptsFS("body"), config.StageOff, false, nil); phase == nil {
		t.Fatal("zero options is today's phase")
	}
}
