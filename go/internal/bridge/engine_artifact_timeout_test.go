package bridge

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestEngineLaunch_ArtifactTimeout_WrapsSentinel(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	fr := &fakeRunner{exit: ExitArtifactTimeout}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Agent: "build-planner",
	})
	if err == nil {
		t.Fatal("expected an error on exit 81")
	}
	if resp.ExitCode != ExitArtifactTimeout {
		t.Errorf("resp.ExitCode = %d, want %d", resp.ExitCode, ExitArtifactTimeout)
	}
	if !errors.Is(err, core.ErrArtifactTimeout) {
		t.Errorf("exit-81 error must wrap core.ErrArtifactTimeout; got %v", err)
	}
}

func TestEngineLaunch_OtherExit_NoSentinel(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	fr := &fakeRunner{exit: ExitSafetyGate}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	_, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Agent: "build-planner",
	})
	if err == nil {
		t.Fatal("expected an error on exit 2")
	}
	if errors.Is(err, core.ErrArtifactTimeout) {
		t.Error("a non-81 exit must NOT wrap core.ErrArtifactTimeout")
	}
}
