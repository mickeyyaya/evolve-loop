package gaterun

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const serveFlag = "--gaterun-serve"

func fixtureEntry(name string) gatecontract.Entry {
	return gatecontract.Entry{Name: name, Run: func(ctx context.Context, in gatecontract.Input, ports gatecontract.Ports) (gatecontract.Verdict, error) {
		if name == "fails" {
			return gatecontract.Verdict{Reds: []string{"ignored"}}, errors.New("did not judge")
		}
		files, err := ports.Lane.TrackedFiles(in.Root, "d")
		if err != nil {
			return gatecontract.Verdict{}, err
		}
		tree, _ := ports.Lane.TrackedTree(in.Root, "d")
		set, _ := ports.Lane.TrackedSet(in.Root, "d", ".go")
		listed, _ := ports.Toolchain.List(ctx, gatecontract.Call{Dir: in.Root, Args: []string{"-json"}})
		tested, _ := ports.Toolchain.Test(ctx, gatecontract.Call{})
		reds := slices.Concat(files, tree, []string{fmt.Sprint(len(set)), string(listed.Stdout), fmt.Sprint(tested.ExitCode)})
		return gatecontract.Verdict{Reds: reds, Notes: []string{"note"}}, nil
	}}
}

func TestMain(m *testing.M) {
	if i := slices.Index(os.Args, serveFlag); i >= 0 {
		if err := Serve(context.Background(), fixtureEntry(os.Args[i+1]), os.Stdin, os.Stdout); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func helperCommand(ctx context.Context, name string) *exec.Cmd {
	return sysexec.Command(ctx, os.Args[0], serveFlag, name)
}

func shellCommand(script string) func(context.Context, string) *exec.Cmd {
	return func(ctx context.Context, _ string) *exec.Cmd { return sysexec.Command(ctx, "sh", "-c", script) }
}

func TestRun_TheChildCallsTheParentPortsAndTheVerdictComesBackUnchanged(t *testing.T) {
	t.Parallel()
	opts := Options{Command: helperCommand, Ports: gatecontract.Ports{Lane: stubLane{}, Toolchain: stubToolchain{}}}
	got, err := Run(context.Background(), gatecontract.Entry{Name: "serves"}, gatecontract.Input{Root: "r", Base: "b"}, opts)
	want, _ := fixtureEntry("serves").Run(context.Background(), gatecontract.Input{Root: "r", Base: "b"}, gatecontract.Ports{Lane: stubLane{}, Toolchain: stubToolchain{}})
	if err != nil || !slices.Equal(got.Reds, want.Reds) || !slices.Equal(got.Notes, want.Notes) {
		t.Fatalf("Run = %+v, %v, want %+v", got, err, want)
	}
	if got.Reds[0] != "files:r:d" || got.Reds[len(got.Reds)-1] != "7" {
		t.Errorf("Reds = %v, want the answers of the parent", got.Reds)
	}
}

func TestRun_ThePrefixWrapsTheCommandAndANilPrefixLeavesItAlone(t *testing.T) {
	t.Parallel()
	ports := gatecontract.Ports{Lane: stubLane{}, Toolchain: stubToolchain{}}
	if _, err := Run(context.Background(), gatecontract.Entry{Name: "serves"}, gatecontract.Input{}, Options{Command: helperCommand, Ports: ports}); err != nil {
		t.Errorf("Run with a nil prefix = %v", err)
	}
	marker := Prefix(func(cmd *exec.Cmd) *exec.Cmd {
		return sysexec.Command(context.Background(), "sh", append([]string{"-c", `echo wrapped-by-prefix >&2; exec "$@"`, "sh"}, cmd.Args...)...)
	})
	opts := Options{Command: shellCommand("read l; echo not-a-frame"), Strategy: prefixStrategy(marker)}
	if _, err := Run(context.Background(), gatecontract.Entry{Name: "x"}, gatecontract.Input{}, opts); err == nil || !strings.Contains(err.Error(), "wrapped-by-prefix") {
		t.Errorf("Run with a prefix = %v, want the stderr of the wrapper", err)
	}
}

func TestRun_AChildThatSendsJunkAndThenHangsIsKilled(t *testing.T) {
	t.Parallel()
	start := time.Now()
	_, err := Run(context.Background(), gatecontract.Entry{Name: "x"}, gatecontract.Input{}, Options{Command: shellCommand(`read l; echo junk; exec sleep 20`)})
	if err == nil || time.Since(start) > 10*time.Second {
		t.Errorf("Run = %v after %v, want an error in under 10 s", err, time.Since(start))
	}
}

func TestRun_AFrameOverTheLimitIsAnErrorAndNeverAPass(t *testing.T) {
	t.Parallel()
	script := `read l; head -c 5000000 /dev/zero | tr '\0' 'a'; echo`
	_, err := Run(context.Background(), gatecontract.Entry{Name: "x"}, gatecontract.Input{}, Options{Command: shellCommand(script)})
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("Run = %v, want a frame-over-the-limit error", err)
	}
}

func TestRun_AnErrorOfTheEntryComesBackAsAnErrorAndNoRed(t *testing.T) {
	t.Parallel()
	verdict, err := Run(context.Background(), gatecontract.Entry{Name: "fails"}, gatecontract.Input{}, Options{Command: helperCommand})
	if err == nil || !strings.Contains(err.Error(), "did not judge") || len(verdict.Reds) != 0 {
		t.Errorf("Run = %+v, %v, want the error of the entry and no red", verdict, err)
	}
}

func TestRun_EachFailureOfTheChildIsAnErrorAndNeverAPass(t *testing.T) {
	t.Parallel()
	verdictLine := `{"kind":"verdict","verdict":{"Reds":null,"Notes":null}}`
	cases := map[string]struct{ script, want string }{
		"error frame":     {`read l; echo '{"kind":"error","message":"the child failed"}'`, "the child failed"},
		"non-zero exit":   {`read l; echo '` + verdictLine + `'; echo oops >&2; exit 4`, "oops"},
		"malformed":       {`read l; echo not-a-frame`, "malformed frame"},
		"no verdict":      {`read l`, "no verdict"},
		"early exit":      {`exit 0`, "no verdict"},
		"unexpected kind": {`read l; echo '{"kind":"input","input":{"Root":"r"}}'`, "unexpected"},
	}
	for name, tc := range cases {
		verdict, err := Run(context.Background(), gatecontract.Entry{Name: "x"}, gatecontract.Input{}, Options{Command: shellCommand(tc.script)})
		if err == nil || !strings.Contains(err.Error(), tc.want) || len(verdict.Reds) != 0 {
			t.Errorf("%s: Run = %+v, %v, want an error that holds %q", name, verdict, err, tc.want)
		}
	}
}

func TestRun_AnUnknownRequestFromTheChildGetsAnErrorResponse(t *testing.T) {
	t.Parallel()
	script := `read l; echo '{"kind":"request","id":1,"method":"os.Remove","params":{}}'; read r; echo "$r" >&2; echo '{"kind":"error","message":"stop"}'`
	_, err := Run(context.Background(), gatecontract.Entry{Name: "x"}, gatecontract.Input{}, Options{Command: shellCommand(script)})
	if err == nil || !strings.Contains(err.Error(), "unknown function") {
		t.Errorf("Run = %v, want the stderr of the child to hold the unknown-function response", err)
	}
}

func TestRun_ACommandThatCannotStartAndAMissingCommandAreErrors(t *testing.T) {
	t.Parallel()
	missing := func(ctx context.Context, _ string) *exec.Cmd { return sysexec.Command(ctx, "/no/such/binary") }
	if _, err := Run(context.Background(), gatecontract.Entry{Name: "x"}, gatecontract.Input{}, Options{Command: missing}); err == nil {
		t.Error("Run with a missing binary = nil, want an error")
	}
	if _, err := Run(context.Background(), gatecontract.Entry{Name: "x"}, gatecontract.Input{}, Options{}); err == nil {
		t.Error("Run with no command = nil, want an error")
	}
}

func TestEvolveCommand_BuildsTheGateRunCommandOfTheEntry(t *testing.T) {
	t.Parallel()
	cmd := EvolveCommand("/bin/evolve")(context.Background(), "size-ratchet")
	if got := strings.Join(cmd.Args, " "); got != "/bin/evolve gate run --entry size-ratchet" {
		t.Errorf("Args = %q", got)
	}
}

func frames(t *testing.T, list ...Frame) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	for _, f := range list {
		if err := WriteFrame(&buf, f); err != nil {
			t.Fatal(err)
		}
	}
	return &buf
}

