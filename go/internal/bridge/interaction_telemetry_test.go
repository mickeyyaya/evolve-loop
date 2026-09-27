package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// readInteractionLedger parses <ws>/<phase>-interactions.ndjson.
func readInteractionLedger(t *testing.T, ws, phase string) []interaction.Outcome {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(ws, phase+"-interactions.ndjson"))
	if err != nil {
		t.Fatalf("interaction ledger for %q must exist: %v", phase, err)
	}
	var outs []interaction.Outcome
	for _, ln := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if ln == "" {
			continue
		}
		var o interaction.Outcome
		if err := json.Unmarshal([]byte(ln), &o); err != nil {
			t.Fatalf("ledger line must parse: %v\n%s", err, ln)
		}
		outs = append(outs, o)
	}
	return outs
}

// nudgeReactiveTmux simulates an agent that reacts to the nudge: when the nudge text (naming the artifact
// path) is sent into the pane, the "agent" writes the artifact, so the nudge outcome resolves artifact_appeared.
type nudgeReactiveTmux struct {
	*fakeTmux
	artifact string
	token    string
}

func (n *nudgeReactiveTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if strings.Contains(keys, n.artifact) {
		_ = os.WriteFile(n.artifact, []byte("<!-- challenge-token: "+n.token+" -->\nDONE\n"), 0o644)
	}
	return n.fakeTmux.SendKeys(ctx, session, keys, enter)
}

type nudgeCaptureFailureTmux struct {
	*fakeTmux
	artifact    string
	token       string
	nudgeSent   bool
	captureFail bool
}

