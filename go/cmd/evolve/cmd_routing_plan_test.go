package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	planCycle      = 7
	planGoalMarker = "plan-goal-marker-7f3a"
	planPersona    = "ROUTING-PLAN-TEST-PERSONA"
	planResponse   = `[{"phase":"scout","run":true,"justification":"fresh discovery work"},{"phase":"triage","run":false,"justification":"carryover already queued"}]`
	planFailure    = "boom-cause-xyz"
)

type planProject struct {
	root string
	ws   string
}

func newPlanProject(t *testing.T) planProject {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(ws, "cycle-state.json"), map[string]any{
		"cycle_id":       planCycle,
		"goal_text":      planGoalMarker,
		"workspace_path": ws,
	})
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte("recorded scout report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return planProject{root: root, ws: ws}
}

func snapshotTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			out[rel] = "dir"
			return nil
		}
		buf, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(buf)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func assertTreeUnchanged(t *testing.T, before, after map[string]string) {
	t.Helper()
	for path, sum := range before {
		got, ok := after[path]
		if !ok {
			t.Errorf("routing plan removed %s from the project's runs tree", path)
		} else if got != sum {
			t.Errorf("routing plan changed %s", path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("routing plan wrote %s under the project's runs tree", path)
		}
	}
}

type planAdvisorSeam struct {
	bridge *fixtures.FakeBridge
	calls  int
}

func installPlanAdvisor(t *testing.T, bridge *fixtures.FakeBridge) *planAdvisorSeam {
	t.Helper()
	seam := &planAdvisorSeam{bridge: bridge}
	prev := routingPlanAdvisor
	routingPlanAdvisor = func(string, io.Writer) (*core.PhaseAdvisor, error) {
		seam.calls++
		return core.NewPhaseAdvisor(bridge, core.WithPersona(planPersona), core.WithProposerCLI("claude-tmux")), nil
	}
	t.Cleanup(func() { routingPlanAdvisor = prev })
	return seam
}

func runPlan(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = runRouting(append([]string{"plan"}, args...), nil, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRoutingPlan_PrintsParsedPlanAndWritesNothingUnderTheCycle(t *testing.T) {
	p := newPlanProject(t)
	runsDir := filepath.Join(p.root, ".evolve", "runs")
	before := snapshotTree(t, runsDir)
	bridge := &fixtures.FakeBridge{Resp: core.BridgeResponse{Stdout: planResponse}}
	seam := installPlanAdvisor(t, bridge)

	code, stdout, stderr := runPlan(t, "--cycle", "7", "--project-root", p.root)

	if code != 0 {
		t.Fatalf("routing plan against a fake bridge exit = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}
	if seam.calls != 1 {
		t.Fatalf("the advisor seam was reached %d times, want 1: routing plan must go through routingPlanAdvisor", seam.calls)
	}
	for _, want := range []string{"scout", "RUN", "fresh discovery work", "triage", "SKIP", "carryover already queued"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("plan output missing %q:\n%s", want, stdout)
		}
	}
	assertTreeUnchanged(t, before, snapshotTree(t, runsDir))
	if got := bridge.GotReq.Workspace; got == p.ws || strings.HasPrefix(got, p.ws+string(filepath.Separator)) {
		t.Errorf("the advisor was launched with workspace %s inside the cycle's run dir %s", got, p.ws)
	}
	if got := bridge.GotReq.ArtifactPath; strings.HasPrefix(got, p.ws+string(filepath.Separator)) {
		t.Errorf("the advisor artifact path %s is inside the cycle's run dir", got)
	}
}

func TestRoutingPlan_SendsTheCyclesOwnComposedPrompt(t *testing.T) {
	p := newPlanProject(t)
	bridge := &fixtures.FakeBridge{Resp: core.BridgeResponse{Stdout: planResponse}}
	installPlanAdvisor(t, bridge)

	if code, stdout, stderr := runPlan(t, "--cycle", "7", "--project-root", p.root); code != 0 {
		t.Fatalf("exit = %d; stdout=%s stderr=%s", code, stdout, stderr)
	}

	sent := bridge.GotReq
	in, err := routingPlanInput(p.root, planCycle, "", sent.Workspace)
	if err != nil {
		t.Fatalf("routingPlanInput for the recorded cycle: %v", err)
	}
	composed := advisor.New(nil, advisor.Identity{Persona: planPersona}, nil).ComposePlanPrompt(in, filepath.Base(sent.ArtifactPath))
	if sent.Prompt != composed {
		t.Errorf("the prompt routing plan sent differs from the composer's prompt for the same input\nsent:\n%s\n\ncomposed:\n%s", sent.Prompt, composed)
	}
	if !strings.Contains(sent.Prompt, planGoalMarker) {
		t.Errorf("the prompt does not carry the goal recorded in cycle-state.json (%s):\n%s", planGoalMarker, sent.Prompt)
	}
	if !strings.Contains(sent.Prompt, planPersona) {
		t.Errorf("the prompt does not start from the advisor persona:\n%s", sent.Prompt)
	}
}

func TestRoutingPlan_GoalTextOverridesTheRecordedGoal(t *testing.T) {
	p := newPlanProject(t)

	in, err := routingPlanInput(p.root, planCycle, "operator-supplied-goal-91bc", t.TempDir())

	if err != nil {
		t.Fatalf("routingPlanInput: %v", err)
	}
	if in.GoalText != "operator-supplied-goal-91bc" {
		t.Errorf("GoalText = %q, want the --goal-text value, not the recorded %q", in.GoalText, planGoalMarker)
	}
	if in.Cycle != planCycle {
		t.Errorf("Cycle = %d, want %d", in.Cycle, planCycle)
	}
}

func TestRoutingPlan_UnknownCycleExitsOneWithoutLaunchingTheAdvisor(t *testing.T) {
	p := newPlanProject(t)
	runsDir := filepath.Join(p.root, ".evolve", "runs")
	before := snapshotTree(t, runsDir)
	bridge := &fixtures.FakeBridge{Resp: core.BridgeResponse{Stdout: planResponse}}
	seam := installPlanAdvisor(t, bridge)

	code, stdout, stderr := runPlan(t, "--cycle", "99", "--project-root", p.root)

	if code != 1 {
		t.Errorf("unknown cycle exit = %d, want 1; stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "99") {
		t.Errorf("stderr must name the unknown cycle 99: %q", stderr)
	}
	if seam.calls != 0 || bridge.GotReq.Prompt != "" {
		t.Errorf("an unknown cycle launched the advisor (seam calls=%d, prompt sent=%t)", seam.calls, bridge.GotReq.Prompt != "")
	}
	if strings.Contains(stdout, "RUN") || strings.Contains(stdout, "SKIP") {
		t.Errorf("an unknown cycle printed a plan:\n%s", stdout)
	}
	assertTreeUnchanged(t, before, snapshotTree(t, runsDir))
}

func TestRoutingPlan_AdvisorLaunchFailureExitsTwoWithTheCause(t *testing.T) {
	p := newPlanProject(t)
	runsDir := filepath.Join(p.root, ".evolve", "runs")
	before := snapshotTree(t, runsDir)
	bridge := &fixtures.FakeBridge{Resp: core.BridgeResponse{ExitCode: 1}, Err: errors.New(planFailure)}
	installPlanAdvisor(t, bridge)

	code, stdout, stderr := runPlan(t, "--cycle", "7", "--project-root", p.root)

	if code != 2 {
		t.Errorf("advisor launch failure exit = %d, want 2; stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, planFailure) {
		t.Errorf("stderr must carry the launch failure cause %q: %q", planFailure, stderr)
	}
	if strings.Contains(stdout, "RUN") || strings.Contains(stdout, "SKIP") {
		t.Errorf("a failed launch printed a plan:\n%s", stdout)
	}
	assertTreeUnchanged(t, before, snapshotTree(t, runsDir))
}

func TestRoutingPlan_AdvisorWiringFailureIsNotAnUnknownCycle(t *testing.T) {
	p := newPlanProject(t)
	prev := routingPlanAdvisor
	routingPlanAdvisor = func(string, io.Writer) (*core.PhaseAdvisor, error) {
		return nil, errors.New("wiring-refused-cause")
	}
	t.Cleanup(func() { routingPlanAdvisor = prev })

	code, _, stderr := runPlan(t, "--cycle", "7", "--project-root", p.root)

	if code != 2 {
		t.Errorf("an advisor that cannot be wired exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "wiring-refused-cause") {
		t.Errorf("stderr must carry the wiring cause: %q", stderr)
	}
}

func TestRoutingPlan_UsageErrors(t *testing.T) {
	p := newPlanProject(t)
	bridge := &fixtures.FakeBridge{Resp: core.BridgeResponse{Stdout: planResponse}}
	seam := installPlanAdvisor(t, bridge)

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"missing cycle", []string{"--project-root", p.root}},
		{"zero cycle", []string{"--cycle", "0", "--project-root", p.root}},
		{"negative cycle", []string{"--cycle", "-3", "--project-root", p.root}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if code, _, stderr := runPlan(t, tc.args...); code != 10 {
				t.Errorf("exit = %d, want 10 (usage); stderr=%s", code, stderr)
			}
		})
	}
	if seam.calls != 0 {
		t.Errorf("a usage error launched the advisor %d times", seam.calls)
	}

	var out, errb bytes.Buffer
	if code := runRouting([]string{"bogus"}, nil, &out, &errb); code != 10 || !strings.Contains(errb.String(), "plan") {
		t.Errorf("an unknown subcommand must exit 10 and list plan among the choices: code=%d stderr=%q", code, errb.String())
	}
}
