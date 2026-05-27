package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// --- helpers ---

// seedWorkspace writes a handoff artifact into the cycle workspace so
// router.Digest can read it. Returns the workspace path. The workspace guard
// is disabled by the caller (EVOLVE_DISABLE_WORKSPACE_GUARD=1) so the
// pre-seeded files survive into the run.
func seedWorkspace(t *testing.T, projectRoot string, cycle int, files map[string]string) string {
	t.Helper()
	ws := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	return ws
}

// readRoutingDecisions parses every routing-decision-<seq>.json the
// orchestrator wrote into the workspace.
func readRoutingDecisions(t *testing.T, workspace string) []router.RouterDecision {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join(workspace, "routing-decision-*.json"))
	out := make([]router.RouterDecision, 0, len(paths))
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		var d router.RouterDecision
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatalf("unmarshal %s: %v", p, err)
		}
		out = append(out, d)
	}
	return out
}

func countLedgerKind(entries []LedgerEntry, kind string) int {
	n := 0
	for _, e := range entries {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// shadowCfg is a deterministic StaticPreset config with the tester trigger
// (build.acs_red > 0) the plan's minimal slice exercises.
func shadowCfg(stage config.Stage) config.RoutingConfig {
	return config.RoutingConfig{
		Stage:         stage,
		Mode:          config.ModeStaticPreset,
		Mandatory:     []string{"scout", "build", "audit", "ship"},
		Conditional:   map[string]config.CondRule{"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"}},
		MaxInsertions: 4,
		PhaseEnable:   map[string]config.Enable{},
		Triggers: map[string]config.RoutingBlock{
			"tester": {InsertWhen: []config.Condition{{Field: "build.acs_red", Op: "gt", Value: 0}}},
		},
	}
}

func indexOfPhase(ps []Phase, name string) int {
	for i, p := range ps {
		if string(p) == name {
			return i
		}
	}
	return -1
}

// --- tests ---

// CAPSTONE: a user-defined phase, authored as pure data and spliced into the
// routing order, actually EXECUTES between build and audit when its signal
// trigger fires — end-to-end through RunCycle with no LLM. This is the proof
// that the whole framework (catalog → routing order → signal trigger →
// orchestrator accept/run) works as one pipeline.
func TestOrchestrator_Enforce_RunsUserPhaseBetweenBuildAndAudit(t *testing.T) {
	projectRoot := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	scanRunner := &fakeRunner{name: "security-scan"}
	runners[Phase("security-scan")] = scanRunner

	cycle := 1
	// acs_red=0 so the built-in tester trigger stays quiet; a generic signal
	// (build.cves>0) fires the user phase instead — isolating the new path.
	seedWorkspace(t, projectRoot, cycle, map[string]string{
		"handoff-build.json": `{"verdict":"PASS","acs_result":{"green":5,"red":0},"signals":{"cves":1}}`,
	})

	// Merge returns (Catalog, warnings) — no error. Guard the setup explicitly so
	// a future Merge change that drops the phase fails here, not with a confusing
	// "runner calls = 0" later.
	cat, _ := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "security-scan", Optional: true, After: "build"},
	})
	if _, ok := cat.Get("security-scan"); !ok {
		t.Fatal("setup: security-scan missing from catalog after Merge")
	}
	cfg := shadowCfg(config.StageEnforce)
	cfg.Order = []string{"scout", "triage", "tdd", "build-planner", "build", "tester", "security-scan", "audit", "ship"}
	cfg.Triggers["security-scan"] = config.RoutingBlock{
		InsertWhen: []config.Condition{{Field: "build.cves", Op: "gt", Value: 0}},
	}

	o := NewOrchestrator(st, led, runners, WithRouting(cfg, router.StaticPreset{}), WithCatalog(cat))
	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: projectRoot,
		GoalHash:    "g",
		Budget:      BudgetEnvelope{MaxUSD: 100},
		Env:         map[string]string{"EVOLVE_DISABLE_WORKSPACE_GUARD": "1"},
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	if scanRunner.calls != 1 {
		t.Errorf("security-scan runner calls = %d, want 1 (the user phase must execute)", scanRunner.calls)
	}
	bi := indexOfPhase(res.PhasesRun, "build")
	si := indexOfPhase(res.PhasesRun, "security-scan")
	ai := indexOfPhase(res.PhasesRun, "audit")
	if si < 0 {
		t.Fatalf("security-scan absent from PhasesRun=%v", res.PhasesRun)
	}
	if bi < 0 || bi >= si || si >= ai {
		t.Errorf("ordering wrong: build@%d security-scan@%d audit@%d (PhasesRun=%v)", bi, si, ai, res.PhasesRun)
	}
}

