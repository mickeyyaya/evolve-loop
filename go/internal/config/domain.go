package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Domain is the project-level adapter record .evolve/domain.json has carried
// since v8 (docs/reference/configuration.md) with ZERO Go readers until
// ADR-0099 slice 2. Only Domain is consumed today — it yields the project's
// default deliverable kind when a task declares none; the other fields are the
// legacy prompt-layer vocabulary, parsed for round-trip fidelity only.
type Domain struct {
	Domain         string `json:"domain"`
	EvalMode       string `json:"evalMode,omitempty"`
	ShipMechanism  string `json:"shipMechanism,omitempty"`
	BuildIsolation string `json:"buildIsolation,omitempty"`
}

// LoadDomain reads <projectRoot>/.evolve/domain.json. An absent file is the
// ordinary case (ok=false, nil error — the caller falls back to the code
// default). A file that exists but cannot be read or parsed is an ERROR the
// caller must surface: a writing project with a trailing comma must not become
// a code project silently.
func LoadDomain(projectRoot string) (Domain, bool, error) {
	path := filepath.Join(projectRoot, ".evolve", "domain.json")
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

// DefaultDeliverableKind maps the project domain to the deliverable kind a
// task inherits when it declares none: writing/research projects produce
// documents; everything else (coding, design, mixed, unset) stays code — the
// conservative side that keeps the tdd pin.
func (d Domain) DefaultDeliverableKind() string {
	switch d.Domain {
	case "writing", "research":
		return DeliverableKindDocument
	}
	return DeliverableKindCode
}
