package core

// audit_round_artifacts.go — round-scoped audit artifact retirement
// (cycle-1603, 2026-09-02).
//
// The auditor persona pre-writes acs-verdict.json and audit.Classify honors a
// pre-staged file (the verdict-exists gate skips regeneration). So ANY re-audit
// — the ADR-0092/0093 repair loop, a bookkeeping regrade, a ship-error
// recovery re-audit, a debugger RERUN_PHASE — that leaves the previous round's
// verdict at its canonical path replays SUPERSEDED evidence into the fresh
// round: in cycle-1603 round-1's ship_eligible=false amendment forced every
// repaired PASS back to FAIL, making the repair loop structurally unable to
// succeed. The belief "a re-dispatch of audit supersedes the previous round's
// audit evidence" lives at ONE seam: the audit pre-dispatch block, beside
// resetFloorFailReason, mirrored on both dispatch surfaces (cyclerun_dispatch
// and resume) — never at a branch site, which would cover only a subset of the
// re-audit paths and open an artifact blackout window across the re-entered
// tdd/build phases.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// supersedePreviousAuditRound is the ONE primitive both dispatch surfaces call
// from the audit pre-dispatch block (beside resetFloorFailReason, BEFORE the
// pre-phase cycle-state write): it retires the previous round's verdict
// artifacts and advances the persisted dispatch counter. The archive index is
// the greater of the persisted AuditDispatches counter (crash-correct: an
// interrupted round has already persisted its dispatch, so its dead attempt's
// pre-written verdict is retired on resume instead of honored) and the
// completion-derived count (legacy backstop: a pre-field checkpoint decodes
// AuditDispatches as 0 but its CompletedPhases still names the finished
// rounds). Index 0 — the cycle's true first dispatch — retires nothing, so an
// operator/CI pre-staged acs-verdict.json keeps the honor audit.Classify
// grants it.
func supersedePreviousAuditRound(cs *CycleState) {
	round := cs.AuditDispatches
	if c := completedAuditRounds(cs.CompletedPhases); c > round {
		round = c
	}
	retireSupersededAuditArtifacts(cs.WorkspacePath, round)
	cs.AuditDispatches = round + 1
}

// completedAuditRounds counts how many audit rounds have COMPLETED — the
// legacy backstop index for checkpoints persisted before AuditDispatches
// existed.
func completedAuditRounds(completed []string) int {
	n := 0
	for _, p := range completed {
		if Phase(p) == PhaseAudit {
			n++
		}
	}
	return n
}

// retireSupersededAuditArtifacts renames the superseded round's verdict
// artifacts to round-suffixed archives (acs-verdict.round<N>.json,
// audit-report.round<N>.md), so the fresh audit regenerates its verdict from
// execution rather than replaying the previous round's, while the old round's
// evidence survives for forensics (the cycle-1434 acs-verdict.foreign.json
// precedent). round is the completed-round count; 0 (first dispatch) is a
// no-op. Absent files are the normal fresh shape. A retirement that cannot
// archive falls back to removing the stale file — a stale verdict left behind
// silently recreates the cycle-1603 class — and every evidence-losing or
// failed outcome is reported loudly.
func retireSupersededAuditArtifacts(workspace string, round int) {
	if workspace == "" || round < 1 {
		return
	}
	for _, name := range []string{acssuite.VerdictFilename, phasecontract.ArtifactFilename(string(PhaseAudit))} {
		src := filepath.Join(workspace, name)
		dst := filepath.Join(workspace, phasecontract.RoundArchiveFilename(name, round))
		if _, err := os.Stat(dst); err == nil {
			// The archive already exists (a counter rollback — e.g. a lost
			// cycle-state write). Never clobber it — the first retirement
			// holds that index's evidence — but say so: the copy being
			// dropped may be a complete verdict, and a silent delete here is
			// the same forensic loss the rename-failure branch declares.
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
