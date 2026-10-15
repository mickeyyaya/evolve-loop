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
	t.Parallel()
	fw := &fakeWrap{prefix: []string{"sandbox-exec", "-p", "x"}, available: true}
	deps := Deps{SandboxWrap: fw.wrap()}
	cfg := &Config{Worktree: "", Agent: "scout"}
	prefix, ok := sandboxPrefixForLaunch(deps, cfg, "")
	if ok || prefix != nil {
		t.Errorf("non-worktree phase should skip wrap; got (%v, %v)", prefix, ok)
	}
	if len(fw.calls) != 0 {
		t.Errorf("wrapper should not be consulted; got %d calls", len(fw.calls))
	}
}

func TestSandboxPrefix_RequiredContractConsultsWrapperWithoutWorktree(t *testing.T) {
	t.Parallel()
	fw := &fakeWrap{prefix: []string{"sandbox-exec", "-p", "x"}, available: true}
	deps := Deps{SandboxWrap: fw.wrap()}
	cfg := &Config{RequireSandbox: true, Workspace: "/ws", ProjectRoot: "/repo", Agent: "build"}
	prefix, ok := sandboxPrefixForLaunch(deps, cfg, "")
	if !ok || len(prefix) == 0 || len(fw.calls) != 1 {
		t.Fatalf("required contract did not request confinement: prefix=%v ok=%v calls=%d", prefix, ok, len(fw.calls))
	}
	if !sandboxRequiredButUnavailable(Deps{Env: map[string]string{}}, cfg, false) || sandboxRequiredButUnavailable(Deps{Env: map[string]string{}}, cfg, true) {
		t.Fatal("required-sandbox fail-closed predicate is incorrect")
	}
}

