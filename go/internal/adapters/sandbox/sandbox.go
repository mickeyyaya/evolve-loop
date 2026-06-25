// Package sandbox ports the OS-level sandboxing layer from
// scripts/cli_adapters/claude.sh:generate_macos_sandbox_profile
// (macOS sandbox-exec / SBPL) and its Linux bwrap equivalent.
//
// The generators (GenerateSBPL / GenerateBwrapArgv) are pure functions
// so unit tests run on any host. Probe + Exec dispatch on runtime.GOOS
// and degrade gracefully (NOTE-level WARN) when the host's sandbox
// binary is absent — matching the bash semantics at claude.sh:666.
package sandbox

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Config carries the sandbox parameters extracted from a profile.
// All paths are absolute (no globs, no placeholders); the orchestrator
// resolves these before handing them off.
type Config struct {
	RepoRoot     string
	HomeDir      string
	ReadOnlyRepo bool
	AllowNetwork bool
	WritePaths   []string // explicit write allows
	DenyPaths    []string // explicit write denies (claude.sh:540 deny loop)
}

// ProbeResult reports the host's sandboxing capability.
type ProbeResult struct {
	OS         string // runtime.GOOS
	Available  bool   // sandbox-exec / bwrap present on PATH
	BinaryPath string // resolved path when Available
	Reason     string // diagnostic when !Available or when sandbox_apply fails

	// Capable reports whether the sandbox binary actually APPLIES on this host —
	// a measured fact, not the env-var nested guess. Under nested-Claude on
	// macOS, sandbox_apply() returns EPERM, so the binary is Available but not
	// Capable. Only meaningful when CapabilityChecked is true.
	Capable bool
	// CapabilityChecked distinguishes "Capable=false because measured-incapable"
	// from "Capable=false because never measured" (no binary, or no probe). A
	// consumer treats !CapabilityChecked as "unknown — fall back to the legacy
	// heuristic" so unmeasured callers are byte-identical to pre-measurement.
	CapabilityChecked bool
}

// LookPathFunc is the seam for injecting `exec.LookPath` behavior in
// tests. probeFor exposes both axes (goos + look) for table-driven
// tests; Probe is the production entry point.
type LookPathFunc func(string) (string, error)

// capabilityProbe is the seam for MEASURING whether the sandbox binary applies:
// it execs a trivial no-op confined command and returns the exec error (nil ⇒
// the sandbox applied). Injected in tests; defaultCapabilityProbe is production.
type capabilityProbe func(ctx context.Context, binaryPath string) error

// capabilityTimeout bounds the measurement exec. A trivial confined /usr/bin/true
// returns near-instantly when the sandbox applies and EPERM-fast when it can't;
// the timeout is the guarantee that a pathological host can never reintroduce the
// 80s REPL-boot hang this measurement exists to prevent. Timeout ⇒ not capable.
const capabilityTimeout = 3 * time.Second

// probeOnce caches the process-lifetime probe (including the capability exec) so
// every consumer — bridge ShouldWrap, preflight — shares ONE measured value and
// the exec runs at most once.
var (
	probeOnce   sync.Once
	probedValue ProbeResult
)

// Probe inspects the host for an available sandbox binary AND measures whether
// it actually applies. Cached once per process.
func Probe() ProbeResult {
	probeOnce.Do(func() {
		probedValue = measureCapability(probeFor(runtime.GOOS, exec.LookPath), defaultCapabilityProbe)
	})
	return probedValue
}

// measureCapability augments a PATH-presence probe with a measured "does it
// actually apply?" signal. !Available or a nil probe leaves CapabilityChecked
// false (unknown); otherwise it execs the probe under a bounded timeout and
// records Capable + (on failure) the diagnostic in Reason.
func measureCapability(pr ProbeResult, probe capabilityProbe) ProbeResult {
	if !pr.Available || probe == nil {
		return pr
	}
	ctx, cancel := context.WithTimeout(context.Background(), capabilityTimeout)
	defer cancel()
	pr.CapabilityChecked = true
	if err := probe(ctx, pr.BinaryPath); err != nil {
		pr.Capable = false
		pr.Reason = fmt.Sprintf("sandbox binary present but sandbox_apply failed (%v)", err)
	} else {
		pr.Capable = true
	}
	return pr
}

