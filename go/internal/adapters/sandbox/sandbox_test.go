package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

func canonicalConfig() Config {
	return Config{
		RepoRoot:     "/repo",
		HomeDir:      "/Users/test",
		ReadOnlyRepo: false,
		AllowNetwork: true,
		WritePaths:   []string{"/repo/.evolve/runs"},
		DenyPaths:    []string{"/repo/.evolve/state.json"},
	}
}

func TestGenerateSBPL_HeaderAndBaseline(t *testing.T) {
	out := GenerateSBPL(canonicalConfig())
	mustContain(t, out,
		`(version 1)`,
		`(deny default)`,
		`(import "system.sb")`,
		`(allow process-exec)`,
		`(allow process-fork)`,
	)
}

func TestGenerateSBPL_RepoReadAlwaysAllowed(t *testing.T) {
	out := GenerateSBPL(canonicalConfig())
	if !strings.Contains(out, `(allow file-read* (subpath "/repo"))`) {
		t.Errorf("missing repo read-allow: %s", excerpt(out, "/repo"))
	}
}

func TestGenerateSBPL_HomeDirReadAndKnownWrites(t *testing.T) {
	out := GenerateSBPL(canonicalConfig())
	mustContain(t, out,
		`(allow file-read* (subpath "/Users/test"))`,
		`(allow file-write* (subpath "/Users/test/.claude"))`,
		`(allow file-write* (subpath "/Users/test/.codex"))`,
		`(allow file-write* (subpath "/Users/test/.gemini"))`,
		`(allow file-write* (subpath "/Users/test/.cache"))`,
		`(allow file-write* (subpath "/Users/test/.config"))`,
	)
}

func TestGenerateSBPL_TmpReadWriteBoth(t *testing.T) {
	out := GenerateSBPL(canonicalConfig())
	mustContain(t, out,
		`(allow file-read* (subpath "/tmp"))`,
		`(allow file-write* (subpath "/tmp"))`,
		`(allow file-read* (subpath "/var/folders"))`,
		`(allow file-write* (subpath "/var/folders"))`,
	)
}

func TestGenerateSBPL_ReadOnlyRepo_AddsDenyBeforeWriteAllows(t *testing.T) {
	cfg := canonicalConfig()
	cfg.ReadOnlyRepo = true
	cfg.WritePaths = []string{"/repo/.evolve/runs"}
	out := GenerateSBPL(cfg)
	denyIdx := strings.Index(out, `(deny file-write* (subpath "/repo"))`)
	if denyIdx == -1 {
		t.Fatal("missing read-only-repo deny")
	}
	allowIdx := strings.Index(out, `(allow file-write* (subpath "/repo/.evolve/runs"))`)
	if allowIdx == -1 {
		t.Fatal("missing per-write-path allow")
	}
	if denyIdx >= allowIdx {
		t.Errorf("read-only-repo deny (idx=%d) must precede write-path allow (idx=%d)", denyIdx, allowIdx)
	}
}

func TestGenerateSBPL_WritePathsAddedAsSubpath(t *testing.T) {
	cfg := canonicalConfig()
	cfg.WritePaths = []string{"/repo/a", "/repo/b/c"}
	out := GenerateSBPL(cfg)
	mustContain(t, out,
		`(allow file-write* (subpath "/repo/a"))`,
		`(allow file-write* (subpath "/repo/b/c"))`,
	)
}

func TestGenerateSBPL_WritePathsAreLiteral(t *testing.T) {
	cfg := canonicalConfig()
	cfg.WritePaths = []string{"/repo/.evolve/runs"}
	out := GenerateSBPL(cfg)
	if !strings.Contains(out, `(allow file-write* (subpath "/repo/.evolve/runs"))`) {
		t.Errorf("write path not emitted verbatim: %s", excerpt(out, "runs"))
	}
}

func TestGenerateSBPL_DenyPaths(t *testing.T) {
	cfg := canonicalConfig()
	cfg.DenyPaths = []string{"/repo/.git", "/repo/.evolve/state.json"}
	out := GenerateSBPL(cfg)
	mustContain(t, out,
		`(deny file-write* (subpath "/repo/.git"))`,
		`(deny file-write* (subpath "/repo/.evolve/state.json"))`,
	)
}

