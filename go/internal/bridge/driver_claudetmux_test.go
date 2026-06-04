package bridge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// driver_claudetmux_test.go — parity tests for the claude-tmux driver.
// Ports the claude-tmux cases of permission-mode-drivers.bats
// (T-permmode-drv.2/3/4/11/12) plus the REPL state-machine outcomes
// (happy path, REPL-boot timeout, artifact timeout). A scriptable
// fakeTmux replaces real tmux, and a no-op Sleep makes the poll loops
// iterate instantly — so behaviors the bash BATS could only assert via a
// 6s timeout are deterministic here.

type fakeTmux struct {
	existing      map[string]bool
	sentKeys      []string // recorded SendKeys (keys only)
	sentSeq       []string // recorded SendKeys as "keys|enter", preserving order
	paneSeq       []string // CapturePane returns, consumed in order; last value repeats
	paneIdx       int
	newSessErr    error    // when set, NewSession fails (covers the spawn-error path)
	pipePaneCalls []string // recorded shellCmd args from PipePane (empty string = stop form)
}

func (f *fakeTmux) HasSession(_ context.Context, name string) bool { return f.existing[name] }

func (f *fakeTmux) NewSession(_ context.Context, name string, _, _ int) error {
	if f.newSessErr != nil {
		return f.newSessErr
	}
	if f.existing == nil {
		f.existing = map[string]bool{}
	}
	f.existing[name] = true
	return nil
}

func (f *fakeTmux) SendKeys(_ context.Context, _, keys string, enter bool) error {
	f.sentKeys = append(f.sentKeys, keys)
	f.sentSeq = append(f.sentSeq, fmt.Sprintf("%s|%v", keys, enter))
	return nil
}

func (f *fakeTmux) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	if len(f.paneSeq) == 0 {
		return "", nil
	}
	if f.paneIdx < len(f.paneSeq) {
		v := f.paneSeq[f.paneIdx]
		f.paneIdx++
		return v, nil
	}
	return f.paneSeq[len(f.paneSeq)-1], nil
}

func (f *fakeTmux) LoadBuffer(_ context.Context, _, _ string) error { return nil }
func (f *fakeTmux) PasteBuffer(_ context.Context, _ string) error   { return nil }
func (f *fakeTmux) KillSession(_ context.Context, name string) error {
	delete(f.existing, name)
	return nil
}
func (f *fakeTmux) PipePane(_ context.Context, _ string, shellCmd string) error {
	f.pipePaneCalls = append(f.pipePaneCalls, shellCmd)
	return nil
}

func (f *fakeTmux) sentContains(sub string) bool {
	for _, k := range f.sentKeys {
		if strings.Contains(k, sub) {
			return true
		}
	}
	return false
}

