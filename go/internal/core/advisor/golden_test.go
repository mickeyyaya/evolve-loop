package advisor

// golden_test.go — the goldens captured on 8e8f080f (the pre-extraction
// code) replayed through the leaf: every prompt shape, the launch request per
// decision, the capture artifacts (ADR-0103 unit 04 §6 tests 4-8).

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// goldenRecentFiles is exactly what `git log -n 30 --name-only` printed for
// the two-commit history the recon golden was captured over (duplicates
// kept — frequency = churn).
func goldenRecentFiles(string) ([]string, error) {
	return []string{"a.go", "c.py", "docs/README.md", "a.go", "b_test.go"}, nil
}

// Test 4 — G1/G2/G3: the per-transition prompt, byte for byte.
func TestBuildRoutingPrompt_MatchesGolden(t *testing.T) {
	rich := richRouteInput()
	assertGolden(t, "prompt-routing-happy.golden.txt", buildRoutingPrompt(rich))
	auditFail := rich
	auditFail.Current, auditFail.Verdict = "audit", "FAIL"
	assertGolden(t, "prompt-routing-audit-fail.golden.txt", buildRoutingPrompt(auditFail))
	retro := rich
	retro.Current, retro.Verdict = "retro", "FAIL"
	assertGolden(t, "prompt-routing-retro.golden.txt", buildRoutingPrompt(retro))
}

// Test 5 — G4: the legacy inline plan prompt.
func TestBuildPlanPrompt_MatchesGolden(t *testing.T) {
	assertGolden(t, "prompt-plan-legacy.golden.txt", buildPlanPrompt(richRouteInput()))
}

// Test 6 — G5/G6/G7: the persona-composed plan prompt with the recon off, on
// (the injected reader standing in for the git history) and for the re-plan
// artifact.
func TestComposePlanPrompt_MatchesGolden(t *testing.T) {
	rich := richRouteInput()
	a := New(nil, Identity{Persona: goldenPersona}, nil, WithRecentFiles(goldenRecentFiles))
	assertGolden(t, "prompt-plan-persona.golden.txt", a.ComposePlanPrompt(rich, "routing-plan.json"))
	assertGolden(t, "prompt-plan-replan.golden.txt", a.ComposePlanPrompt(rich, "routing-replan.json"))
	recon := rich
	recon.Cfg.ReconDigest = true
	recon.ProjectRoot = "/repo"
	assertGolden(t, "prompt-plan-persona-recon.golden.txt", a.ComposePlanPrompt(recon, "routing-plan.json"))
}

// Test 7 — G8: the launch request per decision, every one of the fourteen
// fields (the golden was captured as the bridge request; its json keys match).
func TestLaunchRequest_GoldenPerDecision(t *testing.T) {
	ws, root, wt := t.TempDir(), t.TempDir(), t.TempDir()
	for _, d := range []struct {
		name   string
		stdout string
		launch func(*Advisor, router.RouteInput) error
	}{
		{"proposal", `{"next_phase":"audit","justification":"build green"}`, func(a *Advisor, in router.RouteInput) error { _, err := a.Propose(in); return err }},
		{"plan", planJSON(), func(a *Advisor, in router.RouteInput) error { _, err := a.Plan(in); return err }},
		{"replan", planJSON(), func(a *Advisor, in router.RouteInput) error { _, err := a.RePlan(in); return err }},
	} {
		fl := &fakeLauncher{stdout: d.stdout}
		a := New(fl, Identity{CLI: "claude-tmux", Model: "deep", Persona: goldenPersona, AgentLabel: "router"}, plainWriter)
		if err := d.launch(a, launchRouteInput(ws, root, wt)); err != nil {
			t.Fatalf("%s: %v", d.name, err)
		}
		raw := strings.NewReplacer("{{WT}}", wt, "{{WS}}", ws, "{{ROOT}}", root).Replace(readGolden(t, "launch-"+d.name+".golden.json"))
		var want LaunchRequest
		if err := json.Unmarshal([]byte(raw), &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(fl.gotReq, want) {
			t.Errorf("%s: the launch request drifted from the pre-extraction golden:\n got %+v\nwant %+v", d.name, fl.gotReq, want)
		}
	}
}

// Test 8 — G9: the three capture artifacts, byte for byte (the persisted
// prompt redacted, the response raw, the span's keys/order/values with the
// token usage and the re-plan depth).
func TestCaptureArtifacts_MatchGolden(t *testing.T) {
	for _, d := range []struct {
		kind   string
		launch func(*Advisor, router.RouteInput) error
	}{
		{"plan", func(a *Advisor, in router.RouteInput) error { _, err := a.Plan(in); return err }},
		{"replan", func(a *Advisor, in router.RouteInput) error { _, err := a.RePlan(in); return err }},
	} {
		written := map[string][]byte{}
		fl := &fakeLauncher{stdout: planJSON(), durationMS: 1234, tokens: cyclestate.TokenUsage{Input: 10, Output: 20, CacheRead: 3, CacheWrite: 4}}
		a := New(fl, Identity{CLI: "claude-tmux", Model: "deep", Persona: goldenPersona, AgentLabel: "router"},
			func(path string, data []byte) error { written[filepath.Base(path)] = data; return nil })
		if err := d.launch(a, launchRouteInput("/ws/cycle-7", "/proj", "/wt/cycle-7")); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "capture-prompt-"+d.kind+".golden.txt", string(written["advisor-prompt-"+d.kind+".txt"]))
		assertGolden(t, "capture-response-"+d.kind+".golden.txt", string(written["advisor-response-"+d.kind+".txt"]))
		assertGolden(t, "capture-span-"+d.kind+".golden.json", string(written["advisor-span-"+d.kind+".json"]))
		if got, want := string(written["advisor-prompt-"+d.kind+".txt"]), panetrust.RedactSecrets(fl.gotReq.Prompt); got != want {
			t.Errorf("%s: the persisted prompt is the redacted live prompt", d.kind)
		}
	}
}
