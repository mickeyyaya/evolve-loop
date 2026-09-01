package bridge

// sandbox_gate_classification_test.go — the Build-explanation contract's
// fail-closed sandbox gate must distinguish WHY a launch is unwrapped.
//
// ShouldWrap declines to wrap for three different KINDS of reason, and the
// first draft of sandboxRequiredButUnavailable treated all of them as
// violations:
//
//  1. nested LLM-CLI session — the OUTER sandbox + Tier-1 hooks already
//     confine, and on macOS an inner sandbox-exec EPERM-hangs the REPL. The
//     requirement is SATISFIED here, not violated; failing closed makes the
//     contract unrunnable inside every Claude-driven session (this repo's own
//     e2e runs included — observed as exit=2 on every pipeline e2e test).
//  2. explicit EVOLVE_SANDBOX=off — a host operator opt-out, the same posture
//     as --human-input's host opt-in that this exit code was built for.
//     Honoured loudly, never silently.
//  3. sandbox genuinely unavailable under auto/on — the case the gate exists
//     for. Fails closed, unchanged.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func gateCfg() *Config {
	return &Config{Agent: "build", RequireSandbox: true}
}

func TestSandboxGate_NestedSessionSatisfiesTheRequirement(t *testing.T) {
	deps := Deps{Env: map[string]string{"CLAUDECODE": "1"}}
	if sandboxRequiredButUnavailable(deps, gateCfg(), false) {
		t.Fatal("nested LLM-CLI session: the OUTER sandbox already confines — the requirement is satisfied, and failing closed here makes the contract unrunnable in every nested environment")
	}
}

func TestSandboxGate_ExplicitOffIsAHostOptOutAndWarnsLoudly(t *testing.T) {
	var errBuf strings.Builder
	deps := Deps{Env: map[string]string{"EVOLVE_SANDBOX": "off"}, Stderr: &errBuf}
	if sandboxRequiredButUnavailable(deps, gateCfg(), false) {
		t.Fatal("EVOLVE_SANDBOX=off is an explicit host opt-out — same posture as --human-input's host opt-in; it must be honoured")
	}
	if !strings.Contains(strings.ToLower(errBuf.String()), "unconfined") {
		t.Errorf("the opt-out must be LOUD — stderr %q says nothing about running unconfined", errBuf.String())
	}
}

func TestSandboxGate_UnavailableUnderAutoStillFailsClosed(t *testing.T) {
	deps := Deps{Env: map[string]string{}}
	if !sandboxRequiredButUnavailable(deps, gateCfg(), false) {
		t.Fatal("not nested, not opted out, wrap unavailable: THIS is the violation the gate exists for — it must fail closed")
	}
}

func TestSandboxGate_WrappedNeverGates(t *testing.T) {
	deps := Deps{Env: map[string]string{}}
	if sandboxRequiredButUnavailable(deps, gateCfg(), true) {
		t.Fatal("a wrapped launch satisfied the requirement outright")
	}
}

func TestSandboxGate_NoRequirementNeverGates(t *testing.T) {
	deps := Deps{Env: map[string]string{}}
	if sandboxRequiredButUnavailable(deps, &Config{Agent: "build"}, false) {
		t.Fatal("RequireSandbox=false must never gate")
	}
	if sandboxRequiredButUnavailable(deps, nil, false) {
		t.Fatal("nil cfg must never gate")
	}
}

// CRITICAL from adversarial review, reproduced through the real driver: the
// classified gate was wired into the three HEADLESS drivers while
// driver_tmux_repl.go — the shared engine behind claude-tmux/codex-tmux/
// agy-tmux, the DOCUMENTED DEFAULT execution mode — kept the pre-fix
// unconditional `else if cfg.RequireSandbox`. A nested Claude session driving
// a contract-active build through any tmux driver still died ExitSafetyGate:
// the exact bug this classification exists to fix, alive on the path
// production actually uses. The e2e suite never saw it because every pipeline
// e2e drives the headless claude-p path.
func TestSandboxGate_TmuxDriverHonoursNestedSatisfaction(t *testing.T) {
	ws := t.TempDir()
	cfg := paneLiveCfg(t, ws)
	cfg.RequireSandbox = true
	deps := covDeps()
	deps.Env = map[string]string{"CLAUDECODE": "1"} // nested session — outer sandbox confines
	tmux := &flakyPaneTmux{pane: "❯ hi\n\n❯\n"}
	deps.Tmux = tmux
	tick := 0
	deps.Sleep = func(d time.Duration) {
		if d != 2*time.Second {
			return
		}
		tick++
		if tick == 2 {
			_ = os.WriteFile(cfg.Artifact, []byte("done"), 0o644)
		}
	}
	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
	code, _ := runTmuxREPL(context.Background(), cfg, deps, lp)
	if code == ExitSafetyGate {
		t.Fatalf("nested tmux launch died at the safety gate — the DEFAULT driver path still runs the unclassified RequireSandbox check")
	}
	if code != ExitOK {
		t.Fatalf("code=%d, want ExitOK", code)
	}
}
