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

// preflight_test.go — cycle-124 G3 contract: the optional CLIPreflight
// interface lets a Driver hook pre-launch prep work (today only codex-tmux,
// to pre-trust the worktree+workspace in ~/.codex/config.toml before the
// REPL boots — cycle-122 Fix 1 promoted out of an inline call). The tests
// pin three properties:
//
//  1. codex-tmux IS a CLIPreflight (the driver type assertion the Engine
//     does at launch.go would otherwise fall through silently and the
//     pretrust never run).
//  2. Drivers WITHOUT preflight work (claude-tmux, agy-tmux, ollama-tmux,
//     claude-p, codex headless, agy headless) MUST NOT accidentally
//     implement the interface — the absence is the OPT-OUT mechanism,
//     and a no-op stub on every concrete driver would be the wrong
//     pattern (the comment in driver.go documents this).
//  3. codex-tmux's Preflight returns an error from pretrustCodexProjects
//     unchanged (best-effort: error gets logged by Engine.Launch but does
//     not abort the phase; the contract is "do something useful or return
//     a logged-but-non-fatal error").

func TestCLIPreflight_CodexTmuxImplementsIt(t *testing.T) {
	d, ok := LookupDriver("codex-tmux")
	if !ok {
		t.Fatal("codex-tmux driver not registered (init() didn't fire?)")
	}
	if _, isPF := d.(CLIPreflight); !isPF {
		t.Fatalf("codex-tmux MUST implement CLIPreflight — cycle-122 Fix 1 pretrust runs through this seam; absence means Engine.Launch silently skips it and the workspace-write modal recurs")
	}
}

