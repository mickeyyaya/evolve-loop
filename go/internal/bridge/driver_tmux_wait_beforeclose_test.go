package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const floorFindingsFixture = "The build handoff floor found 1 finding(s).\n  - cover-changed: x.go:Foo is at 80.0%"

type sessionAgentTmux struct {
	*FakeTmuxController
	artifact    string
	writes      int
	pasteErrAt  int
	pasted      []string
	lastLoaded  string
	pasteCalls  int
	rewritesOff bool
}

func (s *sessionAgentTmux) LoadBuffer(ctx context.Context, session, file string) error {
	body, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	s.lastLoaded = string(body)
	return s.FakeTmuxController.LoadBuffer(ctx, session, file)
}

func (s *sessionAgentTmux) PasteBuffer(ctx context.Context, session string) error {
	s.pasteCalls++
	if s.pasteCalls == s.pasteErrAt {
		return errors.New("paste refused")
	}
	if err := s.FakeTmuxController.PasteBuffer(ctx, session); err != nil {
		return err
	}
	s.pasted = append(s.pasted, s.lastLoaded)
	if s.writes == 0 {
		return s.write("done\n")
	}
	if s.rewritesOff {
		return nil
	}
	return s.write("done after the findings\n")
}

func (s *sessionAgentTmux) write(body string) error {
	s.writes++
	return os.WriteFile(s.artifact, []byte(body), 0o644)
}

func idleFrames(n int) []string {
	frames := make([]string, n)
	for i := range frames {
		frames[i] = "❯"
	}
	return frames
}

type closeCheckProbe struct {
	calls    atomic.Int32
	reports  []error
	findings string
	block    chan struct{}
	onReport func()
}

func (p *closeCheckProbe) check(context.Context) (string, func(error)) {
	p.calls.Add(1)
	if p.block != nil {
		<-p.block
	}
	if p.findings == "" {
		return "", nil
	}
	return p.findings, func(err error) {
		p.reports = append(p.reports, err)
		if p.onReport != nil {
			p.onReport()
		}
	}
}

func findingsProbe() *closeCheckProbe { return &closeCheckProbe{findings: floorFindingsFixture} }

func runBeforeCloseREPL(t *testing.T, ctx context.Context, tm *sessionAgentTmux, probe *closeCheckProbe) (int, string) {
	t.Helper()
	cfg := fixtureConfig(t)
	tm.artifact = cfg.Artifact
	var stderr bytes.Buffer
	deps := fixtureDeps(tm)
	deps.Stderr = &stderr
	if probe != nil {
		deps.beforeClose = probe.check
	}
	code, err := runTmuxREPL(ctx, cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "fixture-before-close", launchCmd: "claude", promptMarker: "❯", bootIntervalS: 1,
	})
	if err != nil {
		t.Fatalf("runTmuxREPL error: %v", err)
	}
	return code, stderr.String()
}

func TestRunTmuxREPL_TheFloorFindingsEnterTheLiveSessionOnceBeforeItCloses(t *testing.T) {
	t.Parallel()
	tm := &sessionAgentTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: idleFrames(60)}}
	probe := findingsProbe()

	code, stderr := runBeforeCloseREPL(t, context.Background(), tm, probe)

	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%s", code, stderr)
	}
	if probe.calls.Load() != 1 {
		t.Fatalf("the check ran %d times, want once per session", probe.calls.Load())
	}
	if len(tm.pasted) != 2 || tm.pasted[1] != floorFindingsFixture {
		t.Fatalf("pastes = %q; want the prompt, then exactly the floor findings", tm.pasted)
	}
	if len(probe.reports) != 1 || probe.reports[0] != nil {
		t.Fatalf("reports = %v; want one report of a delivered paste", probe.reports)
	}
	if tm.writes != 2 || strings.Contains(stderr, "did not write") || !strings.Contains(stderr, "returned the handoff floor findings") {
		t.Errorf("writes %d; the session must complete on the write that follows the findings:\n%s", tm.writes, stderr)
	}
}

func TestRunTmuxREPL_NoFindingsClosesTheSessionAsToday(t *testing.T) {
	t.Parallel()
	tm := &sessionAgentTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: idleFrames(60)}}
	probe := &closeCheckProbe{}

	code, _ := runBeforeCloseREPL(t, context.Background(), tm, probe)

	if code != ExitOK || probe.calls.Load() != 1 || len(tm.pasted) != 1 {
		t.Fatalf("exit %d, check calls %d, pastes %d; want ExitOK, one check, only the prompt paste", code, probe.calls.Load(), len(tm.pasted))
	}
}

func TestRunTmuxREPL_WithNoCheckTheSessionClosesAsToday(t *testing.T) {
	t.Parallel()
	tm := &sessionAgentTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: idleFrames(60)}}

	code, _ := runBeforeCloseREPL(t, context.Background(), tm, nil)

	if code != ExitOK || len(tm.pasted) != 1 {
		t.Fatalf("exit %d, pastes %d; want ExitOK and only the prompt paste", code, len(tm.pasted))
	}
}