func TestServe_ReadsTheInputCallsTheEntryAndWritesTheVerdict(t *testing.T) {
	t.Parallel()
	entry := gatecontract.Entry{Name: "x", Run: func(_ context.Context, in gatecontract.Input, _ gatecontract.Ports) (gatecontract.Verdict, error) {
		return gatecontract.Verdict{Reds: []string{in.Root + in.Base}}, nil
	}}
	var out bytes.Buffer
	in := frames(t, Frame{Kind: KindInput, Input: &gatecontract.Input{Root: "r", Base: "b"}})
	if err := Serve(context.Background(), entry, in, &out); err != nil {
		t.Fatal(err)
	}
	frame, err := ReadFrame(bufio.NewReader(&out))
	if err != nil || frame.Kind != KindVerdict || frame.Verdict.Reds[0] != "rb" {
		t.Errorf("frame = %+v, %v", frame, err)
	}
}

func TestServe_AWrongFirstFrameAnEmptyStreamAndAnEntryErrorEachWriteAnErrorFrame(t *testing.T) {
	t.Parallel()
	cases := map[string]io.Reader{
		"wrong kind": frames(t, Frame{Kind: KindError, Message: "m"}),
		"empty":      strings.NewReader(""),
		"entry":      frames(t, Frame{Kind: KindInput, Input: &gatecontract.Input{}}),
	}
	for name, in := range cases {
		var out bytes.Buffer
		entry := gatecontract.Entry{Name: "x", Run: func(context.Context, gatecontract.Input, gatecontract.Ports) (gatecontract.Verdict, error) {
			return gatecontract.Verdict{}, errors.New("entry failed")
		}}
		if err := Serve(context.Background(), entry, in, &out); err == nil {
			t.Errorf("%s: Serve = nil, want an error", name)
		}
		if frame, err := ReadFrame(bufio.NewReader(&out)); err != nil || frame.Kind != KindError {
			t.Errorf("%s: frame = %+v, %v, want an error frame", name, frame, err)
		}
	}
}

