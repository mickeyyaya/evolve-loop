// Package modelquery queries each installed CLI for its live models and classifies them into catalog tiers.
// See docs/architecture/packages/internal-modelquery.md.
package modelquery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

type Lister interface {
	List(ctx context.Context, cli string) ([]string, error)
}

type Classifier interface {
	Classify(ctx context.Context, cli string, modelIDs []string) (map[string]string, error)
}

type RefreshDeps struct {
	CLIs            []string
	Lister          Lister
	Classifier      Classifier
	Fallback        map[string]map[string]string
	EffortListers   map[string]EffortLister
	AllowedFamilies map[string][]string
	Prior           modelcatalog.Catalog
	Freshness       map[string]FreshnessPolicy
	Pins            map[string]map[string]string
	Now             func() time.Time
	Log             io.Writer
}

func Refresh(ctx context.Context, deps RefreshDeps) (modelcatalog.Catalog, error) {
	if deps.Lister == nil || deps.Classifier == nil {
		return modelcatalog.Catalog{}, fmt.Errorf("modelquery: Lister and Classifier are required")
	}
	log := deps.Log
	if log == nil {
		log = io.Discard
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}

	snaps := make([]modelcatalog.CLISnapshot, 0, len(deps.CLIs))
	for _, cli := range deps.CLIs {
		if snap, ok := snapshotFor(ctx, cli, deps, log); ok {
			snaps = append(snaps, snap)
		}
	}
	return modelcatalog.BuildFromSnapshots(snaps, now().UTC()), nil
}

func snapshotFor(ctx context.Context, cli string, deps RefreshDeps, log io.Writer) (modelcatalog.CLISnapshot, bool) {
	efforts := discoverEfforts(ctx, cli, deps, log)
	live, err := liveTiers(ctx, cli, deps)
	if err == nil {
		for _, pin := range live.unmetPins {
			fmt.Fprintf(log, "[modelquery] WARN %s: pin %s is not offered; the tier keeps its classified model\n", cli, pin)
		}
		return modelcatalog.CLISnapshot{
			CLI: cli, Ready: true, TierModels: live.tiers,
			Available: live.available, Efforts: efforts,
			Source:         modelcatalog.SourceLive,
			CandidatesHash: live.hash,
		}, true
	}
	fb := deps.Fallback[cli]
	if len(fb) == 0 {
		fmt.Fprintf(log, "[modelquery] WARN %s: %v; no detect fallback, skipping\n", cli, err)
		return modelcatalog.CLISnapshot{}, false
	}
	fmt.Fprintf(log, "[modelquery] WARN %s: %v; using detect fallback\n", cli, err)
	return modelcatalog.CLISnapshot{
		CLI: cli, Ready: true, TierModels: fb, Efforts: efforts,
		Source: modelcatalog.SourceDetect, FallbackReason: err.Error(),
	}, true
}

type liveResult struct {
	tiers     map[string]string
	available []string
	hash      string
	unmetPins []string
}

type offering struct {
	cli    string
	ids    []string
	policy FreshnessPolicy
	pins   map[string]string
	prior  modelcatalog.CLIEntry
}

func (o offering) fingerprint(candidates []string) string {
	return Fingerprint(FingerprintInput{
		CLI: o.cli, Candidates: candidates, Policy: o.policy,
		Tiers: modelcatalog.CanonicalTiers, Pins: o.pins,
	})
}

func liveTiers(ctx context.Context, cli string, deps RefreshDeps) (liveResult, error) {
	ids, err := offeredModels(ctx, cli, deps)
	if err != nil {
		return liveResult{}, err
	}
	o := offering{cli: cli, ids: ids, policy: deps.Freshness[cli], pins: deps.Pins[cli], prior: deps.Prior.CLIs[cli]}
	tiers, err := unpinnedTiers(ctx, deps.Classifier, o)
	if err != nil {
		return liveResult{}, err
	}
	pinned, unmet := modelcatalog.PinTiers(tiers, o.pins, ids)
	return liveResult{tiers: pinned, available: ids, hash: o.fingerprint(ids), unmetPins: unmet}, nil
}

func offeredModels(ctx context.Context, cli string, deps RefreshDeps) ([]string, error) {
	ids, err := deps.Lister.List(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	if len(ids) == 0 {
		return nil, errors.New("CLI offered no models")
	}
	allowed := deps.AllowedFamilies[cli]
	if len(allowed) == 0 {
		return ids, nil
	}
	ids = FilterByFamily(ids, allowed...)
	if len(ids) == 0 {
		return nil, fmt.Errorf("no models in allowed families %v", allowed)
	}
	return ids, nil
}

func unpinnedTiers(ctx context.Context, classifier Classifier, o offering) (map[string]string, error) {
	if isReusablePrior(o.prior, o.fingerprint(o.ids)) {
		return o.prior.TierModels, nil
	}
	if tiers, ok := carryAcrossBump(o); ok {
		return tiers, nil
	}
	mapped, err := classifier.Classify(ctx, o.cli, o.ids)
	if err != nil {
		return nil, fmt.Errorf("classify models: %w", err)
	}
	tiers := CompleteTiers(PromoteLatest(mapped, o.ids, o.policy))
	if len(tiers) == 0 {
		return nil, errors.New("classify models: no tier mapped to an offered model")
	}
	return tiers, nil
}

func isReusablePrior(prior modelcatalog.CLIEntry, fp string) bool {
	return prior.CandidatesHash != "" && prior.CandidatesHash == fp && isSoundPrior(prior)
}

func isSoundPrior(prior modelcatalog.CLIEntry) bool {
	return prior.Source == modelcatalog.SourceLive && coversCanonicalTiers(prior.TierModels)
}

func carryAcrossBump(o offering) (map[string]string, bool) {
	if !isSoundPrior(o.prior) || len(o.prior.Available) == 0 || !sameLineages(o.prior.Available, o.ids) {
		return nil, false
	}
	if !priorDerivedFromItsOffering(o) {
		return nil, false
	}
	tiers := PromoteLatest(o.prior.TierModels, o.ids, o.policy)
	for _, model := range tiers {
		if !slices.Contains(o.ids, model) {
			return nil, false
		}
	}
	return tiers, true
}

func priorDerivedFromItsOffering(o offering) bool {
	return o.prior.CandidatesHash != "" && o.prior.CandidatesHash == o.fingerprint(o.prior.Available)
}

func sameLineages(a, b []string) bool {
	ga, gb := GroupByLineage(a), GroupByLineage(b)
	if len(ga) != len(gb) {
		return false
	}
	for key := range ga {
		if _, ok := gb[key]; !ok {
			return false
		}
	}
	return true
}

func coversCanonicalTiers(tiers map[string]string) bool {
	for _, tier := range modelcatalog.CanonicalTiers {
		if tiers[tier] == "" {
			return false
		}
	}
	return true
}

func discoverEfforts(ctx context.Context, cli string, deps RefreshDeps, log io.Writer) []string {
	l, ok := deps.EffortListers[cli]
	if !ok || l == nil {
		return nil
	}
	rungs, err := l.ListEfforts(ctx, cli)
	if err != nil {
		fmt.Fprintf(log, "[modelquery] WARN %s: effort-ladder discovery failed (%v); models unaffected\n", cli, err)
		return nil
	}
	return rungs
}
