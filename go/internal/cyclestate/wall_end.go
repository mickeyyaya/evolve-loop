package cyclestate

import (
	"encoding/json"
	"time"
)

type WallEnd struct {
	CLI      string    `json:"cli"`
	Pattern  string    `json:"pattern,omitempty"`
	ResetAt  time.Time `json:"reset_at"`
	Evidence string    `json:"evidence,omitempty"`
}

type wallEndWire struct {
	CLI      string     `json:"cli"`
	Pattern  string     `json:"pattern,omitempty"`
	ResetAt  *time.Time `json:"reset_at,omitempty"`
	Evidence string     `json:"evidence,omitempty"`
}

func (e WallEnd) MarshalJSON() ([]byte, error) {
	wire := wallEndWire{CLI: e.CLI, Pattern: e.Pattern, Evidence: e.Evidence}
	if !e.ResetAt.IsZero() {
		wire.ResetAt = &e.ResetAt
	}
	return json.Marshal(wire)
}
