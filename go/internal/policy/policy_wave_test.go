package policy

import (
	"reflect"
	"testing"
)

func TestPolicyWave_DefaultsAndOverrides(t *testing.T) {
	t.Parallel()
	defaults := WaveConfig{
		MaxCycles: DefaultWaveMaxCycles, HistoryK: DefaultWaveHistoryK,
		StandingGoal: DefaultWaveStandingGoal, HealthDrivers: DefaultWaveHealthDrivers(), FactsMaxCycles: DefaultWaveFactsMaxCycles,
	}
	cases := []struct {
		name, json string
		want       WaveConfig
	}{
		{"absent block is the compiled default", `{}`, defaults},
		{"empty block is the compiled default", `{"wave":{}}`, defaults},
		{"non-positive numbers fall back to the default", `{"wave":{"max_cycles":-2,"history_k":0}}`, defaults},
		{"every key overrides", `{"wave":{"max_cycles":4,"history_k":7,"standing_goal":"ops/goal.md","health_drivers":["agy-tmux","claude-tmux"],"facts_max_cycles":9}}`,
			WaveConfig{MaxCycles: 4, HistoryK: 7, StandingGoal: "ops/goal.md", HealthDrivers: []string{"agy-tmux", "claude-tmux"}, FactsMaxCycles: 9}},
		{"an explicit empty driver list turns the health check off", `{"wave":{"health_drivers":[]}}`,
			WaveConfig{MaxCycles: DefaultWaveMaxCycles, HistoryK: DefaultWaveHistoryK, StandingGoal: DefaultWaveStandingGoal, HealthDrivers: []string{}, FactsMaxCycles: DefaultWaveFactsMaxCycles}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := loadCIWatchPolicy(t, c.json).WaveSettings()
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("WaveSettings() = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestDefaultWaveHealthDrivers_IsClaudeTmuxAndACopy(t *testing.T) {
	t.Parallel()
	first := DefaultWaveHealthDrivers()
	if !reflect.DeepEqual(first, []string{"claude-tmux"}) {
		t.Fatalf("DefaultWaveHealthDrivers() = %q, want [claude-tmux]", first)
	}
	first[0] = "changed"
	if got := DefaultWaveHealthDrivers()[0]; got != "claude-tmux" {
		t.Errorf("a caller changed the compiled default to %q", got)
	}
}

func TestPolicyWave_ALiteralBlockResolves(t *testing.T) {
	t.Parallel()
	got := (Policy{Wave: &WavePolicy{MaxCycles: 5, StandingGoal: "/abs/goal.md"}}).WaveSettings()

	want := WaveConfig{MaxCycles: 5, HistoryK: DefaultWaveHistoryK, StandingGoal: "/abs/goal.md", HealthDrivers: DefaultWaveHealthDrivers(), FactsMaxCycles: DefaultWaveFactsMaxCycles}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WaveSettings() = %+v, want %+v", got, want)
	}
}
