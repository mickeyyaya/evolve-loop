// Package sandbox ports OS-level sandboxing from claude.sh:
// generate_macos_sandbox_profile (SBPL for sandbox-exec) and the
// equivalent bwrap argv generator. The generators are pure functions
// so they run on any host; Exec dispatches by runtime.GOOS.
package sandbox

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

// canonicalConfig — minimal Config matching the production scout
// profile shape.
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

// TestGenerateSBPL_HeaderAndBaseline — every SBPL output must start
// with the version directive and import system.sb. This is the macOS
// kernel contract; missing either renders the profile unusable.
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

// TestGenerateSBPL_RepoReadAlwaysAllowed — repo subpath is always
// readable (claude needs to read the codebase regardless of
// read_only_repo).
func TestGenerateSBPL_RepoReadAlwaysAllowed(t *testing.T) {
	out := GenerateSBPL(canonicalConfig())
	if !strings.Contains(out, `(allow file-read* (subpath "/repo"))`) {
		t.Errorf("missing repo read-allow: %s", excerpt(out, "/repo"))
	}
}

// TestGenerateSBPL_HomeDirReadAndKnownWrites — HOME is readable + the
// canonical Claude config dirs are writable. Without these, claude
// blocks at startup trying to read ~/.claude or write a cache.
func TestGenerateSBPL_HomeDirReadAndKnownWrites(t *testing.T) {
	out := GenerateSBPL(canonicalConfig())
	mustContain(t, out,
		`(allow file-read* (subpath "/Users/test"))`,
		`(allow file-write* (subpath "/Users/test/.claude"))`,
		`(allow file-write* (subpath "/Users/test/.cache"))`,
		`(allow file-write* (subpath "/Users/test/.config"))`,
	)
}

// TestGenerateSBPL_TmpReadWriteBoth — both /tmp and /var/folders need
// read+write. claude.sh:477 explains this is load-bearing for bash
// tool output files. Missing the read rule was the root cause of
// cycles 8121-8128 build failures.
func TestGenerateSBPL_TmpReadWriteBoth(t *testing.T) {
	out := GenerateSBPL(canonicalConfig())
	mustContain(t, out,
		`(allow file-read* (subpath "/tmp"))`,
		`(allow file-write* (subpath "/tmp"))`,
		`(allow file-read* (subpath "/var/folders"))`,
		`(allow file-write* (subpath "/var/folders"))`,
	)
}

// TestGenerateSBPL_ReadOnlyRepo_AddsDenyBeforeWriteAllows — when
// ReadOnlyRepo=true, an explicit (deny file-write* (subpath repo))
// must appear, and it must come BEFORE the per-write-path allows so
// the later rules can re-permit the cycle-specific subdirs.
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

// TestGenerateSBPL_WritePathsAddedAsSubpath — each entry of
// WritePaths becomes an `(allow file-write* (subpath "X"))` rule.
func TestGenerateSBPL_WritePathsAddedAsSubpath(t *testing.T) {
	cfg := canonicalConfig()
	cfg.WritePaths = []string{"/repo/a", "/repo/b/c"}
	out := GenerateSBPL(cfg)
	mustContain(t, out,
		`(allow file-write* (subpath "/repo/a"))`,
		`(allow file-write* (subpath "/repo/b/c"))`,
	)
}

// TestGenerateSBPL_GlobInWritePath_WidensToParent — bash:520
// translates "cycle-*" → parent dir because SBPL subpath doesn't
// interpret globs.
func TestGenerateSBPL_GlobInWritePath_WidensToParent(t *testing.T) {
	cfg := canonicalConfig()
	cfg.WritePaths = []string{"/repo/.evolve/runs/cycle-*"}
	out := GenerateSBPL(cfg)
	if !strings.Contains(out, `(allow file-write* (subpath "/repo/.evolve/runs"))`) {
		t.Errorf("glob path not widened to parent: %s", excerpt(out, "cycle"))
	}
	if strings.Contains(out, "cycle-*") {
		t.Errorf("literal glob leaked into SBPL: %s", excerpt(out, "cycle"))
	}
}

// TestGenerateSBPL_DenyPaths — kernel-enforced mirror of
// disallowed_tools file patterns. claude.sh:540+ deny loop.
func TestGenerateSBPL_DenyPaths(t *testing.T) {
	cfg := canonicalConfig()
	cfg.DenyPaths = []string{"/repo/.git", "/repo/.evolve/state.json"}
	out := GenerateSBPL(cfg)
	mustContain(t, out,
		`(deny file-write* (subpath "/repo/.git"))`,
		`(deny file-write* (subpath "/repo/.evolve/state.json"))`,
	)
}

// TestGenerateSBPL_AllowNetwork — when true, no network deny is
// emitted; when false, an explicit deny appears.
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