// runTmux drives a claude-tmux launch with the fake tmux + no-op sleep +
// controlled env.
func runTmux(t *testing.T, fx launchFixture, tmux *fakeTmux, lookup map[string]string, extra ...string) (int, string) {
	t.Helper()
	eng := NewEngine(Deps{
		Tmux:      tmux,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(lookup),
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", extra...), nil, &stdout, &stderr)
	return code, stderr.String()
}

// --- safety gate + permission-mode (permission-mode-drivers.bats) ---------

func TestClaudeTmux_SafetyGate_RequiresAllowBypass(t *testing.T) {
	// T-permmode-drv.4: no permission_mode + no --allow-bypass → gate fires.
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{}
	code, stderr := runTmux(t, fx, tmux, nil)
	if code != ExitSafetyGate {
		t.Fatalf("exit = %d, want %d (ExitSafetyGate)", code, ExitSafetyGate)
	}
	if !strings.Contains(stderr, "safety gate: --allow-bypass is required") {
		t.Fatalf("stderr should carry the safety-gate message; got %q", stderr)
	}
	if len(tmux.sentKeys) != 0 {
		t.Fatalf("driver must not touch tmux when the safety gate fires")
	}
}

func TestClaudeTmux_PermissionModeRelaxesGate(t *testing.T) {
	// T-permmode-drv.2: permission_mode set → no --allow-bypass needed.
	fx := newFixture(t, "claude-tmux", "plan")
	code, stderr := runTmux(t, fx, &fakeTmux{}, nil)
	if strings.Contains(stderr, "safety gate: --allow-bypass is required") {
		t.Fatalf("safety gate must NOT fire when permission_mode is set; got %q", stderr)
	}
	// rc is REPL-boot-timeout here (no marker) — the gate behavior is what
	// this test pins, mirroring the BATS which kill the run at 6s.
	if code == ExitSafetyGate {
		t.Fatalf("must not return ExitSafetyGate with permission_mode set")
	}
}

func TestClaudeTmux_ClaudeCmd_PermissionModePlan(t *testing.T) {
	// T-permmode-drv.3: claude_cmd carries --permission-mode plan and NOT
	// --dangerously-skip-permissions.
	fx := newFixture(t, "claude-tmux", "plan")
	tmux := &fakeTmux{}
	runTmux(t, fx, tmux, nil)
	if !tmux.sentContains("claude --model haiku --permission-mode plan") {
		t.Fatalf("claude_cmd should use --permission-mode plan; sentKeys=%v", tmux.sentKeys)
	}
	if tmux.sentContains("--dangerously-skip-permissions") {
		t.Fatalf("claude_cmd must NOT use --dangerously-skip-permissions in plan mode; sentKeys=%v", tmux.sentKeys)
	}
}

func TestClaudeTmux_ClaudeCmd_BypassWhenNoPermissionMode(t *testing.T) {
	// T-permmode-drv.11: --allow-bypass + no permission_mode → bypass flag,
	// NOT --permission-mode.
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{}
	runTmux(t, fx, tmux, nil, "--allow-bypass")
	if !tmux.sentContains("--dangerously-skip-permissions") {
		t.Fatalf("claude_cmd should use --dangerously-skip-permissions; sentKeys=%v", tmux.sentKeys)
	}
	if tmux.sentContains("--permission-mode") {
		t.Fatalf("claude_cmd must NOT use --permission-mode here; sentKeys=%v", tmux.sentKeys)
	}
}

func TestClaudeTmux_ClaudeCmd_PlanWinsOverBypass(t *testing.T) {
	// T-permmode-drv.12: --allow-bypass + permission_mode=plan → plan wins.
	fx := newFixture(t, "claude-tmux", "plan")
	tmux := &fakeTmux{}
	runTmux(t, fx, tmux, nil, "--allow-bypass")
	if !tmux.sentContains("--permission-mode plan") {
		t.Fatalf("plan must win over bypass; sentKeys=%v", tmux.sentKeys)
	}
	if tmux.sentContains("--dangerously-skip-permissions") {
		t.Fatalf("bypass must be dropped when permission_mode is set; sentKeys=%v", tmux.sentKeys)
	}
}

func TestClaudeTmux_CostLeak_AnthropicAPIKey(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan") // gate relaxed so we reach the cost guard
	tmux := &fakeTmux{}
	code, stderr := runTmux(t, fx, tmux, map[string]string{"ANTHROPIC_API_KEY": "sk-x"})
	if code != ExitCostLeak {
		t.Fatalf("exit = %d, want ExitCostLeak", code)
	}
	if !strings.Contains(stderr, "ANTHROPIC_API_KEY") {
		t.Fatalf("stderr should name ANTHROPIC_API_KEY; got %q", stderr)
	}
	if len(tmux.sentKeys) != 0 {
		t.Fatalf("driver must not touch tmux on a credential leak")
	}
}

// --- REPL state-machine outcomes ------------------------------------------

func TestClaudeTmux_HappyPath_ArtifactAppears(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// Pre-create the artifact so the wait loop exits on its first check.
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // REPL boots immediately
	code, stderr := runTmux(t, fx, tmux, nil, "--allow-bypass")
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr)
	}
	if !tmux.sentContains("/exit") {
		t.Fatalf("ephemeral session should be closed with /exit; sentKeys=%v", tmux.sentKeys)
	}
}

func TestClaudeTmux_REPLBootTimeout(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{} // CapturePane always "" → marker never seen
	code, _ := runTmux(t, fx, tmux, nil, "--allow-bypass")
	if code != ExitREPLBootTimeout {
		t.Fatalf("exit = %d, want %d (ExitREPLBootTimeout)", code, ExitREPLBootTimeout)
	}
}

func TestClaudeTmux_ArtifactTimeout(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// REPL boots, but the artifact never appears → ExitArtifactTimeout.
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	code, _ := runTmux(t, fx, tmux, nil, "--allow-bypass")
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout)", code, ExitArtifactTimeout)
	}
}