func TestGenerateSBPL_AllowNetwork(t *testing.T) {
	cfg := canonicalConfig()
	cfg.AllowNetwork = true
	if strings.Contains(GenerateSBPL(cfg), "(deny network*)") {
		t.Error("AllowNetwork=true should not deny network")
	}
	cfg.AllowNetwork = false
	if !strings.Contains(GenerateSBPL(cfg), "(deny network*)") {
		t.Error("AllowNetwork=false should deny network")
	}
}

func TestGenerateBwrapArgv_BaseRules(t *testing.T) {
	cfg := canonicalConfig()
	innerArgv := []string{"claude", "-p", "go"}
	args := GenerateBwrapArgv(cfg, innerArgv)
	mustContainArg(t, args, "--ro-bind", "/usr", "/usr")
	mustContainArg(t, args, "--ro-bind", "/bin", "/bin")
	mustContainArg(t, args, "--ro-bind", "/etc", "/etc")
	mustContainArg(t, args, "--proc", "/proc")
	mustContainArg(t, args, "--dev", "/dev")
	mustContainArg(t, args, "--tmpfs", "/tmp")
	mustContainArg(t, args, "--unshare-user")
	mustContainArg(t, args, "--unshare-pid")
	if !endsWithArgv(args, innerArgv) {
		t.Errorf("inner argv missing at tail: %v", args)
	}
}

func TestGenerateBwrapArgv_HomeStateDirsTryBoundRW(t *testing.T) {
	args := GenerateBwrapArgv(canonicalConfig(), []string{"codex"})
	mustContainArg(t, args, "--bind-try", "/Users/test/.claude", "/Users/test/.claude")
	mustContainArg(t, args, "--bind-try", "/Users/test/.codex", "/Users/test/.codex")
	mustContainArg(t, args, "--bind-try", "/Users/test/.gemini", "/Users/test/.gemini")
}

func TestGenerateBwrapArgv_RepoBind(t *testing.T) {
	cfg := canonicalConfig()
	cfg.ReadOnlyRepo = false
	args := GenerateBwrapArgv(cfg, []string{"true"})
	mustContainArg(t, args, "--bind", "/repo", "/repo")

	cfg.ReadOnlyRepo = true
	args = GenerateBwrapArgv(cfg, []string{"true"})
	mustContainArg(t, args, "--ro-bind", "/repo", "/repo")
}

func TestGenerateBwrapArgv_WritePathsRebound(t *testing.T) {
	cfg := canonicalConfig()
	cfg.ReadOnlyRepo = true
	cfg.WritePaths = []string{"/repo/.evolve/runs"}
	args := GenerateBwrapArgv(cfg, []string{"true"})
	mustContainArg(t, args, "--bind", "/repo/.evolve/runs", "/repo/.evolve/runs")
}

func TestGenerateBwrapArgv_NetworkOff(t *testing.T) {
	cfg := canonicalConfig()
	cfg.AllowNetwork = false
	args := GenerateBwrapArgv(cfg, []string{"true"})
	mustContainArg(t, args, "--unshare-net")
}

func TestProbe_DetectsAvailableSandboxer(t *testing.T) {
	pr := Probe()
	if pr.OS == "" {
		t.Error("Probe.OS empty")
	}
	if pr.OS != runtime.GOOS {
		t.Errorf("Probe.OS=%q, want runtime.GOOS=%q", pr.OS, runtime.GOOS)
	}
	if runtime.GOOS == "darwin" && !pr.Available {
		t.Errorf("on macOS, Probe.Available should be true (sandbox-exec at /usr/bin/sandbox-exec)")
	}
}

func TestProbe_BinaryPathPopulated(t *testing.T) {
	pr := Probe()
	if pr.Available && pr.BinaryPath == "" {
		t.Errorf("Available=true but BinaryPath empty: %+v", pr)
	}
}

