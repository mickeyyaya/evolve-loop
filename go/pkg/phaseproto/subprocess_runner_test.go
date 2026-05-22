package phaseproto

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// SubprocessRunner must satisfy core.PhaseRunner — compile-time assert.
var _ core.PhaseRunner = (*SubprocessRunner)(nil)

// TestHelperProcess is the canonical Go re-exec trick. When the parent
// launches `go test -run=TestHelperProcess` with GO_WANT_HELPER_PROCESS=1,
// this function plays the role of "the subprocess" the SubprocessRunner
// is talking to. Behaviour is steered by HELPER_MODE.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)
	mode := os.Getenv("HELPER_MODE")

	switch mode {
	case "echo_pass":
		err := ServeStdio(os.Stdin, os.Stdout, func(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
			return core.PhaseResponse{
				Phase:        "test",
				Verdict:      core.VerdictPASS,
				ArtifactsDir: req.Workspace,
				CostUSD:      0.01,
				DurationMS:   1,
			}, nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "ServeStdio: %v\n", err)
			os.Exit(2)
		}

	case "return_error":
		err := ServeStdio(os.Stdin, os.Stdout, func(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
			return core.PhaseResponse{}, &WireError{
				Code:      "BUDGET_EXCEEDED",
				Message:   "phase blew the cap",
				Retryable: false,
			}
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "ServeStdio: %v\n", err)
			os.Exit(2)
		}

	case "plain_error":
		// Handler returns a non-WireError; ServeStdio must convert it.
		err := ServeStdio(os.Stdin, os.Stdout, func(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
			return core.PhaseResponse{}, errors.New("something bad")
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "ServeStdio: %v\n", err)
			os.Exit(2)
		}

	case "garbage_stdout":
		// Print non-envelope JSON; parent's DecodeResponse must reject.
		fmt.Fprintln(os.Stdout, `{"this":"is","not":"an envelope"}`)

	case "crash":
		os.Exit(7)

	case "hang":
		// Block forever; parent must use ctx timeout.
		time.Sleep(60 * time.Second)

	default:
		fmt.Fprintf(os.Stderr, "unknown HELPER_MODE=%q\n", mode)
		os.Exit(1)
	}
}

// helperCmd builds the re-exec command for a given HELPER_MODE.
func helperCmd(mode string) (string, []string, []string) {
	bin := os.Args[0]
	args := []string{"-test.run=TestHelperProcess"}
	env := []string{
		"GO_WANT_HELPER_PROCESS=1",
		"HELPER_MODE=" + mode,
	}
	return bin, args, env
}

