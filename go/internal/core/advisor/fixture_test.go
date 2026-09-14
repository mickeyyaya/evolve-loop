package advisor

// fixture_test.go — the ONE rich RouteInput the unit-04 goldens
// (ADR-0103, docs/architecture/decomposition/04-advisor.md §6 step 1) were
// captured over on the pre-extraction code and are replayed through the leaf:
// every prompt section rendered at once — a 13-card catalog spanning the
// three enrichment buckets, three on-demand names, 23 carryover todos across
// every priority spelling with one 700-rune action, two benches (one walled),
// recall memory, two unavailable phases, conditional rules + triggers +
// rubric hints, a 4100-rune goal and all four signal blocks.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
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

// baseRouteInput is the compact input the launch tests use (the core
// helper's shape, so moved tests keep their spelling).
func baseRouteInput() router.RouteInput {
	return router.RouteInput{
		Current:     "build",
		Verdict:     "PASS",
		Workspace:   "/tmp/ws",
		ProjectRoot: "/proj",
		Cycle:       7,
		Env:         map[string]string{"EVOLVE_CLI": "claude-tmux"},
	}
}

// scriptedResp is one scripted launcher reply.
type scriptedResp struct {
	resp LaunchResponse
	err  error
}

// fakeLauncher records every request and replies with the canned stdout, or
// with one scripted reply per call in order (clamped to the last entry once
// exhausted) — the core fakeBridge and sequencedBridge folded into one.
type fakeLauncher struct {
	stdout     string
	err        error
	durationMS int64
	tokens     cyclestate.TokenUsage
	seq        []scriptedResp
	gotReq     LaunchRequest
	reqs       []LaunchRequest
	calls      int
}

func (f *fakeLauncher) Launch(_ context.Context, req LaunchRequest) (LaunchResponse, error) {
	f.calls++
	f.gotReq = req
	f.reqs = append(f.reqs, req)
	if len(f.seq) > 0 {
		i := min(f.calls-1, len(f.seq)-1)
		return f.seq[i].resp, f.seq[i].err
	}
	if f.err != nil {
		return LaunchResponse{}, f.err
	}
	return LaunchResponse{Stdout: f.stdout, DurationMS: f.durationMS, Tokens: f.tokens}, nil
}

func (f *fakeLauncher) calledCLIs() []string {
	out := make([]string, len(f.reqs))
	for i, r := range f.reqs {
		out[i] = r.CLI
	}
	return out
}

// refusingLauncher fails the test if the advisor launches through it.
type refusingLauncher struct{ t *testing.T }

func (r refusingLauncher) Launch(context.Context, LaunchRequest) (LaunchResponse, error) {
	r.t.Fatal("the launcher must not be reached")
	return LaunchResponse{}, nil
}

// defaultIdentity mirrors core's NewPhaseAdvisor default.
func defaultIdentity() Identity {
	return Identity{CLI: "claude-tmux", Model: "opus", AgentLabel: "router"}
}

// observed builds an advisor reporting into a recording Center; the writer is
// the plain os.WriteFile (a temp workspace) unless the test injects one.
func observed(t *testing.T, l Launcher, id Identity, opts ...Option) (*Advisor, *[]signalcenter.Event) {
	t.Helper()
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	opts = append(opts, WithSignals(func() *signalcenter.Center { return c }))
	return New(l, id, plainWriter, opts...), got
}

// plainWriter is the os.WriteFile-shaped capture writer the leaf tests use
// (core injects its atomic writer in production).
func plainWriter(path string, data []byte) error { return os.WriteFile(path, data, 0o644) }

// eventsWithCode filters a recording by code.
func eventsWithCode(got []signalcenter.Event, code signalcenter.Code) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range got {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

// tempInput is baseRouteInput over a temp workspace, so a wired Center sees
// no capture-write faults on the happy path.
func tempInput(t *testing.T) router.RouteInput {
	t.Helper()
	in := baseRouteInput()
	in.Workspace = t.TempDir()
	return in
}

func planJSON() string { return `[{"phase":"scout","run":true,"justification":"x"}]` }

func readArtifact(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read capture artifact %s: %v", path, err)
	}
	return string(b)
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("golden %s: %v", name, err)
	}
	return string(b)
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	if want := readGolden(t, name); got != want {
		t.Errorf("%s drifted from the pre-extraction golden (first difference at byte %d):\n got: %q\nwant: %q", name, firstDiff(got, want), got, want)
	}
}

func firstDiff(a, b string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// routingCfg is the enforce-stage config the clamp integration uses.
func routingCfg() config.RoutingConfig {
	return config.RoutingConfig{
		Stage:         config.StageEnforce,
		Mandatory:     []string{"scout", "build", "audit", "ship"},
		MaxInsertions: 4,
		PhaseEnable:   map[string]config.Enable{},
		Triggers:      map[string]config.RoutingBlock{},
	}
}
