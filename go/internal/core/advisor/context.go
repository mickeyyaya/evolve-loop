package advisor

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/textcap"
)

// MaxGoalTextRunes bounds the goal text rendered into the advisor prompt.
const MaxGoalTextRunes = 4000

// TruncateGoal trims s and caps it at MaxGoalTextRunes with a truncation marker.
func TruncateGoal(s string) string { return textcap.TruncateRunes(s, MaxGoalTextRunes) }

// WriteRoutingContext writes the deterministic decision context both the routing and the plan prompts share.
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

// writeGoal renders ahead of the per-cycle signals: the goal is stable across a run, so the prompt prefix stays cacheable.
func writeGoal(b *strings.Builder, in router.RouteInput) {
	if g := TruncateGoal(in.GoalText); g != "" {
		fmt.Fprintf(b, "## Goal\n%s\n\n", g)
	}
}

// writeCLIHealth lets the advisor plan around a benched family instead of discovering it one phase at a time.
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

func writeObjectiveSignals(b *strings.Builder, in router.RouteInput) {
	b.WriteString("## Objective signals (digested from handoff artifacts)\n")
	writeSignals(b, in.Signals)

	WriteCarryoverTodos(b, in.CarryoverTodos)

	writeRecallMemory(b, in)
}

func writeOptionalPhases(b *strings.Builder, in router.RouteInput) {
	if len(in.Cfg.Triggers) == 0 {
		return
	}
	b.WriteString("\n## Optional phases available (insert only on objective signal)\n")
	names := make([]string, 0, len(in.Cfg.Triggers))
	for name := range in.Cfg.Triggers {
		if slices.Contains(in.UnavailablePhases, name) {
			continue // a phase without a persona doc is not selectable
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(b, "- %s\n", name)
	}
}

// writeUnavailablePhases says why the persona-less phases are missing, so the advisor never proposes them from memory.
func writeUnavailablePhases(b *strings.Builder, in router.RouteInput) {
	if len(in.UnavailablePhases) == 0 {
		return
	}
	b.WriteString("\n## Unavailable phases (persona doc missing — NOT selectable this cycle)\n")
	for _, name := range in.UnavailablePhases {
		fmt.Fprintf(b, "- %s\n", name)
	}
}

// writeRubric hardcodes only the FORBIDDEN line: it is a kernel invariant, not phase data.
func writeRubric(b *strings.Builder, in router.RouteInput) {
	b.WriteString("\n## Decision rubric (justify each optional phase by an objective signal)\n")
	writeRubricLines(b, in.Cfg)
	b.WriteString("FORBIDDEN: never propose reaching ship without audit. Any justification for skipping audit is rejected by the kernel.\n")
}

// writeRecallMemory renders the last failure's reason and its matching lessons, which the orchestrator pre-computes.
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
