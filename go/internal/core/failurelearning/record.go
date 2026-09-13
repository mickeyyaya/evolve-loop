package failurelearning

import (
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const (
	maxAdoptedDefects     = 20                       // entries per adopted list
	maxAdoptedDefectRunes = carryover.MaxActionRunes // runes per adopted entry — the carryover unit's cap, projected
)

// RecordFailedApproach records the learn-from-failure STATE for a failed
// phase over the SAME state (mutated in place, as the callers that read the
// mutations back require): the FailedRecord appended to FailedAt, a deduped P0
// carryover todo through the Lifecycle, LastCycleNumber stamped — and returns
// the summary, todo id and adopted structured block for callers that continue
// with a retro or the floor. It never persists: that is the caller's concern.
// A nil state is a programming error (callers guarantee it) and panics.
func (e *Engine) RecordFailedApproach(state *cyclestate.State, f Failure) Learned {
	summary := carryover.Summary(f.Cycle, f.Phase, f.Err)
	todoID := fmt.Sprintf("cycle-%d-failed-%s", f.Cycle, f.Phase)
	now := e.now().UTC()
	nowTS := now.Format(time.RFC3339)
	record := cyclestate.FailedRecord{
		TS:             nowTS,
		Cycle:          f.Cycle,
		Verdict:        cyclestate.VerdictFAIL,
		Classification: cyclestate.ClassificationMidExecutionFail,
		RecordedAt:     nowTS,
		Summary:        summary,
		Defects:        []string{summary},
		Retrospected:   true,
	}
	// ADR-0039 §7: a phase healthy enough to self-report owns its failure
	// description — its structured block beats the supervisor's synthesis.
	// Read ONCE here and threaded to the floor, so state.json and the lesson
	// artifacts can never diverge on the same failure event.
	structured := adoptStructuredFailure(f.Workspace, string(f.Phase))
	if structured != nil {
		record.Classification = structured.Class
		if len(structured.Defects) > 0 {
			record.Defects = structured.Defects
		}
	}
	// The TTL from the FINAL classification, computed once and shared so the
	// todo inherits the record's stamp rather than re-deriving it.
	record.ExpiresAt = failurelog.ComputeExpiresAt(failurelog.NormalizeLegacy(record.Classification), now)
	e.todos.Append(state, cyclestate.CarryoverTodo{
		ID: todoID, Action: summary, Priority: carryover.PriorityBlocking,
		FirstSeenCycle: f.Cycle, ExpiresAt: record.ExpiresAt,
	})
	state.FailedAt = append(state.FailedAt, record)
	state.LastCycleNumber = f.Cycle
	return Learned{Summary: summary, TodoID: todoID, Structured: structured}
}

// adoptStructuredFailure is the trust boundary for agent-written failure
// blocks (ADR-0039 §7): adopt the failed phase's self-report ONLY when its
// class normalizes into the canonical taxonomy (never blind trust — an
// out-of-taxonomy class would round-trip to UnknownClassification on the next
// state read), and cap list/entry sizes so a misbehaving agent cannot bloat
// state.json or the lesson corpus.
func adoptStructuredFailure(workspace, phase string) *phasecontract.FailureBlock {
	fb, ok := phasecontract.ReadFailureBlock(workspace, phase)
	if !ok {
		return nil
	}
	if failurelog.NormalizeLegacy(fb.Class) == failurelog.UnknownClassification {
		return nil
	}
	fb.Defects = capStrings(fb.Defects, maxAdoptedDefects, maxAdoptedDefectRunes)
	fb.EvidencePaths = capStrings(fb.EvidencePaths, maxAdoptedDefects, maxAdoptedDefectRunes)
	return fb
}

// capStrings bounds an agent-written string list at the adoption boundary.
func capStrings(in []string, maxEntries, maxRunes int) []string {
	if len(in) > maxEntries {
		in = in[:maxEntries]
	}
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = carryover.CapRunes(s, maxRunes)
	}
	return out
}
