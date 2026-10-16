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
	maxAdoptedDefects     = 20
	maxAdoptedDefectRunes = carryover.MaxActionRunes
)

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
	structured := adoptStructuredFailure(f.Workspace, string(f.Phase))
	if structured != nil {
		record.Classification = structured.Class
		if len(structured.Defects) > 0 {
			record.Defects = structured.Defects
		}
	}
	record.ExpiresAt = failurelog.ComputeExpiresAt(failurelog.NormalizeLegacy(record.Classification), now)
	e.todos.Append(state, cyclestate.CarryoverTodo{
		ID: todoID, Action: summary, Priority: carryover.PriorityBlocking,
		FirstSeenCycle: f.Cycle, ExpiresAt: record.ExpiresAt,
	})
	state.FailedAt = append(state.FailedAt, record)
	state.LastCycleNumber = f.Cycle
	return Learned{Summary: summary, TodoID: todoID, Structured: structured}
}

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
