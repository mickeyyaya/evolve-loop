package attemptpostmortem

import (
	"fmt"
	"time"
)

type Config struct {
	MaxCommands      int
	MaxCommandRunes  int
	MaxPaneTailRunes int
	MaxDeltaRunes    int
	SuspectWindow    time.Duration
}

func DefaultConfig() Config {
	return Config{
		MaxCommands:      8,
		MaxCommandRunes:  400,
		MaxPaneTailRunes: 2000,
		MaxDeltaRunes:    1500,
		SuspectWindow:    30 * time.Second,
	}
}

func (c Config) Validate() error {
	limits := []struct {
		name  string
		value int64
	}{
		{"MaxCommands", int64(c.MaxCommands)},
		{"MaxCommandRunes", int64(c.MaxCommandRunes)},
		{"MaxPaneTailRunes", int64(c.MaxPaneTailRunes)},
		{"MaxDeltaRunes", int64(c.MaxDeltaRunes)},
		{"SuspectWindow", int64(c.SuspectWindow)},
	}
	for _, l := range limits {
		if l.value <= 0 {
			return fmt.Errorf("attemptpostmortem: config %s must be positive, got %d", l.name, l.value)
		}
	}
	return nil
}
