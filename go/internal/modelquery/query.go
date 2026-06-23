// Package modelquery is the LIVE acquisition layer for the model catalog.
//
// modelcatalog (the leaf data layer) holds the Catalog schema + cache I/O.
// modelquery sits above it: it queries each installed CLI for the models it
// currently offers (Lister), classifies those raw model ids into the abstract
// tiers fast/balanced/deep (Classifier — an LLM call, the one judgment step),
// and assembles a fresh Catalog. It therefore imports both modelcatalog and
// bridge; keeping it separate is what lets modelcatalog stay a dependency-free
// leaf.
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

	"github.com/mickeyyaya/evolveloop/go/internal/modelcatalog"
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
		tiers, available := liveTiers(ctx, cli, deps, log)
		if len(tiers) > 0 {
			// Live-queried → authoritative; only these entries drive dispatch
			// (modelcatalog.DispatchModel gates on SourceLive).
			snaps = append(snaps, modelcatalog.CLISnapshot{
				CLI: cli, Ready: true, TierModels: tiers,
				Available: available, Source: modelcatalog.SourceLive,
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
			CLI: cli, Ready: true, TierModels: fb, Source: modelcatalog.SourceDetect,
		})
	}
	return modelcatalog.BuildFromSnapshots(snaps, now().UTC()), nil
}

// liveTiers runs the List → Classify pipeline for one CLI. It returns empty
// tiers (signalling fallback) on any error or empty result, but still returns
// the enumerated ids as the audit trail when listing succeeded.
func liveTiers(ctx context.Context, cli string, deps RefreshDeps, log io.Writer) (tiers map[string]string, available []string) {
	ids, err := deps.Lister.List(ctx, cli)
	if err != nil {
		fmt.Fprintf(log, "[modelquery] WARN %s: list models: %v\n", cli, err)
		return nil, nil
	}
	if len(ids) == 0 {
		fmt.Fprintf(log, "[modelquery] WARN %s: CLI offered no models\n", cli)
		return nil, nil
	}
	mapped, err := deps.Classifier.Classify(ctx, cli, ids)
	if err != nil {
		fmt.Fprintf(log, "[modelquery] WARN %s: classify models: %v\n", cli, err)
		return nil, ids
	}
	return mapped, ids
}
