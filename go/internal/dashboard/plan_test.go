package dashboard

// plan_test.go — the per-cycle phase plan the board renders: the registry's
// mandatory set (config.mandatory_phases — the set the router's floor
// enforces), every phase the cycle ran with a status
// (pass/warn/fail/ongoing/pending/unreached/skipped), the contract-gate mark
// from the cycle's Signal Center stream, the counts, and how the advisor's
// proposal fared. Operator ask, 2026-09-14: "how many phases are required,
// how many passed, which is ongoing" — on the board.

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// planRegistry mirrors the live registry's shape: triage is mandatory (the
// floor never skips it) although its phases[] entry says optional; tdd is
// conditional-mandatory; retro is neither.
const planRegistry = `{"config":{"mandatory_phases":["scout","triage","build","audit","ship"],"spine_order":["scout","triage","tdd","build","audit","ship"],"conditional_mandatory":{"tdd":"cycle_size!=trivial"}},"phases":[{"name":"scout"},{"name":"triage","optional":true},{"name":"tdd","optional":true},{"name":"build"},{"name":"audit"},{"name":"ship"},{"name":"retro","optional":true}]}`

func outcomeLine(cycle int, phase, verdict string, ms int) string {
	return `{"schema_version":"signal/1.0","seq":2,"cycle":` + itoa(cycle) + `,"phase":"` + phase + `","attempt":1,"module":"orchestrator","kind":"phase.outcome","severity":"INFO","reason":"` + phase + ` verdict=` + verdict + `","fields":{"verdict":"` + verdict + `","duration_ms":"` + itoa(ms) + `"}}`
}

func gateLine(cycle int, phase string) string {
	return `{"schema_version":"signal/1.0","seq":1,"cycle":` + itoa(cycle) + `,"phase":"` + phase + `","module":"gate.contract","origin":"Reviewer.Review","kind":"gate.passed","code":"GATE_CONTRACT_VERIFIED","severity":"INFO","reason":"` + phase + `: deliverables verified"}`
}

func writePlanFixture(t *testing.T, root string, id int, timing []phasetiming.Entry, streamLines []string, replan string) string {
	t.Helper()
	writeFile(t, filepath.Join(root, "docs", "architecture", "phase-registry.json"), planRegistry)
	ws := writeWorkspace(t, root, id, timing, 0)
	writeNDJSON(t, filepath.Join(ws, signalcenter.StreamFileName), streamLines...)
	if replan != "" {
		writeFile(t, filepath.Join(ws, "phase-replan.json"), replan)
	}
	return ws
}

func statuses(p *PhasePlan) []string {
	var out []string
	for _, s := range p.Steps {
		out = append(out, s.Phase+":"+s.Status)
	}
	return out
}

func planFor(t *testing.T, root string, id int, loop LoopStatus) (*PhasePlan, []string) {
	t.Helper()
	cs, _ := readCycle(root, id, nil)
	cs = assignState(cs, loop)
	set, _ := readMandatory(root, nil)
	return readPlan(set, core.RunWorkspacePath(root, id), cs, loop, newStreamReader())
}

