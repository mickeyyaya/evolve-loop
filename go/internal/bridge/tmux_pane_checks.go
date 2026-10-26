package bridge

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

func paneProfileFor(lp tmuxLaunch) panestream.PaneProfile {
	m, err := LoadManifest(lp.name)
	p, ok := panestream.Profiles[m.Parsers[ParserPane]]
	if !ok {
		p = panestream.PaneProfile{Name: driverBinary(lp.name), BoundaryMarker: lp.promptMarker}
	}
	if err == nil {
		p.ExhaustedRegex = manifestExhaustedPattern(m)
		p.BusyLineRegex = m.BusyLineRegex
		p.TokenLineRegex = m.TokenLineRegex
		p.ModelLabelRegex = m.ModelLabelRegex
	}
	return p
}

func detectorFor(lp tmuxLaunch) panestream.LivenessProbe {
	return panestream.DetectorFor(paneProfileFor(lp))
}

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
