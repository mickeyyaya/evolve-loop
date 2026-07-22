package triagecap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

// wave_seed.go — the production seam that finally wires SelectFleetWidthTopN
// (fleet-width-aware, file-disjoint top_n packing) into the wave-seed path. Two
// prior cycles (508, 518) shipped the packing algorithm fully tested but with
// ZERO callers; the wave planner still seeded from a raw weight-sorted top-N,
// which can hand the fleet two lanes that collide on a shared file. This is the
// caller.

// SelectWaveSeedTopN reads the inbox backlog under <evolveDir>/inbox/*.json and
// returns up to `count` mutually file-disjoint lane representatives (highest
// weight first) via SelectFleetWidthTopN — safe to fan out 1:1 into concurrent
// `evolve cycle run` lanes without a cross-lane file collision.
//
// count<2 reproduces the legacy single-focus pick (the single highest-weight
// candidate). Unreadable / malformed inbox files and empty-id todos are skipped;
// a bad inbox never breaks dispatch (best-effort). Files are read in filename
// order so equal-weight ties are deterministic.
func SelectWaveSeedTopN(evolveDir string, count int, isProtected func(string) bool) []FleetCandidate {
	return SelectFleetWidthTopN(ReadInboxBacklog(evolveDir, isProtected), count)
}

// ReadInboxBacklog reads every <evolveDir>/inbox/*.json todo into an unpacked
// []FleetCandidate (id + weight + declared files), in filename order so
// equal-weight ties stay deterministic. Unreadable / malformed files and
// empty-id todos are skipped (best-effort — a bad inbox never breaks dispatch).
// It is the single source for "inbox backlog as fleet candidates," shared by the
// wave-seed fallback (SelectWaveSeedTopN) and the widen-narrow-decision seam
// (WidenTopNToFleetWidth's caller).
// isProtected is the ADR-0074 control-plane predicate (guards.IsProtectedSurface
// at composition roots; nil disables only the files-derived routing rule).
// Console-routed items are EXCLUDED at read time — batch-7 wave-0 starved
// ("planned zero lanes") because the raw top-N seeded exclusively console
// items the plan-time gate then rightly refused; skipping them here lets the
// seed backfill from the next dispatchable candidates instead.
func ReadInboxBacklog(evolveDir string, isProtected func(string) bool) []FleetCandidate {
	entries, _ := filepath.Glob(filepath.Join(evolveDir, "inbox", "*.json"))
	sort.Strings(entries)
	candidates := make([]FleetCandidate, 0, len(entries))
	for _, p := range entries {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var doc inboxbatch.Item
		if json.Unmarshal(raw, &doc) != nil || doc.ID == "" {
			continue
		}
		if routed, _ := inboxbatch.ConsoleRouted(doc, isProtected); routed {
			continue
		}
		candidates = append(candidates, FleetCandidate{ID: doc.ID, Weight: doc.Weight, Files: doc.Files})
	}
	return candidates
}
