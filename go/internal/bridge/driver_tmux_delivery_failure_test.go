package bridge

// driver_tmux_delivery_failure_test.go — RED contract for cycle-1562 tasks
// `retrospective-delivery-relaunch` and the bridge half of
// `retrospective-delivery-evidence-contract`.
//
// Evidence (.evolve/runs/cycle-1510/retrospective-launch-error.txt and
// retrospective-interactions.ndjson): the retro launch logged "prompt
// delivered", produced ZERO tokens and zero cost, and then burned two full
// 900s stop-review intervals before dying with ExitArtifactTimeout. The
// submit-verify guard (driver_tmux_submitverify.go) had ALREADY classified
// that pane as `submit_wedged` within milliseconds — but both call sites in
// driver_tmux_repl.go pipe verifySubmitted's outcome straight into
// recordSubmitVerify, which only appends to the ndjson ledger and returns
// nothing. The classification is produced and never consumed: at the
// control-flow level a detected delivery failure is indistinguishable from a
// healthy launch that simply never speaks.
//
// Contract, in two halves:
//
//   1. RELAUNCH — a `submit_wedged` outcome must short-circuit the artifact
//      wait immediately (ExitArtifactTimeout, which cyclerun_dispatch.go
//      already treats as retryable via IsInfraTeardownError and relaunches
//      exactly once), instead of consuming the full silence budget first.
//   2. EVIDENCE — that early exit must reuse the existing artifactTimeoutMarker
//      shape with a CLASSIFIED reason naming the site and the resend count, so
//      artifactTimeoutSummary lifts it into phaseErr unchanged and the cause
//      survives into failure-learning as data rather than discarded stderr.
//
// The false-negative guards are load-bearing and tested here as negatives: a
// clean submission and a generically silent pane must NOT be classified as
// delivery failures. Over-firing this classifier would convert every ordinary
// slow phase into a bridge relaunch.
//
// Every test drives the REAL production entry point (runTmuxREPL /
// Engine.Launch) over a fake tmux. A helper called directly would prove
// nothing about reachability.

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// deliveryFailureReasonToken is the classified-cause vocabulary the timeout
// marker's reason= field must carry when the driver short-circuits on a
// verified delivery failure. It is the interaction.ResultSubmitWedged value,
// so the marker text, the ndjson ledger record, and any downstream classifier
// all agree on one word.
const deliveryFailureReasonToken = "submit_wedged"

// parkedPromptTmux keeps the pasted prompt visible at the `❯` input line
// forever: no number of bare Enters clears it. This is the cycle-1510 pane
// shape — the paste landed, the submit never took.
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

// TestTmuxREPL_CleanSubmit_NeverClassifiesDeliveryFailure is the anti-over-fire
// negative: the ordinary happy path (prompt submits, artifact lands) must exit
// ExitOK and must not emit an artifact-timeout marker or the submit_wedged
// token anywhere on stderr. If the short-circuit added for AC-001 fires on a
// verified-clean submission, every healthy phase becomes a bridge relaunch.
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

// TestTmuxREPL_SilentPaneTimeout_NotClassifiedAsDeliveryFailure is the second
// false-negative guard, and the one the scout's acceptance criteria name
// explicitly: a pane whose input line IS clear (the prompt submitted fine) but
// which then produces nothing must still burn the normal silence budget and
// die with the GENERIC reason. Only an evidenced submit-verification failure
// may claim the delivery-failure cause; generic silence and agent-typed text
// stay non-transient, exactly as today.
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

// TestTmuxREPL_NudgeSubmitWedged_ClassifiedCauseSurvivesIntoMarker covers the
// SECOND consumer site. The nudge fires from inside the stop-review pause
// branch, so it cannot skip a silence budget it has already spent — but its
// wedged outcome is the same evidence, and the terminal artifact-timeout
// marker must name it instead of reporting the generic stall reason. Without
// this, cycle-1510's ndjson (`"result":"no_effect"` on every nudge) stays the
// only place the cause exists.
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
	if !strings.Contains(summary, "nudge") {
		t.Errorf("delivery-failure cause does not name the submission SITE — an operator cannot tell an "+
			"undelivered prompt from an undelivered nudge\n  summary: %s", summary)
	}
}

// TestEngineLaunch_PromptSubmitWedged_PhaseErrorCarriesClassifiedCause is the
// evidence-contract wiring proof at the bridge boundary: the classified cause
// must ride the existing artifactTimeoutSummary path into the error Launch
// returns, wrapped in core.ErrArtifactTimeout so the dispatcher still sees a
// retryable infra teardown. A short-circuit that skipped the marker shape
// would silently drop the cause on the floor (engine.go discards driver stderr
// past the launch-error file).
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
