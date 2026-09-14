package core

// phase_advisor_seam_test.go — ADR-0103 unit 04 §6 step 4: the core seam —
// the ONE wired construction, the Bridge→Launcher projection, the facades the
// composition root, resume, the judge/adjudicator, the failure digest and the
// by-name tests keep, the git reader the seam injects, the Center reaching
// the brain through the option, and the unit-05 handoff pin.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Test 39 — the ONE projection of the leaf's fourteen-field request onto the
// bridge request (every field set to a distinct value so a dropped or swapped
// mapping shows; the leaf's own positional pin makes a NEW field a compile
// error) and of the four response fields back.
func TestBridgeRequestOf_ProjectsEveryLaunchField(t *testing.T) {
	env := map[string]string{"EVOLVE_CLI": "claude-tmux"}
	req := advisor.LaunchRequest{CLI: "claude-tmux", Profile: "/p.json", Model: "deep", Skills: []string{"fable"}, Prompt: "prompt", Workspace: "/ws", Worktree: "/wt",
		ProjectRoot: "/root", ArtifactPath: "/ws/routing-plan.json", Completion: "artifact", Agent: "router", Contract: "router-replan", Cycle: 7, Env: env}
	got := bridgeRequestOf(req)
	want := BridgeRequest{CLI: "claude-tmux", Profile: "/p.json", Model: "deep", Skills: []string{"fable"}, Prompt: "prompt", Workspace: "/ws", Worktree: "/wt",
		ProjectRoot: "/root", ArtifactPath: "/ws/routing-plan.json", Completion: "artifact", Agent: "router", Contract: "router-replan", Cycle: 7, Env: env}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bridgeRequestOf:\n got %+v\nwant %+v", got, want)
	}
	resp := launchResponseOf(BridgeResponse{ExitCode: 81, Stdout: "out", DurationMS: 12, Tokens: TokenUsage{Input: 1, Output: 2}, Stderr: "ignored", CostUSD: 1.5, BootMS: 9})
	if resp != (advisor.LaunchResponse{ExitCode: 81, Stdout: "out", DurationMS: 12, Tokens: TokenUsage{Input: 1, Output: 2}}) {
		t.Errorf("launchResponseOf: %+v", resp)
	}
	if launcherOf(nil) != nil {
		t.Error("a nil bridge stays a nil launcher so the legacy nil-bridge fail-safe survives")
	}
}

// Test 40 — the leaf is constructed in exactly one non-test file, and the
// composition root constructs the seam in exactly one.
func TestPhaseAdvisor_OneConstructionSite(t *testing.T) {
	for needle, onlySite := range map[string]string{
		"advisor.New(":          "internal/core/phase_advisor.go",
		"core.NewPhaseAdvisor(": "cmd/evolve/cmd_cycle.go",
	} {
		if offenders := nonTestSourcesMentioning(t, needle, onlySite); len(offenders) > 0 {
			t.Errorf("%q belongs to ONE non-test file (%s); these non-test files use it too: %v", needle, onlySite, offenders)
		}
		body, err := os.ReadFile(filepath.Join("..", "..", onlySite))
		if err != nil || !strings.Contains(string(body), needle) {
			t.Errorf("%s must spell %q (%v)", onlySite, needle, err)
		}
	}
}

// Test 41 — a literal PhaseAdvisor lazily builds ONE brain; a nil bridge
// yields the legacy error; the seam injects the atomic capture writer (no
// .tmp sibling survives, a directory at the artifact path is a capture WARN
// while the plan still returns).
func TestPhaseAdvisor_LiteralGetsTheBrainOnce(t *testing.T) {
	p := &PhaseAdvisor{identity: AgentIdentity{CLI: "claude-tmux", Model: "opus", AgentLabel: "router"}}
	if p.brain != nil {
		t.Fatal("a literal starts without a brain")
	}
	first := p.advisor()
	if first == nil || p.advisor() != first {
		t.Fatal("the literal builds its brain once and caches it")
	}
	if _, err := p.Propose(baseRouteInput()); err == nil || err.Error() != "routing proposer: nil bridge" {
		t.Fatalf("a literal with no bridge fails the legacy way: %v", err)
	}
}

