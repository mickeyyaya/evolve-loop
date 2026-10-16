package guards

import (
	"context"
	"path"
	"regexp"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type Ship struct {
	bypass bool
}

func NewShip(bypass bool) *Ship { return &Ship{bypass: bypass} }

func (s *Ship) Name() string { return "ship" }

var shipVerbRe = regexp.MustCompile(`\b(git[ \t]+commit|git[ \t]+push|gh[ \t]+release[ \t]+(create|edit))\b`)

func (s *Ship) Decide(_ context.Context, in core.GuardInput) core.GuardDecision {
	if s.bypass || in.ToolName != "Bash" {
		return core.GuardDecision{Allow: true}
	}
	for _, c := range splitShellCommands(cmdString(in)) {
		if isShipClass(c) && !isNativeShip(c.words) {
			return core.GuardDecision{
				Allow:  false,
				Reason: "ship-class command must invoke the native 'evolve ship' CLI; pass --bypass to the guard for emergencies",
			}
		}
	}
	return core.GuardDecision{Allow: true}
}

func isShipClass(c shellCommand) bool {
	if shipVerbRe.MatchString(c.text) {
		return true
	}
	for i, w := range c.words {
		if programName(w) != "git" {
			continue
		}
		if sub := gitSubcommand(c.words[i+1:]); sub == "commit" || sub == "push" {
			return true
		}
	}
	return false
}

func isNativeShip(words []string) bool {
	for len(words) > 0 && shellAssignmentRe.MatchString(words[0]) {
		words = words[1:]
	}
	return len(words) >= 2 && path.Base(words[0]) == "evolve" && words[1] == "ship"
}
