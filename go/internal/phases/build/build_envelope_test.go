package build

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestComposePrompt_BuildPlanEnvelope(t *testing.T) {
	t.Run("envelope build-plan preferred when req.BuildPlan set (advisory+)", func(t *testing.T) {
		got := hooks{}.ComposePrompt("body", core.PhaseRequest{BuildPlan: "ENVELOPE PLAN BODY"})
		if !strings.Contains(got, "## Build Plan\nENVELOPE PLAN BODY") {
			t.Errorf("want envelope plan injected, got:\n%s", got)
		}
	})

	t.Run("no injection when BuildPlan empty", func(t *testing.T) {
		got := hooks{}.ComposePrompt("body", core.PhaseRequest{BuildPlan: ""})
		if strings.Contains(got, "## Build Plan") {
			t.Errorf("empty BuildPlan must not inject; got:\n%s", got)
		}
	})
}