// Stage:Off (default) must add NO routing forensics — byte-identical to legacy.
func TestOrchestrator_StageOff_EmitsNoRoutingLedgerEntries(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	// Explicit WithRouting at StageOff confirms the Off branch short-circuits
	// even when a (non-default) Mode is configured.
	cfg := shadowCfg(config.StageOff)
	o := NewOrchestrator(st, led, runners, WithRouting(cfg, router.StaticPreset{}))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if n := countLedgerKind(led.entries, "routing_decision"); n != 0 {
		t.Errorf("routing_decision entries=%d, want 0 in Stage:Off", n)
	}
	if n := countLedgerKind(led.entries, "phase_skipped"); n != 0 {
		t.Errorf("phase_skipped entries=%d, want 0 in Stage:Off", n)
	}
	want := []Phase{PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip}
	if len(res.PhasesRun) != len(want) {
		t.Fatalf("phases=%v, want %v", res.PhasesRun, want)
	}
}

// Shadow: the router computes + logs a decision every iteration but the static
// state machine still drives. With handoff-build.json acs_red>0 the post-build
// decision must propose inserting tester — WITHOUT altering the path run.
func TestOrchestrator_Shadow_LogsTesterInsert_StaticPathUnchanged(t *testing.T) {
	projectRoot := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)

	cycle := 1 // LastCycleNumber 0 + 1
	ws := seedWorkspace(t, projectRoot, cycle, map[string]string{
		"handoff-build.json": `{"verdict":"PASS","acs_result":{"green":3,"red":2,"total":5}}`,
	})

	o := NewOrchestrator(st, led, runners, WithRouting(shadowCfg(config.StageShadow), router.StaticPreset{}))
	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: projectRoot,
		GoalHash:    "g",
		Budget:      BudgetEnvelope{MaxUSD: 100}, // positive ⇒ content inserts not budget-clamped
		// Keep the pre-seeded handoff-build.json from being archived.
		Env: map[string]string{"EVOLVE_DISABLE_WORKSPACE_GUARD": "1"},
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	// Static path is unchanged in shadow.
	want := []Phase{PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip}
	if len(res.PhasesRun) != len(want) {
		t.Fatalf("phases=%v, want %v (shadow must not change transitions)", res.PhasesRun, want)
	}
	for i := range want {
		if res.PhasesRun[i] != want[i] {
			t.Errorf("phase[%d]=%s, want %s", i, res.PhasesRun[i], want[i])
		}
	}

	// Forensic routing_decision entries exist (one per iteration).
	if n := countLedgerKind(led.entries, "routing_decision"); n == 0 {
		t.Errorf("expected routing_decision ledger entries in shadow, got 0")
	}

	// The post-build decision proposed inserting tester (acs_red>0 trigger).
	decs := readRoutingDecisions(t, ws)
	if len(decs) == 0 {
		t.Fatalf("no routing-decision artifacts written")
	}
	foundTester := false
	for _, d := range decs {
		if contains(d.InsertPhases, "tester") {
			foundTester = true
		}
	}
	if !foundTester {
		t.Errorf("no routing decision proposed inserting tester despite acs_red>0; decisions=%+v", decs)
	}
}

