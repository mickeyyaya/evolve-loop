package llmroute

import (
	"errors"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type DispatchResult struct {
	CLI      string
	Attempts []string
	Err      error
}

func Dispatch(plan Plan, launch func(cli string) (exitCode int, err error)) DispatchResult {
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

func ChainFor(primary string, prof *profiles.Profile) Plan {
	return Plan{
		Candidates: buildCandidates(primary, prof, true),
		Triggers:   resolveTriggers(prof),
	}
}

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