// TestGenerateBwrapArgv_BaseRules — bwrap argv must include the
// essential bind-mounts: /usr ro, /lib ro, /bin ro, /etc ro, repo
// (ro/rw depending on ReadOnlyRepo), $HOME (ro for read-only Claude
// config), and an unprivileged user namespace.
func TestGenerateBwrapArgv_BaseRules(t *testing.T) {
	cfg := canonicalConfig()
	innerArgv := []string{"claude", "-p", "go"}
	args := GenerateBwrapArgv(cfg, innerArgv)
	// First arg is the bwrap binary's expected first flag block.
	mustContainArg(t, args, "--ro-bind", "/usr", "/usr")
	mustContainArg(t, args, "--ro-bind", "/bin", "/bin")
	mustContainArg(t, args, "--ro-bind", "/etc", "/etc")
	mustContainArg(t, args, "--proc", "/proc")
	mustContainArg(t, args, "--dev", "/dev")
	mustContainArg(t, args, "--tmpfs", "/tmp")
	mustContainArg(t, args, "--unshare-user")
	mustContainArg(t, args, "--unshare-pid")
	// Inner argv must be present at the tail.
	if !endsWithArgv(args, innerArgv) {
		t.Errorf("inner argv missing at tail: %v", args)
	}
}

// TestGenerateBwrapArgv_RepoBind — bind-mount the repo dir, read-write
// by default and read-only when ReadOnlyRepo=true.
func TestGenerateBwrapArgv_RepoBind(t *testing.T) {
	cfg := canonicalConfig()
	cfg.ReadOnlyRepo = false
	args := GenerateBwrapArgv(cfg, []string{"true"})
	mustContainArg(t, args, "--bind", "/repo", "/repo")

	cfg.ReadOnlyRepo = true
	args = GenerateBwrapArgv(cfg, []string{"true"})
	mustContainArg(t, args, "--ro-bind", "/repo", "/repo")
}

// TestGenerateBwrapArgv_WritePathsRebound — when ReadOnlyRepo=true,
// each WritePaths entry needs an explicit --bind to re-grant write.
func TestGenerateBwrapArgv_WritePathsRebound(t *testing.T) {
	cfg := canonicalConfig()
	cfg.ReadOnlyRepo = true
	cfg.WritePaths = []string{"/repo/.evolve/runs"}
	args := GenerateBwrapArgv(cfg, []string{"true"})
	mustContainArg(t, args, "--bind", "/repo/.evolve/runs", "/repo/.evolve/runs")
}

// TestGenerateBwrapArgv_NetworkOff — when AllowNetwork=false, bwrap
// must --unshare-net (deny networking via namespace isolation).
func TestGenerateBwrapArgv_NetworkOff(t *testing.T) {
	cfg := canonicalConfig()
	cfg.AllowNetwork = false
	args := GenerateBwrapArgv(cfg, []string{"true"})
	mustContainArg(t, args, "--unshare-net")
}

// TestProbe_DetectsAvailableSandboxer — on macOS we expect
// sandbox-exec available; on Linux we expect bwrap (or none if not
// installed). The test asserts the probe doesn't error and returns
// reasonable info for the current host.
func TestProbe_DetectsAvailableSandboxer(t *testing.T) {
	pr := Probe()
	if pr.OS == "" {
		t.Error("Probe.OS empty")
	}
	if pr.OS != runtime.GOOS {
		t.Errorf("Probe.OS=%q, want runtime.GOOS=%q", pr.OS, runtime.GOOS)
	}
	// Available bool reflects sandbox-exec or bwrap presence; on
	// dev macOS sandbox-exec is always present.
	if runtime.GOOS == "darwin" && !pr.Available {
		t.Errorf("on macOS, Probe.Available should be true (sandbox-exec at /usr/bin/sandbox-exec)")
	}
}

// TestProbe_BinaryPathPopulated — when Available, BinaryPath must be
// non-empty so the caller can log it.
func TestProbe_BinaryPathPopulated(t *testing.T) {
	pr := Probe()
	if pr.Available && pr.BinaryPath == "" {
		t.Errorf("Available=true but BinaryPath empty: %+v", pr)
	}
}

// TestProbeFor_LinuxWithBwrap — probe linux branch with mocked LookPath.
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

// TestProbeFor_LinuxNoBwrap — probe linux branch when bwrap missing.
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

// TestProbeFor_DarwinNoSandboxExec — degraded macOS.
func TestProbeFor_DarwinNoSandboxExec(t *testing.T) {
	look := func(string) (string, error) { return "", fakeNotFound("missing") }
	pr := probeFor("darwin", look)
	if pr.Available {
		t.Error("Available=true with no sandbox-exec")
	}
}

