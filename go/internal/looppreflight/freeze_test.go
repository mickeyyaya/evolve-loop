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
	"os"
	"path/filepath"
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

// TestDefaultSelfUpdateEvidence_Unregistered pins the registry default: a CLI
// with no self-update entry (here "agy") has no evidence and no error. (This
// test previously probed "claude"; cycle-297 adds claude to the registry, so
// the non-registered example moved to a binary that genuinely has no entry —
// otherwise it would now collide with the real claude evidence on a host that
// has ~/.claude/settings.json.)
func TestDefaultSelfUpdateEvidence_Unregistered(t *testing.T) {
	ok, evidence, err := defaultSelfUpdateEvidence("agy")
	if err != nil {
		t.Fatalf("unexpected error for unregistered binary: %v", err)
	}
	if ok {
		t.Fatal("unregistered binary must not have self-update evidence")
	}
	if evidence != "" {
		t.Fatalf("unregistered evidence must be empty; got %q", evidence)
	}
}

// TestDefaultSelfUpdateEvidence_ClaudePresent is the load-bearing RED test for
// cycle-297 Task 2 (claude-cli-version-freeze inbox HIGH). claude 2.1.173
// self-updated mid-soak (removed the `esc to interrupt` affordance), breaking
// PaneBusy detection and causing exit=81 in cycles 286/288/289/291. The freeze
// registry must recognize claude as self-updating via its updater state file
// ~/.claude/settings.json, exactly as it recognizes codex via
// ~/.codex/version.json. HOME is redirected to a temp dir with the file present
// so the assertion is deterministic and host-independent. RED baseline:
// defaultSelfUpdateEvidence("claude") returns (false,"",nil) because the
// function only handles "codex" — this test fails until the claude case lands.
func TestDefaultSelfUpdateEvidence_ClaudePresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, evidence, err := defaultSelfUpdateEvidence("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("claude with ~/.claude/settings.json present must report self-update evidence")
	}
	if !strings.Contains(evidence, settings) {
		t.Errorf("evidence must name the settings.json path; got %q", evidence)
	}
}

// TestDefaultSelfUpdateEvidence_ClaudeAbsent is the negative (anti-no-op) axis:
// with no ~/.claude/settings.json the claude case must report NO evidence (and
// no error) — absence of the updater state means nothing to freeze, exactly
// like the codex no-state path. HOME is redirected to an empty temp dir.
func TestDefaultSelfUpdateEvidence_ClaudeAbsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // empty home: no ~/.claude/settings.json
	ok, evidence, err := defaultSelfUpdateEvidence("claude")
	if err != nil {
		t.Fatalf("absent settings file is not ambiguity; want no error, got %v", err)
	}
	if ok {
		t.Fatal("claude without ~/.claude/settings.json must report no self-update evidence")
	}
	if evidence != "" {
		t.Fatalf("absent-evidence detail must be empty; got %q", evidence)
	}
}

// TestRun_VersionFreeze_ClaudeUnpinnedRealEvidence_Halts is the end-to-end RED
// test: it wires the REAL defaultSelfUpdateEvidence (not the injected stub)
// behind a claude-tmux profile with HOME holding ~/.claude/settings.json and an
// empty pin list. The Specification risky(claude) ∧ tmuxDriven(claude) ∧
// ¬pinned(claude) must HALT with the exact `brew pin claude` convergent action.
// RED baseline: the real evidence function ignores claude → no risky binary →
// LevelPass, so this fails until the claude registry case lands.
func TestRun_VersionFreeze_ClaudeUnpinnedRealEvidence_Halts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux"}, nil
	}
	opts.SelfUpdateEvidence = defaultSelfUpdateEvidence // exercise the REAL registry
	opts.PinnedLister = func() ([]string, error) { return nil, nil }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-freeze")
	if c.Level != LevelHalt {
		t.Fatalf("self-updating claude-tmux without a pin must HALT (claude 2.1.173 broke 4 soak cycles); got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Detail, "brew pin claude") {
		t.Errorf("halt detail must carry the exact convergent-action guidance for claude; got %q", c.Detail)
	}
	if !r.Halted() {
		t.Errorf("overall verdict must halt")
	}
}

// TestRun_VersionFreeze_ClaudeNoSettings_Passes is the end-to-end negative axis
// of the real-evidence wiring: a claude-tmux profile with NO ~/.claude/settings.json
// has no evidence to freeze → LevelPass. Guards against a no-op that flags
// claude unconditionally regardless of host updater state.
func TestRun_VersionFreeze_ClaudeNoSettings_Passes(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // empty home
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux"}, nil
	}
	opts.SelfUpdateEvidence = defaultSelfUpdateEvidence // real registry
	opts.PinnedLister = func() ([]string, error) { return nil, nil }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c := findCheck(t, r, "cli-version-freeze"); c.Level != LevelPass {
		t.Fatalf("claude with no updater state → nothing to freeze → pass; got %s (%s)", c.Level, c.Detail)
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