// A LIVE lane has no phase-timing.json yet (the orchestrator flushes it at
// closeout): what ran so far is on the Signal Center stream. scout, triage
// and tdd passed (tdd conditional-mandatory), build is ongoing, audit and
// ship are pending; scout's and triage's gates verified, tdd's did not; the
// advisor proposed two insertions that have not run and a skip of scout
// the floor overrode.
func TestPhasePlan_LiveLaneCountsRequiredPassedOngoingAndRemaining(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs", "architecture", "phase-registry.json"), planRegistry)
	ws := core.RunWorkspacePath(root, 1676)
	writeFile(t, filepath.Join(ws, core.RunStateFile), `{"cycle_id":1676,"phase":"build","completed_phases":["scout","triage","tdd"],"started_at":"2026-09-14T08:12:00Z"}`)
	writeNDJSON(t, filepath.Join(ws, signalcenter.StreamFileName),
		gateLine(1676, "scout"), outcomeLine(1676, "scout", "PASS", 196243),
		gateLine(1676, "triage"), outcomeLine(1676, "triage", "PASS", 109000),
		outcomeLine(1676, "tdd", "PASS", 1019000))
	writeFile(t, filepath.Join(ws, "phase-replan.json"), `[{"phase":"scout","run":false},{"phase":"api-contract-design","run":true},{"phase":"bug-reproduction","run":true},{"phase":"tdd","run":true},{"phase":"build","run":true},{"phase":"retro","run":false}]`)
	loop := LoopStatus{Running: true, CycleID: 1676, Phase: "build", PhaseStartedAt: time.Date(2026, 9, 14, 8, 35, 0, 0, time.UTC)}
	plan, warns := planFor(t, root, 1676, loop)
	if plan == nil || len(warns) != 0 {
		t.Fatalf("plan=%v warns=%v", plan, warns)
	}
	if want := []string{"scout", "triage", "build", "audit", "ship"}; !reflect.DeepEqual(plan.Mandatory, want) {
		t.Errorf("mandatory = %v, want config.mandatory_phases %v", plan.Mandatory, want)
	}
	if want := []string{"scout:pass", "triage:pass", "tdd:pass", "build:ongoing", "audit:pending", "ship:pending"}; !reflect.DeepEqual(statuses(plan), want) {
		t.Errorf("steps = %v, want %v", statuses(plan), want)
	}
	if plan.Required != 6 || plan.PassedRequired != 3 || plan.Total != 6 || plan.Passed != 3 || plan.Ongoing != "build" || !reflect.DeepEqual(plan.Remaining, []string{"audit", "ship"}) {
		t.Errorf("counts: %+v", plan)
	}
	if !plan.OngoingSince.Equal(loop.PhaseStartedAt) {
		t.Errorf("ongoing since = %v", plan.OngoingSince)
	}
	tdd := plan.Steps[2]
	if plan.Steps[0].Optional || plan.Steps[1].Optional || !tdd.Conditional || tdd.Optional || !plan.Steps[0].GateVerified || !plan.Steps[1].GateVerified || tdd.GateVerified {
		t.Errorf("marks: %+v", plan.Steps)
	}
	if plan.Steps[0].DurationMS != 196243 {
		t.Errorf("scout duration = %d", plan.Steps[0].DurationMS)
	}
	if !reflect.DeepEqual(plan.AdvisorProposed, []string{"api-contract-design", "bug-reproduction"}) || !reflect.DeepEqual(plan.AdvisorSkips, []string{"retro"}) || !reflect.DeepEqual(plan.AdvisorOverridden, []string{"scout"}) {
		t.Errorf("advisor: proposed=%v skips=%v overridden=%v", plan.AdvisorProposed, plan.AdvisorSkips, plan.AdvisorOverridden)
	}
}

// A sealed FAIL cycle reads phase-timing.json: what never ran is unreached,
// a repeated phase shows its rounds and last verdict, an optional phase that
// ran (retro) is drawn but not required, and nothing is ongoing.
func TestPlanStep_SealedCycleMarksUnreachedAndRounds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	timing := []phasetiming.Entry{
		entry("scout", "PASS", "2026-09-14T08:15:00Z", "2026-09-14T08:18:00Z", 1),
		entry("triage", "PASS", "2026-09-14T08:18:00Z", "2026-09-14T08:20:00Z", 1),
		entry("build", "PASS", "2026-09-14T08:20:00Z", "2026-09-14T08:40:00Z", 1),
		entry("audit", "FAIL", "2026-09-14T08:40:00Z", "2026-09-14T09:00:00Z", 1),
		entry("audit", "FAIL", "2026-09-14T09:00:00Z", "2026-09-14T09:20:00Z", 1),
		entry("retro", "PASS", "2026-09-14T09:20:00Z", "2026-09-14T09:25:00Z", 1),
	}
	writePlanFixture(t, root, 1673, timing, []string{gateLine(1673, "scout"), gateLine(1673, "build")}, "")
	cs, _ := readCycle(root, 1673, nil)
	cs.HasDossier, cs.Verdict = true, "FAIL"
	cs = assignState(cs, LoopStatus{})
	set, _ := readMandatory(root, nil)
	plan, _ := readPlan(set, core.RunWorkspacePath(root, 1673), cs, LoopStatus{}, newStreamReader())
	if want := []string{"scout:pass", "triage:pass", "build:pass", "audit:fail", "retro:pass", "ship:unreached"}; !reflect.DeepEqual(statuses(plan), want) {
		t.Errorf("steps = %v, want %v", statuses(plan), want)
	}
	if plan.Steps[3].Rounds != 2 || plan.Steps[3].DurationMS != 20*60*1000 || plan.Ongoing != "" || len(plan.Remaining) != 0 {
		t.Errorf("plan = %+v", plan)
	}
	if !plan.Steps[4].Optional || plan.Required != 5 || plan.PassedRequired != 3 || plan.Total != 6 || plan.Passed != 4 {
		t.Errorf("counts: required %d/%d total %d/%d optional=%v", plan.PassedRequired, plan.Required, plan.Passed, plan.Total, plan.Steps[4].Optional)
	}
}

