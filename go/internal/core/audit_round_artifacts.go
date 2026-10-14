package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func supersedePreviousAuditRound(cs *CycleState) {
	round := cs.AuditDispatches
	if c := completedAuditRounds(cs.CompletedPhases); c > round {
		round = c
	}
	retireSupersededAuditArtifacts(cs.WorkspacePath, round, os.Stderr)
	cs.AuditDispatches = round + 1
}

func completedAuditRounds(completed []string) int {
	n := 0
	for _, p := range completed {
		if Phase(p) == PhaseAudit {
			n++
		}
	}
	return n
}

func retireSupersededAuditArtifacts(workspace string, round int, warn io.Writer) {
	if workspace == "" || round < 1 {
		return
	}
	for _, name := range []string{acsverdict.Filename, phasecontract.ArtifactFilename(string(PhaseAudit))} {
		src := filepath.Join(workspace, name)
		dst := filepath.Join(workspace, phasecontract.RoundArchiveFilename(name, round))
		if _, err := os.Stat(dst); err == nil {
			rmErr := os.Remove(src)
			switch {
			case rmErr == nil:
				fmt.Fprintf(warn, "[orchestrator] WARN audit-repair: %s already exists; superseded %s removed, not archived — its evidence was lost\n", filepath.Base(dst), name)
			case !os.IsNotExist(rmErr):
				fmt.Fprintf(warn, "[orchestrator] WARN audit-repair: %s exists but stale %s could not be removed (%v) — the next audit round may read a stale verdict\n", filepath.Base(dst), name, rmErr)
			}
			continue
		}
		err := os.Rename(src, dst)
		if err == nil || os.IsNotExist(err) {
			continue
		}
		if rmErr := os.Remove(src); rmErr != nil {
			fmt.Fprintf(warn, "[orchestrator] WARN audit-repair: could not retire superseded %s (rename: %v; remove: %v) — the next audit round may read a stale verdict\n", name, err, rmErr)
			continue
		}
		fmt.Fprintf(warn, "[orchestrator] WARN audit-repair: superseded %s removed, not archived (rename: %v) — round-%d forensic evidence was lost\n", name, err, round)
	}
}
