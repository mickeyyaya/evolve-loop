package bridge

import (
	"errors"
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

// TestSandboxPrefix_ForcesNetwork pins the structural invariant: every phase that
// reaches the sandbox (cfg.Worktree != "") runs a cloud model-reaching CLI —
// ollama-tmux, the only local driver, rejects any Worktree — so AllowNetwork is
// FORCED true regardless of the profile value, guarding source-writing phases
// (incl. future custom ones) and scratch-CWD probes from booting network-denied.
// The profile value controls only whether a misconfig WARN fires.
func TestSandboxPrefix_ForcesNetwork(t *testing.T) {
	base := func() *Config {
		return &Config{Worktree: "/wt", Workspace: "/ws", ProjectRoot: "/repo", Agent: "tdd"}
	}
	t.Run("false profile overridden with WARN", func(t *testing.T) {
		fw := &fakeWrap{prefix: []string{"x"}, available: true}
		var stderr strings.Builder
		cfg := base()
		cfg.AllowNetwork = false
		_, _ = sandboxPrefixForLaunch(Deps{SandboxWrap: fw.wrap(), Stderr: &stderr}, cfg)
		if len(fw.calls) != 1 || !fw.calls[0].AllowNetwork {
			t.Fatalf("must force AllowNetwork=true; got %+v", fw.calls)
		}
		if !strings.Contains(stderr.String(), "allow_network=false; forcing true") {
			t.Errorf("expected a forcing WARN; stderr=%q", stderr.String())
		}
	})
	t.Run("true profile forces true without WARN", func(t *testing.T) {
		fw := &fakeWrap{prefix: []string{"x"}, available: true}
		var stderr strings.Builder
		cfg := base()
		cfg.AllowNetwork = true
		_, _ = sandboxPrefixForLaunch(Deps{SandboxWrap: fw.wrap(), Stderr: &stderr}, cfg)
		if len(fw.calls) != 1 || !fw.calls[0].AllowNetwork {
			t.Fatalf("must keep AllowNetwork=true; got %+v", fw.calls)
		}
		if stderr.Len() != 0 {
			t.Errorf("no WARN expected when the profile already allows network; stderr=%q", stderr.String())
		}
	})
	t.Run("nil Stderr still forces true", func(t *testing.T) {
		fw := &fakeWrap{prefix: []string{"x"}, available: true}
		cfg := base()
		cfg.AllowNetwork = false
		_, _ = sandboxPrefixForLaunch(Deps{SandboxWrap: fw.wrap(), Stderr: nil}, cfg)
		if len(fw.calls) != 1 || !fw.calls[0].AllowNetwork {
			t.Fatalf("must force AllowNetwork=true even with nil Stderr; got %+v", fw.calls)
		}
	})
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

func TestDefaultSandboxWrap_NestedViaLookupEnv_DoesNotWrap(t *testing.T) {
	// depEnvGetter consults the Env map FIRST, then falls back to deps.LookupEnv.
	// The sibling nested tests cover the Env-map branch; this pins the LookupEnv
	// fallback branch (a nested signal present only via LookupEnv must still be
	// detected, so the wrap is skipped). Audit follow-up (cycle-990613 LOW).
	deps := Deps{
		Env: map[string]string{"EVOLVE_SANDBOX": config.SandboxModeAuto},
		LookupEnv: func(k string) (string, bool) {
			if k == "CLAUDE_CODE_SESSION_ID" {
				return "sess-xyz", true
			}
			return "", false
		},
	}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))
	prefix, ok := wrap(SandboxWrapRequest{
		Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(),
	})
	if ok || prefix != nil {
		t.Errorf("nested signal via LookupEnv must skip wrap; got (%v, %v)", prefix, ok)
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

func TestDefaultSandboxWrap_Darwin_AllowsHomeReadAndProfileNetwork(t *testing.T) {
	ws := t.TempDir()
	deps := Deps{Env: map[string]string{"EVOLVE_SANDBOX": config.SandboxModeOn}}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))
	prefix, ok := wrap(SandboxWrapRequest{
		Phase:        "build",
		Worktree:     t.TempDir(),
		Workspace:    ws,
		RepoRoot:     t.TempDir(),
		AllowNetwork: true,
	})
	if !ok {
		t.Fatal("sandbox wrapper did not return a prefix")
	}
	raw, err := os.ReadFile(prefix[2])
	if err != nil {
		t.Fatalf("read SBPL: %v", err)
	}
	sbpl := string(raw)
	home, _ := os.UserHomeDir()
	if home != "" && !strings.Contains(sbpl, `(allow file-read* (subpath "`+home+`"))`) {
		t.Fatalf("SBPL missing HOME read allow for tmux CLI config/auth state:\n%s", sbpl)
	}
	if strings.Contains(sbpl, "(deny network*)") {
		t.Fatalf("SBPL denied network even though profile allowed it:\n%s", sbpl)
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

func TestDefaultSandboxWrap_UnknownMode_NestedClaude_DoesNotWrap(t *testing.T) {
	// Regression (2026-06-13 soak, cycles 324-326): an UNRECOGNIZED
	// EVOLVE_SANDBOX value (operator typo "1" instead of auto|on|off) was
	// neither "off" nor "auto", so it slipped past the nested-claude skip
	// (which only fired for the literal "auto") and forced a sandbox-exec
	// wrap on nested macOS. That hung claude's REPL boot >60s
	// (exit=80 ExitREPLBootTimeout), failing every cycle at scout. An
	// unrecognized value MUST normalize to auto so the nested-skip applies.
	//
	// Workspace must be writable: with the bug, the darwin branch writes an
	// SBPL file there and returns the wrap prefix — t.TempDir() ensures that
	// write SUCCEEDS, so a missing fix is a true RED (wrap returned), not a
	// false pass via os.WriteFile failure.
	deps := Deps{Env: map[string]string{
		"EVOLVE_SANDBOX":         "1", // unrecognized — not auto|on|off
		"CLAUDE_CODE_ENTRYPOINT": "cli",
	}}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))
	prefix, ok := wrap(SandboxWrapRequest{
		Phase: "scout", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(),
	})
	if ok || prefix != nil {
		t.Errorf("unrecognized EVOLVE_SANDBOX value must normalize to auto and skip wrap under nested-claude; got (%v, %v)", prefix, ok)
	}
}