// A WARN phase is not a passed phase; a mandatory phase the cycle went past
// without running is skipped, not pending; a phase both executed and ongoing
// (audit round 2 dispatched after round 1 failed) is ongoing with its rounds.
func TestPhasePlan_WarnSkippedAndRepeatedOngoing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	timing := []phasetiming.Entry{
		entry("scout", "WARN", "2026-09-14T08:15:00Z", "2026-09-14T08:18:00Z", 1),
		entry("build", "PASS", "2026-09-14T08:18:00Z", "2026-09-14T08:40:00Z", 1),
		entry("audit", "FAIL", "2026-09-14T08:40:00Z", "2026-09-14T09:00:00Z", 1),
	}
	writePlanFixture(t, root, 1678, timing, nil, "")
	loop := LoopStatus{Running: true, CycleID: 1678, Phase: "audit", PhaseStartedAt: time.Date(2026, 9, 14, 9, 1, 0, 0, time.UTC)}
	plan, _ := planFor(t, root, 1678, loop)
	if want := []string{"scout:warn", "build:pass", "audit:ongoing", "triage:skipped", "ship:pending"}; !reflect.DeepEqual(statuses(plan), want) {
		t.Errorf("steps = %v, want %v", statuses(plan), want)
	}
	if plan.Passed != 1 || plan.PassedRequired != 1 || plan.Ongoing != "audit" || plan.Steps[2].Rounds != 1 || !reflect.DeepEqual(plan.Remaining, []string{"ship"}) {
		t.Errorf("plan = %+v", plan)
	}
}

// A run whose verdict is not PASS/WARN/FAIL (a torn timing entry) is
// incomplete — drawn, never counted as passed.
func TestPlanStep_UnknownVerdictIsIncomplete(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePlanFixture(t, root, 1681, []phasetiming.Entry{entry("scout", "", "2026-09-14T08:15:00Z", "2026-09-14T08:18:00Z", 1)}, nil, "")
	plan, _ := planFor(t, root, 1681, LoopStatus{})
	if plan.Steps[0].Status != StateIncomplete || plan.Passed != 0 {
		t.Errorf("plan = %+v", plan)
	}
}

// Both records present and disagreeing: a running cycle trusts the stream
// (the live record); a sealed one trusts phase-timing.json (the durable one).
func TestPhasePlan_PhaseHistoryPrecedenceIsStateDriven(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	timing := []phasetiming.Entry{entry("scout", "PASS", "2026-09-14T08:15:00Z", "2026-09-14T08:18:00Z", 1)}
	writePlanFixture(t, root, 1680, timing, []string{outcomeLine(1680, "scout", "PASS", 1000), outcomeLine(1680, "triage", "PASS", 2000)}, "")
	running, _ := planFor(t, root, 1680, LoopStatus{Running: true, CycleID: 1680, Phase: "tdd"})
	if want := []string{"scout:pass", "triage:pass", "tdd:ongoing", "build:pending", "audit:pending", "ship:pending"}; !reflect.DeepEqual(statuses(running), want) {
		t.Errorf("running reads the stream: %v", statuses(running))
	}
	sealed, _ := planFor(t, root, 1680, LoopStatus{})
	if want := []string{"scout:pass", "triage:unreached", "build:unreached", "audit:unreached", "ship:unreached"}; !reflect.DeepEqual(statuses(sealed), want) {
		t.Errorf("sealed reads phase-timing.json: %v", statuses(sealed))
	}
}

// Sources the reader cannot trust are said, never substituted: a torn
// phase-replan.json yields no proposal (not the older phase-plan.json); a
// stream that stops early is said; a malformed registry is said and the
// compiled baseline stands in; a dossier-only cycle has no plan.
func TestPhasePlan_TornSourcesAreSaidNotSubstituted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := writePlanFixture(t, root, 1679, []phasetiming.Entry{entry("scout", "PASS", "2026-09-14T08:15:00Z", "2026-09-14T08:18:00Z", 1)}, nil, "")
	writeFile(t, filepath.Join(ws, "phase-plan.json"), `[{"phase":"api-contract-design","run":true}]`)
	writeFile(t, filepath.Join(ws, "phase-replan.json"), `[{"phase":"bug-repro`)
	plan, warns := planFor(t, root, 1679, LoopStatus{})
	if len(plan.AdvisorProposed) != 0 || len(warns) != 1 || !strings.Contains(warns[0], "phase-replan.json") {
		t.Errorf("torn replan: proposed=%v warns=%v", plan.AdvisorProposed, warns)
	}
	writeFile(t, filepath.Join(ws, signalcenter.StreamFileName), "{not json}\n"+strings.Repeat("x", 2*1024*1024)+"\n")
	_, warns = planFor(t, root, 1679, LoopStatus{})
	if len(warns) != 2 || !strings.Contains(strings.Join(warns, " "), signalcenter.StreamFileName) {
		t.Errorf("a stream that stops early is said: %v", warns)
	}
	writeFile(t, filepath.Join(root, "docs", "architecture", "phase-registry.json"), "{")
	set, mw := readMandatory(root, nil)
	if len(mw) != 1 || !strings.Contains(mw[0], "malformed") || !reflect.DeepEqual(set.mandatory, []string{"scout", "build", "audit", "ship"}) {
		t.Errorf("malformed registry: set=%v warns=%v", set.mandatory, mw)
	}
	if p, _ := readPlan(set, core.RunWorkspacePath(root, 9), CycleSummary{ID: 9}, LoopStatus{}, newStreamReader()); p != nil {
		t.Errorf("dossier-only cycle has no plan, got %+v", p)
	}
}

