package core

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

type gitMutationLocker func(projectRoot string) (release func(), err error)

type cycleDossierParams struct {
	ProjectRoot        string
	WorkspacePath      string
	Cycle              int
	Goal               string
	RunID              string
	Outcome            string
	SystemFailure      *SystemFailureSignal
	SkippedPhases      []SkippedPhase
	VerdictsNotAdopted []VerdictNotAdopted
	SpineFailOpens     []SpineFailOpen
	PhaseTimings       []phaseTimingEntry
	Destination        DossierDestination
}

func defaultGitMutationLock(projectRoot string) (func(), error) {
	return flock.Lock(flock.ShipLockPath(projectRoot))
}

func dossierVerdict(outcome string) string {
	switch outcome {
	case VerdictPASS, CycleOutcomeShippedViaBuild:
		return dossier.VerdictPass
	case VerdictFAIL:
		return dossier.VerdictFail
	default:
		return dossier.VerdictWarn
	}
}

// writeCycleDossier builds and persists the closeout dossier for one completed
// cycle as cycle-N.{json,md} in the corpus (knowledge-base/cycles), or in the
// pending dir the loop publishes from, as p.Destination says. goal must be
// non-blank; callers fall back to the goal hash when there is no human-readable
// text. Returns an error the best-effort caller logs; it never panics.
func writeCycleDossier(lock gitMutationLocker, p cycleDossierParams) error {
	d, err := dossier.Build(p.Cycle, dossier.BuildOpts{
		WorkspacePath:      p.WorkspacePath,
		Goal:               p.Goal,
		RunID:              p.RunID,
		FinalVerdict:       dossierVerdict(p.Outcome),
		SystemFailure:      p.SystemFailure,
		SkippedPhases:      p.SkippedPhases,
		VerdictsNotAdopted: p.VerdictsNotAdopted,
		SpineFailOpens:     p.SpineFailOpens,
		// Passed explicitly, not re-read from phase-timing.json: RunCycle
		// writes that file via a deferred call that lands after this producer
		// runs, so reading it here would see no phases.
		PhaseTimings: p.PhaseTimings,
	})
	if err != nil {
		return fmt.Errorf("build dossier: %w", err)
	}
	dir, commit := dossier.CyclesDir(p.ProjectRoot), p.Destination == DossierCommitted
	if p.Destination == DossierPending {
		dir = dossier.PendingDir(p.ProjectRoot)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("dossier dir: %w", err)
	}
	if lock != nil && commit {
		if release, lerr := lock(p.ProjectRoot); lerr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN dossier git-mutation lock: %v (proceeding unserialized; a concurrent index collision would skip this dossier)\n", lerr)
		} else {
			defer release()
		}
	}
	if err := dossier.Write(d, dir, commit); err != nil {
		return fmt.Errorf("write dossier: %w", err)
	}
	return nil
}
