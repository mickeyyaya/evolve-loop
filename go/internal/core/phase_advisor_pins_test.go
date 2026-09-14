package core

// phase_advisor_pins_test.go — ADR-0103 unit 04 §6 step 1: the pre-move pins
// on the phase advisor, green on the pre-extraction code and each proven red
// against its named mutant before a line moved. They pin the seam the
// composition root, the orchestrator and the ledger read: the error texts
// cyclerun.go prints, the depth guard's refusal, capture-before-parse, the
// prompt/launch/capture goldens and the stderr + stream silence on the happy
// paths. After the move they run through core's seam (the one wired
// construction + the Bridge→Launcher projection); the leaf carries its own
// copies against its exported spellings, and the pure-render goldens (the
// routing/plan prompts, the capture artifacts) live only in the leaf.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// refusingBridge fails the test if the advisor launches through it.
type refusingBridge struct {
	t *testing.T
	fakeBridge
}

func (r *refusingBridge) Launch(_ context.Context, _ BridgeRequest) (BridgeResponse, error) {
	r.t.Fatal("the bridge must not be launched")
	return BridgeResponse{}, nil
}

// readAdvisorArtifact reads a capture artifact, failing the test if it is
// absent — the capture is the behavior under test, so a missing file is a
// real failure, not a skip.
func readAdvisorArtifact(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read capture artifact %s: %v", path, err)
	}
	return string(b)
}

// goldenGitRepo builds the fixed five-touch history the recon golden was
// captured over: a.go twice (a hotspot), b_test.go, c.py and docs/README.md
// once each.
func goldenGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "user.name=g", "-c", "user.email=g@g", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	touch := func(rel string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(filepath.Join(root, rel), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString("x\n"); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	touch("a.go")
	touch("b_test.go")
	run("add", "-A")
	run("commit", "-q", "-m", "one")
	touch("a.go")
	touch("c.py")
	touch("docs/README.md")
	run("add", "-A")
	run("commit", "-q", "-m", "two")
	return root
}

func readAdvisorGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("advisor", "testdata", name))
	if err != nil {
		t.Fatalf("golden %s: %v", name, err)
	}
	return string(b)
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	if want := readAdvisorGolden(t, name); got != want {
		t.Errorf("%s drifted from the pre-extraction golden (first difference at byte %d):\n got: %q\nwant: %q", name, firstDiff(got, want), got, want)
	}
}

// jsonUnmarshalGolden reads a templated launch golden back into v.
func jsonUnmarshalGolden(t *testing.T, name, ws, root, wt string, v any) error {
	t.Helper()
	raw := strings.NewReplacer("{{WT}}", wt, "{{WS}}", ws, "{{ROOT}}", root).Replace(readAdvisorGolden(t, name))
	return json.Unmarshal([]byte(raw), v)
}

func bridgeRequestEqual(a, b BridgeRequest) bool { return reflect.DeepEqual(a, b) }

