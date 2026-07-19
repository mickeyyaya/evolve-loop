package bridge

// engine_ctxcancel_deliverable_test.go — deliverable-authority-centralization
// (cycle-859 false-FAIL, the 5th rung of the class). A phase whose completion
// signal never fires rides its wait window until the orchestrator cancels the
// context; exec.CommandContext then SIGKILLs the driver subprocess, and Go's
// (*exec.ExitError).ExitCode() reports -1 for a signal death. The engine mapped
// -1 to a PLAIN error (neither 81 nor the transient set), so the generic phase
// runner took its substantive-error door and hard-FAILed WITHOUT consulting the
// on-disk deliverable — discarding a green-ACS PASS audit whose report, sentinel,
// and challenge token were all well-formed on disk.
//
// A context-cancellation kill is infra teardown, the sibling of the 124 cmd-
// timeout and the 81 artifact-timeout: classify it as transient so the runner's
// reconcile door consults the deliverable. Gated on ctx.Err() so a genuine
// start/launch failure (also -1, but with a live context) still fails loud.

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestEngineLaunch_CtxCancelSignalKill_WrapsTransient: a driver SIGKILL'd by
// context cancellation (exit -1, ctx.Err() != nil) must wrap
// core.ErrTransientBridgeFailure so IsInfraTeardownError routes it to the
// runner's reconcile-against-the-deliverable door (cycle-859).
func TestEngineLaunch_CtxCancelSignalKill_WrapsTransient(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	ctx, cancel := context.WithCancel(context.Background())

	// Model exec.CommandContext's teardown: the context is cancelled (as the
	// orchestrator's phase timeout does) and the subprocess dies with the
	// signal-death exit code -1.
	runner := func(_ context.Context, name, dir string, args, env []string,
		_ io.Reader, _, _ io.Writer) (int, error) {
		cancel()
		return -1, nil
	}
	eng := NewEngine(Deps{Runner: runner, LookupEnv: mapLookup(nil)})

	resp, err := eng.Launch(ctx, core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Agent: "auditor",
	})
	if err == nil {
		t.Fatal("expected an error on a ctx-cancel signal-kill (-1)")
	}
	if resp.ExitCode != -1 {
		t.Errorf("resp.ExitCode = %d, want -1 (signal death)", resp.ExitCode)
	}
	if !errors.Is(err, core.ErrTransientBridgeFailure) {
		t.Errorf("ctx-cancel signal-kill (-1) must wrap core.ErrTransientBridgeFailure so the runner's reconcile door consults the deliverable (cycle-859 deliverable-authority); got %v", err)
	}
	if errors.Is(err, core.ErrArtifactTimeout) {
		t.Error("ctx-cancel kill must NOT wrap core.ErrArtifactTimeout (that sentinel is the 81 artifact-wait contract)")
	}
}

// TestEngineLaunch_CtxCancelNonMinusOneExit_WrapsTransient (cycle-931, the 6th rung of
// the class): a STALLED agent we cancel after injecting /exit can exit with a NON-(-1)
// code (a shell/REPL-specific kill code, not a Go signal-death -1) while the context is
// cancelled. cycle-859 gated the transient classification on code==-1, so this slipped
// through to a plain hard-FAIL — discarding a green, ship-eligible on-disk PASS audit
// (reproduced LIVE on v22.4.1). ANY cancelled-context exit is a teardown → must wrap
// core.ErrTransientBridgeFailure so the runner's reconcile door consults the deliverable.
func TestEngineLaunch_CtxCancelNonMinusOneExit_WrapsTransient(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	ctx, cancel := context.WithCancel(context.Background())

	// Our context is cancelled (as the stall-teardown does), but the driver exits with a
	// NON-(-1) code (e.g. 143 = SIGTERM) rather than a Go signal-death -1.
	runner := func(_ context.Context, name, dir string, args, env []string,
		_ io.Reader, _, _ io.Writer) (int, error) {
		cancel()
		return 143, nil
	}
	eng := NewEngine(Deps{Runner: runner, LookupEnv: mapLookup(nil)})

	_, err := eng.Launch(ctx, core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Agent: "auditor",
	})
	if !errors.Is(err, core.ErrTransientBridgeFailure) {
		t.Errorf("a cancelled-context exit with a NON-(-1) code must wrap core.ErrTransientBridgeFailure so the runner reconciles against the on-disk deliverable (cycle-931); got %v", err)
	}
}

// TestEngineLaunch_StartFailure_MinusOne_StaysPlain: a -1 with a LIVE context is
// a genuine start/launch failure (bad path, exec error), NOT a teardown — it must
// stay a plain error so it fails loud and never reconciles against stale disk.
func TestEngineLaunch_StartFailure_MinusOne_StaysPlain(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")

	runner := func(_ context.Context, name, dir string, args, env []string,
		_ io.Reader, _, _ io.Writer) (int, error) {
		return -1, nil // no ctx cancellation — a real start failure
	}
	eng := NewEngine(Deps{Runner: runner, LookupEnv: mapLookup(nil)})

	_, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Agent: "auditor",
	})
	if err == nil {
		t.Fatal("expected an error on exit -1")
	}
	if errors.Is(err, core.ErrTransientBridgeFailure) || errors.Is(err, core.ErrArtifactTimeout) {
		t.Errorf("a start-failure -1 with a live context must stay PLAIN (fail loud), not a teardown sentinel; got %v", err)
	}
}
