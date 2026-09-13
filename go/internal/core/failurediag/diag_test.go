package failurediag

// diag_test.go — unit 02: the failure-diag sidecar writer. The diagnosis is
// pure over the injected clock and predicate (the mutant seam); Write owns the
// only side effects (one file, at most one signal). RED first.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func fixedNow() time.Time { return time.Unix(1_700_000_000, 0) }

func recordingSignals() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

func newTestWriter(c *signalcenter.Center) *Writer {
	return NewWriter(fixedNow, isTimeout, WithSignals(func() *signalcenter.Center { return c }))
}

func wedgedErr() error {
	return fmt.Errorf("bridge: launch exit=81: artifact-timeout: phase=retro reason=%q: %w", "prompt submit_wedged (resends=3)", errTimeout)
}

const wedgedSidecar = `{"phase":"retro","cycle":1562,"error_message":"bridge: launch exit=81: artifact-timeout: phase=retro reason=\"prompt submit_wedged (resends=3)\": core: bridge artifact timeout","delivery_failure":"prompt submit_wedged (resends=3)","exit_code":81,"attempt_count":2,"timestamp":"2023-11-14T22:13:20Z"}`

func TestWriter_Diagnose_IsByteIdenticalToThePreExtractionSidecar(t *testing.T) {
	w := newTestWriter(nil)
	data, err := json.Marshal(w.diagnose("retro", 1562, wedgedErr(), 2))
	if err != nil || string(data) != wedgedSidecar {
		t.Fatalf("the seven fields in wire order, compact, no omitempty:\n got %s\nwant %s (%v)", data, wedgedSidecar, err)
	}
	var back Sidecar
	if err := json.Unmarshal(data, &back); err != nil || back.DeliveryFailure != "prompt submit_wedged (resends=3)" || back.ExitCode != ExitCodeArtifactTimeout {
		t.Fatalf("the declared contract round-trips: %+v %v", back, err)
	}
}

func TestWriter_Diagnose_MapsExitCodeTimeoutOnly(t *testing.T) {
	w := newTestWriter(nil)
	exitErr := exec.Command("sh", "-c", "exit 7").Run()
	var ee *exec.ExitError
	if !errors.As(exitErr, &ee) {
		t.Fatalf("precondition: a real *exec.ExitError: %v", exitErr)
	}
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"bare timeout":        {errTimeout, 81},
		"wrapped timeout":     {fmt.Errorf("phase scout: %w", errTimeout), 81},
		"subprocess exit 7":   {exitErr, 7},
		"plain error":         {errors.New("index out of range"), 1},
		"transient-like":      {fmt.Errorf("bridge: launch exit=85: %w", errors.New("core: transient bridge failure")), 1},
		"timeout under never": {errTimeout, 1},
	} {
		got := w.diagnose("scout", 1, tc.err, 1).ExitCode
		if name == "timeout under never" {
			got = NewWriter(fixedNow, func(error) bool { return false }).diagnose("scout", 1, tc.err, 1).ExitCode
		}
		if got != tc.want {
			t.Errorf("%s: exit_code = %d, want %d", name, got, tc.want)
		}
	}
}

func TestWriter_Diagnose_TimestampIsUTCFromTheInjectedClock(t *testing.T) {
	tokyo := func() time.Time { return time.Date(2026, 9, 13, 12, 0, 0, 500_000_000, time.FixedZone("JST", 9*3600)) }
	w := NewWriter(tokyo, isTimeout)
	d := w.diagnose("build", 7, errors.New("boom"), 3)
	if d.Timestamp != "2026-09-13T03:00:00Z" || d.ErrorMessage != "boom" || d.Phase != "build" || d.Cycle != 7 || d.AttemptCount != 3 || d.DeliveryFailure != "" {
		t.Fatalf("UTC at second precision, the message verbatim, the rest as given: %+v", d)
	}
}

func TestWriter_Write_LandsTheSidecarAtomically(t *testing.T) {
	ws := t.TempDir()
	newTestWriter(nil).Write(ws, "retro", 1562, wedgedErr(), 2)
	data, err := os.ReadFile(SidecarPath(ws, "retro"))
	if err != nil || string(data) != wedgedSidecar {
		t.Fatalf("the file equals the diagnosis bytes: %s (%v)", data, err)
	}
	if info, _ := os.Stat(SidecarPath(ws, "retro")); info.Mode().Perm() != 0o644 {
		t.Fatalf("mode 0644: %v", info.Mode())
	}
	if _, err := os.Stat(SidecarPath(ws, "retro") + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("the temp file is renamed away")
	}
}

