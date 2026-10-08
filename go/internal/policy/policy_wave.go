package policy

const (
	DefaultWaveMaxCycles      = 1
	DefaultWaveHistoryK       = 3
	DefaultWaveStandingGoal   = ".evolve/wave-goal.md"
	DefaultWaveFactsMaxCycles = 20
)

type WavePolicy struct {
	MaxCycles      int      `json:"max_cycles,omitempty"`
	HistoryK       int      `json:"history_k,omitempty"`
	StandingGoal   string   `json:"standing_goal,omitempty"`
	HealthDrivers  []string `json:"health_drivers,omitempty"`
	FactsMaxCycles int      `json:"facts_max_cycles,omitempty"`
}

type WaveConfig struct {
	MaxCycles      int
	HistoryK       int
	StandingGoal   string
	HealthDrivers  []string
	FactsMaxCycles int
}

func DefaultWaveHealthDrivers() []string { return []string{"claude-tmux"} }

func (p Policy) WaveSettings() WaveConfig {
	c := WaveConfig{
		MaxCycles: DefaultWaveMaxCycles, HistoryK: DefaultWaveHistoryK,
		StandingGoal: DefaultWaveStandingGoal, HealthDrivers: DefaultWaveHealthDrivers(),
		FactsMaxCycles: DefaultWaveFactsMaxCycles,
	}
	if p.Wave == nil {
		return c
	}
	if p.Wave.MaxCycles > 0 {
		c.MaxCycles = p.Wave.MaxCycles
	}
	if p.Wave.HistoryK > 0 {
		c.HistoryK = p.Wave.HistoryK
	}
	if p.Wave.FactsMaxCycles > 0 {
		c.FactsMaxCycles = p.Wave.FactsMaxCycles
	}
	if p.Wave.StandingGoal != "" {
		c.StandingGoal = p.Wave.StandingGoal
	}
	if p.Wave.HealthDrivers != nil {
		c.HealthDrivers = append([]string{}, p.Wave.HealthDrivers...)
	}
	return c
}
