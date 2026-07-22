package build

// prior_findings_test.go — ADR-0076 slice C (C7): an adopted continuation's
// prior-attempt findings render as a dedicated prompt block so the builder
// resumes INFORMED (what failed, what to finish) instead of rediscovering.
// Absent findings pin a byte-identical legacy prompt.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestComposePrompt_RendersPriorAttemptFindings(t *testing.T) {
	req := core.PhaseRequest{
		Context: map[string]string{"continuation_findings": `{"phase":"build","summary":"export X unnamed"}`},
	}
	got := (hooks{}).ComposePrompt("body", req)
	if !strings.Contains(got, "## Prior Attempt Findings") {
		t.Errorf("prompt must carry the findings block; got:\n%s", got)
	}
	if !strings.Contains(got, "export X unnamed") {
		t.Errorf("prompt must carry the findings content; got:\n%s", got)
	}
	if !strings.Contains(got, "resume") {
		t.Errorf("the block must instruct resumption, not restart; got:\n%s", got)
	}
}

func TestComposePrompt_NoFindingsIsByteIdentical(t *testing.T) {
	req := core.PhaseRequest{Context: map[string]string{}}
	if got := (hooks{}).ComposePrompt("body", req); strings.Contains(got, "Prior Attempt") {
		t.Errorf("no findings ⇒ no block:\n%s", got)
	}
}
