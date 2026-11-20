package carryover

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
)

func (l *Lifecycle) RetireDocumentedJudgments(state *cyclestate.State, documentOnMain func(id string) bool) {
	if state == nil || documentOnMain == nil || len(state.CarryoverTodos) == 0 {
		return
	}
	lessons := recoveryretro.JudgmentLessons(state.CarryoverTodos)
	if len(lessons) == 0 {
		return
	}
	var documented []string
	for _, lesson := range lessons {
		if documentOnMain(lesson.ID) {
			documented = append(documented, lesson.ID)
		}
	}
	if len(documented) > 0 {
		state.CarryoverTodos = Retire(state.CarryoverTodos, documented)
	}
}