// TestProbeFor_UnsupportedOS — windows/freebsd/etc. yield no impl.
func TestProbeFor_UnsupportedOS(t *testing.T) {
	pr := probeFor("windows", func(string) (string, error) { return "", nil })
	if pr.Available {
		t.Error("Available=true on unsupported OS")
	}
	if !strings.Contains(pr.Reason, "windows") {
		t.Errorf("Reason=%q missing GOOS", pr.Reason)
	}
}

// TestGenerateSBPL_EmptyHomeDirSkipsHomeRules — when HomeDir is unset
// (no operator HOME context), the HOME-keyed rules are skipped entirely.
func TestGenerateSBPL_EmptyHomeDirSkipsHomeRules(t *testing.T) {
	cfg := canonicalConfig()
	cfg.HomeDir = ""
	out := GenerateSBPL(cfg)
	if strings.Contains(out, ".claude") {
		t.Errorf("HomeDir empty but SBPL has .claude rule: %s", excerpt(out, ".claude"))
	}
}

// TestGenerateSBPL_EmptyWritePathSkipped — entries that are "" must be
// silently dropped (defensive against caller bugs).
func TestGenerateSBPL_EmptyWritePathSkipped(t *testing.T) {
	cfg := canonicalConfig()
	cfg.WritePaths = []string{"", "/repo/.evolve/runs", ""}
	out := GenerateSBPL(cfg)
	// Empty rule would render as `(allow file-write* (subpath ""))` — verify absent.
	if strings.Contains(out, `(subpath "")`) {
		t.Errorf("empty write path leaked into SBPL: %s", excerpt(out, "subpath \"\""))
	}
}

// TestGenerateSBPL_EmptyDenyPathSkipped — same defense for deny paths.
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

// TestGenerateBwrapArgv_EmptyHomeAndRepo — defensive: empty paths
// don't emit malformed bind args.
func TestGenerateBwrapArgv_EmptyHomeAndRepo(t *testing.T) {
	cfg := Config{AllowNetwork: true}
	args := GenerateBwrapArgv(cfg, []string{"true"})
	// No --bind with empty src/dst pairs.
	for i, a := range args {
		if a == "--bind" || a == "--ro-bind" {
			// Next two args must be non-empty.
			if i+2 >= len(args) || args[i+1] == "" || args[i+2] == "" {
				t.Errorf("bind/ro-bind with empty path at idx=%d: %v", i, args)
			}
		}
	}
}

// TestGenerateBwrapArgv_GlobInWritePathWidens — same parity as SBPL.
func TestGenerateBwrapArgv_GlobInWritePathWidens(t *testing.T) {
	cfg := canonicalConfig()
	cfg.WritePaths = []string{"/repo/.evolve/runs/cycle-*"}
	args := GenerateBwrapArgv(cfg, []string{"true"})
	mustContainArg(t, args, "--bind", "/repo/.evolve/runs", "/repo/.evolve/runs")
}

// TestNewAndExec_Unwrapped — when sandbox binary is absent (or OS
// unsupported), Exec falls back to plain exec and logs a NOTE.
// Uses /usr/bin/true (universal POSIX no-op).
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

// TestExec_EmptyArgvFails — explicit error contract.
func TestExec_EmptyArgvFails(t *testing.T) {
	s := New(canonicalConfig())
	err := s.Exec(context.Background(), nil, nil, nil, nil)
	if err == nil {
		t.Error("Exec(nil argv): want error")
	}
}

// TestNew_BindsProbeAtConstruction — calling New once must capture the
// probe result; subsequent calls return the same Sandbox config.
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

// TestExec_DarwinSandboxExec — on darwin with sandbox-exec available,
// Exec actually wraps the inner command. /usr/bin/true returns 0 with
// no file I/O, so even a minimal SBPL grants enough.
func TestExec_DarwinSandboxExec(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only")
	}
	pr := Probe()
	if !pr.Available {
		t.Skip("sandbox-exec not available on this host")
	}
	s := New(canonicalConfig())
	var stderr strings.Builder
	err := s.Exec(context.Background(), []string{"/usr/bin/true"}, nil, nil, &stderr)
	if err != nil {
		t.Errorf("Exec /usr/bin/true under sandbox-exec: %v (stderr: %s)", err, stderr.String())
	}
}

// TestExec_LinuxBwrap — on linux with bwrap available; skipped on
// darwin. The shape is symmetric with the darwin test.
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

// fakeNotFound — minimal os.ErrNotExist-shaped error used by mocked
// LookPath callbacks.
type fakeNotFoundErr struct{ name string }

func (e fakeNotFoundErr) Error() string { return "not found: " + e.name }
func fakeNotFound(name string) error    { return fakeNotFoundErr{name} }

// Helpers ------------------------------------------------------------

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