// Test 41b (review fold, Go MINOR) — the lazy cache is safe by construction:
// concurrent FIRST uses of a literal PhaseAdvisor build ONE brain and every
// caller sees the same one. Red under -race on the unsynchronized nil-check
// (a read-check-write on the pointer field); green under sync.Once.
func TestPhaseAdvisor_LiteralBuildsOneBrainUnderConcurrentFirstUse(t *testing.T) {
	p := &PhaseAdvisor{identity: AgentIdentity{CLI: "claude-tmux", Model: "opus", AgentLabel: "router"}}
	const callers = 8
	brains := make([]*advisor.Advisor, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			brains[i] = p.advisor()
		}(i)
	}
	wg.Wait()
	for i, b := range brains {
		if b == nil || b != brains[0] {
			t.Fatalf("caller %d saw brain %p, caller 0 saw %p: the literal must build exactly one brain", i, b, brains[0])
		}
	}
	if p.advisor() != brains[0] {
		t.Fatal("a later call returns the cached brain")
	}
}

func TestNewPhaseAdvisor_NilBridgeYieldsTheLegacyError(t *testing.T) {
	p := NewPhaseAdvisor(nil)
	if p.brain == nil {
		t.Fatal("NewPhaseAdvisor constructs eagerly")
	}
	if _, err := p.Propose(baseRouteInput()); err == nil || err.Error() != "routing proposer: nil bridge" {
		t.Fatalf("Propose: %v", err)
	}
	if _, err := p.Plan(baseRouteInput()); err == nil || err.Error() != "phase advisor: nil bridge" {
		t.Fatalf("Plan: %v", err)
	}
}

func TestNewPhaseAdvisor_WiresTheAtomicCaptureWriter(t *testing.T) {
	ws := t.TempDir()
	in := baseRouteInput()
	in.Workspace = ws
	fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}
	if _, err := NewPhaseAdvisor(fb).Plan(in); err != nil {
		t.Fatal(err)
	}
	if got := readAdvisorArtifact(t, filepath.Join(ws, "advisor-prompt-plan.txt")); got != fb.gotReq.Prompt {
		t.Error("the persisted prompt is the prompt sent")
	}
	if _, err := os.Stat(filepath.Join(ws, "advisor-prompt-plan.txt.tmp")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the atomic writer leaves no .tmp sibling: %v", err)
	}
	c, got := recordingCenter()
	blocked := t.TempDir()
	if err := os.Mkdir(filepath.Join(blocked, "advisor-prompt-plan.txt"), 0o755); err != nil { // a directory at the path: the rename fails
		t.Fatal(err)
	}
	in.Workspace = blocked
	plan, err := NewPhaseAdvisor(&fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}, WithAdvisorSignals(c)).Plan(in)
	if err != nil || plan == nil {
		t.Fatalf("a capture fault never fails the plan: %v", err)
	}
	if evs := eventsOfKind(*got, signalcenter.KindAdvisorWarning); len(evs) != 1 || evs[0].Code != advisor.CodeCaptureWriteFailed || evs[0].Fields["artifact"] != "prompt" {
		t.Errorf("the write fault is one capture WARN: %+v", *got)
	}
}

