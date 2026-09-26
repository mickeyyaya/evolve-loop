package advisor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestLaunch_DepthGuardRefusesBeforeLaunch(t *testing.T) {
	a := New(refusingLauncher{t}, defaultIdentity(), nil, WithDepthCheck(func(map[string]string) bool { return true }))
	if _, err := a.Plan(baseRouteInput()); err == nil || err.Error() != "phase advisor: recursion guard: depth check failed" {
		t.Fatalf("Plan under a tripped depth guard: %v", err)
	}
	if _, err := a.Propose(baseRouteInput()); err == nil || err.Error() != "routing proposer: recursion guard: depth check failed" {
		t.Fatalf("Propose under a tripped depth guard: %v", err)
	}
}

func TestAdvisor_ErrorTextsAreTheOrchestratorsStderrText(t *testing.T) {
	noWs := baseRouteInput()
	noWs.Workspace = ""
	propose := func(a *Advisor, in router.RouteInput) error { _, err := a.Propose(in); return err }
	plan := func(a *Advisor, in router.RouteInput) error { _, err := a.Plan(in); return err }
	cases := []struct {
		name   string
		l      Launcher
		in     router.RouteInput
		launch func(*Advisor, router.RouteInput) error
		want   string
	}{
		{"proposal nil bridge", nil, baseRouteInput(), propose, "routing proposer: nil bridge"},
		{"plan nil bridge", nil, baseRouteInput(), plan, "phase advisor: nil bridge"},
		{"proposal empty workspace", &fakeLauncher{stdout: "{}"}, noWs, propose, "routing proposer: empty workspace"},
		{"plan empty workspace", &fakeLauncher{stdout: "[]"}, noWs, plan, "phase advisor: empty workspace"},
		{"proposal bridge error", &fakeLauncher{err: errors.New("boom")}, baseRouteInput(), propose, "routing proposer: bridge launch: boom"},
		{"plan bridge error", &fakeLauncher{err: errors.New("boom")}, baseRouteInput(), plan, "phase advisor: bridge launch: boom"},
		{"no object", &fakeLauncher{stdout: "I could not decide."}, baseRouteInput(), propose, "routing proposer: no JSON object in proposer output"},
		{"malformed object", &fakeLauncher{stdout: `{"next_phase": }`}, baseRouteInput(), propose, "routing proposer: parse proposal: invalid character '}' looking for beginning of value"},
		{"empty proposal", &fakeLauncher{stdout: `{"justification":"nothing"}`}, baseRouteInput(), propose, "routing proposer: empty proposal"},
		{"no array", &fakeLauncher{stdout: "I could not decide."}, baseRouteInput(), plan, "phase advisor: no JSON array in plan output"},
		{"malformed array", &fakeLauncher{stdout: `[{"phase":}]`}, baseRouteInput(), plan, "phase advisor: parse phase plan: invalid character '}' looking for beginning of value"},
		{"empty plan", &fakeLauncher{stdout: "[]"}, baseRouteInput(), plan, "phase advisor: empty phase plan"},
	}
	for _, c := range cases {
		var l Launcher
		if c.l != nil {
			l = c.l
		}
		if err := c.launch(New(l, defaultIdentity(), nil), c.in); err == nil || err.Error() != c.want {
			t.Errorf("%s: got %v, want %q", c.name, err, c.want)
		}
	}
}

func TestAdvisor_HappyPathsEmitNoSignal(t *testing.T) {
	in := tempInput(t)
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	a, got := observed(t, &fakeLauncher{stdout: planJSON()}, Identity{CLI: "claude-tmux", Model: "deep", Persona: "PERSONA", AgentLabel: "router"})
	if _, err := a.Plan(in); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RePlan(in); err != nil {
		t.Fatal(err)
	}
	p, pgot := observed(t, &fakeLauncher{stdout: `{"next_phase":"audit","justification":"ok"}`}, defaultIdentity())
	if _, err := p.Propose(in); err != nil {
		t.Fatal(err)
	}
	os.Stderr = orig
	_ = w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if n != 0 {
		t.Errorf("happy paths must write nothing to stderr: %q", buf[:n])
	}
	if len(*got) != 0 || len(*pgot) != 0 {
		t.Errorf("happy paths must emit nothing on the stream: %+v %+v", *got, *pgot)
	}
	if !a.SignalsWired() {
		t.Error("the observed advisor reaches its Center")
	}
}

func TestWithSignals_NilAccessorAndNilCenterAreTheNullObject(t *testing.T) {
	for _, opts := range [][]Option{nil, {WithSignals(nil)}, {WithSignals(func() *signalcenter.Center { return nil })}} {
		a := New(nil, defaultIdentity(), nil, opts...)
		if a.SignalsWired() {
			t.Fatal("no Center ⇒ not wired")
		}
		if _, err := a.Plan(baseRouteInput()); err == nil {
			t.Fatal("a nil launcher still fails")
		}
	}
}

func TestWithSignals_ReadsTheCenterLive(t *testing.T) {
	var c *signalcenter.Center
	a := New(nil, defaultIdentity(), nil, WithSignals(func() *signalcenter.Center { return c }))
	if a.SignalsWired() {
		t.Fatal("not wired before the Center exists")
	}
	c = signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	if !a.SignalsWired() {
		t.Fatal("wired once the accessor's target exists")
	}
	if _, err := a.Plan(baseRouteInput()); err == nil || len(got) != 1 || got[0].Code != CodeLaunchFailed {
		t.Fatalf("the swapped-in Center receives the fault: %v %+v", err, got)
	}
}

