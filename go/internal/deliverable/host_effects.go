package deliverable

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagedecision"
)

// Claimer claims the ids still pending in inboxDir for cycle.
type Claimer func(inboxDir string, cycle int, ids []string) error

// HostEffects performs the declared effects the host owns, resolving the
// phase and its roots exactly as the gate that verifies them.
type HostEffects struct {
	resolver phasecontract.Resolver
	claim    Claimer
}

// NewHostEffects resolves effects from cat, the catalog the gate reads.
func NewHostEffects(cat phasespec.Catalog, claim Claimer) *HostEffects {
	return &HostEffects{resolver: phasecontract.NewCatalogResolver(cat.Get), claim: claim}
}

// Perform runs the host's half of each effect in.Phase declares.
func (h *HostEffects) Perform(_ context.Context, in core.ReviewInput) error {
	c, ok := h.resolver.Resolve(in.Phase)
	if !ok {
		return nil
	}
	roots := rootsFor(in)
	var errs []error
	for _, name := range c.Effects {
		perform := effects[name].perform
		if perform == nil {
			continue
		}
		if err := perform(h, roots); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	errs = append(errs, h.derive(c, roots)...)
	return errors.Join(errs...)
}

// deriver builds an owed secondary from the phase's primary output; a decline is an error naming why.
type deriver func(primary []byte, roots phasecontract.Roots) ([]byte, error)

var derivers = map[string]deriver{
	"triage-decision.json": func(primary []byte, roots phasecontract.Roots) ([]byte, error) {
		return triagedecision.Derive(primary, roots.Cycle, committedset.LanePin(roots.Workspace))
	},
}

// derive writes each declared derivable secondary the agent left absent or empty, from the primary the
// registry names; a file the agent wrote is never touched (the read waits out a write in flight, as the gate's
// does), and a decline leaves it absent for the gate.
func (h *HostEffects) derive(c phasecontract.Contract, roots phasecontract.Roots) []error {
	var errs []error
	for _, owed := range slices.Sorted(maps.Keys(c.DerivedFrom)) {
		source := c.DerivedFrom[owed]
		path := phasecontract.OwedPath(roots.Workspace, owed)
		written, exists, err := readDeliverableWithGrace(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("derive %s: %w", owed, err))
			continue
		}
		if exists && strings.TrimSpace(written) != "" {
			continue
		}
		primary := c.ArtifactPath(roots)
		if filepath.Base(primary) != source {
			errs = append(errs, fmt.Errorf("derive %s: the registry derives it from %s, but the contract's primary is %s", owed, source, filepath.Base(primary)))
			continue
		}
		d := derivers[owed]
		if d == nil {
			errs = append(errs, fmt.Errorf("derive %s: no deriver is registered for it", owed))
			continue
		}
		report, err := os.ReadFile(primary)
		if err != nil {
			errs = append(errs, fmt.Errorf("derive %s: %w", owed, err))
			continue
		}
		out, err := d(report, roots)
		if err != nil {
			errs = append(errs, fmt.Errorf("derive %s: %w", owed, err))
			continue
		}
		if err := atomicwrite.Bytes(path, out); err != nil {
			errs = append(errs, fmt.Errorf("derive %s: %w", owed, err))
		}
	}
	return errs
}

func claimCommitted(h *HostEffects, roots phasecontract.Roots) error {
	ids, recorded := committedset.Committed(roots.Workspace)
	if !recorded || len(ids) == 0 {
		return nil
	}
	if roots.Cycle < 1 || !filepath.IsAbs(roots.EvolveDir) {
		return errors.New("no cycle or project root to claim into")
	}
	return h.claim(inboxDir(roots), roots.Cycle, ids)
}