// Test 42 — every facade projects the leaf; resume and replay re-parse a
// reserved mint silently.
func TestCoreFacades_ProjectTheLeaf(t *testing.T) {
	raw := `[{"phase":"scout","run":true,"tier":"opus"},{"phase":"router","run":true,"mint":{"prompt":"x"}},{"phase":"new-helper","run":true,"mint":{"prompt":"y"}}]`
	c, got := recordingCenter()
	_ = c
	var stderr string
	stderr = captureStderr(t, func() {
		plan, err := parsePhasePlan(raw)
		if want, _ := advisor.ParsePhasePlan(raw); err != nil || !reflect.DeepEqual(plan, want.Plan) || len(plan.MintPhases) != 1 {
			t.Errorf("parsePhasePlan projects ParsePhasePlan(·).Plan: %+v %v", plan, err)
		}
		replayed, clamps, err := ReplayPlanFromResponse(raw, router.RouteInput{}, router.DefaultShipFloor())
		wantR, wantC, werr := advisor.ReplayPlanFromResponse(raw, router.RouteInput{}, router.DefaultShipFloor())
		if err != nil || werr != nil || !reflect.DeepEqual(replayed, wantR) || !reflect.DeepEqual(clamps, wantC) {
			t.Errorf("ReplayPlanFromResponse projects the leaf: %v %v", err, werr)
		}
	})
	if strings.Contains(stderr, "dropping minted phase") || len(*got) != 0 {
		t.Errorf("resume/replay re-parse a reserved mint silently (reported once at decision time): %q %+v", stderr, *got)
	}
	if s, e, ok := lastBalancedSpan("x [1] [2]", '[', ']'); s != 6 || e != 8 || !ok {
		t.Errorf("lastBalancedSpan: %d %d %v", s, e, ok)
	}
	long := strings.Repeat("g", maxGoalTextChars+7)
	if truncateGoal("  "+long+"  ") != advisor.TruncateGoal(long) || maxGoalTextChars != advisor.MaxGoalTextRunes {
		t.Error("truncateGoal / maxGoalTextChars project the advisor's cap")
	}
	if maxCarryoverTodosInPrompt != advisor.MaxCarryoverTodosInPrompt || maxEnrichedCatalogCards != advisor.MaxEnrichedCatalogCards {
		t.Error("the prompt bounds project the leaf's")
	}
	for _, tier := range []string{"fast", "top", "opus", ""} {
		if sanitizeAdvisorTier(tier) != advisor.SanitizeTier(tier) {
			t.Errorf("sanitizeAdvisorTier(%q)", tier)
		}
	}
	in := richRouteInput()
	var a, b strings.Builder
	writeRoutingContext(&a, in)
	advisor.WriteRoutingContext(&b, in)
	if a.String() != b.String() {
		t.Error("writeRoutingContext projects WriteRoutingContext")
	}
	a.Reset()
	b.Reset()
	writeCatalog(&a, in.Catalog)
	advisor.WriteCatalog(&b, in.Catalog)
	if a.String() != b.String() {
		t.Error("writeCatalog projects WriteCatalog")
	}
	a.Reset()
	b.Reset()
	writeCatalogWithOnDemand(&a, in.Catalog, in.OnDemandPhases)
	advisor.WriteCatalogWithOnDemand(&b, in.Catalog, in.OnDemandPhases)
	if a.String() != b.String() {
		t.Error("writeCatalogWithOnDemand projects WriteCatalogWithOnDemand")
	}
	a.Reset()
	b.Reset()
	writeCarryoverTodos(&a, in.CarryoverTodos)
	advisor.WriteCarryoverTodos(&b, in.CarryoverTodos)
	if a.String() != b.String() {
		t.Error("writeCarryoverTodos projects WriteCarryoverTodos")
	}
	entries := []router.PhasePlanEntry{{Phase: "router", Mint: &router.MintSpec{Prompt: "x"}}, {Phase: "ok", Mint: &router.MintSpec{Prompt: "y"}}}
	wantMints, _ := advisor.MintConfigsFrom(entries)
	if !reflect.DeepEqual(mintConfigsFrom(entries), wantMints) || reservedAdvisorMintReason("Router") != advisor.ReservedMintReason("Router") || reservedAdvisorMintReason("ok") != "" {
		t.Error("mintConfigsFrom / reservedAdvisorMintReason project the leaf")
	}
	p := NewPhaseAdvisor(nil, WithPersona("PERSONA"))
	if p.composePlanPrompt(in, "routing-plan.json") != p.advisor().ComposePlanPrompt(in, "routing-plan.json") {
		t.Error("composePlanPrompt projects ComposePlanPrompt")
	}
	var _ AdvisorSpan = advisor.Span{}
	var _ AgentIdentity = advisor.Identity{}
}

