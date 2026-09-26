package bridge

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestEngineLaunch_CmdTimeout124_WrapsTransient(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	fr := &fakeRunner{exit: ExitCmdTimeout}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Agent: "build-planner",
	})
	if err == nil {
		t.Fatal("expected an error on exit 124")
	}
	if resp.ExitCode != ExitCmdTimeout {
		t.Errorf("resp.ExitCode = %d, want %d", resp.ExitCode, ExitCmdTimeout)
	}
	if !errors.Is(err, core.ErrTransientBridgeFailure) {
		t.Errorf("exit-124 error must wrap core.ErrTransientBridgeFailure (infra weather, sibling of 81); got %v", err)
	}
	if errors.Is(err, core.ErrArtifactTimeout) {
		t.Error("exit-124 must NOT wrap core.ErrArtifactTimeout (that sentinel is the 81 artifact-wait contract)")
	}
}

func TestEngineLaunch_MissingBinary127_StaysPlain(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	fr := &fakeRunner{exit: ExitMissingBinary}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	_, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Agent: "build-planner",
	})
	if err == nil {
		t.Fatal("expected an error on exit 127")
	}
	if errors.Is(err, core.ErrTransientBridgeFailure) || errors.Is(err, core.ErrArtifactTimeout) {
		t.Errorf("exit-127 must stay a plain error (missing binary is an env defect, not weather); got %v", err)
	}
}
