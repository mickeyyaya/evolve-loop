package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// goldenWorkspace is rendered into the artifact-path instruction and never touched on disk, so it needs no templating.
const goldenWorkspace = "/ws/cycle-42"

// goldenPersona is a stub, because the real agents/evolve-router.md would break the goldens on every persona edit.
const goldenPersona = "# evolve-router\nYou are the ROUTER persona stub (PERSONA_MARKER_42)."

// richRouteInput renders every advisor prompt section at once, so the goldens cover all of them.
func richRouteInput() router.RouteInput {
	return router.RouteInput{
		Current:   "build",
		Verdict:   "PASS",
		Cycle:     42,
		Completed: []string{"scout", "triage", "build"},
		Workspace: goldenWorkspace,
		Env:       map[string]string{"EVOLVE_CLI": "claude-tmux"},
		GoalText:  "  " + strings.Repeat("Refactor the auth token rotation with a security fix and a migration; ", 60) + "  ",
		Signals: router.RoutingSignals{
			Scout:  router.ScoutSignals{Present: true, CycleSizeEstimate: "medium", GoalType: "feature", DeliverableKind: "code", ItemCount: 3, CarryoverCount: 1, BacklogSize: 9},
			Triage: router.TriageSignals{Present: true, CycleSize: "small", DeliverableKind: "code", PhaseSkip: []string{"plan-review", "memo"}},
			Build:  router.BuildSignals{Present: true, Verdict: "PASS", ACSGreen: 5, ACSRed: 1, ACSRegression: 0, SeverityMax: router.SevHigh, FilesTouched: 4, DiffLOC: 120},
			Audit:  router.AuditSignals{Present: true, Verdict: "PASS", Confidence: 0.875, RedCount: 0},
		},
		Cfg: config.RoutingConfig{
			Mandatory:     []string{"scout", "build", "audit", "ship"},
			MaxInsertions: 4,
			ReconDigest:   false,
			Conditional: map[string]config.CondRule{
				"tdd":  {Field: "cycle_size", Op: "!=", Value: "trivial", And: []config.CondRule{{Field: "deliverable_kind", Op: "ne", Value: "document"}}},
				"memo": {Field: "audit.red_count", Op: "gt", Value: "0"},
			},
			Triggers: map[string]config.RoutingBlock{
				"tester": {InsertWhen: []config.Condition{
					{Field: "build.acs_red", Op: "gt", Value: 0},
					{Field: "build.severity_max", Op: "gte", Value: "HIGH"},
				}},
				"plan-review":         {RubricHint: []string{"a novel/cross-cutting goal warrants plan-review"}},
				"architecture-design": {RubricHint: []string{"a novel/cross-cutting goal also warrants architecture-design"}},
				"security-sweep":      {InsertWhen: []config.Condition{{Field: "scout.goal_type", Op: "eq", Value: "security"}}},
			},
		},
		BenchedCLIs: []router.BenchedCLI{
			{Family: "codex", Reason: "rate_limit", Until: time.Date(2026, 6, 11, 6, 13, 0, 0, time.UTC)},
			{Family: "agy", Reason: "quota_exhausted", Until: time.Date(2026, 6, 11, 18, 0, 0, 0, time.FixedZone("PDT", -7*3600))},
		},
		UnavailablePhases: []string{"security-sweep", "ghost-phase"},
		LastReason:        "EGPS red_count=3",
		Lessons:           []string{"inst-L001 (egps-red): run the suite first", "inst-L007 (worktree-drift): rebase before ship"},
		CarryoverTodos:    richCarryoverTodos(),
		Catalog:           richCatalog(),
		OnDemandPhases:    []string{"market-sizing", "okr-draft", "migration-safety-check"},
	}
}

// richCarryoverTodos mixes every priority spelling and both FirstSeenCycle orders to exercise rank, recency and ties.
func richCarryoverTodos() []router.CarryoverTodo {
	priorities := []string{"P0", "P1", "H", "HIGH", "P2", "P3", "M", "MED", "MEDIUM", "L", "LOW", "", "blocking", " p1 ", "p0", "P2", "P1", "P0", "LOW", "P3", "H", "P0", "M"}
	todos := make([]router.CarryoverTodo, 0, len(priorities))
	for i, p := range priorities {
		action := fmt.Sprintf("carryover action %02d", i)
		if i == 4 {
			action = strings.Repeat("é", 700) // 700 runes, capped at 600 in the prompt
		}
		todos = append(todos, router.CarryoverTodo{
			ID:             fmt.Sprintf("cycle-%d-todo-%02d", 40-(i%5), i),
			Action:         action,
			Priority:       p,
			FirstSeenCycle: 40 - (i % 5),
			CyclesUnpicked: i % 3,
		})
	}
	return todos
}

// richCatalog interleaves its cards so the stable partition is visible in the golden.
func richCatalog() []router.PhaseCard {
	return []router.PhaseCard{
		{Name: "scout", Role: "plan"},
		{Name: "bug-reproduction", Role: "evaluate", Optional: true, Categories: []string{"bugfix"}, WhenToUse: "bugfix cycles, before tdd/build"},
		{Name: "triage", Role: "plan", Optional: true},
		{Name: "build", Role: "build", WritesSource: true},
		{Name: "plan-review", Role: "evaluate", Optional: true, Description: "Reviews the plan for scope creep."},
		{Name: "memo", Role: "plan", Optional: true},
		{Name: "security-sweep", Role: "evaluate", Optional: true, WritesSource: true, Categories: []string{"security", "auth"}, WhenToUse: strings.Repeat("w", 141), AllowedCLIs: []string{"claude-tmux", "codex-tmux"}, ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Default: "deep", Max: "top"}},
		{Name: "audit", Role: "evaluate"},
		{Name: "perf-benchmark", Role: "evaluate", Optional: true, Description: "Benchmarks the hot path.", WhenToUse: "performance goals"},
		{Name: "retro", Role: "plan", Optional: true},
		{Name: "ship", Role: "control"},
		{Name: "doc-sync", Role: "build", Optional: true},
		{Name: "architecture-design", Role: "plan", Optional: true, Categories: []string{"design"}, Description: "Designs before building.", WhenToUse: "novel or cross-cutting goals"},
	}
}

// launchRouteInput carries a secret-shaped token in the goal so the capture golden covers redaction.
func launchRouteInput(ws, root, wt string) router.RouteInput {
	return router.RouteInput{
		Current:        "build",
		Verdict:        "PASS",
		Cycle:          7,
		Completed:      []string{"scout", "build"},
		Workspace:      ws,
		ProjectRoot:    root,
		ActiveWorktree: wt,
		Env:            map[string]string{"EVOLVE_CLI": "claude-tmux"},
		GoalText:       "rotate the leaked key sk-livesecret0123456789ABCDEF across the fleet",
		Signals:        router.RoutingSignals{Scout: router.ScoutSignals{Present: true, ItemCount: 5, CycleSizeEstimate: "large"}},
		Cfg:            config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}, MaxInsertions: 2},
	}
}
