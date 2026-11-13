package policy

import (
	"errors"
	"time"
)

const (
	defaultExpectedWaveSeconds  = 57600
	defaultExpectedPhaseSeconds = 5400
)

type ExpectedSignalPolicy struct {
	WaveSeconds  int `json:"wave_seconds,omitempty"`
	PhaseSeconds int `json:"phase_seconds,omitempty"`
}

func (e *ExpectedSignalPolicy) validate() error {
	switch {
	case e == nil:
		return nil
	case e.WaveSeconds < 0:
		return errors.New("wave_seconds must not be negative")
	case e.PhaseSeconds < 0:
		return errors.New("phase_seconds must not be negative")
	}
	return nil
}

func (e *ExpectedSignalPolicy) Deadline(key string) (time.Duration, bool) {
	var set int
	var fallback int
	switch key {
	case "expected_signal.wave_seconds":
		fallback = defaultExpectedWaveSeconds
		if e != nil {
			set = e.WaveSeconds
		}
	case "expected_signal.phase_seconds":
		fallback = defaultExpectedPhaseSeconds
		if e != nil {
			set = e.PhaseSeconds
		}
	default:
		return 0, false
	}
	if set > 0 {
		fallback = set
	}
	return time.Duration(fallback) * time.Second, true
}
