package bridge

// bridge_contract_tail_test.go — WIRING proof for the generation-point
// deliverable contract (inbox contract-requirements-at-generation-point, 0.90).
//
// A renderer nobody calls is dead plumbing (the caller-proof class:
// builder-persona-requires-caller-proof). These tests assert the block reaches
// the prompt the ENGINE receives, through the production Launch path — the same
// seam TestLaunch_InjectsDeliverableContract already guards for the prefix
// block — and that it lands in the TAIL, after the persona body, which is the
// entire point of the change.

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// TestLaunch_AppendsDeliverableContractTailBlock — the production caller of
// phasecontract.RenderContractTail is Adapter.injectContract (bridge.go), which
// Launch invokes for every contracted phase. The XML block must appear AFTER the
// persona body and carry the exact path, the verbatim section heading, and the
// sentinel template for a verdict-bearing phase.
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
	// audit declares RequireFailureContext, and a FAIL/WARN sentinel WITHOUT the
	// structured failure block is a contract violation (deliverable.go). The tail
	// is the recency-dominant copy, so its exemplar must be the failure-bearing
	// one — a bare PASS exemplar here is the version the auditor would follow,
	// on the one phase whose verdict gates ship (review HIGH).
	if !c.RequireFailureContext {
		t.Fatal("test premise: the audit contract must declare RequireFailureContext")
	}
	// Asserted on the observable SHAPE, not via a new exported accessor: adding
	// an export whose only caller is this test is the dead-seam pattern this
	// very change forbids.
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
	// ONE tail restatement, not two. The prefix block legitimately quotes the
	// marker as a cross-reference ("the path shown under DELIVERABLE PATH:"), so
	// only the region AFTER the persona body is counted: exactly one path
	// declaration and exactly one contract block live there.
	after := got[strings.Index(got, "PERSONA-BODY"):]
	if n := strings.Count(after, phasecontract.FooterMarker); n != 1 {
		t.Errorf("the tail must declare the path exactly once, got %d occurrences", n)
	}
	if n := strings.Count(after, "<deliverable-contract"); n != 1 {
		t.Errorf("the tail must carry exactly one <deliverable-contract> block, got %d", n)
	}
}

// TestLaunch_NoTailBlockForUnregisteredAgent — non-phase bridge callers must be
// untouched (the contract-free pass-through path stays byte-identical).
func TestLaunch_NoTailBlockForUnregisteredAgent(t *testing.T) {
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "BODY",
		Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "not-a-phase",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if strings.Contains(fe.gotReq.Prompt, "<deliverable-contract") {
		t.Errorf("unregistered agent must get no tail block; got:\n%s", fe.gotReq.Prompt)
	}
}
