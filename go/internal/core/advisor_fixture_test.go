package core

// advisor_fixture_test.go — the ONE rich RouteInput the unit-04 goldens
// (ADR-0103, docs/architecture/decomposition/04-advisor.md §6 step 1) are
// captured over on the pre-extraction code and replayed through the leaf:
// every prompt section rendered at once — a 13-card catalog spanning the
// three enrichment buckets, three on-demand names, 23 carryover todos across
// every priority spelling with one 700-rune action, two benches (one walled),
// recall memory, two unavailable phases, conditional rules + triggers +
// rubric hints, a 4100-rune goal and all four signal blocks.

import (
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// goldenWorkspace is the literal workspace the prompt goldens carry: the
// composer renders it into the absolute artifact-path instruction and never
// touches the disk, so no templating is needed.
const goldenWorkspace = "/ws/cycle-42"

// goldenPersona is the stub persona the persona-path goldens compose over —
// the real agents/evolve-router.md would break on every persona edit.
const goldenPersona = "# evolve-router\nYou are the ROUTER persona stub (PERSONA_MARKER_42)."

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

// richCarryoverTodos: 23 todos over every priority spelling the rank table
// knows plus the malformed ones, with descending/ascending FirstSeenCycle so
// the rank-then-recency order and the stable ties are all exercised.
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

// richCatalog: 13 cards — 5 Optional with metadata, 4 Optional without, 4
// spine — in a deliberately interleaved order so the stable partition is
// visible in the golden. One card carries every guardrail line.
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

// launchRouteInput is the compact input the launch-request and capture
// goldens use: the fields the launcher threads (workspace, worktree, root,
// cycle, env) plus a secret-shaped token in the goal so the persisted prompt's
// redaction is part of the capture golden.
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
