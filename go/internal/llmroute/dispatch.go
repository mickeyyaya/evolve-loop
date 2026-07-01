package llmroute

import (
	"errors"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// DispatchResult is the outcome of walking a Plan's CLI chain: the CLI that
// was last attempted, every CLI attempted in order, and the terminal error
// (nil on success).
type DispatchResult struct {
	CLI      string   // the CLI that produced the terminal result (success or final failure)
	Attempts []string // every CLI launched, in order
	Err      error    // nil on success; the terminal attempt's error otherwise
}

// Dispatch walks plan.Candidates in order, calling launch(cli) for each. A
// nil error stops the walk on success. A non-nil error advances to the next
// candidate ONLY when plan.TriggersFallback(exitCode) — a real failure (a
// non-trigger exit) stops the walk immediately so a legitimate FAIL is never
// silently rerouted to a different CLI. When every candidate is exhausted on
// a trigger exit, the LAST candidate's error is returned so the caller can
// degrade to its own backstop.
//
// This is the single home for the "advance the CLI chain on a trigger exit"
// algorithm — extracted from the runner's WS-G1 inline loop so the advisor
// and the runner consume exactly one implementation
// ([[never_duplicate_centralize_via_design_patterns]]).
func Dispatch(plan Plan, launch func(cli string) (exitCode int, err error)) DispatchResult {
	// An empty chain is never a successful dispatch: launching nothing must fail
	// loudly rather than return Err=nil, which a caller checking only Err would
	// treat as a successful dispatch to no CLI at all (Rule 12). ChainFor always
	// seeds >=1 candidate, so this is a defensive guard, not a reachable path.
	if len(plan.Candidates) == 0 {
		return DispatchResult{Err: errors.New("llmroute: Dispatch called with no candidates")}
	}
	var attempts []string
	var cli string
	var err error
	for _, cli = range plan.Candidates {
		var exitCode int
		exitCode, err = launch(cli)
		attempts = append(attempts, cli)
		if err == nil {
			break
		}
		if !plan.TriggersFallback(exitCode) {
			break
		}
	}
	return DispatchResult{CLI: cli, Attempts: attempts, Err: err}
}

// ChainFor builds a Plan from an EXPLICIT already-resolved primary CLI plus
// the profile's declared fallback chain — never re-deriving the primary via
// resolvePrimary (which reads profile.cli and would ignore a bench-aware
// composition-root swap, e.g. the advisor already routed away from a benched
// family before ChainFor is ever called). prof.CLI itself is excluded from
// the appended fallback (it names the CLI the composition root already chose
// not to use as primary, so re-appending it as a "fallback" would just walk
// back into the same swapped-away CLI); a prof-declared fallback entry that
// duplicates the explicit primary is likewise deduped. prof may be nil (no
// profile on disk) — the chain degrades to a single candidate on the
// package default trigger set.
func ChainFor(primary string, prof *profiles.Profile) Plan {
	return Plan{
		Candidates: chainCandidates(primary, prof),
		Triggers:   resolveTriggers(prof),
	}
}

// chainCandidates builds the deduped chain: primary first, then prof's
// declared fallback (whitespace-trimmed, empties dropped), skipping any entry
// that repeats the primary or prof.CLI (the swapped-away original primary).
func chainCandidates(primary string, prof *profiles.Profile) []string {
	candidates := []string{primary}
	if prof == nil {
		return candidates
	}
	seen := map[string]struct{}{primary: {}}
	if prof.CLI != "" {
		seen[prof.CLI] = struct{}{}
	}
	for _, c := range prof.CLIFallback {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		candidates = append(candidates, c)
	}
	return candidates
}