func (n *nudgeCaptureFailureTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if strings.Contains(keys, n.artifact) && strings.Contains(keys, "complete the phase") {
		n.nudgeSent = true
	}
	return n.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func (n *nudgeCaptureFailureTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	if n.nudgeSent && !n.captureFail {
		n.captureFail = true
		if err := os.WriteFile(n.artifact, []byte("<!-- challenge-token: "+n.token+" -->\nDONE\n"), 0o644); err != nil {
			return "", err
		}
		return "", errCapture
	}
	return n.fakeTmux.CapturePane(ctx, session, scrollback)
}

// runTelemetryLaunch drives a claude-tmux launch with --agent=build so the
// ledger lands at build-interactions.ndjson.
func runTelemetryLaunch(t *testing.T, fx launchFixture, tm TmuxController, lookup map[string]string) int {
	t.Helper()
	eng := NewEngine(Deps{
		Tmux:      tm,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(lookup),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var stdout, stderr strings.Builder
	return eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=7"), nil, &stdout, &stderr)
}

func TestOutcome_ArtifactAppearedWithinWindow(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tm := &nudgeReactiveTmux{
		fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
		artifact: fx.artifact,
		token:    fx.token,
	}
	if code := runTelemetryLaunch(t, fx, tm, nil); code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK (agent reacted to the nudge)", code)
	}
	outs := readInteractionLedger(t, fx.ws, "build")
	var nudges []interaction.Outcome
	for _, o := range outs {
		if o.Kind == interaction.KindNudge {
			nudges = append(nudges, o)
		}
	}
	if len(nudges) != 1 {
		t.Fatalf("nudge outcomes = %d, want exactly 1; ledger=%+v", len(nudges), outs)
	}
	n := nudges[0]
	if n.Result != interaction.ResultArtifactAppeared {
		t.Errorf("result = %q, want %q", n.Result, interaction.ResultArtifactAppeared)
	}
	if n.Phase != "build" || n.Cycle != 7 || n.Trigger != "idle_no_artifact" {
		t.Errorf("event identity wrong: %+v", n.Event)
	}
	if n.LatencyMS < 0 {
		t.Errorf("latency must be non-negative; got %d", n.LatencyMS)
	}
	if !strings.Contains(n.Payload, "deliverable") {
		t.Errorf("payload should digest the injected nudge text; got %q", n.Payload)
	}
}

func TestOutcome_NudgeCaptureFailureContinuesAndRecordsDeferredResult(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &nudgeCaptureFailureTmux{
		fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
		artifact: fx.artifact,
		token:    fx.token,
	}
	eng := newTestEngine(Deps{
		Tmux:             tmux,
		Sleep:            func(time.Duration) {},
		ArtifactTimeoutS: 4,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var stdout, stderr strings.Builder
	code := eng.LaunchArgs(ctx,
		fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=7"), nil, &stdout, &stderr)

	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK after the non-wedged verification failure; stderr=%q", code, stderr.String())
	}
	if !tmux.captureFail {
		t.Fatal("test premise failed: the nudge verification capture did not fail")
	}
	if got := strings.Count(stderr.String(), "submit-verify: nudge NOT verified \u2014 capture failed"); got != 1 {
		t.Fatalf("nudge capture warnings = %d, want exactly 1; stderr=%q", got, stderr.String())
	}

	var verify, nudge []interaction.Outcome
	for _, outcome := range readInteractionLedger(t, fx.ws, "build") {
		switch {
		case outcome.Kind == interaction.KindSubmitVerify && strings.Contains(outcome.Payload, "site=nudge"):
			verify = append(verify, outcome)
		case outcome.Kind == interaction.KindNudge:
			nudge = append(nudge, outcome)
		}
	}
	if len(verify) != 1 || verify[0].Result != interaction.ResultNotVerified {
		t.Fatalf("nudge submit-verification records = %+v, want one %q result", verify, interaction.ResultNotVerified)
	}
	if len(nudge) != 1 || nudge[0].Result != interaction.ResultArtifactAppeared {
		t.Fatalf("deferred nudge outcomes = %+v, want one %q result", nudge, interaction.ResultArtifactAppeared)
	}
}

func TestOutcome_NoEffectRecordedHonestly(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tm := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	if code := runTelemetryLaunch(t, fx, tm, nil); code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout", code)
	}
	outs := readInteractionLedger(t, fx.ws, "build")
	var nudges []interaction.Outcome
	for _, o := range outs {
		if o.Kind == interaction.KindNudge {
			nudges = append(nudges, o)
		}
	}
	if len(nudges) != 1 {
		t.Fatalf("nudge outcomes = %d, want exactly 1; ledger=%+v", len(nudges), outs)
	}
	if nudges[0].Result != interaction.ResultNoEffect {
		t.Errorf("result = %q, want %q (honest no-effect)", nudges[0].Result, interaction.ResultNoEffect)
	}
}

func TestRecorder_RecordsAtStageOff(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tm := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	code := runTelemetryLaunch(t, fx, tm, map[string]string{"EVOLVE_PHASE_RECOVERY": "off"})
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout", code)
	}
	outs := readInteractionLedger(t, fx.ws, "build")
	if len(outs) == 0 {
		t.Fatal("interaction ledger must be written at stage off — telemetry is decoupled from the dial")
	}
}

// autoRespondHarness builds a recording autoResponder over scripted panes.
func autoRespondHarness(t *testing.T, panes []string, prompts []ManifestPrompt) (*autoResponder, *interaction.Recorder) {
	t.Helper()
	ws := t.TempDir()
	deps := Deps{Tmux: &fakeTmux{paneSeq: panes}, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)}.withDefaults()
	rec := interaction.NewRecorder(ws)
	ar := newAutoResponder("claude-tmux", ws, deps, false, 0)
	ar.prompts = prompts
	ar.rec, ar.phase, ar.cycle = rec, "build", 7
	return ar, rec
}

var trustPrompt = []ManifestPrompt{{Name: "trust_folder", Regex: "Do you trust", Policy: "auto_respond", ResponseKeys: "y,Enter"}}

func TestAutoRespondOutcome_PromptClearedOnNextCapture(t *testing.T) {
	t.Parallel()
	ar, rec := autoRespondHarness(t,
		[]string{"Do you trust this folder?", "● working on the task…"}, trustPrompt)
	ctx := context.Background()
	if _, rc := ar.tick(ctx, "s"); rc != 1 {
		t.Fatalf("first tick must auto-respond (rc=1)")
	}
	if _, rc := ar.tick(ctx, "s"); rc != 0 {
		t.Fatalf("second tick must noop on the cleared pane")
	}
	outs := rec.Outcomes()
	if len(outs) != 1 {
		t.Fatalf("outcomes = %d, want 1", len(outs))
	}
	o := outs[0]
	if o.Kind != interaction.KindAutoRespond || o.RuleID != "trust_folder" {
		t.Errorf("kind/rule wrong: %+v", o.Event)
	}
	if o.Result != interaction.ResultPromptCleared {
		t.Errorf("result = %q, want %q", o.Result, interaction.ResultPromptCleared)
	}
	if o.Phase != "build" || o.Cycle != 7 {
		t.Errorf("event identity wrong: %+v", o.Event)
	}
}

func TestAutoRespondOutcome_NoEffectWhenRuleRefires(t *testing.T) {
	t.Parallel()
	ar, rec := autoRespondHarness(t,
		[]string{"Do you trust this folder?", "Do you trust this folder?"}, trustPrompt)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, rc := ar.tick(ctx, "s"); rc != 1 {
			t.Fatalf("tick %d must auto-respond (rc=1)", i)
		}
	}
	outs := rec.Outcomes()
	if len(outs) != 1 {
		t.Fatalf("outcomes = %d, want 1 (first send resolved; second still pending)", len(outs))
	}
	if outs[0].Result != interaction.ResultNoEffect {
		t.Errorf("result = %q, want %q", outs[0].Result, interaction.ResultNoEffect)
	}
}

func TestAutoRespondOutcome_FlushResolvesPendingAtRunEnd(t *testing.T) {
	t.Parallel()
	ar, rec := autoRespondHarness(t, []string{"Do you trust this folder?"}, trustPrompt)
	if _, rc := ar.tick(context.Background(), "s"); rc != 1 {
		t.Fatalf("tick must auto-respond (rc=1)")
	}
	ar.flushPending()
	outs := rec.Outcomes()
	if len(outs) != 1 {
		t.Fatalf("outcomes = %d, want 1 after flush", len(outs))
	}
	if outs[0].Result != interaction.ResultRunEnded {
		t.Errorf("result = %q, want %q", outs[0].Result, interaction.ResultRunEnded)
	}
}
