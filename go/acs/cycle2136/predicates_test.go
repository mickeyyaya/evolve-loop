//go:build acs

package cycle2136

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type recordingBridge struct {
	stdout string
	calls  int
	got    core.BridgeRequest
}

func (b *recordingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.calls++
	b.got = req
	return core.BridgeResponse{Stdout: b.stdout}, nil
}

func (b *recordingBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

type decision struct {
	name   string
	stdout string
	run    func(*core.PhaseAdvisor, router.RouteInput) error
}

func decisions() []decision {
	return []decision{
		{"plan", `[{"phase":"scout","run":true,"justification":"x"}]`, func(a *core.PhaseAdvisor, in router.RouteInput) error { _, err := a.Plan(in); return err }},
		{"replan", `[{"phase":"audit","run":true,"justification":"x"}]`, func(a *core.PhaseAdvisor, in router.RouteInput) error { _, err := a.RePlan(in); return err }},
		{"proposal", `{"next_phase":"audit"}`, func(a *core.PhaseAdvisor, in router.RouteInput) error { _, err := a.Propose(in); return err }},
	}
}

func advisorInput(t *testing.T, fleetValue, activeWorktree string) router.RouteInput {
	t.Helper()
	env := map[string]string{"EVOLVE_CLI": "claude-tmux"}
	if fleetValue != "" {
		env["EVOLVE_FLEET"] = fleetValue
	}
	return router.RouteInput{
		Current:        "build",
		Verdict:        "PASS",
		Workspace:      t.TempDir(),
		ProjectRoot:    "/proj",
		ActiveWorktree: activeWorktree,
		Cycle:          7,
		Env:            env,
	}
}

func TestC2136_001_FleetAdvisorInputWithEmptyActiveWorktreeIsRefused(t *testing.T) {
	for _, fleetValue := range []string{"1", "true"} {
		for _, d := range decisions() {
			bridge := &recordingBridge{stdout: d.stdout}
			in := advisorInput(t, fleetValue, "")
			err := d.run(core.NewPhaseAdvisor(bridge), in)
			if err == nil {
				t.Errorf("%s under EVOLVE_FLEET=%s with an empty ActiveWorktree launched with worktree=%q; the advisor must refuse it", d.name, fleetValue, bridge.got.Worktree)
				continue
			}
			if !strings.Contains(strings.ToLower(err.Error()), "worktree") {
				t.Errorf("%s: refusal %q does not name the worktree", d.name, err)
			}
			if bridge.calls != 0 {
				t.Errorf("%s: %d bridge launch(es) happened before the refusal; the guard must run before any side effect", d.name, bridge.calls)
			}
		}
	}
}

func TestC2136_002_FleetAdvisorInputWithWhitespaceActiveWorktreeIsRefused(t *testing.T) {
	bridge := &recordingBridge{stdout: decisions()[0].stdout}
	in := advisorInput(t, "1", "   ")
	if _, err := core.NewPhaseAdvisor(bridge).Plan(in); err == nil {
		t.Errorf("a whitespace ActiveWorktree under fleet mode launched with worktree=%q; it is as empty as no worktree", bridge.got.Worktree)
	}
}

func TestC2136_003_FleetAdvisorInputWithActiveWorktreeThreadsItToTheBridge(t *testing.T) {
	for _, d := range decisions() {
		bridge := &recordingBridge{stdout: d.stdout}
		in := advisorInput(t, "1", "/wt/cycle-7")
		if err := d.run(core.NewPhaseAdvisor(bridge), in); err != nil {
			t.Fatalf("%s: a fleet input that carries its worktree must launch: %v", d.name, err)
		}
		if bridge.got.Worktree != "/wt/cycle-7" {
			t.Errorf("%s: BridgeRequest.Worktree=%q, want /wt/cycle-7", d.name, bridge.got.Worktree)
		}
	}
}

func TestC2136_004_NonFleetEmptyActiveWorktreeKeepsTheWorkspaceFallback(t *testing.T) {
	for _, fleetValue := range []string{"", "0", "false"} {
		for _, d := range decisions() {
			bridge := &recordingBridge{stdout: d.stdout}
			in := advisorInput(t, fleetValue, "")
			if err := d.run(core.NewPhaseAdvisor(bridge), in); err != nil {
				t.Fatalf("%s with EVOLVE_FLEET=%q: single mode must not be refused: %v", d.name, fleetValue, err)
			}
			if bridge.got.Worktree != in.Workspace {
				t.Errorf("%s with EVOLVE_FLEET=%q: Worktree=%q, want the workspace %q", d.name, fleetValue, bridge.got.Worktree, in.Workspace)
			}
		}
	}
}

func goTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	full := append([]string{"-C", goDir}, args...)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", full...)
	if err != nil && code == 0 {
		t.Fatalf("go %v: %v", full, err)
	}
	return stdout + stderr, code
}

func TestC2136_005_AdvisorPackageHoldsAFleetWorktreeTestThatPasses(t *testing.T) {
	const pattern = "Fleet.*Worktree|Worktree.*Fleet"
	listing, code := goTest(t, "test", "-count=1", "-list", pattern, "./internal/core/advisor")
	if code != 0 {
		t.Fatalf("go test -list exited %d:\n%s", code, acsassert.OutputTail(listing, 20))
	}
	var named []string
	for _, line := range strings.Split(listing, "\n") {
		if strings.HasPrefix(line, "Test") {
			named = append(named, strings.TrimSpace(line))
		}
	}
	if len(named) == 0 {
		t.Fatalf("internal/core/advisor holds no test whose name matches %q; the fleet worktree invariant is unpinned", pattern)
	}
	out, code := goTest(t, "test", "-count=1", "-run", pattern, "./internal/core/advisor")
	if code != 0 {
		t.Errorf("fleet worktree tests %v failed:\n%s", named, acsassert.OutputTail(out, 30))
	}
}

func TestC2136_006_RouterAndAdvisorPackagesPassWithCountOne(t *testing.T) {
	for _, pkg := range []string{"./internal/router", "./internal/core/advisor"} {
		out, code := goTest(t, "test", "-count=1", pkg)
		if code != 0 {
			t.Errorf("go test -count=1 %s exited %d:\n%s", pkg, code, acsassert.OutputTail(out, 30))
		}
	}
}
