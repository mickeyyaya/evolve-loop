package policy

import (
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
)

type AttemptPostmortemPolicy struct {
	MaxCommands      *int `json:"max_commands,omitempty"`
	MaxCommandRunes  *int `json:"max_command_runes,omitempty"`
	MaxPaneTailRunes *int `json:"max_pane_tail_runes,omitempty"`
	MaxDeltaRunes    *int `json:"max_delta_runes,omitempty"`
	SuspectWindowS   *int `json:"suspect_window_s,omitempty"`
	MaxRecords       *int `json:"max_records,omitempty"`
}

func (b *AttemptPostmortemPolicy) UnmarshalJSON(raw []byte) error {
	type block AttemptPostmortemPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("attempt_postmortem: %w", err)
	}
	parsed := AttemptPostmortemPolicy(decoded)
	if err := parsed.config().Validate(); err != nil {
		return fmt.Errorf("attempt_postmortem: %w", err)
	}
	*b = parsed
	return nil
}

func (p Policy) AttemptPostmortemConfig() attemptpostmortem.Config {
	if p.AttemptPostmortem == nil {
		return attemptpostmortem.DefaultConfig()
	}
	return p.AttemptPostmortem.config()
}

func (b AttemptPostmortemPolicy) config() attemptpostmortem.Config {
	cfg := attemptpostmortem.DefaultConfig()
	window := int(cfg.SuspectWindow / time.Second)
	for _, knob := range []struct{ from, to *int }{
		{b.MaxCommands, &cfg.MaxCommands},
		{b.MaxCommandRunes, &cfg.MaxCommandRunes},
		{b.MaxPaneTailRunes, &cfg.MaxPaneTailRunes},
		{b.MaxDeltaRunes, &cfg.MaxDeltaRunes},
		{b.SuspectWindowS, &window},
		{b.MaxRecords, &cfg.MaxRecords},
	} {
		if knob.from != nil {
			*knob.to = *knob.from
		}
	}
	cfg.SuspectWindow = time.Duration(window) * time.Second
	return cfg
}
