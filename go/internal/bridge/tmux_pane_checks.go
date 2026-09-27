package bridge

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// paneProfileFor resolves the panestream PaneProfile for a tmux driver by
// stripping the "-tmux" suffix from the driver name (claude-tmux → claude). An
// unknown driver (e.g. the test "itest-tmux") falls back to a profile built
// from the launch's own prompt marker so the delta extractor still has a
// content boundary.
func paneProfileFor(lp tmuxLaunch) panestream.PaneProfile {
	cli := strings.TrimSuffix(lp.name, "-tmux")
	p, ok := panestream.Profiles[cli]
	if !ok {
		p = panestream.PaneProfile{Name: cli, BoundaryMarker: lp.promptMarker}
	}
	if m, err := LoadManifest(lp.name); err == nil {
		p.ExhaustedRegex = manifestExhaustedPattern(m)
	}
	return p
}

// detectorFor returns a per-run LivenessProbe for the tmux driver identified
// by lp.
func detectorFor(lp tmuxLaunch) panestream.LivenessProbe {
	return panestream.DetectorFor(paneProfileFor(lp))
}

func tmuxPaneLooksLikeUpdateMenu(pane string) bool {
	return strings.Contains(pane, "Update available!") &&
		strings.Contains(pane, "Update now") &&
		strings.Contains(pane, "Skip")
}

// isShellProcess reports whether a pane_current_command value names a known
// interactive shell. Login shells report with a leading dash ("-zsh").
func isShellProcess(cmd string) bool {
	switch strings.TrimPrefix(cmd, "-") {
	case "zsh", "bash", "sh", "fish", "dash", "tcsh", "ksh":
		return true
	}
	return false
}

func paneShellProcess(ctx context.Context, tm TmuxController, session string) (string, bool) {
	pc, ok := tm.(PaneCommander)
	if !ok {
		return "", false
	}
	cmd, err := pc.PaneCommand(ctx, session)
	if err != nil {
		return "", false
	}
	return cmd, isShellProcess(cmd)
}

// paneLooksLikeShellSpill reports shell continuation/paste-spill signatures: a
// shell continuation prompt (quote>/bquote>/dquote>/heredoc>) as the LAST
// non-blank line, or zsh's command-not-found echo anywhere. Callers MUST pair
// this with the authoritative paneShellProcess check — agent output may
// legitimately quote these strings.
func paneLooksLikeShellSpill(pane string) bool {
	if strings.Contains(pane, "command not found") {
		return true
	}
	lines := strings.Split(strings.TrimRight(pane, "\n \t"), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	for _, w := range []string{"quote>", "bquote>", "dquote>", "heredoc>"} {
		if last == w {
			return true
		}
	}
	return false
}
