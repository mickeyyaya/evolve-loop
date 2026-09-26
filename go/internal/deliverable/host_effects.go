package deliverable

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
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
	return errors.Join(errs...)
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
