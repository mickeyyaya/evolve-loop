package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// deliveryFailureReasonToken mirrors interaction.ResultSubmitWedged, so the
// timeout marker, the ndjson ledger, and any downstream classifier agree on
// one word for a verified delivery failure.
const deliveryFailureReasonToken = "submit_wedged"

// parkedPromptTmux keeps the pasted prompt visible at the input line forever:
// no number of bare Enters clears it (the paste landed, the submit never took).
type parkedPromptTmux struct {
	*fakeTmux
	mu         sync.Mutex
	pasted     bool
	parkedText string
}

func (p *parkedPromptTmux) PasteBuffer(ctx context.Context, session string) error {
	err := p.fakeTmux.PasteBuffer(ctx, session)
	p.mu.Lock()
	p.pasted = true
	p.mu.Unlock()
	return err
}

func (p *parkedPromptTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	out, err := p.fakeTmux.CapturePane(ctx, session, scrollback)
	p.mu.Lock()
	pasted := p.pasted
	p.mu.Unlock()
	if pasted {
		return unsubmittedPane(p.parkedText), err
	}
	return out, err
}

func TestTmuxREPL_CleanSubmit_NeverClassifiesDeliveryFailure(t *testing.T) {
	cfg := fixtureConfig(t)
	base := &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "working ❯", "final scrollback", "cleanup scrollback"}}
	tm := &artifactOnPasteTmux{FakeTmuxController: base, artifact: cfg.Artifact}
	deps := fixtureDeps(tm)
	var stderr bytes.Buffer
	deps.Stderr = &stderr

	code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "clean-submit", launchCmd: "claude",
		promptMarker: tmuxPromptMarkerDefault, inputLineMarker: tmuxPromptMarkerDefault, bootIntervalS: 1,
	})
	if err != nil || code != ExitOK {
		t.Fatalf("runTmuxREPL = (%d, %v), want (%d, nil); stderr=%s", code, err, ExitOK, stderr.String())
	}
	if strings.Contains(stderr.String(), artifactTimeoutMarker) {
		t.Errorf("a verified-clean submission emitted an artifact-timeout marker — the delivery-failure "+
			"short-circuit fires on healthy launches; stderr=%s", stderr.String())
	}
	if strings.Contains(stderr.String(), deliveryFailureReasonToken) {
		t.Errorf("a verified-clean submission was classified %q — false delivery-failure attribution; stderr=%s",
			deliveryFailureReasonToken, stderr.String())
	}
}

func TestTmuxREPL_SilentPaneTimeout_NotClassifiedAsDeliveryFailure(t *testing.T) {
	cfg := fixtureConfig(t)
	// Input line clear after the paste (nothing follows the marker) => the
	// submit verifies clean; the artifact simply never appears.
	tm := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	deps := fixtureDeps(tm)
	var stderr bytes.Buffer
	deps.Stderr = &stderr
	artifactWaitPolls := 0
	deps.Sleep = func(d time.Duration) {
		if d == 2*time.Second {
			artifactWaitPolls++
		}
	}

	code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "silent-pane", launchCmd: "claude",
		promptMarker: tmuxPromptMarkerDefault, inputLineMarker: tmuxPromptMarkerDefault, bootIntervalS: 1,
	})
	if err != nil || code != ExitArtifactTimeout {
		t.Fatalf("runTmuxREPL = (%d, %v), want (%d, nil); stderr=%s", code, err, ExitArtifactTimeout, stderr.String())
	}
	if artifactWaitPolls == 0 {
		t.Errorf("a generically silent pane short-circuited the artifact wait (0 polls) — the delivery-failure "+
			"exit fires without a submit-verification failure; stderr=%s", stderr.String())
	}
	summary := artifactTimeoutSummary(stderr.String())
	if summary == "" {
		t.Fatalf("no %q line on stderr; stderr=%s", artifactTimeoutMarker, stderr.String())
	}
	if strings.Contains(summary, deliveryFailureReasonToken) {
		t.Errorf("generic silence was recorded as a delivery failure — cause must stay the stop-review reason\n  summary: %s", summary)
	}
}

func TestTmuxREPL_NudgeSubmitWedged_ClassifiedCauseSurvivesIntoMarker(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tm := &stickyInputTmux{
		fakeTmux:      &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
		trigger:       fx.artifact,
		clearOnResend: false, // pathological pane: the input line never clears
	}
	code, stderr := runSubmitVerify(t, fx, tm)
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%s", code, stderr)
	}
	if ni := nudgeSeqIdx(tm.fakeTmux.sentSeq, fx.artifact); ni < 0 {
		t.Fatalf("precondition: the one-shot nudge was never sent; sentSeq=%v", tm.fakeTmux.sentSeq)
	}
	summary := artifactTimeoutSummary(stderr)
	if summary == "" {
		t.Fatalf("no %q line on stderr; stderr=%s", artifactTimeoutMarker, stderr)
	}
	if !strings.Contains(summary, deliveryFailureReasonToken) {
		t.Errorf("a wedged NUDGE died with the generic stall reason — the classified delivery-failure cause "+
			"never reached the marker, so failure-learning cannot tell an undelivered nudge from a silent agent\n  summary: %s", summary)
	}
	if !strings.Contains(summary, "cause=submit_wedged") {
		t.Errorf("a wedged nudge lacks its stable cause code; summary=%q", summary)
	}
	if !strings.Contains(summary, "nudge") {
		t.Errorf("delivery-failure cause does not name the submission SITE — an operator cannot tell an "+
			"undelivered prompt from an undelivered nudge\n  summary: %s", summary)
	}
}

