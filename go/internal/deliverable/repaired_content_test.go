package deliverable

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestRepairedVerdictContent_RepairsTheSoleRecoverableBadVerdictInMemory(t *testing.T) {
	fenced := "# Audit Report\n\n## Verdict\n\n**PASS** — every criterion carries evidence.\n\n## Ledger Entry\n\n```json\n{\"verdict\": \"PASS\", \"green\": 196, \"red\": 0}\n```\n"
	res := Result{OK: false, Phase: "audit", ArtifactPath: "audit-report.md", Content: fenced,
		Violations: []Violation{{Code: CodeBadVerdict, Message: "no parseable verdict"}}}
	repaired, ok := RepairedVerdictContent(res)
	if !ok {
		t.Fatalf("a sole recoverable bad_verdict repairs in memory")
	}
	if v, ok := phasecontract.ParseVerdictSentinel(repaired); !ok || v != "PASS" {
		t.Fatalf("the repaired bytes carry a parseable sentinel, got ok=%v v=%q:\n%s", ok, v, repaired)
	}
	if strings.Contains(repaired, "```json") {
		t.Errorf("the fence is replaced by the canonical sentinel line:\n%s", repaired)
	}
	if _, ok := RepairedVerdictContent(Result{OK: true, Content: fenced}); ok {
		t.Error("a verified result needs no repair")
	}
	other := res
	other.Violations = []Violation{{Code: CodeBadVerdict}, {Code: CodeMissingSection, Message: "x"}}
	if _, ok := RepairedVerdictContent(other); ok {
		t.Error("bad_verdict beside another violation is not a sole salvage")
	}
	two := res
	two.Content = fenced + "\n```json\n{\"verdict\": \"FAIL\"}\n```\n"
	if _, ok := RepairedVerdictContent(two); ok {
		t.Error("two candidates are ambiguous — never repaired")
	}
}