// defaultCapabilityProbe runs the smallest possible confined no-op (/usr/bin/true
// has no REPL, so it cannot hang) to measure whether sandbox_apply succeeds.
func defaultCapabilityProbe(ctx context.Context, binaryPath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, binaryPath, "-p", "(version 1)(allow default)", "/usr/bin/true")
	case "linux":
		cmd = exec.CommandContext(ctx, binaryPath, "--ro-bind", "/", "/", "/bin/true")
	default:
		// Unreachable in production: measureCapability only probes when Available,
		// which probeFor sets solely for darwin/linux.
		return fmt.Errorf("no capability probe for GOOS=%s", runtime.GOOS)
	}
	// WaitDelay bounds the post-kill drain: after the context timeout fires and
	// the child is signalled, Run() is still guaranteed to return promptly even
	// if a pathological host (the exact case this probe detects) wedges a
	// grandchild holding the pipes — it can never reintroduce the boot-hang.
	cmd.WaitDelay = capabilityTimeout
	return cmd.Run()
}

func probeFor(goos string, look LookPathFunc) ProbeResult {
	pr := ProbeResult{OS: goos}
	switch goos {
	case "darwin":
		if p, err := look("sandbox-exec"); err == nil {
			pr.Available = true
			pr.BinaryPath = p
		} else {
			pr.Reason = "sandbox-exec not on PATH"
		}
	case "linux":
		if p, err := look("bwrap"); err == nil {
			pr.Available = true
			pr.BinaryPath = p
		} else {
			pr.Reason = "bwrap not on PATH (install bubblewrap)"
		}
	default:
		pr.Reason = fmt.Sprintf("no sandbox impl for GOOS=%s", goos)
	}
	return pr
}

// GenerateSBPL emits a macOS sandbox-exec profile string for the
// given Config. Mirrors generate_macos_sandbox_profile at
// claude.sh:437-560.
//
// SBPL rule ordering matters: later rules override earlier ones for
// overlapping subpaths. The function preserves bash's order so that
// (deny file-write* (subpath repo)) followed by per-write-path allows
// produces a read-only repo with cycle-specific write zones.
func GenerateSBPL(cfg Config) string {
	var b strings.Builder
	// Header — system.sb provides baseline access (dyld, libsystem, etc).
	b.WriteString("(version 1)\n")
	b.WriteString("(deny default)\n")
	b.WriteString(`(import "system.sb")` + "\n")
	b.WriteString("(allow process-exec)\n")
	b.WriteString("(allow process-fork)\n")
	b.WriteString("(allow signal)\n")
	b.WriteString("(allow sysctl-read)\n")
	b.WriteString("(allow mach-lookup)\n")
	b.WriteString("(allow ipc-posix-shm)\n")
	b.WriteString("(allow file-read-metadata)\n")
	// Always-readable subpaths.
	for _, p := range []string{
		cfg.RepoRoot, "/usr", "/System", "/Library",
		"/private/etc", "/opt", "/bin", "/sbin", "/var",
		"/private/var", "/dev",
	} {
		fmt.Fprintf(&b, "(allow file-read* (subpath %q))\n", p)
	}
	// HOME read.
	if cfg.HomeDir != "" {
		fmt.Fprintf(&b, "(allow file-read* (subpath %q))\n", cfg.HomeDir)
	}
	// /tmp + /var/folders: read+write both (bash:477 load-bearing note).
	for _, p := range []string{"/tmp", "/private/tmp", "/var/folders", "/private/var/folders"} {
		fmt.Fprintf(&b, "(allow file-read* (subpath %q))\n", p)
		fmt.Fprintf(&b, "(allow file-write* (subpath %q))\n", p)
	}
	// HOME writes for known Claude config dirs.
	if cfg.HomeDir != "" {
		for _, sub := range []string{".claude", ".cache", ".config", "Library/Caches", "Library/Application Support"} {
			fmt.Fprintf(&b, "(allow file-write* (subpath %q))\n", filepath.Join(cfg.HomeDir, sub))
		}
	}
	// ReadOnlyRepo: explicit deny before per-write-path allows so later
	// rules re-permit specific subdirs. Mirrors bash:511-517 contract.
	if cfg.ReadOnlyRepo && cfg.RepoRoot != "" {
		fmt.Fprintf(&b, "(deny file-write* (subpath %q))\n", cfg.RepoRoot)
	}
	// Per-write-path allows. Globs widen to parent dir (bash:520).
	for _, wp := range cfg.WritePaths {
		if wp == "" {
			continue
		}
		if strings.Contains(wp, "*") {
			wp = filepath.Dir(wp)
		}
		fmt.Fprintf(&b, "(allow file-write* (subpath %q))\n", wp)
	}
	// Per-agent deny paths (kernel-enforced mirror of disallowed_tools).
	for _, dp := range cfg.DenyPaths {
		if dp == "" {
			continue
		}
		fmt.Fprintf(&b, "(deny file-write* (subpath %q))\n", dp)
	}
	// Network.
	if !cfg.AllowNetwork {
		b.WriteString("(deny network*)\n")
	}
	return b.String()
}

