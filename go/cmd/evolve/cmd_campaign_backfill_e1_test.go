package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/campaign"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseregistrar"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
)

func campaignRunCLI(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := runCampaignRun(args, &out, &errb)
	return out.String(), errb.String(), code
}

func TestRunCampaignRun_ArgumentAndPlanRefusals(t *testing.T) {
	t.Parallel()
	bad := filepath.Join(t.TempDir(), "bad-plan.json")
	if err := os.WriteFile(bad, []byte(`{"version":1,"goal":"","cycles":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"an unknown flag", []string{"--bogus"}, 2, "flag provided but not defined: -bogus"},
		{"no plan", []string{"--simulate"}, 2, "evolve campaign run: --plan is required\n"},
		{"an absent plan", []string{"--plan", filepath.Join(t.TempDir(), "none.json")}, 1, "evolve campaign run: "},
		{"a plan that fails verification", []string{"--plan", bad}, 1, "evolve campaign run: campaign: plan goal is empty\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, errOut, code := campaignRunCLI(tc.args...)
			if code != tc.code || out != "" || !strings.Contains(errOut, tc.want) {
				t.Errorf("code=%d stdout=%q stderr=%q, want %d and %q", code, out, errOut, tc.code, tc.want)
			}
		})
	}
}

func TestRunCampaignRun_EachSetupFailureExits1BeforeAnyLaunch(t *testing.T) {
	launched := fakeLaunchTracking(t)
	plan := writeCampaignTestPlan(t)
	prevAbs, prevExe, prevRead, prevWaves := campaignAbs, campaignExecutable, campaignReadPlan, campaignPlanWaves
	t.Cleanup(func() {
		campaignAbs, campaignExecutable, campaignReadPlan, campaignPlanWaves = prevAbs, prevExe, prevRead, prevWaves
	})
	boom := errors.New("boom")
	cases := []struct {
		name  string
		arm   func()
		extra []string
		want  string
	}{
		{"the project root cannot be resolved", func() { campaignAbs = func(string) (string, error) { return "", boom } },
			[]string{"--project-root", "rel"}, "evolve campaign run: resolve project root: boom\n"},
		{"the plan cannot be ordered into waves", func() {
			campaignPlanWaves = func(*campaign.Plan) ([][]fleet.CycleSpec, error) { return nil, boom }
		}, nil, "evolve campaign run: plan waves: boom\n"},
		{"the binary cannot be resolved", func() { campaignExecutable = func() (string, error) { return "", boom } },
			nil, "evolve campaign run: resolve binary: boom\n"},
		{"the plan cannot be re-read for its progress key", func() { campaignReadPlan = func(string) ([]byte, error) { return nil, boom } },
			nil, "evolve campaign run: read plan for progress key: boom\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			campaignAbs, campaignExecutable, campaignReadPlan, campaignPlanWaves = prevAbs, prevExe, prevRead, prevWaves
			tc.arm()
			out, errOut, code := campaignRunCLI(append([]string{"--plan", plan, "--simulate"}, tc.extra...)...)
			if code != 1 || out != "" || errOut != tc.want {
				t.Errorf("code=%d stdout=%q stderr=%q, want 1 and %q", code, out, errOut, tc.want)
			}
		})
	}
	if len(*launched) != 0 {
		t.Errorf("launched = %v, want no cycle after a setup failure", *launched)
	}
}

func TestRunCampaignRun_AnUnacquirableLeaseExits1(t *testing.T) {
	launched := fakeLaunchTracking(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := campaignRunCLI("--plan", writeCampaignTestPlan(t), "--project-root", root)

	if code != 1 || out != "" || !strings.HasPrefix(errOut, "evolve campaign run: ownership lease: ") {
		t.Errorf("code=%d stdout=%q stderr=%q, want 1 and the lease error", code, out, errOut)
	}
	if len(*launched) != 0 {
		t.Errorf("launched = %v, want none without a lease", *launched)
	}
}

type campaignFakeStudyRunner struct {
	resp core.PhaseResponse
	err  error
	got  *core.PhaseRequest
}

func (f campaignFakeStudyRunner) Name() string { return "preliminary-study" }

func (f campaignFakeStudyRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	*f.got = req
	return f.resp, f.err
}

type campaignStudyFixture struct {
	project, worktree, workspace string
	registered                   *phaseconfig.PhaseConfig
	req                          *core.PhaseRequest
}

func newCampaignStudyFixture(t *testing.T, policyBody string, withPhase, withPrompt bool) campaignStudyFixture {
	t.Helper()
	f := campaignStudyFixture{project: t.TempDir(), worktree: t.TempDir(), registered: &phaseconfig.PhaseConfig{}, req: &core.PhaseRequest{}}
	f.workspace = filepath.Join(f.project, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(f.workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.project, ".evolve", "policy.json"), []byte(policyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	phaseDir := filepath.Join(f.worktree, ".evolve", "phases", "preliminary-study")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if withPhase {
		if err := os.WriteFile(filepath.Join(phaseDir, "phase.json"), []byte(`{"name":"preliminary-study","writes_source":false}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if withPrompt {
		if err := os.WriteFile(filepath.Join(phaseDir, "agent.md"), []byte("study the goal\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(ipcenv.WorktreeRootKey, "")
	t.Setenv("EVOLVE_PROJECT_ROOT", f.worktree)
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	return f
}

func (f campaignStudyFixture) register(t *testing.T, r core.PhaseRunner, err error) {
	t.Helper()
	prev := campaignStudyRegister
	t.Cleanup(func() { campaignStudyRegister = prev })
	campaignStudyRegister = func(_ phaseregistrar.Registrar, cfg phaseconfig.PhaseConfig) (phaseregistrar.Result, error) {
		*f.registered = cfg
		return phaseregistrar.Result{Runner: r}, err
	}
}

func TestRunPreliminaryStudy_RunsThePhaseWithTheCampaignRequest(t *testing.T) {
	for _, verdict := range []string{"", core.VerdictPASS} {
		t.Run("verdict="+verdict, func(t *testing.T) {
			f := newCampaignStudyFixture(t, `{}`, true, true)
			f.register(t, campaignFakeStudyRunner{resp: core.PhaseResponse{Verdict: verdict}, got: f.req}, nil)

			err := runPreliminaryStudy(f.workspace, "split wave two")

			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if f.registered.Prompt != "study the goal\n" || f.registered.Name != "preliminary-study" {
				t.Errorf("registered = %+v, want the phase.json carrying agent.md as its prompt", *f.registered)
			}
			want := core.PhaseRequest{Cycle: 42, ProjectRoot: f.project, Workspace: f.workspace, Worktree: f.worktree, WorktreeReadOnly: true,
				Context: map[string]string{"campaign_feedback": "split wave two"}}
			if f.req.Cycle != want.Cycle || f.req.ProjectRoot != want.ProjectRoot || f.req.Workspace != want.Workspace ||
				f.req.Worktree != want.Worktree || f.req.WorktreeReadOnly != want.WorktreeReadOnly || f.req.Context["campaign_feedback"] != "split wave two" {
				t.Errorf("request = %+v, want %+v", *f.req, want)
			}
			if runner.DefaultRouter == nil {
				t.Error("the study installs the project's router before it runs")
			}
		})
	}
}

func TestRunPreliminaryStudy_Failures(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name              string
		policy            string
		withPhase, withMD bool
		registerErr       error
		runErr            error
		verdict           string
		want              string
	}{
		{"the routing table refuses", `{"cli_routing":`, true, true, nil, nil, "", "the CLI routing table refuses to route"},
		{"no phase.json", `{}`, false, true, nil, nil, "", "phaseconfig: read "},
		{"no agent.md", `{}`, true, false, nil, nil, "", "read preliminary-study prompt: "},
		{"registration refuses", `{}`, true, true, boom, nil, "", "boom"},
		{"the runner fails", `{}`, true, true, nil, boom, "", "boom"},
		{"the verdict is not PASS", `{}`, true, true, nil, nil, core.VerdictFAIL, "preliminary-study verdict=FAIL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newCampaignStudyFixture(t, tc.policy, tc.withPhase, tc.withMD)
			f.register(t, campaignFakeStudyRunner{resp: core.PhaseResponse{Verdict: tc.verdict}, err: tc.runErr, got: f.req}, tc.registerErr)

			err := runPreliminaryStudy(f.workspace, "")

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestRunPreliminaryStudy_AnUnresolvableWorkspaceIsAnError(t *testing.T) {
	prev := campaignAbs
	t.Cleanup(func() { campaignAbs = prev })
	campaignAbs = func(string) (string, error) { return "", errors.New("getwd failed") }

	err := runPreliminaryStudy("rel/cycle-1", "")

	if err == nil || err.Error() != "resolve workspace: getwd failed" {
		t.Errorf("err = %v, want the wrapped resolve error", err)
	}
}
