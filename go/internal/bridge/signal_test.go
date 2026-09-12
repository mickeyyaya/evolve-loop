package bridge

// signal_test.go — ADR-0101 S3: the bridge engine receives the Signal Center
// at construction (Deps.Signals, proven by SignalsWired), its telemetry
// warnings and tripwires are bridge.warning / bridge.tripwire events with the
// call identity in fields, and the tmux-pane liveness edges the LivenessCenter
// dispatches become pane.liveness events. Hand-written "[engine] WARN" lines
// for those facts are gone; the WARN-filtered stderr sink renders them.

import (
	"bytes"
	"context"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func recordingSignals() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

func TestNewEngine_SignalsWiredReportsTheInjectedCenter(t *testing.T) {
	t.Parallel()
	if NewEngine(Deps{Stderr: io.Discard}).SignalsWired() {
		t.Error("no Center → not wired (the nil Null Object is a test affordance; the production root always injects one)")
	}
	c, _ := recordingSignals()
	if !NewEngine(Deps{Signals: c, Stderr: io.Discard}).SignalsWired() {
		t.Error("an injected Center must be reported wired")
	}
}

func TestNewEngine_MissingTokenResolverIsABridgeWarningNotAStderrLine(t *testing.T) {
	t.Parallel()
	c, got := recordingSignals()
	var stderr bytes.Buffer
	NewEngine(Deps{Signals: c, Stderr: &stderr})
	if strings.Contains(stderr.String(), "TokenResolver") {
		t.Errorf("the hand-written [engine] WARN line is gone; the sink renders the event: %q", stderr.String())
	}
	if len(*got) != 1 {
		t.Fatalf("one bridge.warning for a missing resolver, got %d: %+v", len(*got), *got)
	}
	e := (*got)[0]
	if e.Module != signalcenter.ModuleBridge || e.Kind != signalcenter.KindBridgeWarning || e.Severity != signalcenter.SeverityWarn ||
		e.Code != CodeTokenResolverMissing || e.Origin != "NewEngine" || e.Cycle != 0 || !strings.Contains(e.Reason, "token usage") {
		t.Errorf("a construction-time warning: module bridge, no cycle yet, its own code: %+v", e)
	}
	*got = nil
	NewEngine(Deps{Signals: c, Stderr: &stderr, TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tokenusage.Result{}, nil }})
	if len(*got) != 0 {
		t.Errorf("a wired resolver warns nothing: %+v", *got)
	}
}

func TestAttemptWarn_IsABridgeWarningWithTheCallIdentity(t *testing.T) {
	t.Parallel()
	c, got := recordingSignals()
	ctx := attemptLogContext{signals: c, dispatchIdentity: dispatchIdentity{cycle: 1640, runID: "run-1640", phase: "build"}, callID: "call-1", cli: "claude-tmux", agent: "build", attempt: 2}
	ctx.warn(CodeTokenUsageWarning, "token resolver returned an invalid context-fill value")
	if len(*got) != 1 {
		t.Fatalf("one bridge.warning per warn, got %d", len(*got))
	}
	e := (*got)[0]
	if e.Module != signalcenter.ModuleBridge || e.Kind != signalcenter.KindBridgeWarning || e.Severity != signalcenter.SeverityWarn || e.Code != CodeTokenUsageWarning ||
		e.Cycle != 1640 || e.RunID != "run-1640" || e.Phase != "build" || e.Attempt != 2 || e.Origin != "attemptLogContext.warn" ||
		e.Reason != "token resolver returned an invalid context-fill value" ||
		e.Fields["call_id"] != "call-1" || e.Fields["cli"] != "claude-tmux" || e.Fields["agent"] != "build" {
		t.Errorf("the event carries the call identity the old line carried, plus cycle/run/phase/attempt: %+v", e)
	}
}

