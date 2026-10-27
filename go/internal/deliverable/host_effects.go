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

type Claimer func(inboxDir string, cycle int, ids []string) error

type HostEffects struct {
	resolver phasecontract.Resolver
	spec     func(string) (phasespec.PhaseSpec, bool)
	claim    Claimer
}

func NewHostEffects(cat phasespec.Catalog, claim Claimer) *HostEffects {
	return &HostEffects{resolver: phasecontract.NewCatalogResolver(cat.Get), spec: cat.Get, claim: claim}
}

func (h *HostEffects) Perform(_ context.Context, in core.ReviewInput) error {
	c, ok := h.resolver.Resolve(in.Phase)
	if !ok {
		return nil
	}
	roots := RootsFor(in)
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
	if err := h.writeHandoffSignals(c, roots); err != nil {
		errs = append(errs, fmt.Errorf("handoff signals: %w", err))
	}
	return errors.Join(errs...)
}

type deriver func(primary []byte, roots phasecontract.Roots) ([]byte, error)

var derivers = map[string]deriver{
	"triage-decision.json": func(primary []byte, roots phasecontract.Roots) ([]byte, error) {
		return triagedecision.Derive(primary, roots.Cycle, committedset.LanePin(roots.Workspace))
	},
}

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
