package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
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

// TestLaunchFailurePersistsLaunchErrorFile — R3.6 acceptance (inbox
// bridge-launch-validation-stderr-lost): a launch dying in the validate
// gauntlet (here: LoadProfile on a missing profile, the cycle-270 shape)
// must leave the cause string in the run dir as <agent>-launch-error.txt,
// because the failure precedes per-agent stderr-log creation.
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
	// The orchestrator's bridgeExitCode digit-scan reads the number right
	// after "launch exit=" — the appended cause must not break that linkage.
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

// TestFirstDiagnosticLine_PrefersCausalLine — cycle-286 field evidence: a
// driver-timeout stderr starts with chatter ("[claude-tmux] NOTE: …") and
// ends with the causal line; the gauntlet failures put the cause first
// ("[bridge] …"). The picker must prefer a [bridge]-prefixed line, else the
// LAST non-empty line — never leading chatter.
func TestFirstDiagnosticLine_PrefersCausalLine(t *testing.T) {
	tests := []struct {
		name, stderr, want string
	}{
		{
			name:   "gauntlet cause first",
			stderr: "[bridge] invalid --permission-mode value: 'x'\n[bridge] valid: plan, default\n",
			want:   "[bridge] invalid --permission-mode value: 'x'",
		},
		{
			name: "driver chatter then causal tail",
			stderr: "[claude-tmux] NOTE: stream_output=true is no-op for this driver\n" +
				"[claude-tmux] session=evolve-bridge-c286-scout\n" +
				"[claude-tmux] prompt delivered\n" +
				"[claude-tmux] FAIL: completion never signalled\n",
			want: "[claude-tmux] FAIL: completion never signalled",
		},
		{
			name:   "single line",
			stderr: "[bridge] bridge:profile: file not found\n",
			want:   "[bridge] bridge:profile: file not found",
		},
		{
			name:   "non-bridge preamble before the bridge cause",
			stderr: "some launcher preamble\n[bridge] the cause\ntrailing chatter\n",
			want:   "[bridge] the cause",
		},
		{
			name:   "empty",
			stderr: "\n\n",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstDiagnosticLine(tt.stderr); got != tt.want {
				t.Errorf("firstDiagnosticLine = %q, want %q", got, tt.want)
			}
		})
	}
}