// GenerateBwrapArgv emits the bubblewrap argv for the given Config and
// inner command. Linux equivalent of generate_macos_sandbox_profile.
// Equivalent to append(BwrapPrefix(cfg), innerArgv...) — kept for backcompat;
// new callers that only need the prefix (e.g. the bridge SandboxWrap seam)
// should call BwrapPrefix directly to avoid the throwaway append.
func GenerateBwrapArgv(cfg Config, innerArgv []string) []string {
	return append(BwrapPrefix(cfg), innerArgv...)
}

// BwrapPrefix returns the bwrap prefix argv (no inner). Workstream B uses it
// from the bridge SandboxWrap seam, which composes the full argv at the
// driver call site (the inner argv is the per-CLI binary + flags resolved
// later).
func BwrapPrefix(cfg Config) []string {
	out := []string{
		"--unshare-user",
		"--unshare-pid",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
	}
	// Baseline read-only system paths.
	for _, p := range []string{"/usr", "/bin", "/lib", "/lib64", "/etc"} {
		out = append(out, "--ro-bind", p, p)
	}
	// HOME read-only by default.
	if cfg.HomeDir != "" {
		out = append(out, "--ro-bind", cfg.HomeDir, cfg.HomeDir)
	}
	// Repo: rw or ro by ReadOnlyRepo.
	if cfg.RepoRoot != "" {
		if cfg.ReadOnlyRepo {
			out = append(out, "--ro-bind", cfg.RepoRoot, cfg.RepoRoot)
		} else {
			out = append(out, "--bind", cfg.RepoRoot, cfg.RepoRoot)
		}
	}
	// Re-bind write paths as rw (overrides repo ro for these subdirs).
	for _, wp := range cfg.WritePaths {
		if wp == "" {
			continue
		}
		if strings.Contains(wp, "*") {
			wp = filepath.Dir(wp)
		}
		out = append(out, "--bind", wp, wp)
	}
	// Network: bwrap uses namespace isolation, not allow/deny rules.
	if !cfg.AllowNetwork {
		out = append(out, "--unshare-net")
	}
	return out
}

// Sandbox is the runtime-dispatching exec wrapper for ad-hoc CLI use. Exec wraps
// the given argv in the host's sandbox primitive (sandbox-exec/bwrap)
// when available, else runs unwrapped with a stderr NOTE.
type Sandbox struct {
	probe ProbeResult
	cfg   Config
}

// New constructs a Sandbox bound to a Config. Probe runs once at
// construction; subsequent Exec calls don't re-probe.
func New(cfg Config) *Sandbox {
	return &Sandbox{probe: Probe(), cfg: cfg}
}

// Exec runs argv under the host's sandbox. When the host has no
// available sandbox impl, Exec falls back to plain exec and logs a
// NOTE to stderr (mirroring bash:666 behavior).
func (s *Sandbox) Exec(ctx context.Context, argv []string,
	stdin io.Reader, stdout, stderr io.Writer) error {
	if len(argv) == 0 {
		return fmt.Errorf("sandbox: argv empty")
	}
	var cmd *exec.Cmd
	switch {
	case s.probe.Available && runtime.GOOS == "darwin":
		profile := GenerateSBPL(s.cfg)
		args := append([]string{"-p", profile}, argv...)
		cmd = exec.CommandContext(ctx, s.probe.BinaryPath, args...)
	case s.probe.Available && runtime.GOOS == "linux":
		args := GenerateBwrapArgv(s.cfg, argv)
		cmd = exec.CommandContext(ctx, s.probe.BinaryPath, args...)
	default:
		fmt.Fprintf(stderr, "[sandbox] NOTE: %s; running unwrapped\n", s.probe.Reason)
		cmd = exec.CommandContext(ctx, argv[0], argv[1:]...)
	}
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