func TestAdvisorCodes_AreRegisteredWithDocsUnderModuleAdvisor(t *testing.T) {
	for _, code := range []signalcenter.Code{CodeLaunchFailed, CodeResponseUnparseable, CodeMintRejected, CodeProfileLoadFailed, CodeReconGitFailed, CodeCaptureWriteFailed} {
		m, ok := signalcenter.IsRegistered(code)
		if !ok || m != signalcenter.ModuleAdvisor {
			t.Errorf("%s: registered=%v module=%s", code, ok, m)
		}
		if !code.BelongsTo(signalcenter.ModuleAdvisor) || !code.Valid() {
			t.Errorf("%s: carries the ADVISOR_ prefix and the MODULE_SNAKE_CASE shape", code)
		}
	}
	if !signalcenter.KindAdvisorWarning.Known() || signalcenter.KindAdvisorWarning.Terminal() {
		t.Error("advisor.warning is a known, non-terminal kind")
	}
}

func TestAdvisorEvents_FieldVocabularyMatchesTheRegisteredReasons(t *testing.T) {
	docs := map[signalcenter.Code]string{}
	for _, cd := range signalcenter.RegisteredCodes()[signalcenter.ModuleAdvisor] {
		docs[cd.Code] = cd.Doc
	}
	in := tempInput(t)
	in.Cfg.ReconDigest = true
	if err := os.Mkdir(filepath.Join(in.Workspace, "advisor-prompt-plan.txt"), 0o755); err != nil { // the plan capture's write fails
		t.Fatal(err)
	}
	persona := Identity{CLI: "claude-tmux", Model: "deep", Persona: "PERSONA", AgentLabel: "router"}
	gitFault := WithRecentFiles(func(string) ([]string, error) { return nil, errors.New("git: boom") })
	profileFault := WithProfileLoader(func(string) (*profiles.Profile, error) { return nil, errors.New("bad json") })
	plan := func(a *Advisor) { _, _ = a.Plan(in) }
	propose := func(a *Advisor) { _, _ = a.Propose(in) }
	var events []signalcenter.Event
	for _, run := range []struct {
		l    Launcher
		opts []Option
		call func(*Advisor)
	}{
		{nil, nil, plan}, // preflight
		{&fakeLauncher{err: errors.New("boom")}, []Option{profileFault}, plan},                                     // dispatch: the profile fault, then the chain
		{&fakeLauncher{stdout: "no decision"}, nil, propose},                                                       // parse / no_json
		{&fakeLauncher{stdout: `{"next_phase": }`}, nil, propose},                                                  // parse / invalid_json
		{&fakeLauncher{stdout: "[]"}, nil, plan},                                                                   // parse / empty (+ the capture write fault)
		{&fakeLauncher{stdout: `[{"phase":"router","run":true,"mint":{"prompt":"x"}}]`}, []Option{gitFault}, plan}, // compose, capture, mint
	} {
		a, got := observed(t, run.l, persona, run.opts...)
		run.call(a)
		events = append(events, *got...)
	}
	steps := map[string]bool{}
	for _, e := range events {
		steps[e.Fields["step"]] = true
		for field, form := range map[string]string{"step": "fields.step=", "cause": "", "op": ""} {
			if v, ok := e.Fields[field]; ok && !strings.Contains(docs[e.Code], form+v) {
				t.Errorf("%s emits fields.%s=%q, which its registered reason does not name: %q", e.Code, field, v, docs[e.Code])
			}
		}
	}
	for _, want := range []string{stepPreflight, stepDispatch, stepParse, stepMint, stepCompose, stepCapture} {
		if !steps[want] {
			t.Errorf("fields.step=%q was never emitted; steps seen: %v", want, steps)
		}
	}
}

// Positional literals on purpose: a new field breaks the build here, so the core adapter is revisited.
func TestRequestShapes_HaveExactlyTheDeclaredFields(t *testing.T) {
	req := LaunchRequest{"claude-tmux", "/p.json", "deep", []string{"fable"}, "prompt", "/ws", "/wt", "/root", "/ws/routing-plan.json", "artifact", "router", "router", 7, map[string]string{"k": "v"}}
	resp := LaunchResponse{0, "out", 12, cyclestate.TokenUsage{Input: 1}}
	span := Span{"m", "s", "p", "r", 5, 1, cyclestate.TokenUsage{}}
	id := Identity{"claude-tmux", "opus", "", "", "router"}
	if req.Cycle != 7 || resp.DurationMS != 12 || span.ReplanDepth != 1 || id.AgentLabel != "router" {
		t.Fatal("positional shapes")
	}
	var (
		_ ArtifactWriter  = plainWriter
		_ ProfileLoader   = defaultProfileLoader
		_ RecentFiles     = goldenRecentFiles
		_ DepthCheck      = func(map[string]string) bool { return false }
		_ OverlayResolver = func(string) []string { return nil }
		_ Launcher        = (*fakeLauncher)(nil)
		_ Option          = WithSignals(nil)
		_ *Advisor        = New(nil, id, nil)
		_ RejectedMint    = RejectedMint{}
		_ ParsedPlan      = ParsedPlan{}
	)
	if prof, err := defaultProfileLoader("/nonexistent/dir/router.json"); prof != nil || err == nil {
		t.Error("the default loader returns the read error")
	}
	if _, err := (&fakeLauncher{stdout: "x"}).Launch(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	_ = profiles.Profile{}
	if strings.TrimSpace(string(CodeLaunchFailed)) != "ADVISOR_LAUNCH_FAILED" {
		t.Error("code spelling")
	}
}
