package guards

import (
	"context"
	"regexp"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// Ship denies ship-class commands unless the canonical scripts/lifecycle/
// ship.sh is the entry point. Port of scripts/guards/ship-gate.sh.
type Ship struct {
	bypass bool
}

func NewShip(bypass bool) *Ship { return &Ship{bypass: bypass} }

func (s *Ship) Name() string { return "ship" }

// Ship-class verb patterns (canonical bash plus common bypass shapes).
var (
	shipVerbRe   = regexp.MustCompile(`\b(git[ \t]+commit|git[ \t]+push|gh[ \t]+release[ \t]+(create|edit))\b`)
	shipScriptRe = regexp.MustCompile(`scripts/lifecycle/ship\.sh(?:[ \t]|$)`)
	// nativeShipRe matches the native Go CLI invocations:
	//   evolve ship
	//   go/bin/evolve ship
	//   /abs/path/to/evolve ship
	// Token boundary on the left (word boundary or path separator) prevents
	// false positives like "devolve ship".
	nativeShipRe = regexp.MustCompile(`(^|[ \t/])evolve[ \t]+ship\b`)
)

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
	// v11.8.3+: strip heredoc bodies before the verb regex so commit
	// message bodies that legitimately mention `git push` / `git commit`
	// (e.g. describing what a script does) don't trip the gate. Mirrors
	// the awk pre-processor in legacy/scripts/guards/ship-gate.sh.
	stripped := stripHeredocs(cmd)
	if !shipVerbRe.MatchString(stripped) {
		return core.GuardDecision{Allow: true}
	}
	// Verb present — require the canonical script path OR the native
	// `evolve ship` CLI (v11.3.0+).
	if shipScriptRe.MatchString(cmd) || nativeShipRe.MatchString(cmd) {
		return core.GuardDecision{Allow: true}
	}
	return core.GuardDecision{
		Allow:  false,
		Reason: "ship-class command must invoke the native 'evolve ship' CLI; pass --bypass to the guard for emergencies",
	}
}
