package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestWriteCarryoverTodos_SevereRecentSurvivesTheCut(t *testing.T) {
	todos := make([]router.CarryoverTodo, 0, maxCarryoverTodosInPrompt+1)
	// 20 low-priority, oldest entries first (insertion order == on-disk order).
	for i := 0; i < maxCarryoverTodosInPrompt; i++ {
		todos = append(todos, router.CarryoverTodo{
			ID:             fmt.Sprintf("cycle-%d-failed-scout", 366+i),
			Action:         "transient historical failure-log echo",
			Priority:       "L",
			FirstSeenCycle: 366 + i,
		})
	}
	// The 21st entry, last in insertion order, is the newest and most severe.
	const critical = "cycle-505-failed-changelog-sync"
	todos = append(todos, router.CarryoverTodo{
		ID:             critical,
		Action:         "evolve-bin leaked into main tree; boot must quarantine",
		Priority:       "P0",
		FirstSeenCycle: 505,
	})

	var b strings.Builder
	writeCarryoverTodos(&b, todos)
	out := b.String()

	if !strings.Contains(out, critical) {
		t.Errorf("the most severe + most recent carryover todo (%s) must survive the top-%d cut; insertion-order slicing hides it.\n---\n%s", critical, maxCarryoverTodosInPrompt, out)
	}
	if got := strings.Count(out, "- ["); got != maxCarryoverTodosInPrompt {
		t.Errorf("count cap must still hold at %d rendered todos; got %d", maxCarryoverTodosInPrompt, got)
	}
	if !strings.Contains(out, "1 more carryover todo(s) omitted") {
		t.Errorf("with cap+1 entries exactly 1 must be reported omitted; got %q", out)
	}
}

func TestWriteCarryoverTodos_MalformedPriorityDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("writeCarryoverTodos panicked on a malformed Priority: %v", r)
		}
	}()
	var b strings.Builder
	writeCarryoverTodos(&b, []router.CarryoverTodo{
		{ID: "cycle-502-failed-ship", Action: "sha tampered", Priority: "P0", FirstSeenCycle: 502},
		{ID: "cycle-499-weird", Action: "garbage priority", Priority: "¿not-a-priority?", FirstSeenCycle: 499},
	})
	out := b.String()
	if !strings.Contains(out, "cycle-502-failed-ship") || !strings.Contains(out, "cycle-499-weird") {
		t.Errorf("both entries (including the malformed-priority one) must render; got %q", out)
	}
}

func TestWriteCarryoverTodos_CapBoundaryExact(t *testing.T) {
	mk := func(n int) []router.CarryoverTodo {
		out := make([]router.CarryoverTodo, n)
		for i := range out {
			out[i] = router.CarryoverTodo{ID: fmt.Sprintf("t-%d", i), Action: "a", Priority: "P0", FirstSeenCycle: 400 + i}
		}
		return out
	}

	var atCap strings.Builder
	writeCarryoverTodos(&atCap, mk(maxCarryoverTodosInPrompt))
	if strings.Contains(atCap.String(), "omitted") {
		t.Errorf("exactly %d todos must render no omitted-count trailer; got %q", maxCarryoverTodosInPrompt, atCap.String())
	}

	var overCap strings.Builder
	writeCarryoverTodos(&overCap, mk(maxCarryoverTodosInPrompt+1))
	if !strings.Contains(overCap.String(), "1 more carryover todo(s) omitted") {
		t.Errorf("cap+1 todos must report exactly 1 omitted; got %q", overCap.String())
	}
}
