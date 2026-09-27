package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestLaunchErrorIncludesCapturedStderrCause(t *testing.T) {
	ws := t.TempDir()
	req := core.BridgeRequest{
		CLI:            "claude-p",
		Profile:        writeProfile(t, ws, "stderr-cause", "bypassPermissions"),
		Model:          "auto",
		Prompt:         "produce an artifact",
		Workspace:      ws,
		ArtifactPath:   filepath.Join(ws, "artifact.md"),
		PermissionMode: "definitely-invalid",
	}

	resp, err := NewEngine(Deps{}).Launch(context.Background(), req)
	if err == nil {
		t.Fatal("Launch returned nil error for invalid permission mode")
	}
	if resp.ExitCode != ExitBadFlags {
		t.Fatalf("ExitCode=%d, want %d", resp.ExitCode, ExitBadFlags)
	}
	const want = "invalid --permission-mode value"
	if !strings.Contains(resp.Stderr, want) {
		t.Fatalf("response stderr missing diagnostic %q:\n%s", want, resp.Stderr)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("returned error lost stderr diagnostic %q:\nerr=%v\nstderr=%s", want, err, resp.Stderr)
	}
}

func TestLaunchFailurePersistsLaunchErrorFile(t *testing.T) {
	ws := t.TempDir()
	req := core.BridgeRequest{
		CLI:          "claude-p",
		Agent:        "debugger",
		Profile:      filepath.Join(ws, "no-such-profile.json"),
		Model:        "auto",
		Prompt:       "produce an artifact",
		Workspace:    ws,
		ArtifactPath: filepath.Join(ws, "artifact.md"),
	}

	resp, err := NewEngine(Deps{}).Launch(context.Background(), req)
	if err == nil {
		t.Fatal("Launch returned nil error for missing profile")
	}
	if resp.ExitCode != ExitBadFlags {
		t.Fatalf("ExitCode=%d, want %d", resp.ExitCode, ExitBadFlags)
	}
	if !strings.Contains(err.Error(), "[bridge]") {
		t.Errorf("returned error lost the [bridge] diagnostic: %v", err)
	}
	if !strings.Contains(err.Error(), "launch exit=10:") {
		t.Errorf("error must keep the parseable exit code before the cause: %v", err)
	}
	data, rerr := os.ReadFile(filepath.Join(ws, "debugger-launch-error.txt"))
	if rerr != nil {
		t.Fatalf("launch-error file not persisted: %v", rerr)
	}
	if !strings.Contains(string(data), "[bridge]") {
		t.Errorf("launch-error file missing diagnostic:\n%s", data)
	}
}
