// Package sandbox generates OS sandbox profiles and measures whether the host applies them.
package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Config struct {
	TerminalPath  string
	RepoRoot      string
	HomeDir       string
	ReadOnlyRepo  bool
	AllowNetwork  bool
	WritePaths    []string
	DenyPaths     []string
	DenyReadPaths []string
	DenySockets   []string
	DenyLiterals  []string

	ExecDenyPaths   []string
	ExecDenyRoots   []string
	ExecDenyEnforce bool

	ReadIsolation ReadIsolation
}

func homeStateWriteDirs(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".codex"),
		filepath.Join(home, ".gemini"),
		filepath.Join(home, ".cache"),
		filepath.Join(home, ".config"),
		filepath.Join(home, "Library/Caches"),
		filepath.Join(home, "Library/Application Support"),
	}
}

type ProbeResult struct {
	OS         string
	Available  bool
	BinaryPath string
	Reason     string

	Capable           bool
	CapabilityChecked bool
}

type LookPathFunc func(string) (string, error)

type capabilityProbe func(ctx context.Context, binaryPath string) error

const capabilityTimeout = 3 * time.Second

var hostOS = runtime.GOOS

var (
	probeOnce   sync.Once
	probedValue ProbeResult
)

func Probe() ProbeResult {
	probeOnce.Do(func() {
		probedValue = measureCapability(probeFor(hostOS, exec.LookPath), defaultCapabilityProbe)
	})
	return probedValue
}

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

