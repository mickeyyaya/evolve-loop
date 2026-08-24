package scout

// scope_paths_render_test.go — the scope-paths disclosure must reach the PROMPT
// with the directive that makes it load-bearing: read these exact files, ignore
// same-named consumed/processed namesakes.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestComposePrompt_ScopePathsRenderTheLiveRecordsDirective(t *testing.T) {
	h := hooks{}
	req := core.PhaseRequest{Cycle: 1548, Context: map[string]string{
		"fleet_scope":       "pipeline-defect-pipeline-blocker",
		"fleet_scope_paths": "pipeline-defect-pipeline-blocker=/abs/inbox/2026-08-22T15-02-52Z-pipeline-defect-pipeline-blocker.json",
	}}
	prompt := h.ComposePrompt("BODY", req)

	if !strings.Contains(prompt, "/abs/inbox/2026-08-22T15-02-52Z-pipeline-defect-pipeline-blocker.json") {
		t.Fatalf("the LIVE path must reach the prompt; got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "consumed") || !strings.Contains(prompt, "READ THESE EXACT FILES") {
		t.Fatalf("the directive must forbid same-named consumed namesakes explicitly — a path without the warning still invites the name-search; got:\n%s", prompt)
	}
}

// NO-REGRESSION: absent key ⇒ byte-identical prompt (every pre-fix run).
func TestComposePrompt_NoScopePathsIsByteIdentical(t *testing.T) {
	h := hooks{}
	base := core.PhaseRequest{Cycle: 1, Context: map[string]string{"fleet_scope": "a-task"}}
	with := core.PhaseRequest{Cycle: 1, Context: map[string]string{"fleet_scope": "a-task", "fleet_scope_paths": ""}}
	if h.ComposePrompt("B", base) != h.ComposePrompt("B", with) {
		t.Fatalf("an empty/absent fleet_scope_paths must not change the prompt")
	}
	if strings.Contains(h.ComposePrompt("B", base), "READ THESE EXACT FILES") {
		t.Fatalf("no paths, no directive")
	}
}