func TestRecordModelAttempt_ATripwireIsABridgeTripwire(t *testing.T) {
	t.Parallel()
	c, got := recordingSignals()
	e := NewEngine(Deps{Signals: c, Stderr: io.Discard})
	req := core.BridgeRequest{CLI: "codex", Agent: "build", Cycle: 7, RunID: "run-7", Workspace: t.TempDir()}
	end := time.Now()
	start := end.Add(-tripwireSuccessThreshold - time.Minute)
	e.recordModelAttempt(req, "auto", ExitOK, start, end, modelDispatch{}, "", &core.BridgeResponse{})
	var trip []signalcenter.Event
	for _, ev := range *got {
		if ev.Kind == signalcenter.KindBridgeTripwire {
			trip = append(trip, ev)
		}
	}
	if len(trip) != 1 {
		t.Fatalf("a successful-but-silent attempt beyond the threshold is exactly one bridge.tripwire, got %d: %+v", len(trip), *got)
	}
	tw := trip[0]
	if tw.Severity != signalcenter.SeverityWarn || tw.Code != CodeTelemetryTripwire || tw.Cycle != 7 || tw.RunID != "run-7" || tw.Phase != "build" ||
		tw.Origin != "attemptLogContext.tripwire" || tw.Fields["cli"] != "codex" || tw.Fields["call_id"] == "" || tw.Fields["duration_ms"] == "" {
		t.Errorf("the tripwire names the call and its duration: %+v", tw)
	}
}

func TestPaneLivenessHandler_ProjectsStateEdgesToPaneLiveness(t *testing.T) {
	t.Parallel()
	c, got := recordingSignals()
	h := paneLivenessHandler(c, dispatchIdentity{cycle: 1640, runID: "run-1640", phase: "build"})
	for _, tc := range []struct {
		state panestream.LivenessState
		sev   signalcenter.Severity
		code  signalcenter.Code
	}{
		{panestream.LivenessIdle, signalcenter.SeverityInfo, ""},
		{panestream.LivenessConverging, signalcenter.SeverityInfo, ""},
		{panestream.LivenessBusyButStagnant, signalcenter.SeverityWarn, CodePaneStagnant},
		{panestream.LivenessHung, signalcenter.SeverityWarn, CodePaneHung},
		{panestream.LivenessExhausted, signalcenter.SeverityWarn, CodePaneExhausted},
	} {
		*got = nil
		h(panestream.LivenessEvent{SessionKey: "evolve-r1640-build", State: tc.state})
		if len(*got) != 1 {
			t.Fatalf("one pane.liveness per edge (%s), got %d", tc.state, len(*got))
		}
		e := (*got)[0]
		if e.Module != signalcenter.ModuleLiveness || e.Kind != signalcenter.KindPaneLiveness || e.Severity != tc.sev || e.Code != tc.code ||
			e.Cycle != 1640 || e.RunID != "run-1640" || e.Phase != "build" || e.Origin != "paneLivenessHandler" ||
			e.Fields["session"] != "evolve-r1640-build" || e.Fields["state"] != tc.state.String() {
			t.Errorf("state %s: %+v", tc.state, e)
		}
	}
}

