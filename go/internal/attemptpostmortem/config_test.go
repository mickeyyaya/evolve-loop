package attemptpostmortem

import (
	"strings"
	"testing"
)

func TestDefaultConfig_IsValidAndBoundsEveryCap(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() = %v, want nil", err)
	}
	if cfg.MaxCommands != 8 || cfg.MaxCommandRunes != 400 || cfg.MaxPaneTailRunes != 2000 || cfg.MaxDeltaRunes != 1500 || cfg.SuspectWindow.Seconds() != 30 {
		t.Fatalf("DefaultConfig() = %+v, want 8/400/2000/1500/30s", cfg)
	}
}

func TestConfigValidate_RefusesEachCapThatIsNotPositive(t *testing.T) {
	cases := map[string]func(*Config){
		"MaxCommands":      func(c *Config) { c.MaxCommands = 0 },
		"MaxCommandRunes":  func(c *Config) { c.MaxCommandRunes = -1 },
		"MaxPaneTailRunes": func(c *Config) { c.MaxPaneTailRunes = 0 },
		"MaxDeltaRunes":    func(c *Config) { c.MaxDeltaRunes = 0 },
		"SuspectWindow":    func(c *Config) { c.SuspectWindow = 0 },
	}
	for field, mutate := range cases {
		t.Run(field, func(t *testing.T) {
			cfg := DefaultConfig()
			mutate(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), field) {
				t.Fatalf("Validate() = %v, want an error that names %s", err, field)
			}
		})
	}
}
