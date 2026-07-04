package core

// carryover_prompt_order_test.go — RED tests (cycle 507, task
// fix-carryover-prompt-truncation-order). White-box (`package core`) because
// writeCarryoverTodos is unexported.
//
// writeCarryoverTodos (phase_advisor.go) is the SOLE injection site for
// carryoverTodos into any router/advisor prompt. It caps the rendered COUNT at
// maxCarryoverTodosInPrompt (20) but slices `todos[:20]` in on-disk INSERTION
// order (oldest-first). With 65 entries on disk today, the prompt permanently
// shows only ~cycles 366-402 and hides everything newer behind "... N more
// omitted" — including the two most severe still-open items (cycle 502
// SELF_SHA_TAMPERED, cycle 505 evolve-bin leak). Routing/advisor decisions have
// therefore been made blind to the last 100+ cycles of carryover.
//
// The fix: when the array exceeds the cap, render the HIGHEST-PRIORITY /
// MOST-RECENT entries, not a naive insertion-order prefix. These tests pin the
// observable outcome (severe+recent survives the cut; malformed priority is
// safe; the cap boundary is exact), not a specific comparator. RED now:
// insertion-order slicing hides the tail entry. Do NOT modify this file —
// implement the ordering in writeCarryoverTodos.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// AC (positive, the headline bug): given more than the cap, with the single
// most severe + most recent entry placed LAST in insertion order (exactly where
// today's todos[:20] slice drops it), the rendered prompt MUST still include it.
// A no-op insertion-order slice fails: the P0/cycle-505 entry is entry #21 and
// gets omitted.
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
	// The 21st entry: the newest and highest priority — the one insertion-order
	// slicing silently hides. This is the cycle-505 evolve-bin leak carryover.
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
	// The section must still cap the count and report the remainder — the fix
	// changes WHICH entries render, not that a cap exists.
	if got := strings.Count(out, "- ["); got != maxCarryoverTodosInPrompt {
		t.Errorf("count cap must still hold at %d rendered todos; got %d", maxCarryoverTodosInPrompt, got)
	}
	if !strings.Contains(out, "1 more carryover todo(s) omitted") {
		t.Errorf("with cap+1 entries exactly 1 must be reported omitted; got %q", out)
	}
}

// AC (negative / robustness): a malformed / unknown Priority string must not
// panic and must not drop the entry — an unrankable priority sorts to the
// bottom but the renderer stays total.
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

// AC (boundary): exactly the cap renders no "omitted" trailer; cap+1 renders it
// reporting exactly one remainder. Pins the off-by-one at the truncation edge.
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
