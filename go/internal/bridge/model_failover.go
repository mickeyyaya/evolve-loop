package bridge

import "github.com/mickeyyaya/evolve-loop/go/internal/policy"

// model_failover.go — the within-tier MODEL-failover axis (the innermost of
// model ⊂ tier ⊂ CLI). A phase's tier can map to an ordered chain of concrete
// models (model-catalog tier_fallbacks, e.g. claude deep → [fable, opus, sonnet]);
// on a per-model quota wall (exit=85) dispatch advances to the NEXT model at the
// SAME cli+tier before the caller escalates to the CLI/tier fallback. This is the
// only redundancy for a phase pinned to one tier AND one CLI (e.g. the auditor:
// envelope min=max=deep, cli_fallback=[]) — the failure that hard-blocked the
// audit phase on the Fable-5 per-model limit while Opus (also deep) had budget.

// dispatchModelsFor resolves the model chain to actually dispatch for a launch.
// It is single-shot [model] whenever a tmux SessionName is pinned: named-session
// dispatch (the swarm path) PRESERVES the REPL across attempts (tmuxCleanup keeps
// named sessions), so on attempt 2+ the driver reattaches to the already-running
// process and SKIPS the `--model` reseed — a mid-chain model switch would not take
// effect and would mis-attribute the exhausted model's telemetry to the new one.
// Until named-session model-switch is wired, such dispatch stays single-shot at
// the resolved model. The ephemeral-session path (no SessionName — single-lane
// cycle dispatch, the auditor case this feature fixes) mints a fresh session per
// attempt and cold-boots each model, so it gets the full failover chain.
func dispatchModelsFor(cli, model, sessionName string) []string {
	if sessionName != "" {
		return []string{model}
	}
	return dispatchModelChain(cli, model)
}

// dispatchModelChain resolves the ordered concrete-model chain for (cli,
// tierOrModel) from the live model catalog. Empty catalog / non-live entry / a
// tier with no catalog models ⇒ a single-element [tierOrModel] chain, so dispatch
// is byte-identical to pre-feature behavior: the tier is realized downstream via
// the manifest ModelTierMap, and a concrete model passes the realizer through.
// cli is the driver name (claude-tmux); it is reduced to the catalog base here.
func dispatchModelChain(cli, tierOrModel string) []string {
	if chain := loadCatalogCached().DispatchModelChain(policy.BaseCLI(cli), tierOrModel); len(chain) > 0 {
		return chain
	}
	return []string{tierOrModel}
}

// dispatchModelFailover runs launch(model) for each model in chain, advancing to
// the NEXT model ONLY on a per-model quota wall (ExitUnknownPrompt / exit=85) with
// a model remaining. Every other outcome short-circuits and returns immediately:
// success needs no fallback, and a non-85 failure (boot timeout, missing binary,
// artifact timeout) is not a model-quota problem — switching models would only
// waste attempts and quota. When the whole chain walls, it returns the LAST
// (model, code) so the caller escalates to the CLI/tier fallback exactly as before.
// onStep (optional) logs each advance so the step-down is never silent.
func dispatchModelFailover(chain []string, launch func(model string) int, onStep func(from, to string)) (used string, code int) {
	if len(chain) == 0 {
		// Defensive: dispatchModelsFor/dispatchModelChain always yield ≥1 model, so
		// an empty chain is a caller bug. Fail loud (ExitBadFlags) rather than
		// fabricate a launch-free ExitOK "success" with no artifact read.
		return "", ExitBadFlags
	}
	for i, m := range chain {
		used, code = m, launch(m)
		if code == ExitUnknownPrompt && i < len(chain)-1 {
			if onStep != nil {
				onStep(m, chain[i+1])
			}
			continue
		}
		break
	}
	return used, code
}
