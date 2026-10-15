package scout

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestComposePrompt_RendersDomainDefaults(t *testing.T) {
	h := hooks{}
	with := h.ComposePrompt("body", core.PhaseRequest{Cycle: 1, Context: map[string]string{core.CtxKeyDeliverableKindDefault: "document", core.CtxKeyDeliverableRoot: "solutions"}})
	for _, line := range []string{"- deliverable_kind_default: document", "- deliverable_root: solutions"} {
		if !strings.Contains(with, line) {
			t.Errorf("prompt must render %q:\n%s", line, with)
		}
	}
	without := h.ComposePrompt("body", core.PhaseRequest{Cycle: 1, Context: map[string]string{}})
	if strings.Contains(without, "deliverable_kind_default") || strings.Contains(without, "deliverable_root") {
		t.Fatalf("absent keys must render nothing")
	}
}
