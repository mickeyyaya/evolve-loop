package build

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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

func TestBuild_ComposePrompt_NoFleetScope_NoScopeLine(t *testing.T) {
	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: map[string]string{}})
	if strings.Contains(out, "fleet_scope") {
		t.Errorf("unscoped build prompt must not carry a fleet_scope line:\n%s", out)
	}
}
