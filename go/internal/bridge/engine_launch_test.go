package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// engine_launch_test.go — tests for the core.Bridge entry Engine.Launch
// (the in-process path the M7 adapter cutover routes to): BridgeRequest →
// LaunchArgs pipeline, prompt materialization, ExtraFlags pass-through,
// and artifact-into-response.

func TestEngineLaunch_ClaudeP_MapsToPipeline(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "ARTIFACT-OK\n"}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto",
		Prompt: "do the thing", Workspace: ws, ArtifactPath: artifact,
		Agent: "scout", ExtraFlags: []string{"--bare", "--strict-mcp-config"},
	})
	if err != nil {
		t.Fatalf("Launch err: %v", err)
	}
	if resp.ExitCode != ExitOK {
		t.Fatalf("exit = %d, want 0", resp.ExitCode)
	}
	if !strings.Contains(resp.Stdout, "ARTIFACT-OK") {
		t.Fatalf("resp.Stdout should be the artifact content; got %q", resp.Stdout)
	}
	if _, err := os.Stat(filepath.Join(ws, "scout-prompt.txt")); err != nil {
		t.Fatalf("prompt was not materialized to <agent>-prompt.txt: %v", err)
	}
	if len(fr.calls) == 0 {
		t.Fatal("driver did not invoke the inner CLI")
	}
	joined := strings.Join(fr.calls[0].args, " ")
	if !strings.Contains(joined, "--bare") || !strings.Contains(joined, "--strict-mcp-config") {
		t.Fatalf("ExtraFlags not forwarded into the inner argv; args=%v", fr.calls[0].args)
	}
}

func TestEngineLaunch_MissingCLI_Errors(t *testing.T) {
	_, err := NewEngine(Deps{}).Launch(context.Background(), core.BridgeRequest{
		Profile: "p", Workspace: "w", ArtifactPath: "a",
	})
	if err == nil {
		t.Fatal("expected error when CLI is missing")
	}
}

func TestEngineLaunch_NonZeroExit_Errors(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	resp, err := NewEngine(Deps{LookupEnv: mapLookup(nil)}).Launch(context.Background(), core.BridgeRequest{
		CLI: "nope", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"),
	})
	if err == nil {
		t.Fatal("expected error on non-zero bridge exit (unknown cli)")
	}
	if resp.ExitCode == ExitOK {
		t.Fatalf("exit should be non-zero; got %d", resp.ExitCode)
	}
}
