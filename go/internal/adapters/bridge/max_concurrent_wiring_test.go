package bridge

import (
	"maps"
	"testing"
)

func TestNewDefault_ProductionDepsCarryThePolicyAdmissionCaps(t *testing.T) {
	dir := t.TempDir()
	writePolicyJson(t, dir, `{"bridge":{"max_concurrent":{"claude-tmux":2,"codex-tmux":1}}}`)
	got := NewDefault(dir, nil).productionEngineDeps(nil).MaxConcurrent
	if want := map[string]int{"claude-tmux": 2, "codex-tmux": 1}; !maps.Equal(got, want) {
		t.Errorf("MaxConcurrent = %v, want %v from bridge.max_concurrent", got, want)
	}
}

func TestNewDefault_TheSandboxModeComesFromThePolicyAndOffOnlyFromTheBypass(t *testing.T) {
	dir := t.TempDir()
	writePolicyJson(t, dir, `{"sandbox":{"mode":"on"}}`)
	t.Setenv("EVOLVE_SANDBOX", "off")
	a := NewDefault(dir, nil)
	if got := a.productionEngineDeps(nil).SandboxMode; got != "on" {
		t.Errorf("SandboxMode = %q, want on from sandbox.mode", got)
	}
	a.SetSandboxOff()
	if got := a.productionEngineDeps(nil).SandboxMode; got != "off" {
		t.Errorf("SandboxMode = %q, want off after SetSandboxOff", got)
	}
}
