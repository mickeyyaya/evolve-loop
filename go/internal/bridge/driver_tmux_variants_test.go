package bridge

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// driver_tmux_variants_test.go — parity tests for codex-tmux + agy-tmux,
// ported from permission-mode-drivers.bats (T-permmode-drv.6/8) and the
// codex-tmux/agy-tmux launch-*.bats behavior. Shares the fakeTmux from
// driver_claudetmux_test.go via the generalized runTmuxCLI harness.

func runTmuxCLI(t *testing.T, fx launchFixture, cli string, tmux *fakeTmux, lookup map[string]string, extra ...string) (int, string) {
	t.Helper()
	eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(lookup)})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args(cli, extra...), nil, &stdout, &stderr)
	return code, stderr.String()
}

// --- codex-tmux -----------------------------------------------------------

func TestCodexTmux_PermissionModeRejected(t *testing.T) {
	// T-permmode-drv.6
	fx := newFixture(t, "codex-tmux", "plan")
	code, stderr := runTmuxCLI(t, fx, "codex-tmux", &fakeTmux{}, nil, "--allow-bypass")
	if code == ExitOK {
		t.Fatalf("exit = ExitOK, want non-zero rejection")
	}
	if !strings.Contains(stderr, "permission_mode") || !strings.Contains(stderr, "not supported") {
		t.Fatalf("stderr should reject permission_mode; got %q", stderr)
	}
}

func TestCodexTmux_SafetyGateRequiresBypass(t *testing.T) {
	fx := newFixture(t, "codex-tmux", "")
	code, stderr := runTmuxCLI(t, fx, "codex-tmux", &fakeTmux{}, nil)
	if code != ExitSafetyGate {
		t.Fatalf("exit = %d, want ExitSafetyGate", code)
	}
	if !strings.Contains(stderr, "safety gate: --allow-bypass is required") {
		t.Fatalf("stderr should carry the safety-gate message; got %q", stderr)
	}
}

func TestCodexTmux_SessionNameRejected(t *testing.T) {
	fx := newFixture(t, "codex-tmux", "")
	code, stderr := runTmuxCLI(t, fx, "codex-tmux", &fakeTmux{}, nil, "--allow-bypass", "--session-name=foo")
	if code != ExitBadFlags {
		t.Fatalf("exit = %d, want ExitBadFlags", code)
	}
	if !strings.Contains(stderr, "session-name") {
		t.Fatalf("stderr should reject --session-name; got %q", stderr)
	}
}

func TestCodexTmux_OpenAIKeyCostLeak(t *testing.T) {
	fx := newFixture(t, "codex-tmux", "")
	code, stderr := runTmuxCLI(t, fx, "codex-tmux", &fakeTmux{}, map[string]string{"OPENAI_API_KEY": "sk-x"}, "--allow-bypass")
	if code != ExitCostLeak {
		t.Fatalf("exit = %d, want ExitCostLeak", code)
	}
	if !strings.Contains(stderr, "OPENAI_API_KEY") {
		t.Fatalf("stderr should name OPENAI_API_KEY; got %q", stderr)
	}
}

func TestCodexTmux_LaunchCmd_ModelMapAndMarker(t *testing.T) {
	// haiku → "codex --yolo -m gpt-5.4-mini" reaches the REPL launch line.
	// --yolo is the manifest default_args entry (cycle-124 G1a: codex's
	// undocumented but parsed flag that sets approval=never AND
	// sandbox=danger-full-access at boot, defusing the per-edit-approval
	// modal that hung cycle-123 tdd). It lands FIRST per realizer order
	// (default_args before per-param scalars), then -m gpt-5.4-mini from
	// params.model_tier (tier_alias haiku → gpt-5.4-mini).
	fx := newFixture(t, "codex-tmux", "") // profile model=haiku
	tmux := &fakeTmux{}                   // no marker → REPL boot times out, but launchCmd already sent
	runTmuxCLI(t, fx, "codex-tmux", tmux, nil, "--allow-bypass")
	if !tmux.sentContains("codex --yolo -m gpt-5.4-mini") {
		t.Fatalf("codex-tmux launch should map haiku→gpt-5.4-mini with --yolo prefix; sentKeys=%v", tmux.sentKeys)
	}
}

func TestCodexTmux_HappyPath(t *testing.T) {
	fx := newFixture(t, "codex-tmux", "")
	if err := os.WriteFile(fx.artifact, []byte("DONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &fakeTmux{paneSeq: []string{"›"}} // codex marker
	code, stderr := runTmuxCLI(t, fx, "codex-tmux", tmux, nil, "--allow-bypass")
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr)
	}
	if !tmux.sentContains("/quit") {
		t.Fatalf("codex-tmux should exit via /quit; sentKeys=%v", tmux.sentKeys)
	}
}

// --- agy-tmux -------------------------------------------------------------

func TestAgyTmux_PermissionModeRejected(t *testing.T) {
	// T-permmode-drv.8
	fx := newFixture(t, "agy-tmux", "plan")
	code, stderr := runTmuxCLI(t, fx, "agy-tmux", &fakeTmux{}, nil, "--allow-bypass")
	if code == ExitOK {
		t.Fatalf("exit = ExitOK, want non-zero rejection")
	}
	if !strings.Contains(stderr, "permission_mode") || !strings.Contains(stderr, "not supported") {
		t.Fatalf("stderr should reject permission_mode; got %q", stderr)
	}
}

func TestAgyTmux_SafetyGateRequiresBypass(t *testing.T) {
	fx := newFixture(t, "agy-tmux", "")
	code, stderr := runTmuxCLI(t, fx, "agy-tmux", &fakeTmux{}, nil)
	if code != ExitSafetyGate {
		t.Fatalf("exit = %d, want ExitSafetyGate", code)
	}
	if !strings.Contains(stderr, "safety gate: --allow-bypass is required") {
		t.Fatalf("stderr should carry the safety-gate message; got %q", stderr)
	}
}

func TestAgyTmux_LaunchCmd(t *testing.T) {
	fx := newFixture(t, "agy-tmux", "")
	tmux := &fakeTmux{}
	runTmuxCLI(t, fx, "agy-tmux", tmux, nil, "--allow-bypass")
	// agy 1.0.15 selects its model via the --model launch flag (cycle-447
	// probe); the tokens are display names, shell-quoted by launchCmdLine.
	// 1.0.3 had no model flag at all and `-m` remains undefined — see
	// docs/incidents/cycle-154-agy-tmux-m-flag-repl-boot-timeout.md.
	if !tmux.sentContains("--model") {
		t.Fatalf("agy-tmux launch cmd should carry --model (agy 1.0.15); sentKeys=%v", tmux.sentKeys)
	}
	if !tmux.sentContains("--dangerously-skip-permissions") {
		t.Fatalf("agy-tmux launch cmd should carry the permission flag; sentKeys=%v", tmux.sentKeys)
	}
	if tmux.sentContains(" -m ") {
		t.Fatalf("agy-tmux must NOT emit the undefined -m short flag; sentKeys=%v", tmux.sentKeys)
	}
}

func TestAgyTmux_HappyPath(t *testing.T) {
	fx := newFixture(t, "agy-tmux", "")
	if err := os.WriteFile(fx.artifact, []byte("DONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &fakeTmux{paneSeq: []string{"? for shortcuts"}}
	code, stderr := runTmuxCLI(t, fx, "agy-tmux", tmux, nil, "--allow-bypass")
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr)
	}
	if !tmux.sentContains("C-c") {
		t.Fatalf("agy-tmux should exit via Ctrl+C; sentKeys=%v", tmux.sentKeys)
	}
}