func TestServe_AClosedOutputIsAnError(t *testing.T) {
	t.Parallel()
	entry := gatecontract.Entry{Name: "x", Run: func(context.Context, gatecontract.Input, gatecontract.Ports) (gatecontract.Verdict, error) {
		return gatecontract.Verdict{}, nil
	}}
	if err := Serve(context.Background(), entry, frames(t, Frame{Kind: KindInput, Input: &gatecontract.Input{}}), failWriter{}); err == nil {
		t.Error("Serve to a closed writer = nil, want an error")
	}
}

func TestRun_ACommandThatAlreadyHoldsAPipeIsAnError(t *testing.T) {
	t.Parallel()
	holdsStdin := func(ctx context.Context, _ string) *exec.Cmd {
		cmd := sysexec.Command(ctx, "true")
		cmd.Stdin = strings.NewReader("")
		return cmd
	}
	holdsStdout := func(ctx context.Context, _ string) *exec.Cmd {
		cmd := sysexec.Command(ctx, "true")
		cmd.Stdout = io.Discard
		return cmd
	}
	for name, command := range map[string]func(context.Context, string) *exec.Cmd{"stdin": holdsStdin, "stdout": holdsStdout} {
		if _, err := Run(context.Background(), gatecontract.Entry{Name: "x"}, gatecontract.Input{}, Options{Command: command}); err == nil {
			t.Errorf("%s: Run = nil, want an error", name)
		}
	}
}

