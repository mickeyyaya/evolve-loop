package phasecmd

// phase_observer_root_test.go — ADR-0103 unit 12 §6 tests 39-40: the manual
// subcommand's argument contract pinned verbatim (help text, flags, the three
// usage lines, the discarded Atoi errors — Q8) and the root's stderr-only
// Signal Center rendering the engine's codes on the SAME stderr the replaced
// [phase-observer] lines used.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseobserver"
)

const usageLine = "[phase-observer] usage: phase-observer [--enforce] [--scope=...] <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]\n"

// TestParseObserverArgs_TableVerbatim — kills M55 (a root stderr line
// reworded), M56 (an Atoi error surfaced: a bogus pgid/cycle parses as 0 and
// `cycle must be integer` fires downstream in Run).
func TestParseObserverArgs_TableVerbatim(t *testing.T) {
	t.Parallel()
	five := []string{"/ws", "123", "7", "build", "builder"}
	rows := []struct {
		name          string
		args          []string
		wantRC        int
		wantHandled   bool
		wantStdout    string
		wantStderr    string
		wantEnforce   bool
		wantScope     phaseobserver.Scope
		wantPositions int
	}{
		{"help", []string{"--help"}, 0, true, "Usage: evolve phase-observer [--enforce] [--scope=cycle|phase] \\\n       <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]\n", "", false, phaseobserver.ScopePhase, 0},
		{"short help", []string{"-h"}, 0, true, "Usage: evolve phase-observer [--enforce] [--scope=cycle|phase] \\\n       <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]\n", "", false, phaseobserver.ScopePhase, 0},
		{"enforce + cycle scope", append([]string{"--enforce", "--scope=cycle"}, five...), 0, false, "", "", true, phaseobserver.ScopeCycle, 5},
		{"phase scope explicit", append([]string{"--scope=phase"}, five...), 0, false, "", "", false, phaseobserver.ScopePhase, 5},
		{"unknown scope", append([]string{"--scope=batch"}, five...), phaseobserver.ExitInvalidArgs, true, "", "[phase-observer] unknown --scope value: --scope=batch\n", false, phaseobserver.ScopePhase, 0},
		{"unknown flag", append([]string{"--verbose"}, five...), phaseobserver.ExitInvalidArgs, true, "", "[phase-observer] unknown flag: --verbose\n", false, phaseobserver.ScopePhase, 0},
		{"too few positionals", five[:4], phaseobserver.ExitInvalidArgs, true, "", usageLine, false, phaseobserver.ScopePhase, 4},
		{"six positionals keep the cycle-state", append(five, "state.json"), 0, false, "", "", false, phaseobserver.ScopePhase, 6},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			a, rc, handled := parseObserverArgs(row.args, &out, &errb)
			if rc != row.wantRC || handled != row.wantHandled || out.String() != row.wantStdout || errb.String() != row.wantStderr {
				t.Errorf("rc=%d handled=%v\nstdout=%q\nstderr=%q", rc, handled, out.String(), errb.String())
			}
			if a.enforce != row.wantEnforce || a.scope != row.wantScope || len(a.pos) != row.wantPositions {
				t.Errorf("args = %+v", a)
			}
		})
	}
	// Q8: the Atoi errors are discarded — a bogus pgid/cycle becomes 0.
	shutdown := make(chan struct{})
	cfg := observerConfig(observerArgs{pos: []string{"/ws", "bogus", "x", "build", "builder", "state.json"}}, shutdown, nil)
	if cfg.SubagentPGID != 0 || cfg.Cycle != 0 || cfg.CycleState != "state.json" || cfg.Workspace != "/ws" || cfg.Phase != "build" || cfg.Agent != "builder" {
		t.Errorf("config: %+v", cfg)
	}
	if cfg.ProcessAlive == nil || cfg.ShutdownSig == nil || cfg.Signals == nil {
		t.Error("the root wires the probe, the shutdown channel and the Center accessor")
	}
}

// TestRunPhaseObserver_RendersObserverCodesOnStderr — end to end through the
// subcommand: the report path is a DIRECTORY, so the engine's
// OBSERVER_REPORT_WRITE_FAILED renders on the subcommand's stderr through the
// root's WARN-filtered Center; rc stays 0. Termination: the handler at
// signal.Notify is armed before Run, so once observer_started is on disk a
// SIGUSR1 to this process closes the shutdown channel (critic B3). The
// project root points at an empty temp dir so no real policy.json is read.
// Kills M57 (a nil Signals at the root), M58 (a missing Flush).
func TestRunPhaseObserver_RendersObserverCodesOnStderr(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, "builder-observer-report.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- RunPhaseObserver([]string{ws, "bogus-pgid", "7", "build", "builder"}, nil, &out, &errb)
	}()
	events := filepath.Join(ws, "builder-observer-events.ndjson")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(events); err == nil && strings.Contains(string(raw), "observer_started") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGUSR1); err != nil {
		t.Fatal(err)
	}
	select {
	case rc := <-done:
		if rc != 0 {
			t.Fatalf("rc=%d, want 0 (the report is best-effort)", rc)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunPhaseObserver did not return after SIGUSR1")
	}
	got := errb.String()
	if !strings.Contains(got, "[observer] observer.warning WARN OBSERVER_REPORT_WRITE_FAILED cycle=7 phase=build") || !strings.Contains(got, "origin=Engine.WriteReport") || !strings.Contains(got, "step=report") {
		t.Errorf("the engine's code renders on the subcommand's stderr:\n%s", got)
	}
	if strings.Contains(got, "[phase-observer] WARN") {
		t.Errorf("the replaced prose line is gone:\n%s", got)
	}
	var errb2 bytes.Buffer
	if rc := RunPhaseObserver([]string{"only", "four", "args", "here"}, nil, &out, &errb2); rc != phaseobserver.ExitInvalidArgs || errb2.String() != usageLine {
		t.Errorf("the usage path still prints the verbatim line: rc=%d %q", rc, errb2.String())
	}
}

// TestRunPhaseObserver_FlushesItsCenterOnReturn — the root defers Flush like
// the two orchestrator roots (TestSignalCenterFlush_IsWiredAtBothRoots idiom).
// Behaviourally the subprocess's stderr-only Center delivers synchronously on
// its one goroutine, so a removed Flush is an EQUIVALENT mutant at runtime;
// this source pin keeps the root idiom structurally.
func TestRunPhaseObserver_FlushesItsCenterOnReturn(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("phase_observer.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "defer signals.Flush()") {
		t.Error("RunPhaseObserver must defer signals.Flush() after building its Center")
	}
}
