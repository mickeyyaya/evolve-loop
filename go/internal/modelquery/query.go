// Package modelquery is the LIVE acquisition layer for the model catalog.
//
// modelcatalog (the leaf data layer) holds the Catalog schema + cache I/O.
// modelquery sits above it: it queries each installed CLI for the models it
// currently offers (Lister), classifies those raw model ids into the abstract
// tiers fast/balanced/deep (Classifier — an LLM call, the one judgment step),
// and assembles a fresh Catalog. It imports modelcatalog (NOT bridge — live
// CLI dispatch is injected through the Lister/PromptDispatcher seams by the
// composition root in cmd/evolve); keeping it separate is what lets
// modelcatalog stay a dependency-free leaf.
//
// Robustness contract: a per-CLI live failure (the CLI can't be driven, the
// classifier errors, or either returns nothing) is logged and falls back to
// that CLI's detect-derived tier map — a refresh never aborts wholesale and
// never blocks a cycle (matches "failure → WARN + reuse last-good").
package modelquery

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// Lister enumerates the concrete model ids one CLI currently offers. Strategies
// vary per CLI: a non-interactive listing (`ollama list`) or driving the REPL's
// /model picker and capturing the pane.
type Lister interface {
	List(ctx context.Context, cli string) ([]string, error)
}

// Classifier maps a CLI's raw model ids to canonical tiers (fast/balanced/deep).
// This is the single judgment step (Rule 5): delegated to an LLM so the mapping
// tracks live model naming rather than baking in stale model knowledge.
type Classifier interface {
	Classify(ctx context.Context, cli string, modelIDs []string) (map[string]string, error)
}

// RefreshDeps are the injectable seams Refresh orchestrates over, so the
// orchestration is unit-testable with fakes (no live CLI, no exec).
type RefreshDeps struct {
	// CLIs are the base CLI names to refresh (the ready ones from setup.Detect).
	CLIs []string
	// Lister enumerates models for a CLI. Required.
	Lister Lister
	// Classifier maps ids → tiers. Required.
	Classifier Classifier
	// Fallback maps a CLI to its detect-derived tier map, used when the live
	// path fails for that CLI. Optional; a CLI with neither live data nor a
	// fallback is skipped.
	Fallback map[string]map[string]string
	// EffortListers discovers each CLI's reasoning-effort ladder. Optional and
	// per-CLI: a CLI with no entry simply records no ladder. Enrichment only —
	// a discovery failure is logged and the refresh continues, because models
	// are the payload and losing the catalog to a `--help` hiccup would be a
	// worse outcome than the missing-rung blindness this closes.
	EffortListers map[string]EffortLister
	// AllowedFamilies is a per-CLI model-family allow-list (from policy.json
	// catalog.allowed_families). A CLI's live-queried ids are filtered to its
	// allowed families (via FilterByFamily) BEFORE reaching Classify, so a
	// cross-family id never reaches classification. A CLI with no entry (nil
	// slice) is unconstrained — every listed id passes through unfiltered.
	AllowedFamilies map[string][]string
	// Prior is the catalog from the previous refresh. When a CLI's
	// family-filtered candidate list fingerprints identically to its prior
	// entry — and that entry is live-sourced with full canonical-tier
	// coverage — the prior tier map is reused WITHOUT a classifier call
	// (zero LLM calls on an unchanged offering; the fix for tier flap on
	// identical inputs). Zero value ⇒ nothing reusable, classify as always.
	Prior modelcatalog.Catalog
	// Freshness is the per-CLI freshness policy (declared in the bridge
	// manifest's model_freshness block, threaded here as plain values by the
	// composition root). A missing entry is the zero policy: newest concrete
	// version wins.
	Freshness map[string]FreshnessPolicy
	// Now stamps the catalog's FetchedAt; defaults to time.Now.
	Now func() time.Time
	// Log is the WARN sink; defaults to io.Discard.
	Log io.Writer
}

// Refresh queries every CLI's live models, classifies them into tiers, and
// returns a fresh Catalog. Per-CLI live failures fall back to the detect tier
// map; the whole refresh only errors if deps are structurally invalid.
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
			// Live-queried → authoritative; only these entries drive dispatch
			// (modelcatalog.DispatchModel gates on SourceLive).
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
		// Detect fallback is informational only — NOT dispatch-authoritative.
		fmt.Fprintf(log, "[modelquery] WARN %s: live query unavailable; using detect fallback\n", cli)
		snaps = append(snaps, modelcatalog.CLISnapshot{
			CLI: cli, Ready: true, TierModels: fb, Efforts: efforts,
			Source: modelcatalog.SourceDetect,
		})
	}
	return modelcatalog.BuildFromSnapshots(snaps, now().UTC()), nil
}

// liveTiers runs the List → [reuse-gate] → Classify → Promote → Complete
// pipeline for one CLI. It returns empty tiers (signalling fallback) on any
// error or empty result, but still returns the enumerated ids as the audit
// trail when listing succeeded. hash is the decision-input fingerprint of the
// family-filtered candidates, stored on the entry so the NEXT refresh can
// reuse it.
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
	// Family gate: restrict candidates to this CLI's allowed families BEFORE
	// classification (D7). A nil/absent allow-list is "no constraint" and
	// FilterByFamily returns ids unchanged, preserving today's behavior exactly.
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
	// Reuse gate: an unchanged offering keeps the prior tier map with ZERO
	// classifier calls. Reuse requires all three conditions, not just the
	// hash match — a detect entry must never be laundered into an
	// authoritative one, and a tier-incomplete prior must reclassify once
	// rather than stay sticky forever.
	if prior, ok := deps.Prior.CLIs[cli]; ok &&
		prior.CandidatesHash != "" && prior.CandidatesHash == fp &&
		prior.Source == modelcatalog.SourceLive &&
		coversCanonicalTiers(prior.TierModels) {
		return prior.TierModels, ids, fp
	}
	mapped, err := deps.Classifier.Classify(ctx, cli, ids)
	if err != nil {
		fmt.Fprintf(log, "[modelquery] WARN %s: classify models: %v\n", cli, err)
		return nil, ids, ""
	}
	// The classifier keeps the qualitative decision (which line serves the
	// tier); promotion keeps the numeric one (newest version of that line,
	// alias-aware per the CLI's freshness policy); completion fills any tier
	// the reply omitted from the nearest-neighbour ladder.
	return CompleteTiers(PromoteLatest(mapped, ids, deps.Freshness[cli])), ids, fp
}

// coversCanonicalTiers reports whether tiers maps every canonical tier to a
// non-empty model — the reuse gate's full-coverage condition.
func coversCanonicalTiers(tiers map[string]string) bool {
	for _, tier := range modelcatalog.CanonicalTiers {
		if tiers[tier] == "" {
			return false
		}
	}
	return true
}

// discoverEfforts asks the CLI's registered EffortLister for its ladder.
//
// Fail-soft by design, and the two silent outcomes are kept distinct in the
// LOG even though both record nothing: "no lister" means this CLI's ladder is
// not discoverable by us (codex — its rungs live only behind a picker
// submenu), while an error means we asked and could not tell. Recording an
// empty ladder for either is correct; claiming a discovered "no dial" for
// either would not be.
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
