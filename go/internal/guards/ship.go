package guards

import (
	"context"
	"regexp"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Ship denies ship-class Bash commands (git commit, git push, gh release) that bypass `evolve ship` and ship.sh.
type Ship struct {
	bypass bool
}

// NewShip returns a Ship guard; bypass allows every call.
func NewShip(bypass bool) *Ship { return &Ship{bypass: bypass} }

// Name reports "ship".
func (s *Ship) Name() string { return "ship" }

var (
	shipVerbRe   = regexp.MustCompile(`\b(git[ \t]+commit|git[ \t]+push|gh[ \t]+release[ \t]+(create|edit))\b`)
	shipScriptRe = regexp.MustCompile(`scripts/lifecycle/ship\.sh(?:[ \t]|$)`)
	// The left boundary (start, blank or path separator) keeps "devolve ship" from matching.
	nativeShipRe = regexp.MustCompile(`(^|[ \t/])evolve[ \t]+ship\b`)
)

// Decide denies a ship-class verb outside heredoc bodies unless the command invokes `evolve ship` or ship.sh.
func (s *Ship) Decide(_ context.Context, in core.GuardInput) core.GuardDecision {
	if s.bypass {
		return core.GuardDecision{Allow: true}
	}
	if in.ToolName != "Bash" {
		return core.GuardDecision{Allow: true}
	}
	cmd := cmdString(in)
	if cmd == "" {
		return core.GuardDecision{Allow: true}
	}
	// A commit message in a heredoc body may legitimately mention `git push` or `git commit`.
	stripped := stripHeredocs(cmd)
	if !shipVerbRe.MatchString(stripped) {
		return core.GuardDecision{Allow: true}
	}
	if shipScriptRe.MatchString(cmd) || nativeShipRe.MatchString(cmd) {
		return core.GuardDecision{Allow: true}
	}
	return core.GuardDecision{
		Allow:  false,
		Reason: "ship-class command must invoke the native 'evolve ship' CLI; pass --bypass to the guard for emergencies",
	}
}