func defaultCapabilityProbe(ctx context.Context, binaryPath string) error {
	var cmd *exec.Cmd
	switch hostOS {
	case "darwin":
		cmd = sysexec.Command(ctx, binaryPath, "-p", "(version 1)(allow default)", "/usr/bin/true")
	case "linux":
		cmd = sysexec.Command(ctx, binaryPath, "--ro-bind", "/", "/", "/bin/true")
	default:
		return fmt.Errorf("no capability probe for GOOS=%s", hostOS)
	}
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

func GenerateSBPL(cfg Config) string {
	var b strings.Builder
	b.WriteString(sbplPreamble)
	writeReadGrants(&b, cfg)
	writeTerminalRules(&b, cfg.TerminalPath)
	writeWriteGrants(&b, cfg)
	for _, dp := range cfg.DenyPaths {
		if dp != "" {
			fmt.Fprintf(&b, "(deny file-write* (subpath %q))\n", dp)
		}
	}
	writeReadIsolation(&b, cfg.ReadIsolation)
	for _, dp := range cfg.DenyReadPaths {
		if dp != "" {
			fmt.Fprintf(&b, "(deny file-read* (subpath %q))\n", dp)
		}
	}
	writeClosingRules(&b, cfg)
	writeExecDenyRules(&b, cfg)
	return b.String()
}

const sandboxExecPath = "/usr/bin/sandbox-exec"

func writeExecDenyRules(b *strings.Builder, cfg Config) {
	if len(cfg.ExecDenyPaths)+len(cfg.ExecDenyRoots) == 0 {
		return
	}
	for _, p := range append(slices.Clone(cfg.ExecDenyPaths), sandboxExecPath) {
		writeExecDenyRule(b, cfg.ExecDenyEnforce, "literal", p)
	}
	for _, root := range cfg.ExecDenyRoots {
		writeExecDenyRule(b, cfg.ExecDenyEnforce, "subpath", root)
	}
}

func writeExecDenyRule(b *strings.Builder, enforce bool, filter, path string) {
	if path == "" {
		return
	}
	match := fmt.Sprintf(`(require-all (%s %q) (require-not (signing-identifier "com.apple.sandbox-exec")))`, filter, path)
	if enforce {
		fmt.Fprintf(b, "(deny process-exec %s)\n", match)
	} else {
		fmt.Fprintf(b, "(allow process-exec %s (with report))\n", match)
	}
}

const sbplPreamble = `(version 1)
(deny default)
(import "system.sb")
(allow process-exec)
(allow process-fork)
(allow signal (target same-sandbox))
(allow sysctl-read)
(allow mach-lookup)
(allow ipc-posix-shm)
(allow file-read-metadata)
`

const devReadGrantWithoutTerminals = `(allow file-read* (require-all (subpath "/dev") (require-not (regex #"^/dev/(tty|pty)[^/]+$"))))
`

func writeReadGrants(b *strings.Builder, cfg Config) {
	for _, p := range []string{
		cfg.RepoRoot, "/usr", "/System", "/Library",
		"/private/etc", "/opt", "/bin", "/sbin", "/var",
		"/private/var",
	} {
		if p != "" {
			fmt.Fprintf(b, "(allow file-read* (subpath %q))\n", p)
		}
	}
	b.WriteString(devReadGrantWithoutTerminals)
	if cfg.HomeDir != "" {
		fmt.Fprintf(b, "(allow file-read* (subpath %q))\n", cfg.HomeDir)
	}
	for _, p := range []string{"/tmp", "/private/tmp", "/var/folders", "/private/var/folders"} {
		fmt.Fprintf(b, "(allow file-read* (subpath %q))\n", p)
		fmt.Fprintf(b, "(allow file-write* (subpath %q))\n", p)
	}
}

func writeWriteGrants(b *strings.Builder, cfg Config) {
	for _, p := range homeStateWriteDirs(cfg.HomeDir) {
		fmt.Fprintf(b, "(allow file-write* (subpath %q))\n", p)
	}
	for _, wp := range cfg.WritePaths {
		if coversRepository(wp, cfg.RepoRoot) {
			fmt.Fprintf(b, "(allow file-write* (subpath %q))\n", wp)
		}
	}
	if cfg.ReadOnlyRepo && cfg.RepoRoot != "" {
		fmt.Fprintf(b, "(deny file-write* (subpath %q))\n", cfg.RepoRoot)
	}
	for _, wp := range cfg.WritePaths {
		if wp != "" && !coversRepository(wp, cfg.RepoRoot) {
			fmt.Fprintf(b, "(allow file-write* (subpath %q))\n", wp)
		}
	}
}

const ownPseudoTerminalRules = `(allow pseudo-tty)
(allow file-read* file-write* file-ioctl (literal "/dev/ptmx"))
(allow file-read* file-write-data file-ioctl (require-all (regex #"^/dev/ttys[0-9]+$") (extension "com.apple.sandbox.pty")))
`

func writeTerminalRules(b *strings.Builder, pane string) {
	b.WriteString(ownPseudoTerminalRules)
	if pane == "" {
		return
	}
	fmt.Fprintf(b, "(allow file-read* file-write-data (literal \"/dev/tty\") (literal %q))\n", pane)
	fmt.Fprintf(b, "(allow file-ioctl (require-all (require-any (literal \"/dev/tty\") (literal %q)) (require-any (ioctl-command TIOCGETA) (ioctl-command TIOCSETA) (ioctl-command TIOCSETAW) (ioctl-command TIOCSETAF) (ioctl-command TIOCGWINSZ))))\n", pane)
}

func writeClosingRules(b *strings.Builder, cfg Config) {
	for _, lit := range cfg.DenyLiterals {
		if lit != "" {
			fmt.Fprintf(b, "(deny file-write* (literal %q))\n", lit)
		}
	}
	if cfg.AllowNetwork {
		b.WriteString("(allow network*)\n")
	} else {
		b.WriteString("(deny network*)\n")
	}
	for _, sp := range cfg.DenySockets {
		if sp != "" {
			fmt.Fprintf(b, "(deny network-outbound (remote unix-socket (path-literal %q)))\n", sp)
		}
	}
}

func GenerateBwrapArgv(cfg Config, innerArgv []string) []string {
	return append(BwrapPrefix(cfg), innerArgv...)
}

func BwrapPrefix(cfg Config) []string {
	out := []string{
		"--unshare-user",
		"--unshare-pid",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
	}
	for _, p := range []string{"/usr", "/bin", "/lib", "/lib64", "/etc"} {
		out = append(out, "--ro-bind", p, p)
	}
	if cfg.HomeDir != "" {
		out = append(out, "--ro-bind", cfg.HomeDir, cfg.HomeDir)
	}
	for _, p := range homeStateWriteDirs(cfg.HomeDir) {
		out = append(out, "--bind-try", p, p)
	}
	for _, wp := range cfg.WritePaths {
		if coversRepository(wp, cfg.RepoRoot) {
			out = append(out, "--bind", wp, wp)
		}
	}
	if cfg.RepoRoot != "" {
		if cfg.ReadOnlyRepo {
			out = append(out, "--ro-bind", cfg.RepoRoot, cfg.RepoRoot)
		} else {
			out = append(out, "--bind", cfg.RepoRoot, cfg.RepoRoot)
		}
	}
	for _, wp := range cfg.WritePaths {
		if wp == "" {
			continue
		}
		if coversRepository(wp, cfg.RepoRoot) {
			continue
		}
		if strings.Contains(wp, "*") {
			wp = filepath.Dir(wp)
		}
		out = append(out, "--bind", wp, wp)
	}
	for _, dp := range cfg.DenyPaths {
		if dp != "" {
			out = append(out, "--ro-bind", dp, dp)
		}
	}
	out = append(out, bwrapReadIsolation(cfg.ReadIsolation)...)
	for _, dp := range cfg.DenyReadPaths {
		if dp != "" {
			out = append(out, "--tmpfs", dp, "--chmod", "000", dp, "--remount-ro", dp)
		}
	}
	if len(cfg.DenyReadPaths) > 0 {
		out = append(out, "--cap-drop", "ALL")
	}
	if !cfg.AllowNetwork {
		out = append(out, "--unshare-net")
	}
	return append(out, bwrapExecMasks(cfg)...)
}

func bwrapExecMasks(cfg Config) []string {
	if !cfg.ExecDenyEnforce {
		return nil
	}
	var out []string
	for _, p := range cfg.ExecDenyPaths {
		if p != "" {
			out = append(out, "--ro-bind", os.DevNull, p)
		}
	}
	for _, root := range cfg.ExecDenyRoots {
		if root != "" {
			out = append(out, "--tmpfs", root)
		}
	}
	return out
}

func coversRepository(path, repo string) bool {
	if path == "" || repo == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(path), filepath.Clean(repo))
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

type Sandbox struct {
	probe ProbeResult
	cfg   Config
}

func New(cfg Config) *Sandbox {
	return &Sandbox{probe: Probe(), cfg: cfg}
}

func (s *Sandbox) Exec(ctx context.Context, argv []string,
	stdin io.Reader, stdout, stderr io.Writer) error {
	if len(argv) == 0 {
		return fmt.Errorf("sandbox: argv empty")
	}
	var cmd *exec.Cmd
	switch {
	case s.probe.Available && hostOS == "darwin":
		profile := GenerateSBPL(s.cfg)
		args := append([]string{"-p", profile}, argv...)
		cmd = sysexec.Command(ctx, s.probe.BinaryPath, args...)
	case s.probe.Available && hostOS == "linux":
		args := GenerateBwrapArgv(s.cfg, argv)
		cmd = sysexec.Command(ctx, s.probe.BinaryPath, args...)
	default:
		fmt.Fprintf(stderr, "[sandbox] NOTE: %s; running unwrapped\n", s.probe.Reason)
		cmd = sysexec.Command(ctx, argv[0], argv[1:]...)
	}
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