// The driver registers the handler on the LivenessCenter it uses for this
// dispatch, so a real liveness edge reaches the Center with the dispatch's
// identity — the wiring proof for pane.liveness.
func TestNewReplWaitState_RegistersThePaneLivenessHandler(t *testing.T) {
	c, got := recordingSignals()
	lc := panestream.NewLivenessCenter()
	workspace := t.TempDir()
	deps := Deps{Signals: c, LivenessCenter: lc, Stderr: io.Discard}.withDefaults()
	profile := panestream.Profiles["claude"]
	waiter := replWaiter{
		ctx: context.Background(),
		cfg: &Config{Agent: "build", Cycle: 7, RunID: "run-7", ArtifactTimeoutS: 2, Workspace: workspace, ProjectRoot: workspace,
			Artifact: filepath.Join(workspace, "artifact.md")},
		deps:      deps,
		launch:    tmuxLaunch{name: "claude-tmux", session: "liveness-unit"},
		prefix:    "[liveness-unit]",
		phaseName: "build",
		responder: &autoResponder{injectedPrompt: "write the artifact"},
		recorder:  interaction.NewRecorder(workspace),
		channel:   &replLiveChannel{profile: profile},
	}
	newReplWaitState(waiter)
	lc.Observe("liveness-unit", tmuxPromptMarkerDefault+"\n⏺ working", profile)
	var edges []signalcenter.Event
	for _, e := range *got {
		if e.Kind == signalcenter.KindPaneLiveness {
			edges = append(edges, e)
		}
	}
	if len(edges) != 1 {
		t.Fatalf("the first observation is an edge and reaches the Center exactly once, got %d: %+v", len(edges), *got)
	}
	if e := edges[0]; e.Cycle != 7 || e.RunID != "run-7" || e.Phase != "build" || e.Fields["session"] != "liveness-unit" || e.Fields["state"] == "" {
		t.Errorf("the edge carries the dispatch identity: %+v", e)
	}
}

// sinkDeps builds the Center a test hands to Deps.Signals with the root's
// WARN-filtered stderr sink rendering into buf — so a test asserts exactly
// what the operator reads for the engine's signals (the one line format),
// never a hand-written line.
func sinkDeps(w io.Writer) *signalcenter.Center {
	c := signalcenter.New()
	c.Subscribe(signalcenter.Filter(signalcenter.StderrSink(w), signalcenter.SeverityWarn))
	return c
}

// The per-attempt ledger append failing is itself a signal (the record the
// operator would read is missing), named by its code and the path.
func TestRecordModelAttempt_LedgerAppendFailureIsABridgeWarning(t *testing.T) {
	t.Parallel()
	c, got := recordingSignals()
	e := NewEngine(Deps{Signals: c, Stderr: io.Discard, TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tokenusage.Result{}, nil }})
	notADir := filepath.Join(t.TempDir(), "workspace-is-a-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := core.BridgeRequest{CLI: "claude-tmux", Agent: "build", Cycle: 9, Workspace: notADir}
	e.recordModelAttempt(req, "auto", ExitOK, time.Now().Add(-time.Second), time.Now(), modelDispatch{}, "", &core.BridgeResponse{})
	var appendFailed []signalcenter.Event
	for _, ev := range *got {
		if ev.Code == CodeTelemetryAppendFailed {
			appendFailed = append(appendFailed, ev)
		}
	}
	if len(appendFailed) != 1 || appendFailed[0].Kind != signalcenter.KindBridgeWarning || appendFailed[0].Cycle != 9 || !strings.Contains(appendFailed[0].Reason, "path=") {
		t.Errorf("one bridge.warning names the failed append and its path: %+v", *got)
	}
}

// One identity rule for every bridge signal of a dispatch: the request's
// cycle/run and its agent role as the phase; the driver's Config carries the
// same values. No fallback — a request without a Cycle stays at cycle 0.
func TestDispatchIdentity_OneRuleFromTheRequestAndTheConfig(t *testing.T) {
	t.Parallel()
	req := core.BridgeRequest{Cycle: 1005, RunID: "run-1005", Agent: "builder", Workspace: "/x/.evolve/runs/cycle-1005"}
	if got := requestIdentity(req); got != (dispatchIdentity{cycle: 1005, runID: "run-1005", phase: "builder"}) {
		t.Errorf("requestIdentity: %+v", got)
	}
	if got := configIdentity(&Config{Cycle: 1005, RunID: "run-1005", Agent: "builder"}); got != requestIdentity(req) {
		t.Errorf("the Config-derived identity must equal the request-derived one: %+v", got)
	}
	if got := requestIdentity(core.BridgeRequest{Workspace: "/x/.evolve/runs/cycle-1005"}); got.cycle != 0 {
		t.Errorf("no workspace-path fallback: a request without a Cycle stays at cycle 0, got %+v", got)
	}
}
