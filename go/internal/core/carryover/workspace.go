package carryover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// memoTodo is the shape evolve-memo and the retro path write to
// <workspace>/carryover-todos.json; evidence_pointer is decoded and dropped.
type memoTodo struct {
	ID              string `json:"id"`
	Action          string `json:"action"`
	Priority        string `json:"priority"`
	EvidencePointer string `json:"evidence_pointer"`
}

// MergeMemo merges the memo's queued follow-up todos into the state: trimmed,
// id/action-less entries skipped, priority defaulted, the action capped, the
// default expiry stamped; the union is by id, disk first. An absent file is a
// silent no-op; a read fault or a malformed file is one WARN and merges nothing.
func (l *Lifecycle) MergeMemo(state *cyclestate.State, workspace string, cycle int, now time.Time) {
	if state == nil || strings.TrimSpace(workspace) == "" {
		return
	}
	var memo []memoTodo
	if !l.decodeDocument(filepath.Join(workspace, "carryover-todos.json"), "carryover-todos.json", cycle, "Lifecycle.MergeMemo", &memo) {
		return
	}
	expiresAt := defaultExpiresAt(now)
	incoming := make([]cyclestate.CarryoverTodo, 0, len(memo))
	for _, m := range memo {
		id, action := strings.TrimSpace(m.ID), strings.TrimSpace(m.Action)
		if id == "" || action == "" {
			continue // tolerant decode: skip id/action-less entries
		}
		priority := strings.TrimSpace(m.Priority)
		if priority == "" {
			priority = PriorityMemoDefault
		}
		incoming = append(incoming, mintWorkspaceTodo(id, action, priority, cycle, expiresAt))
	}
	state.CarryoverTodos = MergeTodos(state.CarryoverTodos, incoming)
}

// MergePrescriptions carries a WARN-shipped audit's OPEN "PRESCRIPTION: "
// ledger rows into the state at PriorityPrescription (the prefix stays inside
// the action); FIXED/DEFERRED rows and ordinary defects are left alone. Same
// read, stamp and union rules as MergeMemo.
func (l *Lifecycle) MergePrescriptions(state *cyclestate.State, workspace string, cycle int, now time.Time) {
	if state == nil || strings.TrimSpace(workspace) == "" {
		return
	}
	var doc defectledger.Doc // the ledger's own wire shape (ADR-0103 unit 09); decoded through THIS reader so a fault stays a carryover code
	if !l.decodeDocument(filepath.Join(workspace, defectledger.LedgerFile), defectledger.LedgerFile, cycle, "Lifecycle.MergePrescriptions", &doc) {
		return
	}
	expiresAt := defaultExpiresAt(now)
	incoming := make([]cyclestate.CarryoverTodo, 0, len(doc.Entries))
	for _, e := range doc.Entries {
		if strings.TrimSpace(e.Status) != defectledger.StatusOpen || !strings.HasPrefix(e.Text, PrescriptionPrefix) {
			continue // resolved prescriptions never re-nag; ordinary defects belong to the ancestor reconciliation
		}
		id, action := strings.TrimSpace(e.ID), strings.TrimSpace(e.Text)
		if id == "" || action == "" {
			continue
		}
		incoming = append(incoming, mintWorkspaceTodo(id, action, PriorityPrescription, cycle, expiresAt))
	}
	state.CarryoverTodos = MergeTodos(state.CarryoverTodos, incoming)
}

// mintWorkspaceTodo is the ONE shape of a todo carried in from a workspace
// document: the action capped at MaxActionRunes, first seen this cycle, the
// default expiry stamped. Both merges mint through it.
func mintWorkspaceTodo(id, action, priority string, cycle int, expiresAt string) cyclestate.CarryoverTodo {
	return cyclestate.CarryoverTodo{ID: id, Action: CapRunes(action, MaxActionRunes), Priority: priority, FirstSeenCycle: cycle, ExpiresAt: expiresAt}
}

// decodeDocument is the ONE tolerant reader of a cycle-workspace document:
// absent → false, silently; a read fault → CARRYOVER_WORKSPACE_READ_FAILED;
// a decode error → CARRYOVER_WORKSPACE_MALFORMED; else true.
func (l *Lifecycle) decodeDocument(path, document string, cycle int, origin string, into any) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			l.warn(origin, cycle, CodeWorkspaceReadFailed, document+" read failed: "+err.Error(), map[string]string{"path": path, "document": document})
		}
		return false
	}
	if err := json.Unmarshal(raw, into); err != nil {
		l.warn(origin, cycle, CodeWorkspaceMalformed, document+" malformed (skipping): "+err.Error(), map[string]string{"path": path, "document": document})
		return false
	}
	return true
}

// RetireTriageDropped removes the todos triage's decision named in dropped[]
// (id-only, reason-blind); absent or malformed decisions retire nothing,
// silently, and survivors keep their order.
func (l *Lifecycle) RetireTriageDropped(state *cyclestate.State, workspace string) {
	if state == nil || workspace == "" || len(state.CarryoverTodos) == 0 {
		return
	}
	dropped := triageDroppedIDs(workspace)
	if len(dropped) == 0 {
		return
	}
	retire := make(map[string]bool, len(dropped))
	for _, id := range dropped {
		retire[id] = true
	}
	kept := make([]cyclestate.CarryoverTodo, 0, len(state.CarryoverTodos))
	for _, t := range state.CarryoverTodos {
		if !retire[t.ID] {
			kept = append(kept, t)
		}
	}
	state.CarryoverTodos = kept
}

// triageDroppedIDs reads the non-blank ids of triage-decision.json's dropped[];
// nil on absence or any decode failure. One of three readers of dropped[]
// (inboxmover.ClosedDroppedIDs, committedset.DispositionsFrom — see the note on
// ClosedDroppedIDs); unifying them behind one declaration in the stdlib-only
// committedset leaf is follow-up decision-document-single-declaration (the
// import cycle F8 cited no longer applies).
func triageDroppedIDs(workspace string) []string {
	body, err := os.ReadFile(filepath.Join(workspace, "triage-decision.json"))
	if err != nil {
		return nil
	}
	var d struct {
		Dropped []struct {
			ID string `json:"id"`
		} `json:"dropped"`
	}
	if json.Unmarshal(body, &d) != nil {
		return nil
	}
	var out []string
	for _, e := range d.Dropped {
		if id := strings.TrimSpace(e.ID); id != "" {
			out = append(out, id)
		}
	}
	return out
}
