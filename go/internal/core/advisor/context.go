package advisor

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/textcap"
)

// MaxGoalTextRunes bounds the goal text rendered into the advisor prompt so an
// oversized operator-pasted goal cannot crowd the catalog + rubric out of the
// context window. Core projects it for the task-recall digest.
const MaxGoalTextRunes = 4000

// TruncateGoal trims surrounding whitespace and caps the goal at
// MaxGoalTextRunes (rune-safe), marking truncation. Empty/whitespace-only ⇒ ""
// (no Goal section). The rune rule is textcap's; the bound is the advisor's.
func TruncateGoal(s string) string { return textcap.TruncateRunes(s, MaxGoalTextRunes) }

// WriteRoutingContext writes the shared, deterministic decision context — cycle
// header, goal, CLI health, digested objective signals, carryover todos, recall
// memory, available optional phases, unavailable phases and the decision
// rubric, in that order — consumed by both the per-transition prompt and the
// whole-cycle plan prompt. Deterministic string ⇒ prompt-prefix cache friendly.
func WriteRoutingContext(b *strings.Builder, in router.RouteInput) {
	writeCycleHeader(b, in)
	writeGoal(b, in)
	writeCLIHealth(b, in)
	writeObjectiveSignals(b, in)
	writeOptionalPhases(b, in)
	writeUnavailablePhases(b, in)
	writeRubric(b, in)
}

func writeCycleHeader(b *strings.Builder, in router.RouteInput) {
	fmt.Fprintf(b, "## Cycle\n- cycle: %d\n- just_completed: %s\n- last_verdict: %s\n", in.Cycle, in.Current, in.Verdict)
	fmt.Fprintf(b, "- completed_phases: %s\n", strings.Join(in.Completed, ", "))
	fmt.Fprintf(b, "- mandatory_spine: %s\n", strings.Join(in.Cfg.Mandatory, ", "))
	fmt.Fprintf(b, "- max_optional_insertions: %d\n\n", in.Cfg.MaxInsertions)
}

// writeGoal renders the goal text (when threaded) — the brain's primary input
// for composing the cycle: it lets the advisor judge whether the work is
// novel/cross-cutting enough to warrant a design phase or a minted phase,
// instead of planning blind. Capped so an oversized operator-pasted goal
// cannot push the catalog/rubric out of the context window. Placed before the
// per-cycle signals: the goal is stable across a run's cycles, so a stable
// section stays ahead of the volatile ones.
func writeGoal(b *strings.Builder, in router.RouteInput) {
	if g := TruncateGoal(in.GoalText); g != "" {
		fmt.Fprintf(b, "## Goal\n%s\n\n", g)
	}
}

// writeCLIHealth renders the environmental CLI health: benched families mean
// dispatch chains start at their fallback — the advisor should plan around the
// degraded family (fewer inserts routed there; scope sized for the fallback
// carrying the cycle) instead of discovering it one phase at a time
// (cycle-283). Family-sorted; a quota wall is named WALLED/unavailable.
func writeCLIHealth(b *strings.Builder, in router.RouteInput) {
	if len(in.BenchedCLIs) == 0 {
		return
	}
	b.WriteString("## CLI health (environmental)\n")
	benched := append([]router.BenchedCLI(nil), in.BenchedCLIs...)
	sort.Slice(benched, func(i, j int) bool { return benched[i].Family < benched[j].Family })
	for _, e := range benched {
		label := "benched"
		if strings.Contains(strings.ToLower(e.Reason), "exhaust") {
			label = "WALLED/unavailable"
		}
		fmt.Fprintf(b, "- %s: %s (%s) until %s — its dispatch chains start at the fallback CLI\n",
			e.Family, label, e.Reason, e.Until.UTC().Format("15:04Z"))
	}
	b.WriteString("\n")
}

// writeObjectiveSignals renders the digested handoff signals, the carryover
// todos and the recall memory.
func writeObjectiveSignals(b *strings.Builder, in router.RouteInput) {
	b.WriteString("## Objective signals (digested from handoff artifacts)\n")
	writeSignals(b, in.Signals)

	WriteCarryoverTodos(b, in.CarryoverTodos)

	writeRecallMemory(b, in)
}