// Enforce: a trivial-cycle digest makes tdd genuinely optional, so the router
// proposes scout→build (skipping triage/tdd/build-planner). The kernel-
// validated override (CanTransition + SpineSatisfiedUpTo) adopts it.
func TestOrchestrator_Enforce_TrivialCycle_SkipsOptionalMiddle(t *testing.T) {
	projectRoot := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)

	cycle := 1
	seedWorkspace(t, projectRoot, cycle, map[string]string{
		// Scout reports a trivial cycle ⇒ tdd's conditional pin (cycle_size
		// != trivial) is NOT satisfied ⇒ tdd is skippable this cycle.
		"handoff-scout.json": `{"cycle_size_estimate":"trivial"}`,
	})

	o := NewOrchestrator(st, led, runners, WithRouting(shadowCfg(config.StageEnforce), router.StaticPreset{}))
	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: projectRoot,
		GoalHash:    "g",
		Budget:      BudgetEnvelope{MaxUSD: 100},
		Env:         map[string]string{"EVOLVE_DISABLE_WORKSPACE_GUARD": "1"},
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	// scout → build (skip triage/tdd/build-planner) → audit → ship.
	want := []Phase{PhaseScout, PhaseBuild, PhaseAudit, PhaseShip}
	if len(res.PhasesRun) != len(want) {
		t.Fatalf("phases=%v, want %v (enforce should skip the optional middle on a trivial cycle)", res.PhasesRun, want)
	}
	for i := range want {
		if res.PhasesRun[i] != want[i] {
			t.Errorf("phase[%d]=%s, want %s", i, res.PhasesRun[i], want[i])
		}
	}
	// The triage/tdd/build-planner runners must never have been called.
	for _, p := range []Phase{PhaseTriage, PhaseTDD, PhaseBuildPlanner} {
		if fr := runners[p].(*fakeRunner); fr.calls != 0 {
			t.Errorf("phase %s ran %d times, want 0 (skipped on trivial enforce)", p, fr.calls)
		}
	}
}

// Full static-path spine-gating (Enforce): when a mandatory predecessor's
// handoff is absent, every transition records a fail-open spine-unsatisfied-warn
// — the integrity signal fires WITHOUT blocking the cycle. Only handoff-scout is
// seeded, so the build→audit and audit→ship transitions warn.
func TestOrchestrator_Enforce_SpineUnsatisfied_WarnsButProceeds(t *testing.T) {
	projectRoot := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)

	cycle := 1
	ws := seedWorkspace(t, projectRoot, cycle, map[string]string{
		"handoff-scout.json": `{"cycle_size_estimate":"medium"}`, // non-trivial ⇒ full spine runs
	})

	o := NewOrchestrator(st, led, runners, WithRouting(shadowCfg(config.StageEnforce), router.StaticPreset{}))
	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: projectRoot,
		GoalHash:    "g",
		Budget:      BudgetEnvelope{MaxUSD: 100},
		Env:         map[string]string{"EVOLVE_DISABLE_WORKSPACE_GUARD": "1"},
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Fail-open: the cycle still reaches ship despite missing build/audit handoffs.
	if len(res.PhasesRun) == 0 || res.PhasesRun[len(res.PhasesRun)-1] != PhaseShip {
		t.Fatalf("phases=%v, want to reach ship (fail-open, not blocked)", res.PhasesRun)
	}
	// The spine-unsatisfied integrity signal was recorded.
	warned := false
	for _, d := range readRoutingDecisions(t, ws) {
		for _, c := range d.Clamps {
			if c.Rule == "spine-unsatisfied-warn" {
				warned = true
			}
		}
	}
	if !warned {
		t.Error("expected a spine-unsatisfied-warn clamp when build/audit handoffs are absent")
	}
}