type failAfter struct{ left int }

func (f *failAfter) Write(p []byte) (int, error) {
	if f.left--; f.left < 0 {
		return 0, errors.New("closed")
	}
	return len(p), nil
}

func TestConverse_AFailedWriteOfTheInputOrOfAResponseIsAnError(t *testing.T) {
	t.Parallel()
	request := Frame{Kind: KindRequest, ID: 1, Method: MethodTrackedFiles, Params: []byte(`{}`)}
	for name, writer := range map[string]io.Writer{"input": failWriter{}, "response": &failAfter{left: 1}} {
		_, err := converse(context.Background(), Broker{Ports: gatecontract.Ports{Lane: stubLane{}}}, gatecontract.Input{}, bufio.NewReader(frames(t, request)), writer)
		if err == nil {
			t.Errorf("%s: converse = nil, want an error", name)
		}
	}
}

func TestRun_AScratchParentThatCannotHoldTheScratchDirIsAnError(t *testing.T) {
	t.Parallel()
	_, err := Run(context.Background(), gatecontract.Entry{Name: "x"}, gatecontract.Input{}, Options{Command: helperCommand, ScratchParent: "/no/such/parent"})
	if err == nil {
		t.Error("Run = nil, want an error")
	}
}

func TestNewScratch_ACanaryThatCannotBePlacedIsAnErrorAndLeavesNoDir(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	if _, err := newScratch(parent, []string{"no-such-dir/canary"}); err == nil || !strings.Contains(err.Error(), "canary") {
		t.Errorf("newScratch = %v, want a canary error", err)
	}
	if left, _ := os.ReadDir(parent); len(left) != 0 {
		t.Errorf("the failed scratch dir must be removed: %v", left)
	}
}

func TestServeWith_ReadsTheStartCanaryBeforeTheEntryAndTheEndCanaryAfterIt(t *testing.T) {
	t.Parallel()
	for name, entryErr := range map[string]error{"pass": nil, "error": errors.New("boom")} {
		var order []string
		entry := gatecontract.Entry{Name: "x", Run: func(context.Context, gatecontract.Input, gatecontract.Ports) (gatecontract.Verdict, error) {
			order = append(order, "entry")
			return gatecontract.Verdict{}, entryErr
		}}
		var in bytes.Buffer
		if err := WriteFrame(&in, Frame{Kind: KindInput, Input: &gatecontract.Input{Root: "/r"}}); err != nil {
			t.Fatal(err)
		}
		_ = serveWith(context.Background(), entry, &in, io.Discard, func(canary string) { order = append(order, canary) })
		if want := []string{sandbox.CanaryStart, "entry", sandbox.CanaryEnd}; !slices.Equal(order, want) {
			t.Errorf("%s: order = %v, want %v: the two canary reports bracket the reports of the entry", name, order, want)
		}
	}
}

func TestServe_OpensBothCanariesOfTheTempDirAroundTheEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	for _, name := range []string{sandbox.CanaryStart, sandbox.CanaryEnd} {
		if err := syscall.Mkfifo(filepath.Join(dir, name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var in bytes.Buffer
	if err := WriteFrame(&in, Frame{Kind: KindInput, Input: &gatecontract.Input{}}); err != nil {
		t.Fatal(err)
	}
	ran := make(chan struct{})
	entry := gatecontract.Entry{Name: "x", Run: func(context.Context, gatecontract.Input, gatecontract.Ports) (gatecontract.Verdict, error) {
		close(ran)
		return gatecontract.Verdict{}, nil
	}}
	done := make(chan error, 1)
	go func() { done <- Serve(context.Background(), entry, &in, io.Discard) }()
	openWriter(t, filepath.Join(dir, sandbox.CanaryStart))
	<-ran
	openWriter(t, filepath.Join(dir, sandbox.CanaryEnd))
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after both canaries closed")
	}
}

func openWriter(t *testing.T, fifo string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		w, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			_ = w.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Serve never opened %s: the write end of a FIFO opens only when a reader holds it", fifo)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
