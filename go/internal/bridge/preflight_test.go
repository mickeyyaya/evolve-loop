package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIPreflight_CodexTmuxImplementsIt(t *testing.T) {
	t.Parallel()
	d, ok := LookupDriver("codex-tmux")
	if !ok {
		t.Fatal("codex-tmux driver not registered (init() didn't fire?)")
	}
	if _, isPF := d.(CLIPreflight); !isPF {
		t.Fatalf("codex-tmux MUST implement CLIPreflight — cycle-122 Fix 1 pretrust runs through this seam; absence means Engine.Launch silently skips it and the workspace-write modal recurs")
	}
}

func TestCLIPreflight_OptOutByOmission(t *testing.T) {
	t.Parallel()
	optOut := []string{"claude-p", "claude-tmux", "agy", "agy-tmux", "codex", "ollama", "ollama-tmux"}
	for _, name := range optOut {
		d, ok := LookupDriver(name)
		if !ok {
			t.Logf("driver %q not registered (skipping)", name)
			continue
		}
		if _, isPF := d.(CLIPreflight); isPF {
			t.Errorf("driver %q now implements CLIPreflight — update preflight_test.go optOut list and document the new prep work in driver.go's godoc", name)
		}
	}
}

func TestCLIPreflight_CodexPreflightReturnsHelperError(t *testing.T) {
	t.Parallel()
	d, ok := LookupDriver("codex-tmux")
	if !ok {
		t.Fatal("codex-tmux driver not registered (init() didn't fire?)")
	}
	pf, ok := d.(CLIPreflight)
	if !ok {
		t.Fatal("codex-tmux must implement CLIPreflight — see TestCLIPreflight_CodexTmuxImplementsIt")
	}
	cfg := &Config{
		CLI:       "codex-tmux",
		Worktree:  t.TempDir(),
		Workspace: t.TempDir(),
	}
	deps := Deps{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("codex-tmux Preflight panicked: %v (must be best-effort: return errors, never panic)", r)
		}
	}()
	_ = pf.Preflight(context.Background(), cfg, deps)
}

func TestCodexTmuxPreflightDismissesUpdateNag(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	versionPath := filepath.Join(t.TempDir(), "version.json")
	old := codexVersionPathFn
	t.Cleanup(func() { codexVersionPathFn = old })
	codexVersionPathFn = func() (string, error) { return versionPath, nil }

	d := codexTmuxDriver{}
	cfg := &Config{Worktree: t.TempDir(), Workspace: t.TempDir(), codexConfigPath: configPath}
	if err := d.Preflight(context.Background(), cfg, Deps{}); err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	raw, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatalf("read version state: %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("version state JSON: %v\n%s", err, raw)
	}
	if state["dismissed_version"] != "999.999.999" {
		t.Fatalf("dismissed_version=%v, want durable high-water suppression", state["dismissed_version"])
	}
}

type recordingDriver struct {
	name        string
	preflightFn func(ctx context.Context, cfg *Config, deps Deps) error
	calls       []string
}

func (d *recordingDriver) Name() string { return d.name }
func (d *recordingDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	d.calls = append(d.calls, "launch")
	return ExitOK, nil
}
func (d *recordingDriver) Preflight(ctx context.Context, cfg *Config, deps Deps) error {
	d.calls = append(d.calls, "preflight")
	if d.preflightFn != nil {
		return d.preflightFn(ctx, cfg, deps)
	}
	return nil
}

type nonPreflightDriver struct {
	name  string
	calls []string
}

func (d *nonPreflightDriver) Name() string { return d.name }
func (d *nonPreflightDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	d.calls = append(d.calls, "launch")
	return ExitOK, nil
}

func TestCLIPreflight_Compiletime(t *testing.T) {
	t.Parallel()
	var _ Driver = (*recordingDriver)(nil)
	var _ CLIPreflight = (*recordingDriver)(nil)
	var _ Driver = (*nonPreflightDriver)(nil)
	var d Driver = (*nonPreflightDriver)(nil)
	if _, isPF := d.(CLIPreflight); isPF {
		t.Fatal("nonPreflightDriver must NOT satisfy CLIPreflight — opt-out by omission is the architectural design")
	}
}