func TestSandboxPrefix_WorktreePhase_PassesAbsolutePaths(t *testing.T) {
	t.Parallel()
	fw := &fakeWrap{prefix: []string{"sandbox-exec", "-p", "/tmp/sb.sb"}, available: true}
	deps := Deps{SandboxWrap: fw.wrap()}
	cfg := &Config{
		Worktree:    "/abs/wt/cycle-5",
		Workspace:   "/abs/ws/cycle-5",
		ProjectRoot: "/abs/repo",
		Agent:       "build",
	}
	prefix, ok := sandboxPrefixForLaunch(deps, cfg, "")
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

func TestSandboxPrefix_ForcesNetwork(t *testing.T) {
	t.Parallel()
	base := func() *Config {
		return &Config{Worktree: "/wt", Workspace: "/ws", ProjectRoot: "/repo", Agent: "tdd"}
	}
	t.Run("false profile overridden with WARN", func(t *testing.T) {
		fw := &fakeWrap{prefix: []string{"x"}, available: true}
		var stderr strings.Builder
		cfg := base()
		cfg.AllowNetwork = false
		_, _ = sandboxPrefixForLaunch(Deps{SandboxWrap: fw.wrap(), Stderr: &stderr}, cfg, "")
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
		_, _ = sandboxPrefixForLaunch(Deps{SandboxWrap: fw.wrap(), Stderr: &stderr}, cfg, "")
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
		_, _ = sandboxPrefixForLaunch(Deps{SandboxWrap: fw.wrap(), Stderr: nil}, cfg, "")
		if len(fw.calls) != 1 || !fw.calls[0].AllowNetwork {
			t.Fatalf("must force AllowNetwork=true even with nil Stderr; got %+v", fw.calls)
		}
	})
}

func TestSandboxPrefix_NoWrapper_Degrades(t *testing.T) {
	t.Parallel()
	deps := Deps{SandboxWrap: nil}
	cfg := &Config{Worktree: "/abs/wt/x", Agent: "build"}
	prefix, ok := sandboxPrefixForLaunch(deps, cfg, "")
	if ok || prefix != nil {
		t.Errorf("nil wrapper should degrade silently; got (%v, %v)", prefix, ok)
	}
}

func TestWrapHeadless_NoWrap_PassesThrough(t *testing.T) {
	t.Parallel()
	fw := &fakeWrap{returnEmpty: true}
	deps := Deps{SandboxWrap: fw.wrap()}
	cfg := &Config{Worktree: "/abs/wt/x", Agent: "build"}
	name, args, _ := wrapHeadlessInvocation(deps, cfg, "/usr/bin/claude", []string{"-p", "do thing"})
	if name != "/usr/bin/claude" || !equalSlice(args, []string{"-p", "do thing"}) {
		t.Errorf("pass-through broken; got name=%q args=%v", name, args)
	}
}

func TestWrapHeadless_WithWrap_PrependsPrefix(t *testing.T) {
	t.Parallel()
	fw := &fakeWrap{prefix: []string{"sandbox-exec", "-p", "/tmp/sb.sb"}, available: true}
	deps := Deps{SandboxWrap: fw.wrap()}
	cfg := &Config{Worktree: "/abs/wt/x", Agent: "build"}
	name, args, _ := wrapHeadlessInvocation(deps, cfg, "/usr/bin/claude", []string{"-p", "do thing"})
	if name != "sandbox-exec" {
		t.Errorf("name=%q, want sandbox-exec", name)
	}
	want := []string{"-p", "/tmp/sb.sb", "/usr/bin/claude", "-p", "do thing"}
	if !equalSlice(args, want) {
		t.Errorf("args=%v, want %v", args, want)
	}
}

func TestJoinPrefixForTmux_QuotesSpecialChars(t *testing.T) {
	t.Parallel()
	got := joinPrefixForTmux([]string{"sandbox-exec", "-p", "/tmp/has space.sb"})
	if !strings.Contains(got, `'/tmp/has space.sb'`) {
		t.Errorf("path with space not single-quoted; got %q", got)
	}
	safe := joinPrefixForTmux([]string{"sandbox-exec", "-p", "/tmp/safe.sb"})
	if safe != "sandbox-exec -p /tmp/safe.sb" {
		t.Errorf("safe path got unnecessary quoting: %q", safe)
	}
}

func TestDefaultSandboxWrap_OffMode_NeverWraps(t *testing.T) {
	t.Parallel()
	deps := Deps{Env: map[string]string{"EVOLVE_SANDBOX": config.SandboxModeOff}}
	wrap := defaultSandboxWrap(deps)
	prefix, ok := wrap(SandboxWrapRequest{Phase: "build", Worktree: "/wt", Workspace: "/ws", RepoRoot: "/r"})
	if ok || prefix != nil {
		t.Errorf("EVOLVE_SANDBOX=off must short-circuit before Probe; got (%v, %v)", prefix, ok)
	}
}

func TestDefaultSandboxWrap_NestedClaudeAuto_DoesNotWrap(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func fakeProbe(os string, available bool) func() sandbox.ProbeResult {
	return func() sandbox.ProbeResult {
		return sandbox.ProbeResult{OS: os, Available: available, BinaryPath: "/usr/bin/" + os, Reason: "fake"}
	}
}

func TestDefaultSandboxWrap_Darwin_WritesSBPLAndUsesDashF(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	var stderr strings.Builder
	deps := Deps{Env: map[string]string{"EVOLVE_SANDBOX": config.SandboxModeAuto}, Stderr: &stderr}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("linux", false))
	_, _ = wrap(SandboxWrapRequest{Phase: "build", Worktree: "/wt", Workspace: "/ws", RepoRoot: "/r"})
	if strings.Contains(stderr.String(), "WARN") {
		t.Errorf("auto-mode degrade must be silent; got WARN: %q", stderr.String())
	}
}

func TestDefaultSandboxWrap_UnknownMode_NestedClaude_DoesNotWrap(t *testing.T) {
	t.Parallel()
	deps := Deps{Env: map[string]string{
		"EVOLVE_SANDBOX":         "1",
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
	t.Parallel()
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
	t.Parallel()
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

func sbplPathOf(t *testing.T, prefix []string) string {
	t.Helper()
	if len(prefix) != 3 || prefix[0] != "sandbox-exec" || prefix[1] != "-f" {
		t.Fatalf("unexpected darwin sandbox prefix %v", prefix)
	}
	return prefix[2]
}

func TestDefaultSandboxWrap_Darwin_PerInvocationProfileDir(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	deps := Deps{Env: map[string]string{}}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))
	req := SandboxWrapRequest{Phase: "build", Worktree: t.TempDir(), Workspace: ws, RepoRoot: t.TempDir()}

	prefixA, okA := wrap(req)
	prefixB, okB := wrap(req)
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

func TestDefaultSandboxWrap_Darwin_ProfileDirMkdirFails_FallsBackToWorkspace(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	var stderr strings.Builder
	deps := Deps{
		Env:          map[string]string{},
		Stderr:       &stderr,
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
	if !strings.Contains(stderr.String(), "isolation lost") {
		t.Fatalf("the fallback must warn that profile isolation is lost; stderr=%q", stderr.String())
	}
}