func TestDefaultSandboxWrap_UnknownMode_NormalizesToAuto_WarnsAndWrapsWhenNotNested(t *testing.T) {
	// The other half of the contract: when NOT nested, an unrecognized value
	// normalizes to auto (so an available probe still confines — fail-safe is
	// preserved) AND emits a one-line WARN so the misconfiguration is
	// observable (mirrors config.applyEnv, which also warns on unknown values).
	var stderr strings.Builder
	deps := Deps{Env: map[string]string{"EVOLVE_SANDBOX": "bogus"}, Stderr: &stderr}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))
	prefix, ok := wrap(SandboxWrapRequest{
		Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(),
	})
	if !ok || len(prefix) == 0 {
		t.Errorf("unknown value normalizes to auto; not-nested + available probe → should wrap; got (%v, %v)", prefix, ok)
	}
	if !strings.Contains(stderr.String(), "EVOLVE_SANDBOX") || !strings.Contains(stderr.String(), "unrecognized") {
		t.Errorf("expected an unrecognized-value WARN; got %q", stderr.String())
	}
}

func TestDefaultSandboxWrap_OnMode_NestedClaude_DoesNotWrap(t *testing.T) {
	// THE footgun the old auto-only nested skip missed: EVOLVE_SANDBOX=on
	// (correctly spelled, mandatory confinement) under nested Claude used to
	// force a sandbox-exec wrap that hangs the REPL boot on macOS (exit=80).
	// Post-SSOT, nested-Claude skips the wrap for ALL modes — and `on` emits a
	// loud WARN since its mandatory-confinement request is delegated to the
	// outer session rather than silently honoured.
	var stderr strings.Builder
	deps := Deps{Env: map[string]string{
		"EVOLVE_SANDBOX":         config.SandboxModeOn,
		"CLAUDE_CODE_ENTRYPOINT": "cli",
	}, Stderr: &stderr}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))
	prefix, ok := wrap(SandboxWrapRequest{
		Phase: "scout", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir(),
	})
	if ok || prefix != nil {
		t.Errorf("on-mode + nested-claude must NOT wrap (inner sandbox-exec hangs on nested macOS); got (%v, %v)", prefix, ok)
	}
	if !strings.Contains(stderr.String(), "EVOLVE_SANDBOX=on") || !strings.Contains(stderr.String(), "UNCONFINED") {
		t.Errorf("on-mode nested skip must emit a loud WARN; got %q", stderr.String())
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

// sbplPathOf extracts the SBPL file path from a darwin sandbox prefix
// (["sandbox-exec","-f","<path>"]).
func sbplPathOf(t *testing.T, prefix []string) string {
	t.Helper()
	if len(prefix) != 3 || prefix[0] != "sandbox-exec" || prefix[1] != "-f" {
		t.Fatalf("unexpected darwin sandbox prefix %v", prefix)
	}
	return prefix[2]
}

// TestDefaultSandboxWrap_Darwin_PerInvocationProfileDir pins ADR-0049 S0 / gap
// G6: two same-phase dispatches sharing ONE workspace must NOT write the same
// sandbox-<phase>.sb. Pre-fix both wrote <workspace>/sandbox-build.sb, so if
// their WritePaths differed, B's profile landing between A's write and A's
// sandbox-exec read confined A to B's allow-list (A's legit source writes
// EPERM-denied). A per-invocation profile dir isolates them. This is a true RED
// before the fix (identical paths) and GREEN after (distinct mktemp -d dirs).
func TestDefaultSandboxWrap_Darwin_PerInvocationProfileDir(t *testing.T) {
	ws := t.TempDir()                      // one shared workspace for both dispatches
	deps := Deps{Env: map[string]string{}} // not nested, mode auto → wraps
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))
	req := SandboxWrapRequest{Phase: "build", Worktree: t.TempDir(), Workspace: ws, RepoRoot: t.TempDir()}

	prefixA, okA := wrap(req)
	prefixB, okB := wrap(req) // identical request: same phase, same workspace
	if !okA || !okB {
		t.Fatalf("darwin not-nested should wrap; got okA=%v okB=%v", okA, okB)
	}
	pathA, pathB := sbplPathOf(t, prefixA), sbplPathOf(t, prefixB)
	if pathA == pathB {
		t.Fatalf("two same-phase dispatches shared sandbox profile path %q — collision (G6); want per-invocation isolation", pathA)
	}
	for _, p := range []string{pathA, pathB} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("sbpl %q not written: %v", p, err)
		}
		if len(b) == 0 {
			t.Fatalf("sbpl %q is empty — wrap must still emit a real profile", p)
		}
	}
}

// TestDefaultSandboxWrap_Darwin_ProfileDirMkdirFails_FallsBackToWorkspace pins
// the degrade contract: if the per-invocation dir can't be created, the wrap
// falls back to the shared workspace profile (confinement preserved, isolation
// lost) rather than running UNCONFINED (returning false).
func TestDefaultSandboxWrap_Darwin_ProfileDirMkdirFails_FallsBackToWorkspace(t *testing.T) {
	ws := t.TempDir()
	deps := Deps{
		Env:          map[string]string{},
		MkScratchDir: func(string, string) (string, error) { return "", errors.New("boom") },
	}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))
	prefix, ok := wrap(SandboxWrapRequest{Phase: "build", Worktree: t.TempDir(), Workspace: ws, RepoRoot: t.TempDir()})
	if !ok {
		t.Fatal("mkdir failure must fall back to the shared workspace profile, not abort confinement")
	}
	if got, want := sbplPathOf(t, prefix), filepath.Join(ws, "sandbox-build.sb"); got != want {
		t.Fatalf("fallback profile path = %q, want %q", got, want)
	}
}
