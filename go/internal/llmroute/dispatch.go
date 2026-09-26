package llmroute

import (
	"errors"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// DispatchResult is the outcome of walking a Plan's CLI chain.
type DispatchResult struct {
	CLI      string   // the CLI that produced the terminal result (success or final failure)
	Attempts []string // every CLI launched, in order
	Err      error    // nil on success; the terminal attempt's error otherwise
}

// Dispatch walks the CLI chain until success, a non-trigger exit, or exhaustion (returning the last error).
func Dispatch(plan Plan, launch func(cli string) (exitCode int, err error)) DispatchResult {
	// A caller checking only Err would read a nil from an empty chain as success.
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

// ChainFor builds a Plan from an already-chosen primary plus the profile's fallback, excluding prof.CLI.
func ChainFor(primary string, prof *profiles.Profile) Plan {
	return Plan{
		Candidates: buildCandidates(primary, prof, true),
		Triggers:   resolveTriggers(prof),
	}
}

// buildCandidates always returns at least the primary. ChainFor excludes prof.CLI because the
// composition root swapped away from it; Resolve keeps it so a forced primary retains the fallback.
func buildCandidates(primary string, prof *profiles.Profile, excludeProfileCLI bool) []string {
	candidates := []string{primary}
	if prof == nil {
		return candidates
	}
	seen := map[string]struct{}{primary: {}}
	if excludeProfileCLI && prof.CLI != "" {
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
