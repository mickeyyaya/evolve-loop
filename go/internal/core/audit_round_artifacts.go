package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// supersedePreviousAuditRound retires the previous round's verdict artifacts and advances the dispatch counter.
// Both dispatch surfaces call it before the pre-phase cycle-state write, so a crashed round's verdict is retired on resume.
func supersedePreviousAuditRound(cs *CycleState) {
	round := cs.AuditDispatches
	if c := completedAuditRounds(cs.CompletedPhases); c > round {
		round = c
	}
	retireSupersededAuditArtifacts(cs.WorkspacePath, round)
	cs.AuditDispatches = round + 1
}

// completedAuditRounds is the round index for checkpoints persisted before AuditDispatches existed.
func completedAuditRounds(completed []string) int {
	n := 0
	for _, p := range completed {
		if Phase(p) == PhaseAudit {
			n++
		}
	}
	return n
}

// retireSupersededAuditArtifacts archives the superseded round's verdict artifacts so the fresh audit regenerates its
// verdict. When archiving fails it removes the stale file instead, because a surviving verdict would be replayed.
func retireSupersededAuditArtifacts(workspace string, round int) {
	if workspace == "" || round < 1 {
		return
	}
	for _, name := range []string{acssuite.VerdictFilename, phasecontract.ArtifactFilename(string(PhaseAudit))} {
		src := filepath.Join(workspace, name)
		dst := filepath.Join(workspace, phasecontract.RoundArchiveFilename(name, round))
		if _, err := os.Stat(dst); err == nil {
			// A counter rollback left the archive in place: never clobber the first retirement's evidence.
			rmErr := os.Remove(src)
			switch {
			case rmErr == nil:
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-repair: %s already exists; superseded %s removed, not archived — its evidence was lost\n", filepath.Base(dst), name)
			case !os.IsNotExist(rmErr):
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-repair: %s exists but stale %s could not be removed (%v) — the next audit round may read a stale verdict\n", filepath.Base(dst), name, rmErr)
			}
			continue
		}
		err := os.Rename(src, dst)
		if err == nil || os.IsNotExist(err) {
			continue
		}
		if rmErr := os.Remove(src); rmErr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-repair: could not retire superseded %s (rename: %v; remove: %v) — the next audit round may read a stale verdict\n", name, err, rmErr)
			continue
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-repair: superseded %s removed, not archived (rename: %v) — round-%d forensic evidence was lost\n", name, err, round)
	}
}
