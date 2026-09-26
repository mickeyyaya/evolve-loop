package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
)

// Reason is pre-created empty so the auditor's upgrade is a value edit, not a
// shape edit.
type seededDisposition struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Text   string `json:"text,omitempty"`
	Reason string `json:"reason"`
}

// SeedDispositionSkeleton writes a status-OPEN disposition skeleton for a
// continuation's inherited defects, or no-ops (never clobbering an existing
// dispositions file); the defect-ledger gate is the loud enforcement.
func SeedDispositionSkeleton(workspace, projectRoot string, ancestorCycle int) {
	if workspace == "" || projectRoot == "" {
		return
	}
	target := filepath.Join(workspace, defectledger.DispositionsFile)
	if _, err := os.Stat(target); err == nil {
		return
	}
	ledger, ok, err := defectledger.Read(RunWorkspacePath(projectRoot, ancestorCycle))
	if err != nil || !ok {
		return
	}
	var seeds []seededDisposition
	for _, e := range ledger.OpenEntries() {
		seeds = append(seeds, seededDisposition{ID: e.ID, Status: defectledger.StatusOpen, Text: e.Text})
	}
	if len(seeds) == 0 {
		return
	}
	if atomicwrite.JSON(target, map[string][]seededDisposition{"dispositions": seeds}) != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] continuation adoption seeded %s with %d OPEN disposition entr(ies) from cycle-%d — auditor upgrades each to FIXED/DEFERRED\n", target, len(seeds), ancestorCycle)
}