// The stream reader re-reads a stream only when it changed on disk.
func TestStreamReader_CachesByModTimeAndSize(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	path := filepath.Join(ws, signalcenter.StreamFileName)
	writeNDJSON(t, path, outcomeLine(1, "scout", "PASS", 1))
	r := newStreamReader()
	if _, outs, _ := r.read(ws); len(outs) != 1 {
		t.Fatalf("first read: %v", outs)
	}
	writeNDJSON(t, path, outcomeLine(1, "scout", "PASS", 1), outcomeLine(1, "triage", "PASS", 2))
	if _, outs, _ := r.read(ws); len(outs) != 2 {
		t.Errorf("a grown stream is re-read: %v", outs)
	}
	if len(r.cache) != 1 {
		t.Errorf("one entry per stream, got %d", len(r.cache))
	}
}

// Collect wires the plan onto every cycle it renders, and the detail
// endpoint (/api/cycle/{id}) carries the same plan — both seams the page reads.
func TestCollect_CyclesCarryTheirPlan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePlanFixture(t, root, 1677, []phasetiming.Entry{entry("scout", "PASS", "2026-09-14T08:15:00Z", "2026-09-14T08:18:00Z", 1)}, []string{gateLine(1677, "scout")}, "")
	snap := Collect(root, time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC))
	found := false
	for _, c := range snap.Cycles {
		if c.ID == 1677 {
			found = true
			if c.Plan == nil || c.Plan.Required != 5 || c.Plan.PassedRequired != 1 || !c.Plan.Steps[0].GateVerified {
				t.Fatalf("cycle 1677 plan = %+v", c.Plan)
			}
		}
	}
	if !found {
		t.Fatalf("cycle 1677 not in snapshot: %+v", snap.Cycles)
	}
}

func TestServer_CycleDetailCarriesThePlan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePlanFixture(t, root, 1677, []phasetiming.Entry{entry("scout", "PASS", "2026-09-14T08:15:00Z", "2026-09-14T08:18:00Z", 1)}, []string{gateLine(1677, "scout")}, "")
	_, ts, _ := newTestServer(t, root, time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC))
	_, body := get(t, ts.URL+"/api/cycle/1677")
	var d cycleDetail
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		t.Fatalf("detail: %v\n%s", err, body)
	}
	if d.Cycle.Plan == nil || d.Cycle.Plan.Required != 5 || d.Cycle.Plan.PassedRequired != 1 || !d.Cycle.Plan.Steps[0].GateVerified {
		t.Fatalf("detail plan = %+v", d.Cycle.Plan)
	}
}

// The board prints the set the floor enforces, operator overrides included:
// the injected environment (Options.Env) narrows the mandatory list exactly
// as it narrows the loop's floor; Server.Snapshot carries it, Collect (no
// env) shows the registry's set alone.
func TestServer_SnapshotReflectsTheInjectedEnvironment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePlanFixture(t, root, 1682, []phasetiming.Entry{entry("scout", "PASS", "2026-09-14T08:15:00Z", "2026-09-14T08:18:00Z", 1)}, nil, "")
	env := map[string]string{"EVOLVE_MANDATORY_PHASES": "scout,build,ship"}
	set, warns := readMandatory(root, env)
	if !reflect.DeepEqual(set.mandatory, []string{"scout", "build", "ship"}) {
		t.Errorf("mandatory = %v (warns %v), want the env override", set.mandatory, warns)
	}
	now := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	snap := New(root, Options{Env: env}).Snapshot(now)
	plain := Collect(root, now)
	var withEnv, without *PhasePlan
	for _, c := range snap.Cycles {
		if c.ID == 1682 {
			withEnv = c.Plan
		}
	}
	for _, c := range plain.Cycles {
		if c.ID == 1682 {
			without = c.Plan
		}
	}
	if withEnv == nil || without == nil || !reflect.DeepEqual(withEnv.Mandatory, []string{"scout", "build", "ship"}) || len(without.Mandatory) != 5 {
		t.Errorf("snapshot mandatory = %v, collect mandatory = %v", withEnv, without)
	}
}
