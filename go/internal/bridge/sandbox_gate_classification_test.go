package bridge

// Required confinement needs an applied wrapper; a nesting marker is not evidence.

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

func TestSandboxGate_NestedSessionDoesNotSatisfyRequirement(t *testing.T) {
	deps := Deps{Env: map[string]string{"CLAUDECODE": "1"}}
	if !sandboxRequiredButUnavailable(deps, gateCfg(), false) {
		t.Fatal("unverified outer confinement must fail a mandatory requirement")
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

func TestSandboxGate_NamedSessionCannotProveRequestedPolicy(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	writeJSON(t, fx.artifact, "done")
	tmux := &fakeTmux{existing: map[string]bool{"evolve-bridge-named-previous-policy": true}}
	code, se := runTmux(t, fx, tmux, nil, "--allow-bypass", "--session-name=previous-policy", "--require-sandbox")
	if code != ExitSafetyGate || !strings.Contains(se, "existing named session") {
		t.Fatalf("unverified reused session: code=%d stderr=%q; want safety gate with policy diagnostic", code, se)
	}
	if len(tmux.sentKeys) != 0 {
		t.Fatalf("unverified session received keystrokes: %v", tmux.sentKeys)
	}
}

// The default tmux driver must enforce the same mandatory boundary as headless.
func TestSandboxGate_TmuxDriverRejectsUnverifiedNestedConfinement(t *testing.T) {
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
	if code != ExitSafetyGate {
		t.Fatalf("nested mandatory launch code=%d, want ExitSafetyGate", code)
	}
}