func TestSubprocessRunner_HappyPath(t *testing.T) {
	bin, args, env := helperCmd("echo_pass")
	r := NewSubprocessRunner("test-phase", bin, args, env)
	if r.Name() != "test-phase" {
		t.Errorf("Name()=%q, want test-phase", r.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := core.PhaseRequest{
		Cycle:     1,
		Workspace: "/tmp/ws",
		GoalHash:  "deadbeef",
		Budget:    core.BudgetEnvelope{MaxUSD: 2.0, BatchCapUSD: 20.0},
	}
	resp, err := r.Run(ctx, req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.ArtifactsDir != "/tmp/ws" {
		t.Errorf("ArtifactsDir=%q, want /tmp/ws (echoed)", resp.ArtifactsDir)
	}
}

func TestSubprocessRunner_HandlerReturnsWireError(t *testing.T) {
	bin, args, env := helperCmd("return_error")
	r := NewSubprocessRunner("test-phase", bin, args, env)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.Run(ctx, core.PhaseRequest{Cycle: 1, Budget: core.BudgetEnvelope{MaxUSD: 1.0}})
	if err == nil {
		t.Fatal("Run must return error from WireError envelope")
	}
	var werr *WireError
	if !errors.As(err, &werr) {
		t.Fatalf("err type=%T, want *WireError; err=%v", err, err)
	}
	if werr.Code != "BUDGET_EXCEEDED" {
		t.Errorf("Code=%q, want BUDGET_EXCEEDED", werr.Code)
	}
}

func TestSubprocessRunner_PlainHandlerErrorWrapped(t *testing.T) {
	bin, args, env := helperCmd("plain_error")
	r := NewSubprocessRunner("test-phase", bin, args, env)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.Run(ctx, core.PhaseRequest{Cycle: 1, Budget: core.BudgetEnvelope{MaxUSD: 1.0}})
	if err == nil {
		t.Fatal("Run must return error")
	}
	var werr *WireError
	if !errors.As(err, &werr) {
		t.Fatalf("plain handler error must be wrapped in WireError; got %T %v", err, err)
	}
	if werr.Code != CodeHandlerError {
		t.Errorf("Code=%q, want %q", werr.Code, CodeHandlerError)
	}
}

func TestSubprocessRunner_GarbageStdout(t *testing.T) {
	bin, args, env := helperCmd("garbage_stdout")
	r := NewSubprocessRunner("test-phase", bin, args, env)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.Run(ctx, core.PhaseRequest{Cycle: 1, Budget: core.BudgetEnvelope{MaxUSD: 1.0}})
	if err == nil {
		t.Fatal("Run must reject garbage stdout")
	}
}

func TestSubprocessRunner_ChildCrash(t *testing.T) {
	bin, args, env := helperCmd("crash")
	r := NewSubprocessRunner("test-phase", bin, args, env)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.Run(ctx, core.PhaseRequest{Cycle: 1, Budget: core.BudgetEnvelope{MaxUSD: 1.0}})
	if err == nil {
		t.Fatal("Run must propagate child crash")
	}
}

func TestSubprocessRunner_ContextCancellation(t *testing.T) {
	bin, args, env := helperCmd("hang")
	r := NewSubprocessRunner("test-phase", bin, args, env)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := r.Run(ctx, core.PhaseRequest{Cycle: 1, Budget: core.BudgetEnvelope{MaxUSD: 1.0}})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Run must error on context cancel")
	}
	// Allow generous slack for CI; we just need to confirm we didn't wait 60s.
	if elapsed > 5*time.Second {
		t.Errorf("Run did not honour context cancel; took %v", elapsed)
	}
}

// ServeStdio direct tests — exercised in-process so coverage covers the
// reader/writer/handler paths without the subprocess re-exec.

func TestServeStdio_HappyPath(t *testing.T) {
	req := core.PhaseRequest{Cycle: 7, Workspace: "/x", Budget: core.BudgetEnvelope{MaxUSD: 1.0}}
	env, _ := EncodeRequest("corr-1", req)
	raw, _ := json.Marshal(env)

	in := bytes.NewReader(append(raw, '\n'))
	out := &bytes.Buffer{}
	err := ServeStdio(in, out, func(ctx context.Context, r core.PhaseRequest) (core.PhaseResponse, error) {
		return core.PhaseResponse{Phase: "p", Verdict: core.VerdictPASS, ArtifactsDir: r.Workspace}, nil
	})
	if err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}
	var got Envelope
	if err := json.Unmarshal(bytes.TrimRight(out.Bytes(), "\n"), &got); err != nil {
		t.Fatalf("output not a single JSON envelope: %v\nraw=%q", err, out.String())
	}
	if got.Kind != KindResponse || got.CorrelationID != "corr-1" {
		t.Errorf("response envelope wrong: %+v", got)
	}
	resp, err := DecodeResponse(got)
	if err != nil {
		t.Fatalf("DecodeResponse: %v", err)
	}
	if resp.ArtifactsDir != "/x" || resp.Verdict != core.VerdictPASS {
		t.Errorf("response payload wrong: %+v", resp)
	}
}

func TestServeStdio_HandlerWireError(t *testing.T) {
	env, _ := EncodeRequest("c", core.PhaseRequest{Cycle: 1})
	raw, _ := json.Marshal(env)

	in := bytes.NewReader(append(raw, '\n'))
	out := &bytes.Buffer{}
	err := ServeStdio(in, out, func(ctx context.Context, r core.PhaseRequest) (core.PhaseResponse, error) {
		return core.PhaseResponse{}, &WireError{Code: "BOOM", Message: "x"}
	})
	if err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}
	var got Envelope
	json.Unmarshal(bytes.TrimRight(out.Bytes(), "\n"), &got)
	if got.Kind != KindError {
		t.Errorf("Kind=%q, want %q", got.Kind, KindError)
	}
	if got.Error == nil || got.Error.Code != "BOOM" {
		t.Errorf("Error envelope wrong: %+v", got.Error)
	}
}

