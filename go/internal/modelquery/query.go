// Package modelquery queries each installed CLI for its live models and classifies them into catalog tiers.
// See docs/architecture/packages/internal-modelquery.md.
package modelquery

import (
	"context"
	"fmt"
	"io"
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
		efforts := discoverEfforts(ctx, cli, deps, log)
		tiers, available, hash := liveTiers(ctx, cli, deps, log)
		if len(tiers) > 0 {
			snaps = append(snaps, modelcatalog.CLISnapshot{
				CLI: cli, Ready: true, TierModels: tiers,
				Available: available, Efforts: efforts,
				Source:         modelcatalog.SourceLive,
				CandidatesHash: hash,
			})
			continue
		}
		fb := deps.Fallback[cli]
		if len(fb) == 0 {
			fmt.Fprintf(log, "[modelquery] WARN %s: no live models and no fallback; skipping\n", cli)
			continue
		}
		fmt.Fprintf(log, "[modelquery] WARN %s: live query unavailable; using detect fallback\n", cli)
		snaps = append(snaps, modelcatalog.CLISnapshot{
			CLI: cli, Ready: true, TierModels: fb, Efforts: efforts,
			Source: modelcatalog.SourceDetect,
		})
	}
	return modelcatalog.BuildFromSnapshots(snaps, now().UTC()), nil
}

func liveTiers(ctx context.Context, cli string, deps RefreshDeps, log io.Writer) (tiers map[string]string, available []string, hash string) {
	ids, err := deps.Lister.List(ctx, cli)
	if err != nil {
		fmt.Fprintf(log, "[modelquery] WARN %s: list models: %v\n", cli, err)
		return nil, nil, ""
	}
	if len(ids) == 0 {
		fmt.Fprintf(log, "[modelquery] WARN %s: CLI offered no models\n", cli)
		return nil, nil, ""
	}
	if allowed := deps.AllowedFamilies[cli]; len(allowed) > 0 {
		ids = FilterByFamily(ids, allowed...)
		if len(ids) == 0 {
			fmt.Fprintf(log, "[modelquery] WARN %s: no models in allowed families %v; skipping\n", cli, allowed)
			return nil, nil, ""
		}
	}
	fp := Fingerprint(FingerprintInput{
		CLI: cli, Candidates: ids,
		Policy: deps.Freshness[cli], Tiers: modelcatalog.CanonicalTiers,
	})
	if prior, ok := deps.Prior.CLIs[cli]; ok && isReusablePrior(prior, fp) {
		return prior.TierModels, ids, fp
	}
	mapped, err := deps.Classifier.Classify(ctx, cli, ids)
	if err != nil {
		fmt.Fprintf(log, "[modelquery] WARN %s: classify models: %v\n", cli, err)
		return nil, ids, ""
	}
	return CompleteTiers(PromoteLatest(mapped, ids, deps.Freshness[cli])), ids, fp
}

func isReusablePrior(prior modelcatalog.CLIEntry, fp string) bool {
	return prior.CandidatesHash != "" && prior.CandidatesHash == fp &&
		prior.Source == modelcatalog.SourceLive &&
		coversCanonicalTiers(prior.TierModels)
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
