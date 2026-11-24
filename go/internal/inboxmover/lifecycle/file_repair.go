package lifecycle

import (
	"encoding/json"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

type RepairStamp struct {
	Tier           string
	ProtectedFiles []string
}

func (s RepairStamp) isZero() bool { return s.Tier == "" && len(s.ProtectedFiles) == 0 }

func (m *Mover) FileRepair(raw []byte, stamp RepairStamp) (FileResult, error) {
	if stamp.isZero() || stamp.invalidTier() {
		return FileResult{}, fmt.Errorf("%w: a repair item is stamped T1 or T2, or carries its protected files; T0 is derived from them", ErrInvalidItem)
	}
	return m.file(raw, stamp)
}

func (s RepairStamp) apply(fields map[string]json.RawMessage) {
	if s.isZero() {
		return
	}
	fields[RouteField] = jsonString(inboxbatch.RouteConsoleAutoValue)
	if s.Tier != "" {
		fields[inboxbatch.TierField] = jsonString(s.Tier)
	}
	if len(s.ProtectedFiles) > 0 {
		fields[inboxbatch.ProtectedFilesField] = jsonStrings(s.ProtectedFiles)
	}
}

func (s RepairStamp) invalidTier() bool {
	return s.Tier != "" && inboxbatch.RepairTierOf(s.Tier, nil) == ""
}
