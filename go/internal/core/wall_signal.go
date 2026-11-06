package core

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeLoginRequired signalcenter.Code = "BRIDGE_LOGIN_REQUIRED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeLoginRequired, "a paused cycle met a wall in which at least one family needs a login; WARN; reason renders the fact table of the wall (one clause for each family), the evidence, the impact on the claims and the next step; fields.stop_reason=login-required, phase")
}

func (w Wall) AllLogin() bool {
	if len(w.Ends) == 0 {
		return false
	}
	for _, e := range w.Ends {
		if !clihealth.LoginPattern(e.Pattern) {
			return false
		}
	}
	return true
}

func (w Wall) Evidence() []string {
	var paths []string
	for _, e := range w.Ends {
		if e.Evidence != "" {
			paths = append(paths, e.Evidence)
		}
	}
	return paths
}

type WallReport struct {
	Cycle     int
	Phase     string
	Wall      Wall
	Workspace string
}

func (r WallReport) text(impact string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "cycle %d phase %s: %s.", r.Cycle, r.Phase, r.Wall.Clauses())
	if paths := r.Wall.Evidence(); len(paths) > 0 {
		fmt.Fprintf(&b, " Evidence: %s.", strings.Join(paths, ", "))
	}
	fmt.Fprintf(&b, " Impact: %s.", impact)
	if r.Wall.AllLogin() {
		b.WriteString(" Time alone does not clear this.")
	}
	fmt.Fprintf(&b, " Next: %s", WallNextStep(r.Wall, r.Workspace))
	return b.String()
}

func (r WallReport) PauseText(impact string) string {
	return r.text("the cycle is paused; " + impact)
}

func (r WallReport) FailText() string {
	return r.text("the phase failed and the cycle ends in failure")
}

func (r WallReport) event(reason, origin string) signalcenter.Event {
	return signalcenter.Event{
		Cycle: r.Cycle, Phase: r.Phase, Module: signalcenter.ModuleBridge, Origin: origin,
		Kind: signalcenter.KindLoginRequired, Severity: signalcenter.SeverityWarn, Code: CodeLoginRequired,
		Reason: reason,
		Fields: map[string]string{"phase": r.Phase, "stop_reason": r.Wall.StopReason()},
	}
}

func (r WallReport) PauseEvent(impact, origin string) signalcenter.Event {
	return r.event(r.PauseText(impact), origin)
}

func (r WallReport) FailEvent(origin string) signalcenter.Event {
	return r.event(r.FailText(), origin)
}