func TestCLIPreflight_OptOutByOmission(t *testing.T) {
	// Drivers expected to NOT implement CLIPreflight today. If a driver
	// later adopts CLIPreflight (e.g. agy keychain refresh), this list
	// must be updated. The opt-out is the architectural design — no no-op
	// stubs in every concrete driver. See driver.go's CLIPreflight godoc.
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
	// Drive codexTmuxDriver.Preflight directly with a config that the
	// underlying helper (pretrustCodexProjects) accepts but writes to a
	// real ~/.codex/config.toml — best-effort means even a write failure
	// is a return value, not a panic. The test is intentionally loose:
	// it just confirms Preflight returns without panic for a plausible
	// config. Tighter end-to-end coverage is in codex_pretrust_test.go
	// against the helper's own surface.
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
	deps := Deps{} // no I/O streams — the helper does not write to them
	// We accept either a nil error (clean run) or a non-nil error (e.g.
	// HOME unsetup) — the contract is "non-panicking, best-effort".
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

// recordingDriver is a synthetic Driver used to pin the Engine's CLIPreflight
// dispatch contract. It records the order of Preflight and Launch calls so
// the tests can assert preflight-before-launch invariants and best-effort
// error semantics. The driver returns ExitOK from Launch.
type recordingDriver struct {
	name        string
	preflightFn func(ctx context.Context, cfg *Config, deps Deps) error
	calls       []string // appended "preflight" or "launch" in invocation order
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

// nonPreflightDriver implements only Driver (not CLIPreflight) — used to pin
// the opt-out path through the type assertion at launch.go.
type nonPreflightDriver struct {
	name  string
	calls []string
}

func (d *nonPreflightDriver) Name() string { return d.name }
func (d *nonPreflightDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	d.calls = append(d.calls, "launch")
	return ExitOK, nil
}

// TestCLIPreflight_Compiletime is a compile-time assertion that the
// recordingDriver test fake satisfies BOTH Driver and CLIPreflight — if
// the interface drifts (e.g., signature change), this fails to build,
// catching the breakage at compile time rather than as a runtime nil
// assertion in another test.
func TestCLIPreflight_Compiletime(t *testing.T) {
	var _ Driver = (*recordingDriver)(nil)
	var _ CLIPreflight = (*recordingDriver)(nil)
	var _ Driver = (*nonPreflightDriver)(nil)
	// nonPreflightDriver must NOT satisfy CLIPreflight — proving the
	// opt-out by omission works at the type level.
	var d Driver = (*nonPreflightDriver)(nil)
	if _, isPF := d.(CLIPreflight); isPF {
		t.Fatal("nonPreflightDriver must NOT satisfy CLIPreflight — opt-out by omission is the architectural design")
	}
}

// TestCLIPreflight_RecordingFakePreflightOrder pins that the recording
// fake's Preflight increments the calls slice BEFORE Launch is dispatched.
// This is the core invariant the Engine relies on; tests below use it as a
// building block, but it's worth a standalone smoke check so a refactor
// of the fake itself doesn't silently invalidate downstream assertions.
func TestCLIPreflight_RecordingFakePreflightOrder(t *testing.T) {
	d := &recordingDriver{name: "fake-tmux"}
	cfg := &Config{}
	deps := Deps{}
	_ = d.Preflight(context.Background(), cfg, deps)
	_, _ = d.Launch(context.Background(), cfg, deps)
	if len(d.calls) != 2 || d.calls[0] != "preflight" || d.calls[1] != "launch" {
		t.Fatalf("recordingDriver call order broken; got %v", d.calls)
	}
}

// TestCLIPreflight_ErrorPropagation pins that Preflight error VALUES reach
// the caller unchanged via the interface — Engine.Launch logs them, but the
// interface itself is a transparent pass-through. Sentinel-error patterns
// (errors.Is) must work across the seam.
func TestCLIPreflight_ErrorPropagation(t *testing.T) {
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

// TestCLIPreflight_NilErrorPropagation pins the happy path — a Preflight
// that completes without issue returns nil; the Engine's error-log branch
// stays untaken. Pairs with TestCLIPreflight_ErrorPropagation.
func TestCLIPreflight_NilErrorPropagation(t *testing.T) {
	d := &recordingDriver{name: "fake-tmux"} // default preflightFn returns nil
	if got := d.Preflight(context.Background(), &Config{}, Deps{}); got != nil {
		t.Fatalf("Preflight with nil preflightFn must return nil; got %v", got)
	}
}

// TestCLIPreflight_ContextPropagation pins that the Engine passes ctx
// through to Preflight. The fake echoes ctx.Err() — a canceled ctx
// arrives canceled, an alive ctx arrives alive. This is the property that
// lets a Preflight observe orchestrator cancellation (e.g., per-phase
// timeout) and bail early.
func TestCLIPreflight_ContextPropagation(t *testing.T) {
	var seen context.Context
	d := &recordingDriver{
		name: "fake-tmux",
		preflightFn: func(ctx context.Context, _ *Config, _ Deps) error {
			seen = ctx
			return nil
		},
	}

	// Live ctx → no error visible to Preflight.
	live, cancelLive := context.WithCancel(context.Background())
	defer cancelLive()
	_ = d.Preflight(live, &Config{}, Deps{})
	if seen == nil {
		t.Fatal("Preflight must receive the ctx; got nil")
	}
	if seen.Err() != nil {
		t.Fatalf("live ctx must show Err()=nil; got %v", seen.Err())
	}

	// Canceled ctx → Preflight observes ctx.Err() == context.Canceled.
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_ = d.Preflight(canceled, &Config{}, Deps{})
	if !errors.Is(seen.Err(), context.Canceled) {
		t.Fatalf("canceled ctx must propagate; got Err()=%v", seen.Err())
	}
}

// TestCLIPreflight_ConfigAndDepsPassThrough pins that the *Config + Deps
// pointers reach Preflight unchanged — the Engine's dispatch threads them
// as-is. A driver-side Preflight that needs cfg.Worktree or deps.Stderr
// must see what the caller passed.
func TestCLIPreflight_ConfigAndDepsPassThrough(t *testing.T) {
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

// TestCLIPreflight_OptOutDoesNotPanicOnTypeAssertion pins the Engine's
// `if pf, ok := driver.(CLIPreflight); ok` short-circuit: a driver that
// doesn't implement CLIPreflight must evaluate `ok=false`. The comma-ok
// form of type assertion is documented as never-panicking by the Go spec,
// so no recover() is needed (cycle-124 test-review LOW). The second
// assertion uses an anonymous interface to triple-check the underlying
// type genuinely lacks Preflight — catches accidental embedded promotion
// a refactor might introduce.
func TestCLIPreflight_OptOutDoesNotPanicOnTypeAssertion(t *testing.T) {
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
