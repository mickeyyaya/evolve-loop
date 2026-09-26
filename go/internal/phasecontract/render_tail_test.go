package phasecontract

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderContractTail_ProjectsPathSectionsAndSentinel(t *testing.T) {
	c := mustContract(t, "audit")
	const path = "/abs/.evolve/runs/cycle-1218/audit-report.md"
	tail := RenderContractTail(c, path, filepath.Dir(path))

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
	for _, s := range c.Sections {
		if !strings.Contains(tail, "<section>"+s.Canonical+"</section>") {
			t.Errorf("tail must name required section %q verbatim; got:\n%s", s.Canonical, tail)
		}
	}
	var want string
	if c.RequireFailureContext || c.RequireFailureContextPhaseIO {
		want = RenderVerdictSentinelWithFailure(c.Phase, "FAIL", failureExemplar(c.Phase))
	} else {
		want = RenderVerdictSentinel(c.Phase, c.Verdicts[0])
	}
	if !strings.Contains(tail, want) {
		t.Errorf("tail sentinel must be the ONE-template output %q; got:\n%s", want, tail)
	}
	_, parsed := ParseVerdictSentinelFull(tail)
	if c.RequireFailureContext || c.RequireFailureContextPhaseIO {
		if parsed {
			t.Error("the failure-bearing exemplar must be REJECTED by the detector (placeholder-echo guard, cycle-603) — a printed example must never read as a real verdict")
		}
	} else if !parsed {
		t.Error("the bare rendered sentinel template must parse with the production detector (writer/detector no-drift)")
	}
}

func TestRenderContractTail_SentinelOnlyForVerdictPhases(t *testing.T) {
	c := mustContract(t, "build")
	tail := RenderContractTail(c, "/ws/build-report.md", "/ws")
	if strings.Contains(tail, "evolve-verdict") {
		t.Errorf("build declares no Verdicts, so the tail must carry no sentinel; got:\n%s", tail)
	}
	if !strings.Contains(tail, "<section>## Changes</section>") {
		t.Errorf("build tail must still carry its required section; got:\n%s", tail)
	}
}

func TestRenderContractTail_JSONUsesRequiredKeys(t *testing.T) {
	c := mustContract(t, "orchestrator")
	tail := RenderContractTail(c, "/ev/cycle-state.json", "/ev")
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

func TestRenderContractTail_NoArtifactEmitsNoBlock(t *testing.T) {
	c := mustContract(t, "ship")
	tail := RenderContractTail(c, "/ws", "/ws")
	if strings.Contains(tail, "<deliverable-contract") {
		t.Errorf("a NoArtifact contract must emit no deliverable-contract block; got:\n%s", tail)
	}
}

func TestRenderContractTail_Deterministic(t *testing.T) {
	c := mustContract(t, "audit")
	if RenderContractTail(c, "/p/audit-report.md", "/p") != RenderContractTail(c, "/p/audit-report.md", "/p") {
		t.Error("RenderContractTail must be deterministic")
	}
}
