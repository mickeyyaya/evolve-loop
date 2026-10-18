package intent

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

func TestIntent_ComposePrompt_EmptyEdgeRendersNoDigest(t *testing.T) {
	upstream := phaseio.NewHandoffs(phaseio.HandoffsInit{
		Scout: &phaseio.ScoutView{CycleSizeEstimate: "LEAK"},
		Audit: &phaseio.AuditView{Verdict: "LEAK"},
	})
	req := core.PhaseRequest{Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{Phase: "intent", Upstream: upstream})}
	got := hooks{}.ComposePrompt("BODY", req)
	if strings.Contains(got, "## Upstream Handoff Digest") || strings.Contains(got, "LEAK") {
		t.Errorf("intent prompt renders upstream data on its empty edge:\n%s", got)
	}
}