// Test 43 — the unit-05 handoff pin: a failing Planner under RunCycle emits
// NO orchestrator code naming the plan (the advisor reports its own fault)
// while the verbatim `[orchestrator] WARN phase advisor Plan failed` line
// still prints. A handoff pin: it cannot go red on the base; its throwaway
// mutant is a hand-added ORCHESTRATOR_* Emit in cyclerun.go's degrade branch.
func TestRunCycle_PlanDegradeEmitsNoOrchestratorCode(t *testing.T) {
	c, got := recordingCenter()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil),
		WithRouting(cfg, router.StaticPreset{}), WithPlanner(&erroringPlanner{}), WithSignalCenter(c))
	out := captureStderr(t, func() {
		if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true}); err != nil {
			t.Fatalf("RunCycle: %v", err)
		}
	})
	if !strings.Contains(out, "[orchestrator] WARN phase advisor Plan failed (degrading to static spine): planner: boom") {
		t.Errorf("the orchestrator's verbatim degrade line stays until unit 05: %q", out)
	}
	for _, e := range *got {
		// An unregistered code is rewritten to SIGNALCENTER_UNREGISTERED_CODE with
		// the raw spelling under fields.raw_code — look at both, and at the reason.
		if e.Module == signalcenter.ModuleOrchestrator && (strings.Contains(string(e.Code)+e.Fields["raw_code"], "PLAN") || strings.Contains(e.Reason, "planner: boom")) {
			t.Errorf("no orchestrator code names the plan degrade before unit 05: %+v", e)
		}
	}
}

type erroringPlanner struct{}

func (erroringPlanner) Plan(router.RouteInput) (*router.PhasePlan, error) {
	return nil, errors.New("planner: boom")
}

// Test 44 — the git reader the seam injects: duplicates kept as git prints
// them, a non-repo is an error, an empty root reads nothing.
func TestRecentlyChangedFiles_ReadsGitLogAndReportsErrors(t *testing.T) {
	root := goldenGitRepo(t)
	files, err := recentlyChangedFiles(root)
	if err != nil || strings.Join(files, " ") != "a.go c.py docs/README.md a.go b_test.go" {
		t.Errorf("recentlyChangedFiles = %v, %v", files, err)
	}
	if files, err := recentlyChangedFiles(t.TempDir()); err == nil || files != nil {
		t.Errorf("a non-repo is a returned error, not a silent nil: %v %v", files, err)
	}
	if files, err := recentlyChangedFiles(""); err != nil || files != nil {
		t.Errorf("an empty root reads nothing: %v %v", files, err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git is required for this test")
	}
}

// Test 45 — the Center reaches the brain through the option: without it a
// reserved mint drops silently; with it the drop is one ADVISOR_MINT_REJECTED
// stamped with the cycle and the decision.
func TestPhaseAdvisor_SeesTheSignalCenterAppliedThroughTheOption(t *testing.T) {
	stdout := `[{"phase":"scout","run":true},{"phase":"router","run":true,"mint":{"prompt":"be a router"}}]`
	in := baseRouteInput()
	in.Workspace = t.TempDir()
	in.Cycle = 1642
	var plan *router.PhasePlan
	var err error
	out := captureStderr(t, func() { plan, err = NewPhaseAdvisor(&fakeBridge{stdout: stdout}).Plan(in) })
	if err != nil || len(plan.MintPhases) != 0 || strings.Contains(out, "dropping minted phase") {
		t.Fatalf("without a Center the drop is silent and the plan stands: %v %+v %q", err, plan, out)
	}
	c, got := recordingCenter()
	p := NewPhaseAdvisor(&fakeBridge{stdout: stdout}, WithAdvisorSignals(c))
	if _, err := p.Plan(in); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 || (*got)[0].Code != advisor.CodeMintRejected || (*got)[0].Cycle != 1642 || (*got)[0].Fields["decision"] != "plan" || (*got)[0].Origin != "Advisor.Plan" {
		t.Fatalf("with the option: one ADVISOR_MINT_REJECTED with the cycle and decision stamp: %+v", *got)
	}
	*got = nil
	if _, err := p.RePlan(in); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 || (*got)[0].Fields["decision"] != "replan" || (*got)[0].Origin != "Advisor.RePlan" {
		t.Fatalf("RePlan stamps replan: %+v", *got)
	}
}
