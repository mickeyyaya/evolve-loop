package core

// dossier_producer.go — ADR-0055 cycle-dossier producer wiring.
//
// Before the 2026-06-22 doc↔impl audit the dossier subsystem (internal/dossier:
// Build/Write/Render) had ZERO production callers: finalizeCycle never emitted a
// dossier, knowledge-base/cycles/ stayed empty, and the policy `floor` gate
// "dossier-closeout" enforced an artifact nobody wrote (Potemkin enforcement).
// This file is the missing producer — RunCycle calls writeCycleDossier after
// finalizeCycle so every completed cycle leaves a committed, validated record.
//
// The write is BEST-EFFORT (RunCycle logs a WARN on error, never fails the
// cycle): a cycle has already finalized by the time we write its closeout
// artifact, so a dossier write failure must not destabilize the loop. Presence
// is enforced separately by `evolve dossier verify` against the policy floor.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// gitMutationLocker serializes ONE shared-main-repo .git/index mutation (the
// dossier closeout commit) against concurrent fleet lanes. Strategy seam: the
// dispatcher injects the real cross-process flock in production; tests inject a
// deterministic spy. release is called exactly once, after the mutation.
type gitMutationLocker func(projectRoot string) (release func(), err error)

// defaultGitMutationLock is the production locker: a blocking cross-process flock
// on the SHARED integrator lock (flock.ShipLockPath → <projectRoot>/.evolve/ship.lock,
// the SAME file internal/phases/ship acquireShipLock takes), so a lane's dossier
// commit and a sibling lane's ship commit are MUTUALLY EXCLUSIVE on the one shared
// .git/index. The kernel releases it on process death, so a crashed lane cannot
// wedge the fleet. Safe against the cycle-819 self-deadlock: the dossier commit
// runs in finalizeCycle AFTER ship has released its own lease, so this is a fresh
// acquire of a lock the lane does not already hold.
func defaultGitMutationLock(projectRoot string) (func(), error) {
	return flock.Lock(flock.ShipLockPath(projectRoot))
}

// dossierVerdict maps a cycle's terminal CycleOutcome to the dossier verdict
// vocabulary. Only a clean ship is PASS; an explicit FAIL is FAIL; every other
// terminal (WARN, the SKIPPED family, advisory no-ships, and any unknown value)
// is WARN — a non-PASS record that preserves the cycle's experience without
// fabricating a pass or requiring synthesized defects. Pure.
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
// cycle to <projectRoot>/knowledge-base/cycles/cycle-N.{json,md}. goal must be
// non-blank (callers pass the human-readable goal text, falling back to the goal
// hash). Returns an error the best-effort caller logs; it never panics.
func writeCycleDossier(lock gitMutationLocker, projectRoot, workspacePath string, cycle int, goal, runID, outcome string, skipped []SkippedPhase) error {
	d, err := dossier.Build(cycle, dossier.BuildOpts{
		WorkspacePath: workspacePath,
		Goal:          goal,
		RunID:         runID,
		FinalVerdict:  dossierVerdict(outcome),
		SkippedPhases: skipped,
	})
	if err != nil {
		return fmt.Errorf("build dossier: %w", err)
	}
	dir := filepath.Join(projectRoot, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("dossier dir: %w", err)
	}
	// Acquire the shared git-mutation lock so this `dossier: cycle-N closeout`
	// commit never races a sibling lane's ship/dossier commit on .git/index.lock
	// (see defaultGitMutationLock for WHY it must be ship's exact lock file and
	// why this acquire cannot self-deadlock). Fail-OPEN: a lock-acquire error
	// (rare — flock/FS failure) must not orphan a best-effort dossier, so we
	// proceed unserialized. The backstop is only partial — commitPairGit retries
	// `git commit` on a busy index.lock but NOT the preceding `git add`
	// (dossier/write.go), so a concurrent collision there just skips this cycle's
	// dossier via the caller's non-fatal WARN. A rare lost closeout record beats
	// failing the cycle.
	if lock != nil {
		if release, lerr := lock(projectRoot); lerr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN dossier git-mutation lock: %v (proceeding unserialized; a concurrent index collision would skip this dossier)\n", lerr)
		} else {
			defer release()
		}
	}
	if err := dossier.Write(d, dir, true); err != nil {
		return fmt.Errorf("write dossier: %w", err)
	}
	return nil
}