func TestRunTmuxREPL_AFailedDeliveryIsReportedAndClosesTheSessionAsToday(t *testing.T) {
	t.Parallel()
	tm := &sessionAgentTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: idleFrames(60)}, pasteErrAt: 2}
	probe := findingsProbe()

	code, stderr := runBeforeCloseREPL(t, context.Background(), tm, probe)

	if code != ExitOK || probe.calls.Load() != 1 || tm.writes != 1 || !strings.Contains(stderr, "did not enter the live session") {
		t.Fatalf("exit %d, check calls %d, writes %d; a delivery that cannot enter the session must complete with the first deliverable:\n%s", code, probe.calls.Load(), tm.writes, stderr)
	}
	if len(probe.reports) != 1 || probe.reports[0] == nil || !strings.Contains(probe.reports[0].Error(), "paste refused") {
		t.Fatalf("reports = %v; want one report that carries the paste error", probe.reports)
	}
}

func TestRunTmuxREPL_AFloorPastTheWaitDeadlineDeliversNothing(t *testing.T) {
	t.Parallel()
	tm := &sessionAgentTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: idleFrames(60)}}
	probe := findingsProbe()
	probe.block = make(chan struct{})
	t.Cleanup(func() { close(probe.block) })

	code, stderr := runBeforeCloseREPL(t, context.Background(), tm, probe)

	if code != ExitOK || len(tm.pasted) != 1 || len(probe.reports) != 0 {
		t.Fatalf("exit %d, pastes %d, reports %d; a floor past its deadline must deliver nothing and close as today", code, len(tm.pasted), len(probe.reports))
	}
	if !strings.Contains(stderr, "WARN: the handoff floor before close passed its deadline of 1s") {
		t.Errorf("stderr must WARN with the deadline and the elapsed time:\n%s", stderr)
	}
}

func TestRunTmuxREPL_ASessionThatNeverRewritesAfterTheFindingsKeepsItsFirstDeliverable(t *testing.T) {
	t.Parallel()
	tm := &sessionAgentTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: idleFrames(200)}, rewritesOff: true}
	probe := findingsProbe()

	code, stderr := runBeforeCloseREPL(t, context.Background(), tm, probe)

	if code != ExitOK || probe.calls.Load() != 1 || len(tm.pasted) != 2 || !strings.Contains(stderr, "the first deliverable stands") {
		t.Fatalf("exit %d, check calls %d, pastes %d; the returned findings must never turn a complete session into a timeout:\n%s", code, probe.calls.Load(), len(tm.pasted), stderr)
	}
	if !strings.Contains(strings.Join(tm.SentKeys, "\n"), tm.artifact) {
		t.Errorf("the idle session must get the nudge to write the deliverable again: sent %q", tm.SentKeys)
	}
}

func TestRunTmuxREPL_ACancelledWaitAfterTheFindingsIsNotASuccess(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tm := &sessionAgentTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: idleFrames(200)}, rewritesOff: true}
	probe := findingsProbe()
	probe.onReport = cancel

	code, stderr := runBeforeCloseREPL(t, ctx, tm, probe)

	if code == ExitOK || probe.calls.Load() != 1 || strings.Contains(stderr, "the first deliverable stands") {
		t.Fatalf("exit %d, check calls %d; a cancelled wait must not report the held deliverable as a success:\n%s", code, probe.calls.Load(), stderr)
	}
}

func TestRunAttempt_TheRequestCheckReachesTheSessionWait(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	tm := &sessionAgentTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: idleFrames(200)}, artifact: fx.artifact}
	eng := newTestEngine(Deps{Tmux: tm, Sleep: func(time.Duration) {}, ArtifactTimeoutS: 1, RecoveryStage: "off"})
	probe := &closeCheckProbe{}

	run := eng.runAttempt(context.Background(), core.BridgeRequest{BeforeClose: probe.check}, fx.args("claude-tmux", "--allow-bypass"))

	if run.code != ExitOK || probe.calls.Load() != 1 {
		t.Fatalf("exit %d, check calls %d; the check of the request must run once in the session wait:\n%s", run.code, probe.calls.Load(), run.stderr)
	}
}

func TestHeldDeliverableStands_OnlyAPlainWaitEndKeepsTheFirstDeliverable(t *testing.T) {
	t.Parallel()
	held := func(mutate func(*replWaitState)) bool {
		state := &replWaitState{heldAfterCompletion: true}
		mutate(state)
		return state.heldDeliverableStands()
	}
	if !held(func(*replWaitState) {}) {
		t.Error("a plain wait end after the findings keeps the first deliverable")
	}
	for name, mutate := range map[string]func(*replWaitState){
		"not held":     func(s *replWaitState) { s.heldAfterCompletion = false },
		"cancelled":    func(s *replWaitState) { s.cancellationErr = context.Canceled },
		"pane lost":    func(s *replWaitState) { s.paneLost = true },
		"transient":    func(s *replWaitState) { s.transientShortcircuit = true },
		"submit stuck": func(s *replWaitState) { s.submitWedged = true },
	} {
		if held(mutate) {
			t.Errorf("%s: the wait end must keep the closeout of today", name)
		}
	}
}