func TestServeStdio_HandlerPlainError(t *testing.T) {
	env, _ := EncodeRequest("c", core.PhaseRequest{Cycle: 1})
	raw, _ := json.Marshal(env)

	in := bytes.NewReader(append(raw, '\n'))
	out := &bytes.Buffer{}
	_ = ServeStdio(in, out, func(ctx context.Context, r core.PhaseRequest) (core.PhaseResponse, error) {
		return core.PhaseResponse{}, errors.New("ugh")
	})
	var got Envelope
	json.Unmarshal(bytes.TrimRight(out.Bytes(), "\n"), &got)
	if got.Error == nil || got.Error.Code != CodeHandlerError {
		t.Errorf("plain error not wrapped as %s: %+v", CodeHandlerError, got.Error)
	}
}

func TestServeStdio_RejectsMalformedInput(t *testing.T) {
	in := strings.NewReader("not-json\n")
	out := &bytes.Buffer{}
	err := ServeStdio(in, out, func(ctx context.Context, r core.PhaseRequest) (core.PhaseResponse, error) {
		t.Fatal("handler must not be called")
		return core.PhaseResponse{}, nil
	})
	if err == nil {
		t.Error("ServeStdio must reject malformed input")
	}
}

func TestServeStdio_RejectsWrongKind(t *testing.T) {
	// A response envelope sent to a server should be rejected.
	env, _ := EncodeResponse("c", core.PhaseResponse{Phase: "p", Verdict: core.VerdictPASS})
	raw, _ := json.Marshal(env)
	in := bytes.NewReader(append(raw, '\n'))
	out := &bytes.Buffer{}
	err := ServeStdio(in, out, func(ctx context.Context, r core.PhaseRequest) (core.PhaseResponse, error) {
		t.Fatal("handler must not be called")
		return core.PhaseResponse{}, nil
	})
	if err == nil {
		t.Error("ServeStdio must reject response-kind input")
	}
}

// Error-envelope-with-nil-Error is a malicious-child case the Run path
// guards against; the helper subprocess can't easily produce one, so we
// drive readOneEnvelopeLine + Unmarshal directly.
func TestSubprocessRunner_ErrorEnvelopeMissingErrorField(t *testing.T) {
	// Run a runner whose subprocess emits an error envelope with no error
	// field — synthesised by a tiny shell command rather than the Go
	// helper process. Use /bin/sh -c to print a hand-crafted line.
	bogus := `{"v":1,"kind":"error","correlation_id":"x"}`
	r := NewSubprocessRunner("p", "/bin/sh", []string{"-c", "echo '" + bogus + "'"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := r.Run(ctx, core.PhaseRequest{Cycle: 1, Budget: core.BudgetEnvelope{MaxUSD: 1.0}})
	if err == nil {
		t.Fatal("Run must reject error envelope with nil Error field")
	}
	var werr *WireError
	if !errors.As(err, &werr) || werr.Code != CodeDecodeFailed {
		t.Errorf("err must be WireError{DECODE_FAILED}, got %T %v", err, err)
	}
}

// Drive the Run path's EncodeRequest failure branch via the testable seam.
func TestSubprocessRunner_EncodeRequestFailure(t *testing.T) {
	saved := marshalJSON
	t.Cleanup(func() { marshalJSON = saved })
	marshalJSON = func(v any) ([]byte, error) {
		return nil, errors.New("synthetic")
	}
	r := NewSubprocessRunner("p", "/bin/sh", []string{"-c", ":"}, nil)
	_, err := r.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), "synthetic") {
		t.Errorf("Run must propagate EncodeRequest failure; got %v", err)
	}
}

