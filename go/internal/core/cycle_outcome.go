package core

import (
	"strings"
)

func preserveOnVerdict(finalVerdict string) bool {
	return finalVerdict == VerdictFAIL
}

// isScoutEvalMaterialization reports whether a main-tree write is scout
// performing its documented eval-materialization contract. Scout writes the
// SELECTED slugs' evals to projectRoot/.evolve/evals/<slug>.md in the MAIN
// tree (internal/evalgate/materialization.go reads them there for Gate A), so
// that write is scout's JOB, not a deliverable escape. Without this carve-out
// a later cycle iterating the same coverage target re-materializes the same
// slug, MODIFYING the prior cycle's committed eval (soak-#6 cycle 318→319
// ledger-seal-io-coverage), and the tree-diff guard aborts the cycle. Scoped
// to scout + .evolve/evals/<slug>.md only: a code phase leaking an eval, or
// scout writing a non-.md file or any other deliverable (phases/, commit-
// prefix-scope.json) or a source file, all still fire the guard.
func isScoutEvalMaterialization(phase Phase, p string) bool {
	return phase == PhaseScout && strings.HasPrefix(p, ".evolve/evals/") && strings.HasSuffix(p, ".md")
}

// gitCapture runs `git -C dir <args...>` and returns (stdout, exitCode, err).
// A non-zero exit is returned as exitCode with nil err (the caller decides
// whether it's fatal — e.g. `git diff HEAD` exit 1 means "differences", not a
// failure). Only a failure to launch git returns a non-nil err.

func (o *Orchestrator) finalizeOutcome(lastPhaseVerdict, retroDecision, preHEAD, postHEAD string) string {
	if lastPhaseVerdict != VerdictSKIPPED {
		return lastPhaseVerdict
	}
	// HEAD moved → something shipped inline (build calling `evolve ship --class manual`).
	if preHEAD != "" && postHEAD != "" && preHEAD != postHEAD {
		return CycleOutcomeShippedViaBuild
	}
	if strings.Contains(retroDecision, "would-have-blocked") {
		return CycleOutcomeSkippedAuditAdvisory
	}
	return CycleOutcomeSkippedUnknown
}

// The retry policy bounds per-phase retries on a recoverable bridge
// ArtifactTimeout (Fix D). 2 = one relaunch after the first timeout; a
// deterministic timeout still aborts the cycle after the cap.
