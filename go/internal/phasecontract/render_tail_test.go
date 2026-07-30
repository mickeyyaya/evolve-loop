package phasecontract

// render_tail_test.go — RED contract for the GENERATION-POINT deliverable
// contract (inbox contract-requirements-at-generation-point, 0.90).
//
// Evidence: Anthropic's guidance is that Claude follows instructions in the
// USER TURN better than in a system-ish preamble, and XML-tagged sections parse
// unambiguously. Our required-sections list and machine-verdict sentinel live in
// the cacheable PREFIX block (RenderContractBlockStage) — far from the
// generation point — yet the CORRECTION prompt, which restates the identical
// requirements in the turn TAIL, is what actually gets compliance.
//
// Contract under test:
//
//	RenderContractTail(c Contract, artifactPath string) string
//
// It is the tail-most region of the dispatch prompt: the existing
// FooterMarker path line PLUS one compact XML-tagged <deliverable-contract>
// block carrying the exact artifact path, the required section headings
// verbatim, and the rendered verdict sentinel.
//
// The load-bearing invariant is NO SECOND TEMPLATE: every string in the block
// must be projected from the SAME sources the prefix block and the detector
// read (Contract.Sections / Contract.RequiredKeys / RenderVerdictSentinel), so
// writer and detector cannot drift. Tests below assert projection, not literals.

import (
	"strings"
	"testing"
)

// TestRenderContractTail_ProjectsPathSectionsAndSentinel is the primary
// acceptance: for the verdict-bearing phase (audit) the tail block carries all
// three machine facts, and each is byte-identical to its single source.
func TestRenderContractTail_ProjectsPathSectionsAndSentinel(t *testing.T) {
	c := mustContract(t, "audit")
	const path = "/abs/.evolve/runs/cycle-1218/audit-report.md"
	tail := RenderContractTail(c, path)

	// The volatile path line the prefix block cross-references must survive —
	// RenderContractTail is a superset of RenderContractFooter, not a rename.
	if !strings.Contains(tail, RenderContractFooter(c, path)) {
		t.Errorf("tail must still carry the verbatim footer path line; got:\n%s", tail)
	}
	if !strings.Contains(tail, "<deliverable-contract phase=\"audit\">") ||
		!strings.Contains(tail, "</deliverable-contract>") {
		t.Fatalf("tail must carry an XML-tagged <deliverable-contract> block; got:\n%s", tail)
	}
	if !strings.Contains(tail, "<artifact-path>"+path+"</artifact-path>") {
		t.Errorf("tail must declare the EXACT artifact path inside the block; got:\n%s", tail)
	}
	// Sections are PROJECTED from the contract, never re-typed here.
	for _, s := range c.Sections {
		if !strings.Contains(tail, "<section>"+s.Canonical+"</section>") {
			t.Errorf("tail must name required section %q verbatim; got:\n%s", s.Canonical, tail)
		}
	}
	// Sentinel comes from the ONE template source, so the detector parses the
	// exact bytes the writer was shown. For a failure-context phase (audit) the
	// exemplar must be the FAILURE-BEARING form: a bare PASS exemplar in the
	// recency-dominant tail is the version the agent follows, and a FAIL/WARN
	// sentinel without the block is a contract violation (review HIGH).
	var want string
	if c.RequireFailureContext || c.RequireFailureContextPhaseIO {
		want = RenderVerdictSentinelWithFailure(c.Phase, "FAIL", failureExemplar(c.Phase))
	} else {
		want = RenderVerdictSentinel(c.Phase, c.Verdicts[0])
	}
	if !strings.Contains(tail, want) {
		t.Errorf("tail sentinel must be the ONE-template output %q; got:\n%s", want, tail)
	}
	// Writer/detector no-drift, with the cycle-603 guard's polarity respected:
	// the failure-bearing EXEMPLAR carries literal placeholder tokens, so the
	// production detector must REJECT it — that is exactly what stops a
	// prompt example captured from scrollback being read as a real verdict.
	// The bare template (non-failure-context phases) must parse.
	_, parsed := ParseVerdictSentinelFull(tail)
	if c.RequireFailureContext || c.RequireFailureContextPhaseIO {
		if parsed {
			t.Error("the failure-bearing exemplar must be REJECTED by the detector (placeholder-echo guard, cycle-603) — a printed example must never read as a real verdict")
		}
	} else if !parsed {
		t.Error("the bare rendered sentinel template must parse with the production detector (writer/detector no-drift)")
	}
}

// TestRenderContractTail_SentinelOnlyForVerdictPhases is the anti-regression
// arm: build/scout/triage emit no verdict by default, and the prefix block
// omits the sentinel for them. The tail must mirror that gate exactly — a tail
// that taught every phase a sentinel would introduce verdict lines the
// always-on classifier Pass 0 has never seen in production.
func TestRenderContractTail_SentinelOnlyForVerdictPhases(t *testing.T) {
	c := mustContract(t, "build")
	tail := RenderContractTail(c, "/ws/build-report.md")
	if strings.Contains(tail, "evolve-verdict") {
		t.Errorf("build declares no Verdicts, so the tail must carry no sentinel; got:\n%s", tail)
	}
	if !strings.Contains(tail, "<section>## Changes</section>") {
		t.Errorf("build tail must still carry its required section; got:\n%s", tail)
	}
}

// TestRenderContractTail_JSONUsesRequiredKeys — a JSON deliverable has keys, not
// markdown headings; the block must project the same RequiredKeys the verifier
// enforces and must not invent a section list.
func TestRenderContractTail_JSONUsesRequiredKeys(t *testing.T) {
	c := mustContract(t, "orchestrator")
	tail := RenderContractTail(c, "/ev/cycle-state.json")
	for _, k := range c.RequiredKeys {
		if !strings.Contains(tail, "<key>"+k+"</key>") {
			t.Errorf("JSON tail must name required key %q; got:\n%s", k, tail)
		}
	}
	if strings.Contains(tail, "<required-sections>") {
		t.Errorf("a JSON deliverable has no markdown sections; got:\n%s", tail)
	}
	if strings.Contains(tail, "evolve-verdict") {
		t.Errorf("a JSON deliverable has no verdict sentinel; got:\n%s", tail)
	}
}

// TestRenderContractTail_NoArtifactEmitsNoBlock — ship's deliverable is a pushed
// commit, not a file (Contract.NoArtifact). Rendering a path contract for it
// would instruct the agent to write a file that must not exist, so the tail
// degrades to the footer alone.
func TestRenderContractTail_NoArtifactEmitsNoBlock(t *testing.T) {
	c := mustContract(t, "ship")
	tail := RenderContractTail(c, "/ws")
	if strings.Contains(tail, "<deliverable-contract") {
		t.Errorf("a NoArtifact contract must emit no deliverable-contract block; got:\n%s", tail)
	}
}

// TestRenderContractTail_Deterministic — the tail is appended per dispatch; a
// non-deterministic render would defeat prompt caching of everything above it
// and make golden-prompt diffs unreadable.
func TestRenderContractTail_Deterministic(t *testing.T) {
	c := mustContract(t, "audit")
	if RenderContractTail(c, "/p/audit-report.md") != RenderContractTail(c, "/p/audit-report.md") {
		t.Error("RenderContractTail must be deterministic")
	}
}