// Drive empty-stdout decode-failed path.
func TestSubprocessRunner_EmptyStdout(t *testing.T) {
	r := NewSubprocessRunner("p", "/bin/sh", []string{"-c", ":"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := r.Run(ctx, core.PhaseRequest{Cycle: 1, Budget: core.BudgetEnvelope{MaxUSD: 1.0}})
	if err == nil {
		t.Fatal("Run must reject empty stdout")
	}
	var werr *WireError
	if !errors.As(err, &werr) || werr.Code != CodeDecodeFailed {
		t.Errorf("want DECODE_FAILED, got %v", err)
	}
}

// Drive ServeStdio's EncodeResponse marshal-failure branch.
func TestServeStdio_EncodeResponseFailure(t *testing.T) {
	saved := marshalJSON
	t.Cleanup(func() { marshalJSON = saved })
	// First call (EncodeRequest is consumed by us before ServeStdio runs);
	// ServeStdio's first marshalJSON is inside EncodeResponse.
	// Build the request envelope WITHOUT the seam by swapping it after.
	env, _ := EncodeRequest("c", core.PhaseRequest{Cycle: 1})
	raw, _ := json.Marshal(env)
	in := bytes.NewReader(append(raw, '\n'))
	out := &bytes.Buffer{}

	marshalJSON = func(v any) ([]byte, error) {
		return nil, errors.New("synthetic encode failure")
	}
	err := ServeStdio(in, out, func(ctx context.Context, r core.PhaseRequest) (core.PhaseResponse, error) {
		return core.PhaseResponse{Verdict: core.VerdictPASS}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "synthetic") {
		t.Errorf("ServeStdio must propagate EncodeResponse failure; got %v", err)
	}
}

// Drive ServeStdio's "read failed with no data" branch.
func TestServeStdio_EmptyInput(t *testing.T) {
	in := bytes.NewReader(nil)
	out := &bytes.Buffer{}
	err := ServeStdio(in, out, func(ctx context.Context, r core.PhaseRequest) (core.PhaseResponse, error) {
		t.Fatal("handler must not run")
		return core.PhaseResponse{}, nil
	})
	if err == nil {
		t.Error("ServeStdio must error on empty stdin")
	}
}

// Drive ServeStdio's write-failure branch via a failing writer.
type failingWriter struct{}

func (f *failingWriter) Write(p []byte) (int, error) { return 0, errors.New("disk full") }

func TestServeStdio_WriteFailure(t *testing.T) {
	env, _ := EncodeRequest("c", core.PhaseRequest{Cycle: 1})
	raw, _ := json.Marshal(env)
	in := bytes.NewReader(append(raw, '\n'))
	err := ServeStdio(in, &failingWriter{}, func(ctx context.Context, r core.PhaseRequest) (core.PhaseResponse, error) {
		return core.PhaseResponse{Verdict: core.VerdictPASS}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("want write failure propagated; got %v", err)
	}
}

// Drive ServeStdio's "request decode rejected (wrong version)" branch.
func TestServeStdio_RejectsBadVersion(t *testing.T) {
	bogus := []byte(`{"v":999,"kind":"request","correlation_id":"x","payload":{}}` + "\n")
	in := bytes.NewReader(bogus)
	out := &bytes.Buffer{}
	err := ServeStdio(in, out, func(ctx context.Context, r core.PhaseRequest) (core.PhaseResponse, error) {
		t.Fatal("handler must not run")
		return core.PhaseResponse{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Errorf("want version rejection; got %v", err)
	}
}

// Smoke: an exec.Cmd built from helperCmd should be plumbable through.
// Guards against accidental Path/Args plumbing regressions.
func TestSubprocessRunner_CmdShape(t *testing.T) {
	r := NewSubprocessRunner("p", "/bin/echo", []string{"hello"}, []string{"K=v"})
	cmd := r.buildCmd(context.Background())
	if cmd.Path != "/bin/echo" {
		t.Errorf("Path=%q, want /bin/echo", cmd.Path)
	}
	if len(cmd.Args) < 1 || cmd.Args[0] != "/bin/echo" {
		t.Errorf("Args[0]=%q, want /bin/echo", cmd.Args[0])
	}
	// env should include our K=v plus inherited PATH.
	found := false
	for _, e := range cmd.Env {
		if e == "K=v" {
			found = true
		}
	}
	if !found {
		t.Errorf("env missing K=v: %v", cmd.Env)
	}
	// Type confirmation it really is exec.Cmd.
	var _ *exec.Cmd = cmd
}
