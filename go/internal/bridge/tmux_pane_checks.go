package bridge

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

func paneProfileFor(lp tmuxLaunch) panestream.PaneProfile {
	binary := driverBinary(lp.name)
	p, ok := panestream.Profiles[binary]
	if !ok {
		p = panestream.PaneProfile{Name: binary, BoundaryMarker: lp.promptMarker}
	}
	if m, err := LoadManifest(lp.name); err == nil {
		p.ExhaustedRegex = manifestExhaustedPattern(m)
		p.BusyLineRegex = m.BusyLineRegex
		p.TokenLineRegex = m.TokenLineRegex
		p.ModelLabelRegex = m.ModelLabelRegex
	}
	return p
}

// detectorFor returns a per-run LivenessProbe for the tmux driver identified
// by lp.
func detectorFor(lp tmuxLaunch) panestream.LivenessProbe {
	return panestream.DetectorFor(paneProfileFor(lp))
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
