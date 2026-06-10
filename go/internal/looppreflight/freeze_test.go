package looppreflight

// freeze_test.go — ADR-0044 C5 (Slice 3) RED tests: the CLI-version-freeze
// readiness check (Specification pattern).
//
// cycle-262 D6: codex self-upgraded its own binary mid-phase (its updater ran
// `brew upgrade` on launch, printed "Update ran successfully! Please restart
// Codex.", and exited the REPL to a bare shell). The host fix was `brew pin
// codex` — a CONVERGENT STEADY STATE, not a per-cycle toggle (an unpin-on-exit
// would leak on any SIGKILL/OOM/reboot). This check makes the steady state a
// verified precondition: a *-tmux CLI with self-update evidence on the host
// must be pinned or the batch Halts with the exact pin guidance.
//
// Scope: interactive *-tmux drivers only — the incident vector is the TUI
// launch path (headless `codex exec` does not trigger the updater). Probes are
// read-only (stat an evidence file; list brew pins), so the check is
// idempotent by construction; the pin itself is the operator's one-time
// convergent action.

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// freezeOptions returns goodPipelineOptions with a codex-tmux profile in use
// and scripted freeze seams.
func freezeOptions(t *testing.T, evidence map[string]string, pinned []string, pinErr error) Options {
	t.Helper()
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "codex-tmux", CLIFallback: []string{"claude-tmux"}}, nil
	}
	opts.SelfUpdateEvidence = func(bin string) (bool, string, error) {
		ev, ok := evidence[bin]
		return ok, ev, nil
	}
	opts.PinnedLister = func() ([]string, error) { return pinned, pinErr }
	return opts
}

// Evidence that cannot be verified (home dir unresolvable, IO error) is
// ambiguity: WARN with manual guidance, never a silent pass (fail loudly) and
// never a false Halt.
func TestRun_VersionFreeze_EvidenceProbeError_Warns(t *testing.T) {
	opts := freezeOptions(t, nil, nil, nil)
	opts.SelfUpdateEvidence = func(bin string) (bool, string, error) {
		return false, "", errors.New("user home dir unresolvable")
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-freeze")
	if c.Level != LevelWarn {
		t.Fatalf("unverifiable evidence is ambiguity → WARN; got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Detail, "unverifiable") {
		t.Errorf("detail must surface the unverifiable trail; got %q", c.Detail)
	}
}

func TestRun_VersionFreeze_AutoUpdateUnpinned_Halts(t *testing.T) {
	opts := freezeOptions(t, map[string]string{"codex": "~/.codex/version.json present (updater state)"}, nil, nil)
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-freeze")
	if c.Level != LevelHalt {
		t.Fatalf("self-updating tmux CLI without a pin must HALT the batch (cycle-262 D6); got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Detail, "brew pin codex") {
		t.Errorf("halt detail must carry the exact convergent-action guidance; got %q", c.Detail)
	}
	if !r.Halted() {
		t.Errorf("overall verdict must halt")
	}
}

func TestRun_VersionFreeze_Pinned_Passes(t *testing.T) {
	opts := freezeOptions(t, map[string]string{"codex": "~/.codex/version.json present"}, []string{"codex"}, nil)
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-freeze")
	if c.Level != LevelPass {
		t.Fatalf("pinned self-updating CLI is the convergent steady state — must pass; got %s (%s)", c.Level, c.Detail)
	}
}

func TestRun_VersionFreeze_NoEvidence_Passes(t *testing.T) {
	opts := freezeOptions(t, map[string]string{}, nil, nil)
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c := findCheck(t, r, "cli-version-freeze"); c.Level != LevelPass {
		t.Fatalf("no self-update evidence → nothing to freeze → pass; got %s (%s)", c.Level, c.Detail)
	}
}

// Headless-only usage is out of scope: the incident vector is the interactive
// TUI launch (headless `codex exec` does not run the updater).
func TestRun_VersionFreeze_HeadlessOnly_NotChecked(t *testing.T) {
	opts := freezeOptions(t, map[string]string{"codex": "evidence"}, nil, nil)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "codex"}, nil // headless driver, no -tmux
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c := findCheck(t, r, "cli-version-freeze"); c.Level != LevelPass {
		t.Fatalf("headless-only codex must not be freeze-checked; got %s (%s)", c.Level, c.Detail)
	}
}

// A pin-listing failure (brew absent, exec error) is ambiguity, not confirmed
// risk: WARN with manual guidance, never a false Halt (same fail-open posture
// as the eval gate).
func TestRun_VersionFreeze_PinProbeError_Warns(t *testing.T) {
	opts := freezeOptions(t, map[string]string{"codex": "evidence"}, nil, errors.New("brew: command not found"))
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-freeze")
	if c.Level != LevelWarn {
		t.Fatalf("pin-probe failure is ambiguity → WARN, not halt/pass; got %s (%s)", c.Level, c.Detail)
	}
}

// Idempotent / crash-safe: probes are read-only, so two Runs over the same
// host state produce the identical verdict (the Specification re-evaluates;
// nothing toggles).
func TestRun_VersionFreeze_Idempotent(t *testing.T) {
	opts := freezeOptions(t, map[string]string{"codex": "evidence"}, []string{"codex"}, nil)
	r1, err := Run(opts)
	if err != nil {
		t.Fatalf("Run 1: %v", err)
	}
	r2, err := Run(opts)
	if err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	c1, c2 := findCheck(t, r1, "cli-version-freeze"), findCheck(t, r2, "cli-version-freeze")
	if c1.Level != c2.Level || c1.Message != c2.Message {
		t.Fatalf("freeze check must be idempotent; run1=(%s,%q) run2=(%s,%q)", c1.Level, c1.Message, c2.Level, c2.Message)
	}
}

// Tests for the real default* implementations in freeze.go.

func TestDefaultSelfUpdateEvidence_NonCodex(t *testing.T) {
	ok, evidence, err := defaultSelfUpdateEvidence("claude")
	if err != nil {
		t.Fatalf("unexpected error for non-codex binary: %v", err)
	}
	if ok {
		t.Fatal("non-codex binary must not have self-update evidence")
	}
	if evidence != "" {
		t.Fatalf("non-codex evidence must be empty; got %q", evidence)
	}
}

func TestDefaultSelfUpdateEvidence_Codex(t *testing.T) {
	// Smoke: result depends on whether ~/.codex/version.json exists on this host.
	_, _, err := defaultSelfUpdateEvidence("codex")
	if err != nil {
		t.Logf("defaultSelfUpdateEvidence(codex): %v (expected only if home dir unresolvable)", err)
	}
}

func TestDefaultPinnedLister_Smoke(t *testing.T) {
	// brew may be absent; error is acceptable — just verify no panic.
	_, _ = defaultPinnedLister()
}
