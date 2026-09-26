package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// Domain is the .evolve/domain.json project record; only Domain is consumed, the other fields round-trip.
type Domain struct {
	Domain         string `json:"domain"`
	EvalMode       string `json:"evalMode,omitempty"`
	ShipMechanism  string `json:"shipMechanism,omitempty"`
	BuildIsolation string `json:"buildIsolation,omitempty"`
}

// LoadDomain reads <projectRoot>/.evolve/domain.json: absent is ok=false with no error, while an
// unreadable or unparseable file is an error so a malformed writing project never becomes a code project.
func LoadDomain(projectRoot string) (Domain, bool, error) {
	path := filepath.Join(paths.EvolveDirOf(projectRoot), "domain.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Domain{}, false, nil
	}
	if err != nil {
		return Domain{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	var d Domain
	if err := json.Unmarshal(raw, &d); err != nil {
		return Domain{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return d, true, nil
}

// DefaultDeliverableKind is the kind a task inherits when it declares none: document for writing
// and research projects, otherwise code, the side that keeps the tdd pin.
func (d Domain) DefaultDeliverableKind() string {
	switch d.Domain {
	case "writing", "research":
		return DeliverableKindDocument
	}
	return DeliverableKindCode
}
