package looppreflight

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// checkBridgeBoot (Halt) is the check that catches the cycle-258 failure: it
// REALLY boots each configured *-tmux driver's REPL (boot-only, no prompt) and
// halts if any fails to reach its prompt marker. When SkipBoot is set it warns
// instead (CI/offline). Boots run sequentially, each under its own BootBudget
// deadline. The sandbox boot path is exercised iff the profiles request it AND
// the host can actually sandbox.
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
		if strings.HasSuffix(d, "-tmux") {
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

// bootOne runs one driver's boot under a per-driver BootBudget deadline.
func bootOne(o resolved, driver string, sandbox bool) (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), o.bootBudget)
	defer cancel()
	return o.bootTester(ctx, driver, sandbox)
}

// newDefaultBootTester returns the production BootTester: a near-copy of
// `evolve doctor boot` that provisions a throwaway workspace (and, for the
// sandbox path, a throwaway worktree + build agent) and calls
// bridge.BootSmokeTest.
func newDefaultBootTester(projectRoot string, stderr io.Writer) func(context.Context, string, bool) (int, string) {
	return func(ctx context.Context, driver string, sandbox bool) (int, string) {
		ws, err := os.MkdirTemp("", "evolve-looppreflight-*")
		if err != nil {
			return exitWorkspaceSetupFailed, "could not create boot workspace: " + err.Error()
		}
		defer func() { _ = os.RemoveAll(ws) }()
		cfg := &bridge.Config{Workspace: ws, ProjectRoot: projectRoot}
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

// exitWorkspaceSetupFailed is a local (negative, never a bridge exit code)
// sentinel for a boot adapter that could not even provision its throwaway
// workspace — kept distinct from bridge.ExitBadFlags so the diagnostic does not
// misreport a disk/`os.MkdirTemp` failure as an unknown-driver error.
const exitWorkspaceSetupFailed = -1

// bootRCName names the bridge exit codes a boot can return, for the diagnostic.
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
		return "boot failure"
	}
}

// indent prefixes every line of s with prefix.
func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
