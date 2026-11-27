package bridge

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

const heldBusyPane = "✻ Working (3s · esc to interrupt)\n❯"

type heldIdleTmux struct {
	*sessionAgentTmux
	captures      int
	captureLimit  int
	cancel        context.CancelFunc
	busyAfterHeld bool
	wedgeFindings bool
	errOnce       bool
}

func (h *heldIdleTmux) CapturePane(context.Context, string, int) (string, error) {
	h.captures++
	if h.captures > h.captureLimit {
		h.cancel()
	}
	if h.pasteCalls >= 2 && h.errOnce {
		h.errOnce = false
		return "", errors.New("capture refused")
	}
	if h.pasteCalls >= 2 && h.wedgeFindings {
		return "❯ " + floorFindingsFixture, nil
	}
	if h.pasteCalls >= 2 && h.busyAfterHeld {
		return heldBusyPane, nil
	}
	return "❯", nil
}

func (h *heldIdleTmux) nudgesAfterHeld() int {
	n := 0
	for _, k := range h.SentKeys {
		if strings.Contains(k, h.artifact) {
			n++
		}
	}
	return n
}

type heldRun struct {
	code   int
	stderr string
	ws     string
}

func runHeldIdle(t *testing.T, h *heldIdleTmux, probe *closeCheckProbe) heldRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.cancel = cancel
	h.captureLimit = 400
	cfg := fixtureConfig(t)
	cfg.ArtifactTimeoutS = 3600
	h.artifact = cfg.Artifact
	var stderr bytes.Buffer
	deps := fixtureDeps(h)
	deps.Stderr = &stderr
	deps.IdleCheckpointGraceS = 10
	deps.beforeClose = probe.check
	code, err := runTmuxREPL(ctx, cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "fixture-held-idle", launchCmd: "claude", promptMarker: "❯", inputLineMarker: "❯", bootIntervalS: 1,
	})
	if err != nil {
		t.Fatalf("runTmuxREPL error: %v", err)
	}
	return heldRun{code: code, stderr: stderr.String(), ws: cfg.Workspace}
}

func newHeldIdleTmux() *heldIdleTmux {
	return &heldIdleTmux{sessionAgentTmux: &sessionAgentTmux{FakeTmuxController: &FakeTmuxController{}, deferFirstWrite: true, rewritesOff: true}}
}

func TestRunTmuxREPL_AHeldSessionAfterAnEarlierNudgeClosesWithinTwoIdleGraces(t *testing.T) {
	t.Parallel()
	h := newHeldIdleTmux()
	probe := findingsProbe()

	run := runHeldIdle(t, h, probe)

	if run.code != ExitOK || !strings.Contains(run.stderr, "the first deliverable stands") {
		t.Fatalf("exit %d; the held session must close with the first deliverable:\n%s", run.code, run.stderr)
	}
	if got := h.nudgesAfterHeld(); got != 2 {
		t.Errorf("nudges = %d, want one before the write and one after the findings", got)
	}
	if h.captures >= h.captureLimit {
		t.Errorf("captures = %d; the held session waited for the full interval instead of one idle grace", h.captures)
	}
}

func TestRunTmuxREPL_ABusyHeldSessionKeepsWaiting(t *testing.T) {
	t.Parallel()
	h := newHeldIdleTmux()
	h.busyAfterHeld = true
	probe := findingsProbe()

	run := runHeldIdle(t, h, probe)

	if run.code == ExitOK || strings.Contains(run.stderr, "the first deliverable stands") {
		t.Fatalf("exit %d; a busy held session must not close early:\n%s", run.code, run.stderr)
	}
	if got := h.nudgesAfterHeld(); got != 1 {
		t.Errorf("nudges = %d, want only the nudge before the write", got)
	}
}

func TestRunTmuxREPL_TheFindingsSubmissionIsVerifiedAndRecorded(t *testing.T) {
	t.Parallel()
	h := newHeldIdleTmux()
	probe := findingsProbe()

	run := runHeldIdle(t, h, probe)

	var sites []string
	for _, o := range readInteractions(t, run.ws, "build") {
		if o.Event.Kind == interaction.KindSubmitVerify && strings.Contains(o.Event.Payload, "site=findings") {
			sites = append(sites, o.Result)
		}
	}
	if len(sites) != 1 || sites[0] != interaction.ResultSubmitVerified {
		t.Fatalf("findings submit_verify results = %v, want one %s:\n%s", sites, interaction.ResultSubmitVerified, run.stderr)
	}
}

func TestRunTmuxREPL_AWedgedFindingsSubmissionIsADeliveryErrorAndDoesNotHoldTheSession(t *testing.T) {
	t.Parallel()
	h := newHeldIdleTmux()
	h.wedgeFindings = true
	probe := findingsProbe()

	run := runHeldIdle(t, h, probe)

	if len(probe.reports) != 1 || probe.reports[0] == nil || !errors.Is(probe.reports[0], errFindingsSubmitWedged) {
		t.Fatalf("reports = %v; want one delivery error that names the wedged submission", probe.reports)
	}
	if strings.Contains(run.stderr, "the first deliverable stands") || strings.Contains(run.stderr, "closes after a new write") {
		t.Errorf("a wedged findings submission must not hold the session:\n%s", run.stderr)
	}
}

func TestIdleCheckpointDue_ANudgedSessionIsDueAgainOnlyWhenHeld(t *testing.T) {
	t.Parallel()
	state := &replWaitState{idleGraceS: 10, nudgeSent: true}
	if state.idleCheckpointDue(20) {
		t.Error("a nudged session that is not held must not get a second idle checkpoint")
	}
	state.heldAfterCompletion = true
	if !state.idleCheckpointDue(20) || state.idleCheckpointDue(5) {
		t.Error("a nudged held session is due after one idle grace and not before")
	}
}

func TestRestartIdleBudget_RecordsThePendingNudgeThenClearsTheNudgeState(t *testing.T) {
	t.Parallel()
	recorder := interaction.NewRecorder(t.TempDir())
	state := &replWaitState{nudgeSent: true, resendWindowS: 4, nudgeEvent: &interaction.Event{Kind: interaction.KindNudge}}

	state.restartIdleBudget(recorder, func() time.Time { return state.nudgeAt })

	out := recorder.Outcomes()
	if len(out) != 1 || out[0].Result != interaction.ResultArtifactAppeared {
		t.Fatalf("outcomes = %v, want one %s", out, interaction.ResultArtifactAppeared)
	}
	if state.nudgeSent || state.nudgeEvent != nil || state.resendWindowS != 0 || state.completed {
		t.Errorf("nudge state not cleared: %+v", state)
	}
}

func TestRunTmuxREPL_AFindingsSubmissionWithoutAPaneCaptureIsNotVerifiedButDelivered(t *testing.T) {
	t.Parallel()
	h := newHeldIdleTmux()
	h.errOnce = true
	probe := findingsProbe()

	run := runHeldIdle(t, h, probe)

	if len(probe.reports) != 1 || probe.reports[0] != nil || !strings.Contains(run.stderr, "findings NOT verified") {
		t.Fatalf("reports = %v; want one delivered report and a NOT verified note:\n%s", probe.reports, run.stderr)
	}
}
