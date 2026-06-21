package phasecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/phaseproto"
)

// envelopeStdin builds an envelope-framed request line ready to feed
// into runServePhase's stdin. Mirrors what SubprocessRunner writes.
func envelopeStdin(t *testing.T, req core.PhaseRequest) []byte {
	t.Helper()
	env, err := phaseproto.EncodeRequest("corr-test", req)
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return append(raw, '\n')
}

// decodeResponseEnvelope parses one envelope from stdout and returns
// it, failing the test on any framing error.
func decodeResponseEnvelope(t *testing.T, out []byte) phaseproto.Envelope {
	t.Helper()
	out = bytes.TrimRight(out, "\n")
	var env phaseproto.Envelope
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("unmarshal response envelope: %v (raw=%q)", err, string(out))
	}
	return env
}

func TestRunServePhase_HappyPath(t *testing.T) {
	stub := &stubPhase{resp: core.PhaseResponse{
		Phase:   "intent",
		Verdict: core.VerdictPASS,
	}}
	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	registry.Register("intent", func(req core.PhaseRequest) core.PhaseRunner { return stub })

	req := core.PhaseRequest{Cycle: 11, ProjectRoot: "/p", Workspace: "/w"}
	stdin := bytes.NewReader(envelopeStdin(t, req))
	var stdout, stderr bytes.Buffer

	code := RunServePhase([]string{"intent"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d want 0; stderr=%s", code, stderr.String())
	}
	if stub.got.Cycle != 11 {
		t.Errorf("stub got Cycle=%d, want 11", stub.got.Cycle)
	}

	env := decodeResponseEnvelope(t, stdout.Bytes())
	if env.Kind != phaseproto.KindResponse {
		t.Errorf("Kind=%q want response", env.Kind)
	}
	if env.CorrelationID != "corr-test" {
		t.Errorf("CorrelationID=%q want corr-test (handler must echo back)", env.CorrelationID)
	}
	resp, err := phaseproto.DecodeResponse(env)
	if err != nil {
		t.Fatalf("DecodeResponse: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q want PASS", resp.Verdict)
	}
}

func TestRunServePhase_MissingPhaseName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunServePhase(nil, bytes.NewReader(nil), &stdout, &stderr)
	if code != 10 {
		t.Errorf("code=%d want 10", code)
	}
	if !strings.Contains(stderr.String(), "missing phase name") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestRunServePhase_UnknownPhase(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunServePhase([]string{"nopephase"}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 10 {
		t.Errorf("code=%d want 10", code)
	}
	if !strings.Contains(stderr.String(), "unknown phase") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestRunServePhase_RunnerErrorEmitsErrorEnvelope(t *testing.T) {
	stub := &stubPhase{
		resp: core.PhaseResponse{Phase: "intent", Verdict: core.VerdictFAIL},
		err:  errors.New("intent boom"),
	}
	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	registry.Register("intent", func(req core.PhaseRequest) core.PhaseRunner { return stub })

	stdin := bytes.NewReader(envelopeStdin(t, core.PhaseRequest{Cycle: 1}))
	var stdout, stderr bytes.Buffer
	// Handler errors are wrapped into a WireError envelope by ServeStdio;
	// the process still exits 0 because the envelope IS the response —
	// surfaced as a transport error on the parent side. (CodeChildCrashed
	// is reserved for non-zero exit codes.)
	code := RunServePhase([]string{"intent"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Errorf("code=%d want 0 (handler errors are wire-level, not exit-level); stderr=%s", code, stderr.String())
	}
	env := decodeResponseEnvelope(t, stdout.Bytes())
	if env.Kind != phaseproto.KindError {
		t.Errorf("Kind=%q want error", env.Kind)
	}
	if env.Error == nil || env.Error.Code != phaseproto.CodeHandlerError {
		t.Errorf("want WireError CodeHandlerError, got %+v", env.Error)
	}
	if !strings.Contains(env.Error.Message, "intent boom") {
		t.Errorf("error message lost original cause: %q", env.Error.Message)
	}
}

func TestRunServePhase_MalformedEnvelopeExits1(t *testing.T) {
	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	registry.Register("intent", func(req core.PhaseRequest) core.PhaseRunner { return &stubPhase{} })

	var stdout, stderr bytes.Buffer
	code := RunServePhase([]string{"intent"}, strings.NewReader("not-an-envelope\n"), &stdout, &stderr)
	if code != 1 {
		t.Errorf("code=%d want 1", code)
	}
	if !strings.Contains(stderr.String(), "serve-phase") {
		t.Errorf("stderr should identify subcommand: %q", stderr.String())
	}
}

// Exercises RunServePhase end-to-end: an envelope-framed request round-trips to
// a response envelope. (Dispatcher routing for "serve-phase" → RunServePhase is
// covered separately in cmd/evolve/dispatch_test.go.)
func TestRunServePhase_EnvelopeRoundTrip(t *testing.T) {
	stub := &stubPhase{resp: core.PhaseResponse{Phase: "scout", Verdict: core.VerdictPASS}}
	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	registry.Register("scout", func(req core.PhaseRequest) core.PhaseRunner { return stub })

	stdin := bytes.NewReader(envelopeStdin(t, core.PhaseRequest{Cycle: 2}))
	var stdout, stderr bytes.Buffer
	code := RunServePhase([]string{"scout"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dispatch serve-phase exit=%d stderr=%s", code, stderr.String())
	}
	env := decodeResponseEnvelope(t, stdout.Bytes())
	if env.Kind != phaseproto.KindResponse {
		t.Errorf("Kind=%q want response", env.Kind)
	}
}

// Defensive: the underlying ServeStdio uses context.Background(), so we
// only need to ensure the context is plumbed through. Smoke check by
// ensuring the stub sees a non-nil context.
type ctxCapturingPhase struct {
	stubPhase
	ctx context.Context
}

func (s *ctxCapturingPhase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	s.ctx = ctx
	return s.stubPhase.Run(ctx, req)
}

func TestRunServePhase_PlumbsContext(t *testing.T) {
	stub := &ctxCapturingPhase{stubPhase: stubPhase{resp: core.PhaseResponse{Phase: "intent", Verdict: core.VerdictPASS}}}
	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	registry.Register("intent", func(req core.PhaseRequest) core.PhaseRunner { return stub })

	stdin := bytes.NewReader(envelopeStdin(t, core.PhaseRequest{Cycle: 1}))
	var stdout, stderr bytes.Buffer
	_ = RunServePhase([]string{"intent"}, stdin, &stdout, &stderr)
	if stub.ctx == nil {
		t.Error("handler invoked with nil ctx")
	}
}
