package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestLaunch_AppendsDeliverableContractTailBlock(t *testing.T) {
	fe := &fakeEngine{}
	artifact := "/abs/.evolve/runs/cycle-1218/audit-report.md"
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "PERSONA-BODY",
		Workspace: t.TempDir(), ArtifactPath: artifact, Agent: "audit",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	got := fe.gotReq.Prompt

	start := strings.Index(got, "<deliverable-contract phase=\"audit\">")
	if start < 0 {
		t.Fatalf("dispatch prompt carries no <deliverable-contract> tail block — the renderer is UNWIRED:\n%s", truncate(got, 600))
	}
	if body := strings.Index(got, "PERSONA-BODY"); start < body {
		t.Errorf("tail block must land AFTER the persona body (generation point / recency); block at %d, body at %d", start, body)
	}
	tail := got[start:]
	c, ok := phasecontract.For("audit")
	if !ok {
		t.Fatal("premise broken: audit has no registered contract")
	}
	if !strings.Contains(tail, "<artifact-path>"+artifact+"</artifact-path>") {
		t.Errorf("tail block must carry the exact artifact path; got:\n%s", tail)
	}
	for _, s := range c.Sections {
		if !strings.Contains(tail, "<section>"+s.Canonical+"</section>") {
			t.Errorf("tail block must restate required section %q verbatim; got:\n%s", s.Canonical, tail)
		}
	}
	// The tail is the copy the agent follows, so audit's exemplar there must carry the failure block.
	if !c.RequireFailureContext {
		t.Fatal("test premise: the audit contract must declare RequireFailureContext")
	}
	// Asserted on the rendered shape: an export whose only caller is a test would be a dead seam.
	for _, want := range []string{
		`"phase":"audit"`, `"schema_version":2`, `"failure"`, `"class"`, `"defects"`, `"evidence_paths"`,
	} {
		if !strings.Contains(tail, want) {
			t.Errorf("tail sentinel exemplar must be the schema-2 failure-bearing form (missing %s); got:\n%s", want, tail)
		}
	}
	if !strings.Contains(tail, "MUST carry the failure block") {
		t.Errorf("tail must say the failure block is mandatory for FAIL/WARN; got:\n%s", tail)
	}
	// Count only after the body: the prefix block quotes the footer marker as a cross-reference.
	after := got[strings.Index(got, "PERSONA-BODY"):]
	if n := strings.Count(after, phasecontract.FooterMarker); n != 1 {
		t.Errorf("the tail must declare the path exactly once, got %d occurrences", n)
	}
	if n := strings.Count(after, "<deliverable-contract"); n != 1 {
		t.Errorf("the tail must carry exactly one <deliverable-contract> block, got %d", n)
	}
}

func TestLaunch_UnregisteredAgentGetsPathDisclosureNotPrefix(t *testing.T) {
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "BODY",
		Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "not-a-phase",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	got := fe.gotReq.Prompt
	if !strings.Contains(got, "/a.md") {
		t.Errorf("unregistered agent must still be told its polled artifact path; got:\n%s", got)
	}
	if strings.Index(got, "BODY") > strings.Index(got, "/a.md") {
		t.Errorf("synthesized disclosure must be a TAIL — body first; got:\n%s", got)
	}
}
