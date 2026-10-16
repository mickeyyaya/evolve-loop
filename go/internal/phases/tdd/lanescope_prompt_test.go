package tdd

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestTDD_ComposePrompt_RendersFleetScope(t *testing.T) {
	req := core.PhaseRequest{Context: map[string]string{"fleet_scope": "todo-lane-a"}}
	out := hooks{}.ComposePrompt("BODY", req)
	if !regexp.MustCompile(`(?m)^.*fleet_scope.*todo-lane-a`).MatchString(out) {
		t.Errorf("tdd prompt has no fleet_scope directive naming todo-lane-a:\n%s", out)
	}
	if strings.Contains(out, "todo-lane-b") {
		t.Errorf("foreign lane id leaked into tdd prompt:\n%s", out)
	}
}

func TestTDD_ComposePrompt_NoFleetScope_NoScopeLine(t *testing.T) {
	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: map[string]string{}})
	if strings.Contains(out, "fleet_scope") {
		t.Errorf("unscoped tdd prompt must not carry a fleet_scope line:\n%s", out)
	}
}
