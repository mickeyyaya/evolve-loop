package build

// Cycle-776 RED contract — fleet-lane-provisioning-split (residual slice):
// build's composed prompt must render the pinned lane scope so the builder
// binds to THIS lane's todo ids (cycle-766 put the scope into Context; only
// triage renders it — see scout/lanescope_prompt_test.go for the incident).

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// RENDER + ISOLATION: the lane scope appears as a fleet_scope directive line
// naming the assigned id, and a foreign lane's id never appears.
func TestBuild_ComposePrompt_RendersFleetScope(t *testing.T) {
	req := core.PhaseRequest{Context: map[string]string{"fleet_scope": "todo-lane-a"}}
	out := hooks{}.ComposePrompt("BODY", req)
	if !regexp.MustCompile(`(?m)^.*fleet_scope.*todo-lane-a`).MatchString(out) {
		t.Errorf("build prompt has no fleet_scope directive naming todo-lane-a:\n%s", out)
	}
	if strings.Contains(out, "todo-lane-b") {
		t.Errorf("foreign lane id leaked into build prompt:\n%s", out)
	}
}

// EDGE: no lane scope ⇒ no fleet_scope line (sequential cycles byte-identical).
func TestBuild_ComposePrompt_NoFleetScope_NoScopeLine(t *testing.T) {
	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: map[string]string{}})
	if strings.Contains(out, "fleet_scope") {
		t.Errorf("unscoped build prompt must not carry a fleet_scope line:\n%s", out)
	}
}
