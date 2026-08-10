package core

// disposition_seed.go — the disposition-skeleton preseed (2026-08-10
// investigation; inbox disposition-skeleton-preseed). At continuation
// adoption the orchestrator already knows the ancestor's OPEN defect ids, so
// it writes <workspace>/defect-dispositions.json as a skeleton — one
// status-OPEN entry per inherited OPEN id — and the auditor only UPGRADES
// entries to FIXED (resolving evidence) or DEFERRED (reason).
//
// Gate semantics are untouched and unweakened: the disposition preflight sees
// the file as present and covering (never MISSING/INCOMPLETE), while the
// per-id reconcile rejects status OPEN ("not FIXED or DEFERRED") — a seeded
// entry the auditor never touches still blocks the cycle by name. A seeded
// DEFERRED would have been laundering; OPEN is the only honest seed.
//
// Known interaction (accepted, documented): the seeded file also satisfies
// the audit phase's Phase-B secondary-artifact hold immediately, so session
// teardown no longer waits for the UPGRADE — the per-id gate plus the
// ADR-0086 bookkeeping regrade cover an auditor that finishes without
// upgrading.
//
// The ancestor-ledger read here duplicates the audit package's wire shape by
// necessity (audit imports core; core cannot import audit). The two readers
// are pinned against each other by
// phases/audit/disposition_seed_singlesource_test.go, which feeds one real
// ledger document through both and asserts the same OPEN id set.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// seededDisposition is one skeleton entry. Text rides along for the auditor
// (the gate's decoder ignores unknown fields); Reason is pre-created empty so
// the upgrade is a value edit, not a shape edit.
type seededDisposition struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Text   string `json:"text,omitempty"`
	Reason string `json:"reason"`
}

// SeedDispositionSkeleton writes the skeleton at adoption. Every failure mode
// is a silent no-op by design: the seed is convenience, the defect-ledger
// GATE remains the loud enforcement on every one of these conditions
// (unreadable ledger ⇒ blocking diagnostic; absent skeleton ⇒ MISSING).
// An existing dispositions file is never clobbered — a prior attempt's or an
// agent's own file wins.
func SeedDispositionSkeleton(workspace, projectRoot string, ancestorCycle int) {
	if workspace == "" || projectRoot == "" {
		return
	}
	target := filepath.Join(workspace, "defect-dispositions.json")
	if _, err := os.Stat(target); err == nil {
		return
	}
	raw, err := os.ReadFile(filepath.Join(projectRoot, ".evolve", "runs", "cycle-"+strconv.Itoa(ancestorCycle), "defect-ledger.json"))
	if err != nil {
		return
	}
	var ledger struct {
		Entries []struct {
			ID     string `json:"id"`
			Text   string `json:"text"`
			Status string `json:"status"`
		} `json:"entries"`
	}
	if json.Unmarshal(raw, &ledger) != nil {
		return
	}
	var seeds []seededDisposition
	for _, e := range ledger.Entries {
		if e.Status != "OPEN" {
			continue
		}
		seeds = append(seeds, seededDisposition{ID: e.ID, Status: "OPEN", Text: e.Text})
	}
	if len(seeds) == 0 {
		return
	}
	if atomicwrite.JSON(target, map[string][]seededDisposition{"dispositions": seeds}) != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] continuation adoption seeded %s with %d OPEN disposition entr(ies) from cycle-%d — auditor upgrades each to FIXED/DEFERRED\n", target, len(seeds), ancestorCycle)
}
