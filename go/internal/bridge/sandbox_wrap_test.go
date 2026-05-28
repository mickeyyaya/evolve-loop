package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// Workstream B unit tests for the bridge-side sandbox seam.
//
// These exercise the decision matrix + the prefix-argv plumbing without
// running real sandbox-exec / bwrap. The "wrap actually confines" property
// is owned by adapters/sandbox (which has its own host-gated integration
// suite); here we prove the bridge calls the seam under the right conditions
// and composes the prefix correctly for each driver's contract.

// fakeWrap returns a SandboxWrapper that records its inputs and returns a
// scripted result. Lets tests assert on what the bridge would have done.
type fakeWrap struct {
	calls       []SandboxWrapRequest
	prefix      []string
	available   bool
	returnEmpty bool
}

func (f *fakeWrap) wrap() SandboxWrapper {
	return func(req SandboxWrapRequest) ([]string, bool) {
		f.calls = append(f.calls, req)
		if f.returnEmpty {
			return nil, false
		}
		return f.prefix, f.available
	}
}

func TestSandboxPrefix_NoWorktree_SkipsWrap(t *testing.T) {
	// Non-source-writing phases (cfg.Worktree=="") MUST NOT consult the
	// wrapper — they don't write source, so confinement adds nothing.
	fw := &fakeWrap{prefix: []string{"sandbox-exec", "-p", "x"}, available: true}
	deps := Deps{SandboxWrap: fw.wrap()}
	cfg := &Config{Worktree: "", Agent: "scout"}
	prefix, ok := sandboxPrefixForLaunch(deps, cfg)
	if ok || prefix != nil {
		t.Errorf("non-worktree phase should skip wrap; got (%v, %v)", prefix, ok)
	}
	if len(fw.calls) != 0 {
		t.Errorf("wrapper should not be consulted; got %d calls", len(fw.calls))
	}
}

func TestSandboxPrefix_WorktreePhase_PassesAbsolutePaths(t *testing.T) {
	// Source-writing phases pass their absolute Worktree+Workspace+RepoRoot
	// to the wrapper — the SBPL profile depends on every path being absolute
	// (matches Workstream A's invariant; relative paths broke cycle-119).
	fw := &fakeWrap{prefix: []string{"sandbox-exec", "-p", "/tmp/sb.sb"}, available: true}
	deps := Deps{SandboxWrap: fw.wrap()}
	cfg := &Config{
		Worktree:    "/abs/wt/cycle-5",
		Workspace:   "/abs/ws/cycle-5",
		ProjectRoot: "/abs/repo",
		Agent:       "build",
	}
	prefix, ok := sandboxPrefixForLaunch(deps, cfg)
	if !ok || len(prefix) != 3 {
		t.Fatalf("worktree phase should wrap; got (%v, %v)", prefix, ok)
	}
	if len(fw.calls) != 1 {
		t.Fatalf("wrapper called %d times, want 1", len(fw.calls))
	}
	got := fw.calls[0]
	if got.Phase != "build" || got.Worktree != "/abs/wt/cycle-5" ||
		got.Workspace != "/abs/ws/cycle-5" || got.RepoRoot != "/abs/repo" {
		t.Errorf("wrong request: %+v", got)
	}
}

func TestSandboxPrefix_NoWrapper_Degrades(t *testing.T) {
	// A deps with no SandboxWrap (older test harness, or explicit off) must
	// return (nil, false) — drivers then run unwrapped. Never panic on nil.
	deps := Deps{SandboxWrap: nil}
	cfg := &Config{Worktree: "/abs/wt/x", Agent: "build"}
	prefix, ok := sandboxPrefixForLaunch(deps, cfg)
	if ok || prefix != nil {
		t.Errorf("nil wrapper should degrade silently; got (%v, %v)", prefix, ok)
	}
}

