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
	MaxRecords       int
}

func DefaultConfig() Config {
	return Config{
		MaxCommands:      8,
		MaxCommandRunes:  400,
		MaxPaneTailRunes: 2000,
		MaxDeltaRunes:    1500,
		SuspectWindow:    30 * time.Second,
		MaxRecords:       3,
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
		{"MaxRecords", int64(c.MaxRecords)},
	}
	for _, l := range limits {
		if l.value <= 0 {
			return fmt.Errorf("attemptpostmortem: config %s must be positive, got %d", l.name, l.value)
		}
	}
	return nil
}
