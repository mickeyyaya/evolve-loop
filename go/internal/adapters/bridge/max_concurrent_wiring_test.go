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
