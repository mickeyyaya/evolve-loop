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

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

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
func writeCycleDossier(projectRoot, workspacePath string, cycle int, goal, runID, outcome string, skipped []SkippedPhase) error {
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
	if err := dossier.Write(d, dir, true); err != nil {
		return fmt.Errorf("write dossier: %w", err)
	}
	return nil
}