func TestWrapHeadless_NoWrap_PassesThrough(t *testing.T) {
	// When wrap is unavailable the (name, args) pair must come out unchanged
	// — the headless driver's runner call is byte-identical to pre-B.
	fw := &fakeWrap{returnEmpty: true}
	deps := Deps{SandboxWrap: fw.wrap()}
	cfg := &Config{Worktree: "/abs/wt/x", Agent: "build"}
	name, args := wrapHeadlessInvocation(deps, cfg, "/usr/bin/claude", []string{"-p", "do thing"})
	if name != "/usr/bin/claude" || !equalSlice(args, []string{"-p", "do thing"}) {
		t.Errorf("pass-through broken; got name=%q args=%v", name, args)
	}
}

func TestWrapHeadless_WithWrap_PrependsPrefix(t *testing.T) {
	// With wrap, name becomes the sandbox binary and args = prefix-flags +
	// [original-binary] + original-args. Order matters — the inner binary
	// MUST come BEFORE its original args for argv parsing to work.
	fw := &fakeWrap{prefix: []string{"sandbox-exec", "-p", "/tmp/sb.sb"}, available: true}
	deps := Deps{SandboxWrap: fw.wrap()}
	cfg := &Config{Worktree: "/abs/wt/x", Agent: "build"}
	name, args := wrapHeadlessInvocation(deps, cfg, "/usr/bin/claude", []string{"-p", "do thing"})
	if name != "sandbox-exec" {
		t.Errorf("name=%q, want sandbox-exec", name)
	}
	want := []string{"-p", "/tmp/sb.sb", "/usr/bin/claude", "-p", "do thing"}
	if !equalSlice(args, want) {
		t.Errorf("args=%v, want %v", args, want)
	}
}

func TestJoinPrefixForTmux_QuotesSpecialChars(t *testing.T) {
	// The tmux driver sends the launch command as a single SHELL LINE via
	// SendKeys, so any path containing spaces / quotes must be POSIX-quoted
	// or the shell will tokenize it incorrectly.
	got := joinPrefixForTmux([]string{"sandbox-exec", "-p", "/tmp/has space.sb"})
	if !strings.Contains(got, `'/tmp/has space.sb'`) {
		t.Errorf("path with space not single-quoted; got %q", got)
	}
	// And no-op for safe inputs.
	safe := joinPrefixForTmux([]string{"sandbox-exec", "-p", "/tmp/safe.sb"})
	if safe != "sandbox-exec -p /tmp/safe.sb" {
		t.Errorf("safe path got unnecessary quoting: %q", safe)
	}
}

func TestDefaultSandboxWrap_OffMode_NeverWraps(t *testing.T) {
	deps := Deps{Env: map[string]string{"EVOLVE_SANDBOX": config.SandboxModeOff}}
	wrap := defaultSandboxWrap(deps)
	prefix, ok := wrap(SandboxWrapRequest{Phase: "build", Worktree: "/wt", Workspace: "/ws", RepoRoot: "/r"})
	if ok || prefix != nil {
		t.Errorf("EVOLVE_SANDBOX=off must short-circuit before Probe; got (%v, %v)", prefix, ok)
	}
}

func TestDefaultSandboxWrap_NestedClaudeAuto_DoesNotWrap(t *testing.T) {
	// Auto mode + nested-claude (the outer Claude Code session already has OS
	// sandbox + Tier-1 hooks) must skip wrapping to avoid EPERM noise. Same
	// signal preflight uses.
	deps := Deps{Env: map[string]string{
		"EVOLVE_SANDBOX":         config.SandboxModeAuto,
		"CLAUDE_CODE_ENTRYPOINT": "cli",
	}}
	wrap := defaultSandboxWrap(deps)
	prefix, ok := wrap(SandboxWrapRequest{Phase: "build", Worktree: "/wt", Workspace: "/ws", RepoRoot: "/r"})
	if ok || prefix != nil {
		t.Errorf("nested-claude+auto must skip wrap; got (%v, %v)", prefix, ok)
	}
}

