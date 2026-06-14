//go:build integration

// Tests for the ship phase dispatcher (ship.go). The ship phase now runs
// the native Go shipper unconditionally (native.go); the full ship state
// machine is exercised by native_test.go and dispatch_test.go. These tests
// cover the phase-level invariants that are independent of the native
// state machine: the runner-required guard, default commit-message
// synthesis, the phase name, and the production execRunner seam.
package ship

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestDefaultCommitMessage(t *testing.T) {
	if got := defaultCommitMessage(core.PhaseRequest{Cycle: 7, GoalHash: "h"}); got != "evolve-cycle 7: goal=h" {
		t.Errorf("got %q", got)
	}
	if got := defaultCommitMessage(core.PhaseRequest{Cycle: 7}); got != "evolve-cycle 7" {
		t.Errorf("no-goalhash got %q", got)
	}
	if got := defaultCommitMessage(core.PhaseRequest{}); got != "evolve-cycle 0" {
		t.Errorf("zero-value got %q", got)
	}
}

func TestRun_MissingRunner_ReturnsError(t *testing.T) {
	phase := New(Config{})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p",
		Context: map[string]string{"commit_message": "x"},
	})
	if err == nil || !strings.Contains(err.Error(), "runner required") {
		t.Fatalf("err=%v, want runner-required", err)
	}
}

func TestName(t *testing.T) {
	p := New(Config{})
	if p.Name() != "ship" {
		t.Errorf("Name=%q, want ship", p.Name())
	}
}

// TestExecRunner_Success drives the production CmdRunner against
// /bin/true (exit 0). Locked to POSIX paths; tests skip on Windows
// (we don't ship Windows per parent plan §7).
func TestExecRunner_Success(t *testing.T) {
	if _, err := os.Stat("/bin/true"); err != nil {
		t.Skip("no /bin/true")
	}
	var stdout, stderr io.Writer = io.Discard, io.Discard
	code, err := execRunner(context.Background(), "/bin/true", "", nil, nil, nil, stdout, stderr)
	if err != nil {
		t.Fatalf("execRunner: %v", err)
	}
	if code != 0 {
		t.Errorf("exit=%d, want 0", code)
	}
}

func TestExecRunner_NonZeroExit(t *testing.T) {
	if _, err := os.Stat("/bin/false"); err != nil {
		t.Skip("no /bin/false")
	}
	code, err := execRunner(context.Background(), "/bin/false", "", nil, nil, nil, io.Discard, io.Discard)
	if err != nil {
		t.Errorf("err=%v, want nil (exit-status mapped to code)", err)
	}
	if code == 0 {
		t.Errorf("exit=%d, want non-zero", code)
	}
}

func TestExecRunner_NotFound(t *testing.T) {
	_, err := execRunner(context.Background(), "/no/such/binary/ever", "", nil, nil, nil, io.Discard, io.Discard)
	if err == nil {
		t.Errorf("err=nil, want non-nil for missing binary")
	}
}

func TestNewWithDefaultRunner_HasRunner(t *testing.T) {
	p := NewWithDefaultRunner()
	if p == nil {
		t.Fatal("NewWithDefaultRunner returned nil")
	}
	if p.runner == nil {
		t.Errorf("runner field is nil; want execRunner")
	}
}
