package looppreflight

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// freezeOptions returns goodPipelineOptions with a codex-tmux profile and scripted freeze seams.
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

func TestRun_VersionFreeze_HeadlessOnly_NotChecked(t *testing.T) {
	opts := freezeOptions(t, map[string]string{"codex": "evidence"}, nil, nil)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "codex"}, nil
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c := findCheck(t, r, "cli-version-freeze"); c.Level != LevelPass {
		t.Fatalf("headless-only codex must not be freeze-checked; got %s (%s)", c.Level, c.Detail)
	}
}

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

// agy has no registry entry; claude or codex would read this host's real updater state.
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

func TestDefaultSelfUpdateEvidence_ClaudeAbsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
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
	opts.SelfUpdateEvidence = defaultSelfUpdateEvidence
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

func TestRun_VersionFreeze_ClaudeNoSettings_Passes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux"}, nil
	}
	opts.SelfUpdateEvidence = defaultSelfUpdateEvidence
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
	// brew may be absent, so only a panic fails this.
	_, _ = defaultPinnedLister()
}

func TestDefaultSelfUpdateEvidence_ClaudeAutoUpdatesDisabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"autoUpdates": false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, evidence, err := defaultSelfUpdateEvidence("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("autoUpdates:false is the native freeze — must report no self-update evidence (got %q)", evidence)
	}
	if evidence != "" {
		t.Fatalf("frozen-host evidence must be empty; got %q", evidence)
	}
}

func TestDefaultSelfUpdateEvidence_ClaudeAutoUpdatesTrueOrGarbage(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantErr   bool
		wantRisky bool
	}{
		{name: "explicit-true", body: `{"autoUpdates": true}`, wantRisky: true},
		{name: "absent-key", body: `{}`, wantRisky: true},
		{name: "garbage-settings", body: `{torn`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			settings := filepath.Join(home, ".claude", "settings.json")
			if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(settings, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			ok, _, err := defaultSelfUpdateEvidence("claude")
			if tc.wantErr {
				if err == nil {
					t.Fatal("unparsable settings.json is AMBIGUITY (error → WARN), never a silent pass")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantRisky && !ok {
				t.Fatal("claude with auto-updates not disabled must stay risky")
			}
		})
	}
}
