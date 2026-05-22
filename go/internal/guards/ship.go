package guards

import (
	"context"
	"regexp"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Ship denies ship-class commands unless the canonical scripts/lifecycle/
// ship.sh is the entry point. Port of scripts/guards/ship-gate.sh.
type Ship struct{}

func NewShip() *Ship { return &Ship{} }

func (s *Ship) Name() string { return "ship" }

// Ship-class verb patterns (canonical bash plus common bypass shapes).
var (
	shipVerbRe   = regexp.MustCompile(`\b(git[ \t]+commit|git[ \t]+push|gh[ \t]+release[ \t]+(create|edit))\b`)
	shipScriptRe = regexp.MustCompile(`scripts/lifecycle/ship\.sh(?:[ \t]|$)`)
)

func (s *Ship) Decide(_ context.Context, in core.GuardInput) core.GuardDecision {
	if envBypass("EVOLVE_BYPASS_SHIP_GATE") {
		return core.GuardDecision{Allow: true}
	}
	if in.ToolName != "Bash" {
		return core.GuardDecision{Allow: true}
	}
	cmd := cmdString(in)
	if cmd == "" {
		return core.GuardDecision{Allow: true}
	}
	if !shipVerbRe.MatchString(cmd) {
		return core.GuardDecision{Allow: true}
	}
	// Verb present — require the canonical script path.
	if shipScriptRe.MatchString(cmd) {
		return core.GuardDecision{Allow: true}
	}
	return core.GuardDecision{
		Allow: false,
		Reason: "ship-class command must invoke scripts/lifecycle/ship.sh; set EVOLVE_BYPASS_SHIP_GATE=1 for emergencies",
	}
}