// writeOptionalPhases lists the trigger-declared optional phases minus the
// persona-less ones (not selectable — token-waste #2), sorted for a
// deterministic, prompt-prefix-cache-friendly prompt.
func writeOptionalPhases(b *strings.Builder, in router.RouteInput) {
	if len(in.Cfg.Triggers) == 0 {
		return
	}
	b.WriteString("\n## Optional phases available (insert only on objective signal)\n")
	names := make([]string, 0, len(in.Cfg.Triggers))
	for name := range in.Cfg.Triggers {
		if slices.Contains(in.UnavailablePhases, name) {
			continue // persona doc absent — not selectable (token-waste #2)
		}
		names = append(names, name)
	}
	sort.Strings(names) // deterministic prompt ⇒ prompt-prefix cache friendly
	for _, name := range names {
		fmt.Fprintf(b, "- %s\n", name)
	}
}

// writeUnavailablePhases names the persona-less phases so the advisor knows
// WHY they are absent from the menus above and never proposes them by memory
// (token-waste #2).
func writeUnavailablePhases(b *strings.Builder, in router.RouteInput) {
	if len(in.UnavailablePhases) == 0 {
		return
	}
	b.WriteString("\n## Unavailable phases (persona doc missing — NOT selectable this cycle)\n")
	for _, name := range in.UnavailablePhases {
		fmt.Fprintf(b, "- %s\n", name)
	}
}

// writeRubric renders the decision rubric — a PROJECTION of the structured
// routing data the kernel walks (failure floor Phase 4b). Only the FORBIDDEN
// line stays hardcoded — it is a kernel invariant, not phase data.
func writeRubric(b *strings.Builder, in router.RouteInput) {
	b.WriteString("\n## Decision rubric (justify each optional phase by an objective signal)\n")
	writeRubricLines(b, in.Cfg)
	b.WriteString("FORBIDDEN: never propose reaching ship without audit. Any justification for skipping audit is rejected by the kernel.\n")
}

// writeRecallMemory renders the WS2 recall section — the most recent failure's
// short reason and the prior lessons that match it — so the advisor plans WITH
// the benefit of what went wrong before (Reflexion-style recall). Both fields
// are pre-computed by the orchestrator (KB lookup is its I/O, not the advisor's),
// so this stays a pure deterministic render. Emits nothing when there is neither
// a reason nor a lesson, keeping the prompt prefix stable for the no-history case.
func writeRecallMemory(b *strings.Builder, in router.RouteInput) {
	if in.LastReason == "" && len(in.Lessons) == 0 {
		return
	}
	b.WriteString("\n## Recall memory (learn from prior cycles — do not repeat these)\n")
	if in.LastReason != "" {
		fmt.Fprintf(b, "- why the last cycle failed: %s\n", in.LastReason)
	}
	for _, lesson := range in.Lessons {
		fmt.Fprintf(b, "- lesson: %s\n", lesson)
	}
}

func writeSignals(b *strings.Builder, s router.RoutingSignals) {
	if s.Scout.Present {
		fmt.Fprintf(b, "- scout: cycle_size_estimate=%s goal_type=%s deliverable_kind=%s item_count=%d carryover=%d backlog=%d\n",
			s.Scout.CycleSizeEstimate, s.Scout.GoalType, s.Scout.DeliverableKind, s.Scout.ItemCount, s.Scout.CarryoverCount, s.Scout.BacklogSize)
	}
	if s.Triage.Present {
		fmt.Fprintf(b, "- triage: cycle_size=%s deliverable_kind=%s phase_skip=%s\n", s.Triage.CycleSize, s.Triage.DeliverableKind, strings.Join(s.Triage.PhaseSkip, ","))
	}
	if s.Build.Present {
		fmt.Fprintf(b, "- build: verdict=%s acs_green=%d acs_red=%d acs_regression=%d severity_max=%s files_touched=%d diff_loc=%d\n",
			s.Build.Verdict, s.Build.ACSGreen, s.Build.ACSRed, s.Build.ACSRegression, s.Build.SeverityMax, s.Build.FilesTouched, s.Build.DiffLOC)
	}
	if s.Audit.Present {
		fmt.Fprintf(b, "- audit: verdict=%s confidence=%.2f red_count=%d\n", s.Audit.Verdict, s.Audit.Confidence, s.Audit.RedCount)
	}
}
