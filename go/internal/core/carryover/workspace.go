package carryover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

type memoTodo struct {
	ID              string `json:"id"`
	Action          string `json:"action"`
	Priority        string `json:"priority"`
	EvidencePointer string `json:"evidence_pointer"`
}

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
			continue
		}
		priority := strings.TrimSpace(m.Priority)
		if priority == "" {
			priority = PriorityMemoDefault
		}
		incoming = append(incoming, mintWorkspaceTodo(id, action, priority, cycle, expiresAt))
	}
	state.CarryoverTodos = MergeTodos(state.CarryoverTodos, incoming)
}

func (l *Lifecycle) MergePrescriptions(state *cyclestate.State, workspace string, cycle int, now time.Time) {
	if state == nil || strings.TrimSpace(workspace) == "" {
		return
	}
	var doc defectledger.Doc
	if !l.decodeDocument(filepath.Join(workspace, defectledger.LedgerFile), defectledger.LedgerFile, cycle, "Lifecycle.MergePrescriptions", &doc) {
		return
	}
	expiresAt := defaultExpiresAt(now)
	incoming := make([]cyclestate.CarryoverTodo, 0, len(doc.Entries))
	for _, e := range doc.Entries {
		if strings.TrimSpace(e.Status) != defectledger.StatusOpen || !strings.HasPrefix(e.Text, PrescriptionPrefix) {
			continue
		}
		id, action := strings.TrimSpace(e.ID), strings.TrimSpace(e.Text)
		if id == "" || action == "" {
			continue
		}
		incoming = append(incoming, mintWorkspaceTodo(id, action, PriorityPrescription, cycle, expiresAt))
	}
	state.CarryoverTodos = MergeTodos(state.CarryoverTodos, incoming)
}

func mintWorkspaceTodo(id, action, priority string, cycle int, expiresAt string) cyclestate.CarryoverTodo {
	return cyclestate.CarryoverTodo{ID: id, Action: CapRunes(action, MaxActionRunes), Priority: priority, FirstSeenCycle: cycle, ExpiresAt: expiresAt}
}

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

func triageDroppedIDs(workspace string) []string {
	body, err := os.ReadFile(filepath.Join(workspace, "triage-decision.json"))
	if err != nil {
		return nil
	}
	var d committedset.DroppedBucket
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
