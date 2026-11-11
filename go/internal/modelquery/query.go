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
}

func liveTiers(ctx context.Context, cli string, deps RefreshDeps) (liveResult, error) {
	ids, err := deps.Lister.List(ctx, cli)
	if err != nil {
		return liveResult{}, fmt.Errorf("list models: %w", err)
	}
	if len(ids) == 0 {
		return liveResult{}, errors.New("CLI offered no models")
	}
	if allowed := deps.AllowedFamilies[cli]; len(allowed) > 0 {
		ids = FilterByFamily(ids, allowed...)
		if len(ids) == 0 {
			return liveResult{}, fmt.Errorf("no models in allowed families %v", allowed)
		}
	}
	fp := Fingerprint(FingerprintInput{
		CLI: cli, Candidates: ids,
		Policy: deps.Freshness[cli], Tiers: modelcatalog.CanonicalTiers,
	})
	if prior, ok := deps.Prior.CLIs[cli]; ok && isReusablePrior(prior, fp) {
		return liveResult{tiers: prior.TierModels, available: ids, hash: fp}, nil
	}
	if tiers, ok := carryAcrossBump(cli, deps.Prior.CLIs[cli], ids, deps.Freshness[cli]); ok {
		return liveResult{tiers: tiers, available: ids, hash: fp}, nil
	}
	mapped, err := deps.Classifier.Classify(ctx, cli, ids)
	if err != nil {
		return liveResult{}, fmt.Errorf("classify models: %w", err)
	}
	tiers := CompleteTiers(PromoteLatest(mapped, ids, deps.Freshness[cli]))
	if len(tiers) == 0 {
		return liveResult{}, errors.New("classify models: no tier mapped to an offered model")
	}
	return liveResult{tiers: tiers, available: ids, hash: fp}, nil
}

func isReusablePrior(prior modelcatalog.CLIEntry, fp string) bool {
	return prior.CandidatesHash != "" && prior.CandidatesHash == fp && isSoundPrior(prior)
}

func isSoundPrior(prior modelcatalog.CLIEntry) bool {
	return prior.Source == modelcatalog.SourceLive && coversCanonicalTiers(prior.TierModels)
}

func carryAcrossBump(cli string, prior modelcatalog.CLIEntry, ids []string, p FreshnessPolicy) (map[string]string, bool) {
	if !isSoundPrior(prior) || len(prior.Available) == 0 || !sameLineages(prior.Available, ids) {
		return nil, false
	}
	if !priorDerivedFromItsOffering(cli, prior, p) {
		return nil, false
	}
	tiers := PromoteLatest(prior.TierModels, ids, p)
	for _, model := range tiers {
		if !slices.Contains(ids, model) {
			return nil, false
		}
	}
	return tiers, true
}

func priorDerivedFromItsOffering(cli string, prior modelcatalog.CLIEntry, p FreshnessPolicy) bool {
	return prior.CandidatesHash != "" && prior.CandidatesHash == Fingerprint(FingerprintInput{
		CLI: cli, Candidates: prior.Available, Policy: p, Tiers: modelcatalog.CanonicalTiers,
	})
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