func TestProbeFor_LinuxWithBwrap(t *testing.T) {
	look := func(name string) (string, error) {
		if name == "bwrap" {
			return "/usr/bin/bwrap", nil
		}
		return "", fakeNotFound(name)
	}
	pr := probeFor("linux", look)
	if !pr.Available || pr.BinaryPath != "/usr/bin/bwrap" {
		t.Errorf("linux+bwrap: %+v", pr)
	}
}

func TestProbeFor_LinuxNoBwrap(t *testing.T) {
	look := func(name string) (string, error) { return "", fakeNotFound(name) }
	pr := probeFor("linux", look)
	if pr.Available {
		t.Error("Available=true when bwrap absent")
	}
	if !strings.Contains(pr.Reason, "bwrap") {
		t.Errorf("Reason=%q missing 'bwrap'", pr.Reason)
	}
}

func TestProbeFor_DarwinNoSandboxExec(t *testing.T) {
	look := func(string) (string, error) { return "", fakeNotFound("missing") }
	pr := probeFor("darwin", look)
	if pr.Available {
		t.Error("Available=true with no sandbox-exec")
	}
}

func TestProbeFor_UnsupportedOS(t *testing.T) {
	pr := probeFor("windows", func(string) (string, error) { return "", nil })
	if pr.Available {
		t.Error("Available=true on unsupported OS")
	}
	if !strings.Contains(pr.Reason, "windows") {
		t.Errorf("Reason=%q missing GOOS", pr.Reason)
	}
}

func TestGenerateSBPL_EmptyHomeDirSkipsHomeRules(t *testing.T) {
	cfg := canonicalConfig()
	cfg.HomeDir = ""
	out := GenerateSBPL(cfg)
	if strings.Contains(out, ".claude") {
		t.Errorf("HomeDir empty but SBPL has .claude rule: %s", excerpt(out, ".claude"))
	}
}

func TestGenerateSBPL_EmptyWritePathSkipped(t *testing.T) {
	cfg := canonicalConfig()
	cfg.WritePaths = []string{"", "/repo/.evolve/runs", ""}
	out := GenerateSBPL(cfg)
	if strings.Contains(out, `(subpath "")`) {
		t.Errorf("empty write path leaked into SBPL: %s", excerpt(out, "subpath \"\""))
	}
}

func TestGenerateSBPL_EmptyDenyPathSkipped(t *testing.T) {
	cfg := canonicalConfig()
	cfg.DenyPaths = []string{"", "/repo/.git"}
	out := GenerateSBPL(cfg)
	if strings.Contains(out, `(deny file-write* (subpath ""))`) {
		t.Error("empty deny path leaked")
	}
	if !strings.Contains(out, `(deny file-write* (subpath "/repo/.git"))`) {
		t.Error("real deny path missing")
	}
}

func TestGenerateBwrapArgv_EmptyHomeAndRepo(t *testing.T) {
	cfg := Config{AllowNetwork: true}
	args := GenerateBwrapArgv(cfg, []string{"true"})
	for i, a := range args {
		if a == "--bind" || a == "--ro-bind" {
			if i+2 >= len(args) || args[i+1] == "" || args[i+2] == "" {
				t.Errorf("bind/ro-bind with empty path at idx=%d: %v", i, args)
			}
		}
	}
}

func TestGenerateBwrapArgv_GlobInWritePathWidens(t *testing.T) {
	cfg := canonicalConfig()
	cfg.WritePaths = []string{"/repo/.evolve/runs/cycle-*"}
	args := GenerateBwrapArgv(cfg, []string{"true"})
	mustContainArg(t, args, "--bind", "/repo/.evolve/runs", "/repo/.evolve/runs")
}

func TestNewAndExec_Unwrapped(t *testing.T) {
	s := &Sandbox{
		probe: ProbeResult{OS: "unsupported", Available: false, Reason: "test stub"},
		cfg:   canonicalConfig(),
	}
	var stderr strings.Builder
	err := s.Exec(context.Background(), []string{"/usr/bin/true"}, nil, nil, &stderr)
	if err != nil {
		t.Errorf("Exec(/usr/bin/true): %v", err)
	}
	if !strings.Contains(stderr.String(), "NOTE") {
		t.Errorf("expected stderr NOTE about unwrapped exec, got %q", stderr.String())
	}
}