func oneWarn(t *testing.T, got []signalcenter.Event) signalcenter.Event {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("exactly one signal: %+v", got)
	}
	e := got[0]
	if e.Module != signalcenter.ModuleFailureDiag || e.Kind != signalcenter.KindFailureDiagWarning || e.Severity != signalcenter.SeverityWarn || e.Code != CodeSidecarWriteFailed || e.Origin != "Writer.Write" || e.Cycle != 1562 || e.Phase != "retro" {
		t.Fatalf("module failurediag, kind failurediag.warning, WARN, the unit's code, origin Writer.Write, the check's identity: %+v", e)
	}
	return e
}

func TestWriter_Write_TempWriteFailureIsAWarnSignal(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(ws, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, got := recordingSignals()
	newTestWriter(c).Write(ws, "retro", 1562, wedgedErr(), 2)
	e := oneWarn(t, *got)
	if !strings.HasPrefix(e.Reason, "failure-diag temp write failed: ") || e.Fields["path"] != SidecarPath(ws, "retro") {
		t.Fatalf("the reason names the step and the error, fields name the path: %+v", e)
	}
}

func TestWriter_Write_RenameFailureIsAWarnSignalAndLeavesTheTemp(t *testing.T) {
	ws := t.TempDir()
	target := SidecarPath(ws, "retro")
	if err := os.MkdirAll(filepath.Join(target, "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}
	c, got := recordingSignals()
	newTestWriter(c).Write(ws, "retro", 1562, wedgedErr(), 2)
	e := oneWarn(t, *got)
	if !strings.HasPrefix(e.Reason, "failure-diag rename failed: ") {
		t.Fatalf("the reason names the rename step: %q", e.Reason)
	}
	if _, err := os.Stat(target + ".tmp"); err != nil {
		t.Fatalf("the temp file is left beside the target (pinned as today's behaviour, follow-up F4): %v", err)
	}
}

func TestWithSignals_NilIsTheNullObject(t *testing.T) {
	opts := []Option{WithSignals(nil)}
	w := NewWriter(fixedNow, isTimeout, opts...)
	if w.SignalsWired() {
		t.Fatal("nil is reported unwired")
	}
	if !newTestWriter(signalcenter.New()).SignalsWired() {
		t.Fatal("a Center is reported wired")
	}
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(SidecarPath(ws, "retro"), "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}
	w.Write(ws, "retro", 1562, wedgedErr(), 2) // the rename-failure path with no Center must not panic
	if _, err := os.Stat(SidecarPath(ws, "retro") + ".tmp"); err != nil {
		t.Fatal("the file was still staged through the Null Object")
	}
}

func TestWithSignals_ReadsTheCenterLive(t *testing.T) {
	var late *signalcenter.Center
	w := NewWriter(fixedNow, isTimeout, WithSignals(func() *signalcenter.Center { return late }))
	if w.SignalsWired() {
		t.Fatal("no Center yet: unwired")
	}
	var got []signalcenter.Event
	late = signalcenter.New()
	late.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	if !w.SignalsWired() {
		t.Fatal("the Center installed after construction is seen")
	}
	ws := filepath.Join(t.TempDir(), "file-as-workspace")
	if err := os.WriteFile(ws, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.Write(ws, "retro", 1562, wedgedErr(), 2)
	if len(got) != 1 || got[0].Code != CodeSidecarWriteFailed {
		t.Fatalf("the late Center receives the unit's signal: %+v", got)
	}
}

func TestFailureDiagCodes_AreRegisteredWithDocs(t *testing.T) {
	if m, ok := signalcenter.IsRegistered(CodeSidecarWriteFailed); !ok || m != signalcenter.ModuleFailureDiag {
		t.Fatalf("FAILUREDIAG_SIDECAR_WRITE_FAILED is registered under module failurediag: %v %v", m, ok)
	}
}

func TestSidecarPath_IsThePhaseFailureDiagFile(t *testing.T) {
	if got := SidecarPath("/ws", "scout"); got != filepath.Join("/ws", "scout-failure-diag.json") {
		t.Fatalf("SidecarPath = %q", got)
	}
}

// The gate is required: a nil predicate panics at first use — never "nothing
// is a timeout" (the doc comment's contract, pinned).
func TestNewWriter_NilGatePanicsAtFirstUse(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a nil gate must panic at first use, not classify everything as a non-timeout")
		}
	}()
	NewWriter(fixedNow, nil).diagnose("scout", 1, errors.New("boom"), 1)
}
