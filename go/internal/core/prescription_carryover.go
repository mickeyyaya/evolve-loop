package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

// prescriptionLedgerPrefix is the exact tag emitDefectLedger already writes
// (go/internal/phases/audit/defect_ledger.go:181) onto a WARN-carried
// structured Prescription[] entry's Text field. Only OPEN rows carrying this
// prefix are this hook's business — an ordinary structured defect row (no
// prefix) is already governed by reconcileAgainstAncestor and must be left
// alone here.
const prescriptionLedgerPrefix = "PRESCRIPTION: "

// prescriptionLedgerEntry mirrors the on-disk shape emitDefectLedger writes to
// <workspace>/defect-ledger.json: {"origin_cycle": N, "entries": [{id, text,
// status, evidence, reason}, ...]}. Only id/text/status are consumed here.
type prescriptionLedgerEntry struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

type prescriptionLedgerDoc struct {
	OriginCycle int                       `json:"origin_cycle"`
	Entries     []prescriptionLedgerEntry `json:"entries"`
}

// MergeWorkspacePrescriptionCarryover closes the F3 class gap
// (batch-integrity-review-2026-08-04.md): emitDefectLedger already tags a
// WARN-carried structured prescription as an OPEN "PRESCRIPTION: <text>" row
// in <workspace>/defect-ledger.json, and reconcileAgainstAncestor already
// blocks PASS on any unaccounted OPEN row — but only when the current cycle
// is formally bound as a continuation of the ledger-holding cycle. An
// ordinary next-lane ship (the common case — triage/scout pick lanes by task
// content, not by ledger lineage) never reconciles, so the prescription is
// silently dropped at ship. cycle-1258's own prescription (materialize
// .evolve/evals/artifact-ready-crosspoll-debounce.md, git add -f past
// .gitignore) is the live instance this closed.
//
// This cycle-terminal hook (wired in finalizeCycle beside
// MergeWorkspaceCarryover, the pattern it mirrors) tolerant-decodes
// <workspace>/defect-ledger.json, keeps only Status=="OPEN" entries whose
// Text carries the prescriptionLedgerPrefix, and merges one CarryoverTodo per
// surviving entry into state.CarryoverTodos — so every future WARN
// prescription reaches the next scout through the already-mandatory
// carryoverTodos flow, regardless of continuation binding. A FIXED or
// DEFERRED prescription is already resolved and must not re-surface. A
// missing or malformed ledger is a no-op (WARN to stderr on malformed, never
// aborting the cycle) — mirroring MergeWorkspaceCarryover's tolerant-decode
// discipline for the same class of best-effort, non-critical artifact.
// Dedup is by id via the existing mergeCarryoverTodos, so re-entry
// (crash-resume / double-invocation) is idempotent.
func MergeWorkspacePrescriptionCarryover(state *State, workspacePath string, cycle int, now time.Time) {
	if state == nil || strings.TrimSpace(workspacePath) == "" {
		return
	}
	path := filepath.Join(workspacePath, "defect-ledger.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN prescription-carryover: read %s: %v\n", path, err)
		}
		return // absent ledger is a no-op
	}
	var doc prescriptionLedgerDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN prescription-carryover: malformed %s (skipping): %v\n", path, err)
		return
	}

	// Same default TTL the loop-start backfill stamps and MergeWorkspaceCarryover
	// uses, so all carryover-todo write paths share one aging discipline.
	expiresAt := now.Add(failurelog.DefaultCarryoverBackfillTTL).Format(time.RFC3339)
	incoming := make([]CarryoverTodo, 0, len(doc.Entries))
	for _, e := range doc.Entries {
		if strings.TrimSpace(e.Status) != "OPEN" {
			continue // FIXED/DEFERRED prescriptions are already resolved — never re-nag
		}
		if !strings.HasPrefix(e.Text, prescriptionLedgerPrefix) {
			continue // ordinary structured defect, not a WARN prescription — leave to reconcileAgainstAncestor
		}
		id := strings.TrimSpace(e.ID)
		action := strings.TrimSpace(e.Text)
		if id == "" || action == "" {
			continue // tolerant decode: skip id/text-less entries
		}
		incoming = append(incoming, CarryoverTodo{
			ID:             id,
			Action:         capRunes(action, maxAdoptedDefectRunes),
			Priority:       "high", // an unenforced audit prescription is a known-live gap, not routine follow-up
			FirstSeenCycle: cycle,
			ExpiresAt:      expiresAt,
		})
	}
	state.CarryoverTodos = mergeCarryoverTodos(state.CarryoverTodos, incoming)
}