func TestExec_EmptyArgvFails(t *testing.T) {
	s := New(canonicalConfig())
	err := s.Exec(context.Background(), nil, nil, nil, nil)
	if err == nil {
		t.Error("Exec(nil argv): want error")
	}
}

func TestNew_BindsProbeAtConstruction(t *testing.T) {
	cfg := canonicalConfig()
	s := New(cfg)
	if s == nil {
		t.Fatal("New=nil")
	}
	if s.cfg.RepoRoot != cfg.RepoRoot {
		t.Errorf("cfg not stored: %+v", s.cfg)
	}
	if s.probe.OS != runtime.GOOS {
		t.Errorf("probe.OS=%q, want runtime.GOOS=%q", s.probe.OS, runtime.GOOS)
	}
}

func TestExec_DarwinSandboxExec(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only")
	}
	requireCapableSandbox(t)
	s := New(canonicalConfig())
	var stderr strings.Builder
	err := s.Exec(context.Background(), []string{"/usr/bin/true"}, nil, nil, &stderr)
	if err != nil {
		t.Errorf("Exec /usr/bin/true under sandbox-exec: %v (stderr: %s)", err, stderr.String())
	}
}

func TestExec_LinuxBwrap(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only")
	}
	pr := Probe()
	if !pr.Available {
		t.Skip("bwrap not available")
	}
	s := New(canonicalConfig())
	err := s.Exec(context.Background(), []string{"/bin/true"}, nil, nil, nil)
	if err != nil {
		t.Errorf("Exec /bin/true under bwrap: %v", err)
	}
}

type fakeNotFoundErr struct{ name string }

func (e fakeNotFoundErr) Error() string { return "not found: " + e.name }
func fakeNotFound(name string) error    { return fakeNotFoundErr{name} }

func mustContain(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(haystack, n) {
			t.Errorf("missing fragment %q", n)
		}
	}
}

func mustContainArg(t *testing.T, args []string, parts ...string) {
	t.Helper()
	for i := 0; i+len(parts) <= len(args); i++ {
		match := true
		for j := range parts {
			if args[i+j] != parts[j] {
				match = false
				break
			}
		}
		if match {
			return
		}
	}
	t.Errorf("argv missing sequence %v\n  full: %v", parts, args)
}

func endsWithArgv(args, suffix []string) bool {
	if len(args) < len(suffix) {
		return false
	}
	for i := range suffix {
		if args[len(args)-len(suffix)+i] != suffix[i] {
			return false
		}
	}
	return true
}

func excerpt(s, needle string) string {
	i := strings.Index(s, needle)
	if i == -1 {
		return "(needle not present)"
	}
	start := i - 40
	if start < 0 {
		start = 0
	}
	end := i + 80
	if end > len(s) {
		end = len(s)
	}
	return "..." + s[start:end] + "..."
}

func requireCapableSandbox(t *testing.T) ProbeResult {
	t.Helper()
	probe := Probe()
	if !probe.Available || !probe.CapabilityChecked || !probe.Capable {
		testhost.Unavailable(t, capabilityDiagnosis(probe))
	}
	return probe
}

func capabilityDiagnosis(probe ProbeResult) string {
	if probe.OS != "darwin" || !probe.Available {
		return fmt.Sprintf("the sandbox is unavailable on %s: %s", probe.OS, probe.Reason)
	}
	const sbpl = "(version 1)(allow default)"
	out, err := exec.Command(probe.BinaryPath, "-p", sbpl, "/usr/bin/true").CombinedOutput()
	return fmt.Sprintf("the sandbox cannot apply on this runner: binary=%s probe=%q apply error=%v output=%q SBPL=%q",
		probe.BinaryPath, probe.Reason, err, strings.TrimSpace(string(out)), sbpl)
}