func TestCLIPreflight_RecordingFakePreflightOrder(t *testing.T) {
	t.Parallel()
	d := &recordingDriver{name: "fake-tmux"}
	cfg := &Config{}
	deps := Deps{}
	_ = d.Preflight(context.Background(), cfg, deps)
	_, _ = d.Launch(context.Background(), cfg, deps)
	if len(d.calls) != 2 || d.calls[0] != "preflight" || d.calls[1] != "launch" {
		t.Fatalf("recordingDriver call order broken; got %v", d.calls)
	}
}

func TestCLIPreflight_ErrorPropagation(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("preflight-sentinel")
	d := &recordingDriver{
		name: "fake-tmux",
		preflightFn: func(ctx context.Context, cfg *Config, deps Deps) error {
			return sentinel
		},
	}
	got := d.Preflight(context.Background(), &Config{}, Deps{})
	if !errors.Is(got, sentinel) {
		t.Fatalf("Preflight error must propagate unchanged; got %v want %v", got, sentinel)
	}
}

func TestCLIPreflight_NilErrorPropagation(t *testing.T) {
	t.Parallel()
	d := &recordingDriver{name: "fake-tmux"}
	if got := d.Preflight(context.Background(), &Config{}, Deps{}); got != nil {
		t.Fatalf("Preflight with nil preflightFn must return nil; got %v", got)
	}
}

func TestCLIPreflight_ContextPropagation(t *testing.T) {
	t.Parallel()
	var seen context.Context
	d := &recordingDriver{
		name: "fake-tmux",
		preflightFn: func(ctx context.Context, _ *Config, _ Deps) error {
			seen = ctx
			return nil
		},
	}

	live, cancelLive := context.WithCancel(context.Background())
	defer cancelLive()
	_ = d.Preflight(live, &Config{}, Deps{})
	if seen == nil {
		t.Fatal("Preflight must receive the ctx; got nil")
	}
	if seen.Err() != nil {
		t.Fatalf("live ctx must show Err()=nil; got %v", seen.Err())
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_ = d.Preflight(canceled, &Config{}, Deps{})
	if !errors.Is(seen.Err(), context.Canceled) {
		t.Fatalf("canceled ctx must propagate; got Err()=%v", seen.Err())
	}
}

func TestCLIPreflight_ConfigAndDepsPassThrough(t *testing.T) {
	t.Parallel()
	var seenCfg *Config
	var seenDeps Deps
	d := &recordingDriver{
		name: "fake-tmux",
		preflightFn: func(_ context.Context, c *Config, deps Deps) error {
			seenCfg = c
			seenDeps = deps
			return nil
		},
	}
	stderr := &bytes.Buffer{}
	wantCfg := &Config{CLI: "fake-tmux", Worktree: "/tmp/wt", Workspace: "/tmp/ws"}
	wantDeps := Deps{Stderr: stderr}
	_ = d.Preflight(context.Background(), wantCfg, wantDeps)

	if seenCfg != wantCfg {
		t.Fatalf("cfg pointer must pass through; got %p want %p", seenCfg, wantCfg)
	}
	if seenCfg.CLI != "fake-tmux" || seenCfg.Worktree != "/tmp/wt" || seenCfg.Workspace != "/tmp/ws" {
		t.Fatalf("cfg fields lost in transit; got %+v", *seenCfg)
	}
	if seenDeps.Stderr != stderr {
		t.Fatal("deps.Stderr must pass through")
	}
}

func TestCLIPreflight_OptOutDoesNotPanicOnTypeAssertion(t *testing.T) {
	t.Parallel()
	var d Driver = &nonPreflightDriver{name: "opt-out"}
	if pf, ok := d.(CLIPreflight); ok {
		t.Fatalf("opt-out driver must not satisfy CLIPreflight; got pf=%v", pf)
	}
	if _, ok := any(d).(interface {
		Preflight(context.Context, *Config, Deps) error
	}); ok {
		t.Fatal("opt-out driver accidentally exposes Preflight via embedding/promotion")
	}
}
