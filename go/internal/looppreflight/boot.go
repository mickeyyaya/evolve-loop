package looppreflight

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// checkBridgeBoot boots each *-tmux driver's REPL in turn and halts if any misses its
// prompt marker; SkipBoot warns instead. Sandbox needs a profile request and a capable host.
func checkBridgeBoot(o resolved) CheckResult {
	const name = "bridge-boot"

	if o.skipBoot {
		return CheckResult{
			Name:    name,
			Level:   LevelWarn,
			Message: "bridge boot skipped (EVOLVE_SKIP_PREFLIGHT_BOOT)",
			Detail:  "cheap checks ran; the real REPL boot — the check that catches an ExitREPLBootTimeout — was not exercised",
		}
	}

	var bootable []string
	for _, d := range distinctDrivers(o.profileLister, o.profileGetter) {
		if bridge.IsTmuxDriver(d) {
			bootable = append(bootable, d)
		}
	}

	sandbox := sandboxWanted(o.profileLister, o.profileGetter) && o.hostProbe().Sandbox.ExpectedToWork

	var fails []string
	for _, driver := range bootable {
		rc, scrollback := bootOne(o, driver, sandbox)
		if rc == bridge.ExitOK {
			continue
		}
		detail := fmt.Sprintf("driver %q boot failed: rc=%d (%s)", driver, rc, bootRCName(rc))
		if tail := bridge.ScrollbackTail(scrollback, 12); tail != "" {
			detail += "\n  final pane:\n" + indent(tail, "    ")
		}
		fails = append(fails, detail)
	}

	if len(fails) > 0 {
		return CheckResult{
			Name:    name,
			Level:   LevelHalt,
			Message: fmt.Sprintf("%d driver(s) failed to boot", len(fails)),
			Detail:  strings.Join(fails, "\n"),
		}
	}
	return CheckResult{
		Name:    name,
		Level:   LevelPass,
		Message: fmt.Sprintf("%d driver(s) booted (sandbox=%v)", len(bootable), sandbox),
	}
}

func bootOne(o resolved, driver string, sandbox bool) (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), o.bootBudget)
	defer cancel()
	return o.bootTester(ctx, driver, sandbox)
}

// newDefaultBootTester mirrors `evolve doctor boot`: it boots in a throwaway workspace,
// plus a throwaway worktree and the build agent on the sandbox path.
func newDefaultBootTester(projectRoot string, stderr io.Writer) func(context.Context, string, bool) (int, string) {
	return func(ctx context.Context, driver string, sandbox bool) (int, string) {
		ws, err := os.MkdirTemp("", "evolve-looppreflight-*")
		if err != nil {
			return exitWorkspaceSetupFailed, "could not create boot workspace: " + err.Error()
		}
		defer func() { _ = os.RemoveAll(ws) }()
		cfg := &bridge.Config{Workspace: ws, ProjectRoot: projectRoot, AllowNetwork: true}
		if sandbox {
			wt, werr := os.MkdirTemp("", "evolve-looppreflight-wt-*")
			if werr == nil {
				defer func() { _ = os.RemoveAll(wt) }()
				cfg.Worktree = wt
				cfg.Agent = "build"
			}
		}
		return bridge.BootSmokeTest(ctx, driver, cfg, bridge.Deps{Stderr: stderr})
	}
}

// exitWorkspaceSetupFailed is negative so it never collides with a bridge exit code
// and a MkdirTemp failure is not misreported as ExitBadFlags.
const exitWorkspaceSetupFailed = -1

func bootRCName(rc int) string {
	switch rc {
	case bridge.ExitREPLBootTimeout:
		return "ExitREPLBootTimeout — REPL never reached its prompt marker"
	case bridge.ExitMissingBinary:
		return "ExitMissingBinary — CLI binary not found"
	case bridge.ExitBadFlags:
		return "ExitBadFlags — unknown or non-tmux driver"
	case exitWorkspaceSetupFailed:
		return "workspace setup failed (os.MkdirTemp)"
	default:
		// Carry the number so distinct unknown codes stay distinguishable.
		return fmt.Sprintf("boot failure (exit=%d)", rc)
	}
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
