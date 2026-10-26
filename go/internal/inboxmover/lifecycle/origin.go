package lifecycle

import (
	"encoding/json"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func WithOrigin(origin string) Option {
	return func(m *Mover) {
		if origin != "" {
			m.origin = origin
		}
	}
}

func WithIntakeRecord(record json.RawMessage) Option {
	return func(m *Mover) {
		if len(record) > 0 {
			m.intakeRecord = record
		}
	}
}

var filingStamps = map[string]bool{inboxbatch.IntakeOverrideField: true}

func (m *Mover) stampIntake(fields map[string]json.RawMessage, item inboxbatch.Item) error {
	origin := item.Origin
	if origin == "" {
		origin = m.origin
	}
	if err := inboxbatch.CheckOrigin(origin); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidItem, err)
	}
	fields["origin"] = jsonString(origin)
	if len(m.intakeRecord) > 0 {
		fields[inboxbatch.IntakeOverrideField] = m.intakeRecord
	}
	return nil
}
