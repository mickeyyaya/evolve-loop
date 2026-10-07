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
func checkBridgeBoot(o resolved, usageEvidence func(driver string) string) CheckResult {
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
	for _, d := range o.drivers() {
		if bridge.IsTmuxDriver(d) {
			bootable = append(bootable, d)
		}
	}

	sandbox := sandboxWanted(o.profileLister, o.profileGetter) && o.hostProbe().Sandbox.ExpectedToWork

	var fails []string
	for _, driver := range bootable {
		out := bootOne(o, driver, sandbox)
		if out.RC == bridge.ExitOK {
			continue
		}
		fails = append(fails, bootFailureDetail(driver, out, usageEvidence))
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

type BootOutcome struct {
	RC         int
	Wall       string
	Scrollback string
}

func bootOne(o resolved, driver string, sandbox bool) BootOutcome {
	var out BootOutcome
	bridge.BootProbe{Driver: driver, Log: o.stderr}.Retry(context.Background(), func() (int, string) {
		ctx, cancel := context.WithTimeout(context.Background(), o.bootBudget)
		defer cancel()
		out = o.bootTester(ctx, driver, sandbox)
		return out.RC, out.Wall
	})
	return out
}

func bootFailureDetail(driver string, out BootOutcome, usageEvidence func(driver string) string) string {
	detail := fmt.Sprintf("driver %q boot failed: rc=%d (%s)%s", driver, out.RC, bootRCName(out.RC), attemptsNote(driver, out))
	if out.Wall != "" {
		detail += "; the pane escalated " + out.Wall
	}
	if tail := bridge.ScrollbackTail(out.Scrollback, 12); tail != "" {
		detail += "\n  final pane:\n" + indent(tail, "    ")
	}
	if usageEvidence != nil {
		detail += "\n  usage: " + usageEvidence(driver)
	}
	return detail
}

func attemptsNote(driver string, out BootOutcome) string {
	if attempts := bridge.ProbeBootAttempts(driver); out.RC == bridge.ExitREPLBootTimeout && out.Wall == "" && attempts > 1 {
		return fmt.Sprintf(" on all %d boot attempts", attempts)
	}
	return ""
}

// newDefaultBootTester mirrors `evolve doctor boot`: it boots in a throwaway workspace,
// plus a throwaway worktree and the build agent on the sandbox path.
func newDefaultBootTester(projectRoot string, stderr io.Writer) func(context.Context, string, bool) BootOutcome {
	return func(ctx context.Context, driver string, sandbox bool) BootOutcome {
		ws, err := os.MkdirTemp("", "evolve-looppreflight-*")
		if err != nil {
			return BootOutcome{RC: exitWorkspaceSetupFailed, Scrollback: "could not create boot workspace: " + err.Error()}
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
		rc, scrollback := bridge.BootSmokeTest(ctx, driver, cfg, bridge.Deps{Stderr: stderr})
		return BootOutcome{RC: rc, Wall: bridge.EscalationPattern(ws), Scrollback: scrollback}
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
