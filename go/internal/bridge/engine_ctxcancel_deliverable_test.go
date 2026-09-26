package bridge

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestEngineLaunch_CtxCancelSignalKill_WrapsTransient(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	ctx, cancel := context.WithCancel(context.Background())

	// Models exec.CommandContext's teardown: cancel the context (as the
	// orchestrator's phase timeout does), then report the signal-death exit
	// code -1.
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
