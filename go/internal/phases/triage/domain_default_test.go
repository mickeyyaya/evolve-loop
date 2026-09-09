package triage

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestComposePrompt_RendersDeliverableKindDefault — ADR-0099 slice 2: triage
// (whose header is the authoritative kind) sees the project default the
// orchestrator seeds from .evolve/domain.json; absent ⇒ byte-identical prompt.
func TestComposePrompt_RendersDeliverableKindDefault(t *testing.T) {
	h := hooks{}
	with := h.ComposePrompt("body", core.PhaseRequest{Cycle: 1, Context: map[string]string{core.CtxKeyDeliverableKindDefault: "document"}})
	if !strings.Contains(with, "- deliverable_kind_default: document") {
		t.Fatalf("prompt must render the default kind line:\n%s", with)
	}
	withRoot := h.ComposePrompt("body", core.PhaseRequest{Cycle: 1, Context: map[string]string{core.CtxKeyDeliverableRoot: "solutions"}})
	if !strings.Contains(withRoot, "- deliverable_root: solutions") {
		t.Fatalf("prompt must render the configured deliverable root (slice 3: the prose never restates it):\n%s", withRoot)
	}
	without := h.ComposePrompt("body", core.PhaseRequest{Cycle: 1, Context: map[string]string{}})
	if strings.Contains(without, "deliverable_kind_default") || strings.Contains(without, "deliverable_root") {
		t.Fatalf("absent keys must render nothing")
	}
}