// fakeProbe returns a deterministic probe so tests don't depend on the host
// or the package-level sync.Once.
func fakeProbe(os string, available bool) func() sandbox.ProbeResult {
	return func() sandbox.ProbeResult {
		return sandbox.ProbeResult{OS: os, Available: available, BinaryPath: "/usr/bin/" + os, Reason: "fake"}
	}
}

func TestDefaultSandboxWrap_Darwin_WritesSBPLAndUsesDashF(t *testing.T) {
	// Darwin path: the SBPL profile is materialized into the workspace
	// (per-phase file) AND the prefix uses `-f` (file path), NOT `-p` (which
	// passes the inline SBPL string). This pins the cycle-119 fix: a `-p
	// <path>` would silently leave the phase unconfined because sandbox-exec
	// would parse the path AS the profile.
	ws := t.TempDir()
	deps := Deps{Env: map[string]string{"EVOLVE_SANDBOX": config.SandboxModeOn}}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))
	prefix, ok := wrap(SandboxWrapRequest{
		Phase: "build", Worktree: t.TempDir(), Workspace: ws, RepoRoot: t.TempDir(),
	})
	if !ok || len(prefix) != 3 || prefix[0] != "sandbox-exec" {
		t.Fatalf("unexpected prefix: %v", prefix)
	}
	if prefix[1] != "-f" {
		t.Errorf("prefix[1]=%q, want -f (sandbox-exec -p <path> would treat the path as inline SBPL — see cycle-119 fix)", prefix[1])
	}
	sbpl := prefix[2]
	if !strings.HasPrefix(sbpl, ws) {
		t.Errorf("SBPL %q not under workspace %q", sbpl, ws)
	}
	if _, err := os.Stat(sbpl); err != nil {
		t.Errorf("SBPL file not materialized: %v", err)
	}
	if filepath.Base(sbpl) != "sandbox-build.sb" {
		t.Errorf("SBPL basename=%q, want sandbox-build.sb", filepath.Base(sbpl))
	}
}

func TestDefaultSandboxWrap_OnMode_UnavailableProbe_LogsWarn(t *testing.T) {
	// EVOLVE_SANDBOX=on declares mandatory confinement. If the host has no
	// sandbox binary, the seam returns false (operator can't be force-
	// confined when there's no kernel to confine with), but it MUST emit a
	// loud WARN so the bypass is observable — otherwise the operator's
	// intent is silently violated.
	var stderr strings.Builder
	deps := Deps{Env: map[string]string{"EVOLVE_SANDBOX": config.SandboxModeOn}, Stderr: &stderr}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("linux", false))
	prefix, ok := wrap(SandboxWrapRequest{Phase: "build", Worktree: "/wt", Workspace: "/ws", RepoRoot: "/r"})
	if ok || prefix != nil {
		t.Errorf("no host sandbox → must return (nil, false); got (%v, %v)", prefix, ok)
	}
	if !strings.Contains(stderr.String(), "EVOLVE_SANDBOX=on") || !strings.Contains(stderr.String(), "UNCONFINED") {
		t.Errorf("expected loud WARN on on-mode + unavailable probe; got: %q", stderr.String())
	}
}

func TestDefaultSandboxWrap_AutoMode_UnavailableProbe_Silent(t *testing.T) {
	// Auto mode degrades silently when probe is unavailable — that's the
	// designed soft-fallback so a missing sandbox binary doesn't kill cycles.
	// Only `on` should escalate via WARN.
	var stderr strings.Builder
	deps := Deps{Env: map[string]string{"EVOLVE_SANDBOX": config.SandboxModeAuto}, Stderr: &stderr}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("linux", false))
	_, _ = wrap(SandboxWrapRequest{Phase: "build", Worktree: "/wt", Workspace: "/ws", RepoRoot: "/r"})
	if strings.Contains(stderr.String(), "WARN") {
		t.Errorf("auto-mode degrade must be silent; got WARN: %q", stderr.String())
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