func TestEngineLaunch_PromptSubmitWedged_PhaseErrorCarriesClassifiedCause(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	const prompt = "Please produce the retrospective deliverable for this cycle now."
	tm := &parkedPromptTmux{
		fakeTmux:   &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
		parkedText: prompt,
	}
	eng := newTestEngine(Deps{
		Tmux:               tm,
		Sleep:              func(time.Duration) {},
		ArtifactTimeoutS:   2,
		ArtifactMaxExtends: 4,
		LookupEnv:          mapLookup(nil),
	})

	_, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: fx.profile, Model: "auto", Prompt: prompt,
		Workspace: fx.ws, ArtifactPath: filepath.Join(fx.ws, "a.md"),
		Agent: "retro", PermissionMode: "plan",
	})
	if err == nil {
		t.Fatal("expected an error when the prompt is never submitted")
	}
	if !errorsIsArtifactTimeout(err) {
		t.Fatalf("delivery failure must stay an ErrArtifactTimeout so cyclerun_dispatch relaunches it once; got %v", err)
	}
	got := err.Error()
	for _, want := range []string{artifactTimeoutMarker, deliveryFailureReasonToken, "prompt", "resends="} {
		if !strings.Contains(got, want) {
			t.Errorf("terminal phase error is missing %q — the classified delivery cause did not survive the "+
				"bridge boundary, leaving an exhausted retro with a bare artifact timeout\n  got: %s", want, got)
		}
	}
}

func TestTmuxREPL_ParkedPaneWithDeliveredArtifact_CompletesOK(t *testing.T) {
	cfg := fixtureConfig(t)
	base := &FakeTmuxController{CaptureFrames: []string{"❯", "❯ still parked prompt text here", "❯ still parked prompt text here", "❯ still parked prompt text here", "❯ still parked prompt text here", "❯ still parked prompt text here", "❯ still parked prompt text here", "❯ still parked prompt text here", "❯ still parked prompt text here", "❯ still parked prompt text here", "❯ still parked prompt text here", "❯ still parked prompt text here"}}
	promptBody, rerr := os.ReadFile(cfg.PromptFile)
	if rerr != nil {
		t.Fatal(rerr)
	}
	tm := &parkedButDeliveringTmux{FakeTmuxController: base, artifact: cfg.Artifact,
		parkedText: firstNonEmptyLine(string(promptBody))}
	deps := fixtureDeps(tm)
	var stderr bytes.Buffer
	deps.Stderr = &stderr

	code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "parked-delivered", launchCmd: "claude",
		promptMarker: tmuxPromptMarkerDefault, inputLineMarker: tmuxPromptMarkerDefault, bootIntervalS: 1,
	})
	if err != nil || code != ExitOK {
		t.Fatalf("runTmuxREPL = (%d, %v), want (%d, nil) — a delivered artifact must beat the parked-pane "+
			"heuristic; stderr=%s", code, err, ExitOK, stderr.String())
	}
	if !strings.Contains(stderr.String(), "submission evidently landed") {
		t.Errorf("the ground-truth override must be loud; stderr=%s", stderr.String())
	}
}

// parkedButDeliveringTmux shows a forever-parked input line AND writes the
// artifact when the prompt is pasted — the side-effect-answering REPL shape.
type parkedButDeliveringTmux struct {
	*FakeTmuxController
	artifact   string
	parkedText string
	pasted     bool
}

func (p *parkedButDeliveringTmux) PasteBuffer(ctx context.Context, session string) error {
	err := p.FakeTmuxController.PasteBuffer(ctx, session)
	p.pasted = true
	_ = os.WriteFile(p.artifact, []byte("delivered\n"), 0o644)
	return err
}

func (p *parkedButDeliveringTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	out, err := p.FakeTmuxController.CapturePane(ctx, session, scrollback)
	if p.pasted {
		return unsubmittedPane(p.parkedText), err
	}
	return out, err
}

func TestTmuxREPL_ParkedPaneWithOnlyStaleArtifact_StillFastFails(t *testing.T) {
	cfg := fixtureConfig(t)
	if err := os.WriteFile(cfg.Artifact, []byte("stale prior report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(cfg.Artifact, old, old); err != nil {
		t.Fatal(err)
	}
	promptBody, rerr := os.ReadFile(cfg.PromptFile)
	if rerr != nil {
		t.Fatal(rerr)
	}
	base := &FakeTmuxController{CaptureFrames: []string{"❯", "❯ parked", "❯ parked", "❯ parked", "❯ parked", "❯ parked", "❯ parked"}}
	tm := &parkedNeverDeliveringTmux{FakeTmuxController: base, parkedText: firstNonEmptyLine(string(promptBody))}
	deps := fixtureDeps(tm)
	deps.CaptureBaseline = captureArtifactBaseline // REAL capture: the stale file becomes the baseline
	var stderr bytes.Buffer
	deps.Stderr = &stderr

	code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "parked-stale", launchCmd: "claude",
		promptMarker: tmuxPromptMarkerDefault, inputLineMarker: tmuxPromptMarkerDefault, bootIntervalS: 1,
	})
	if err != nil || code != ExitArtifactTimeout {
		t.Fatalf("runTmuxREPL = (%d, %v), want (%d, nil) — a stale leftover must not defuse the wedged "+
			"fast-fail; stderr=%s", code, err, ExitArtifactTimeout, stderr.String())
	}
}

type parkedNeverDeliveringTmux struct {
	*FakeTmuxController
	parkedText string
	pasted     bool
}

func (p *parkedNeverDeliveringTmux) PasteBuffer(ctx context.Context, session string) error {
	err := p.FakeTmuxController.PasteBuffer(ctx, session)
	p.pasted = true
	return err
}

func (p *parkedNeverDeliveringTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	out, err := p.FakeTmuxController.CapturePane(ctx, session, scrollback)
	if p.pasted {
		return unsubmittedPane(p.parkedText), err
	}
	return out, err
}