func firstDiff(a, b string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// Test 1 — the depth guard's TRUE branch refuses BEFORE the launch.
func TestAdvisorLaunch_DepthGuardRefusesBeforeLaunch(t *testing.T) {
	rb := &refusingBridge{t: t}
	adv := NewPhaseAdvisor(rb, WithDepthCheck(func(map[string]string) bool { return true }))
	_, err := adv.Plan(baseRouteInput())
	if err == nil || err.Error() != "phase advisor: recursion guard: depth check failed" {
		t.Fatalf("Plan under a tripped depth guard: %v", err)
	}
	if _, err := adv.Propose(baseRouteInput()); err == nil || err.Error() != "routing proposer: recursion guard: depth check failed" {
		t.Fatalf("Propose under a tripped depth guard: %v", err)
	}
}

// Test 2 — the twelve error texts the orchestrator prints, verbatim.
func TestPhaseAdvisor_ErrorTextsAreTheOrchestratorsStderrText(t *testing.T) {
	noWs := baseRouteInput()
	noWs.Workspace = ""
	propose := func(p *PhaseAdvisor, in router.RouteInput) error { _, err := p.Propose(in); return err }
	plan := func(p *PhaseAdvisor, in router.RouteInput) error { _, err := p.Plan(in); return err }
	cases := []struct {
		name   string
		adv    *PhaseAdvisor
		in     router.RouteInput
		launch func(*PhaseAdvisor, router.RouteInput) error
		want   string
	}{
		{"proposal nil bridge", NewPhaseAdvisor(nil), baseRouteInput(), propose, "routing proposer: nil bridge"},
		{"plan nil bridge", NewPhaseAdvisor(nil), baseRouteInput(), plan, "phase advisor: nil bridge"},
		{"proposal empty workspace", NewPhaseAdvisor(&fakeBridge{stdout: "{}"}), noWs, propose, "routing proposer: empty workspace"},
		{"plan empty workspace", NewPhaseAdvisor(&fakeBridge{stdout: "[]"}), noWs, plan, "phase advisor: empty workspace"},
		{"proposal bridge error", NewPhaseAdvisor(&fakeBridge{err: errors.New("boom")}), baseRouteInput(), propose, "routing proposer: bridge launch: boom"},
		{"plan bridge error", NewPhaseAdvisor(&fakeBridge{err: errors.New("boom")}), baseRouteInput(), plan, "phase advisor: bridge launch: boom"},
		{"no object", NewPhaseAdvisor(&fakeBridge{stdout: "I could not decide."}), baseRouteInput(), propose, "routing proposer: no JSON object in proposer output"},
		{"malformed object", NewPhaseAdvisor(&fakeBridge{stdout: `{"next_phase": }`}), baseRouteInput(), propose, "routing proposer: parse proposal: invalid character '}' looking for beginning of value"},
		{"empty proposal", NewPhaseAdvisor(&fakeBridge{stdout: `{"justification":"nothing"}`}), baseRouteInput(), propose, "routing proposer: empty proposal"},
		{"no array", NewPhaseAdvisor(&fakeBridge{stdout: "I could not decide."}), baseRouteInput(), plan, "phase advisor: no JSON array in plan output"},
		{"malformed array", NewPhaseAdvisor(&fakeBridge{stdout: `[{"phase":}]`}), baseRouteInput(), plan, "phase advisor: parse phase plan: invalid character '}' looking for beginning of value"},
		{"empty plan", NewPhaseAdvisor(&fakeBridge{stdout: "[]"}), baseRouteInput(), plan, "phase advisor: empty phase plan"},
	}
	for _, c := range cases {
		err := c.launch(c.adv, c.in)
		if err == nil || err.Error() != c.want {
			t.Errorf("%s: got %v, want %q", c.name, err, c.want)
		}
	}
}

// Test 3 — the capture lands BEFORE the parse, so an unparseable response is
// still forensically debuggable.
func TestAdvisorLaunch_CapturesEvenWhenTheResponseIsUnparseable(t *testing.T) {
	ws := t.TempDir()
	in := baseRouteInput()
	in.Workspace = ws
	if _, err := NewPhaseAdvisor(&fakeBridge{stdout: "no json here"}).Plan(in); err == nil {
		t.Fatal("an unparseable plan must error")
	}
	if got := readAdvisorArtifact(t, filepath.Join(ws, "advisor-response-plan.txt")); got != "no json here" {
		t.Errorf("the raw response is captured before the parse: %q", got)
	}
	readAdvisorArtifact(t, filepath.Join(ws, "advisor-prompt-plan.txt"))
	readAdvisorArtifact(t, filepath.Join(ws, "advisor-span-plan.json"))
}

// Test 6 — G5/G6/G7: the persona-composed plan prompt with the recon off, on
// (through a real git history) and for the re-plan artifact.
func TestComposePlanPrompt_MatchesGolden(t *testing.T) {
	rich := richRouteInput()
	p := NewPhaseAdvisor(nil, WithPersona(goldenPersona))
	assertGolden(t, "prompt-plan-persona.golden.txt", p.composePlanPrompt(rich, "routing-plan.json"))
	assertGolden(t, "prompt-plan-replan.golden.txt", p.composePlanPrompt(rich, "routing-replan.json"))
	recon := rich
	recon.Cfg.ReconDigest = true
	recon.ProjectRoot = goldenGitRepo(t)
	assertGolden(t, "prompt-plan-persona-recon.golden.txt", p.composePlanPrompt(recon, "routing-plan.json"))
}

// Test 7 — G8: the bridge request per decision, every field.
func TestLaunchRequest_GoldenPerDecision(t *testing.T) {
	ws, root, wt := t.TempDir(), t.TempDir(), t.TempDir()
	for _, d := range []struct {
		name   string
		stdout string
		launch func(*PhaseAdvisor, router.RouteInput) error
	}{
		{"proposal", `{"next_phase":"audit","justification":"build green"}`, func(p *PhaseAdvisor, in router.RouteInput) error { _, err := p.Propose(in); return err }},
		{"plan", `[{"phase":"scout","run":true,"justification":"x"}]`, func(p *PhaseAdvisor, in router.RouteInput) error { _, err := p.Plan(in); return err }},
		{"replan", `[{"phase":"scout","run":true,"justification":"x"}]`, func(p *PhaseAdvisor, in router.RouteInput) error { _, err := p.RePlan(in); return err }},
	} {
		fb := &fakeBridge{stdout: d.stdout}
		adv := NewPhaseAdvisor(fb, WithProposerCLI("claude-tmux"), WithProposerModel("deep"), WithPersona(goldenPersona))
		if err := d.launch(adv, launchRouteInput(ws, root, wt)); err != nil {
			t.Fatalf("%s: %v", d.name, err)
		}
		var want BridgeRequest
		if err := jsonUnmarshalGolden(t, "launch-"+d.name+".golden.json", ws, root, wt, &want); err != nil {
			t.Fatal(err)
		}
		if got := fb.gotReq; !bridgeRequestEqual(got, want) {
			t.Errorf("%s: the bridge request drifted from the pre-extraction golden:\n got %+v\nwant %+v", d.name, got, want)
		}
	}
}

// Test 9 — G10: a successful Propose/Plan/RePlan writes nothing to stderr and
// emits nothing on the stream.
func TestAdvisorStderrAndStream_HappyPathAreSilent(t *testing.T) {
	c, got := recordingCenter()
	ws := t.TempDir()
	in := baseRouteInput()
	in.Workspace = ws
	out := captureStderr(t, func() {
		adv := NewPhaseAdvisor(&fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}, WithPersona("PERSONA"), WithAdvisorSignals(c))
		if _, err := adv.Plan(in); err != nil {
			t.Fatal(err)
		}
		if _, err := adv.RePlan(in); err != nil {
			t.Fatal(err)
		}
		prop := NewPhaseAdvisor(&fakeBridge{stdout: `{"next_phase":"audit","justification":"ok"}`}, WithAdvisorSignals(c))
		if _, err := prop.Propose(in); err != nil {
			t.Fatal(err)
		}
	})
	if out != "" {
		t.Errorf("happy paths must write nothing to stderr: %q", out)
	}
	if len(*got) != 0 {
		t.Errorf("happy paths must emit nothing on the stream: %+v", *got)
	}
}
