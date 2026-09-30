//go:build acs

package cycle1307

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const fixtureRelPath = "go/internal/phasecontract/testdata/cycle1298-quoted-decoys.md"

const docRelPath = "docs/architecture/deliverable-contract.md"

var sentinelRE = regexp.MustCompile(`<!--\s*evolve-verdict:\s*(\{.*?\})\s*-->`)

func firstMatchVerdict(content string) (string, bool) {
	m := sentinelRE.FindStringSubmatch(content)
	if m == nil {
		return "", false
	}
	var s struct {
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal([]byte(m[1]), &s); err != nil || s.Verdict == "" {
		return "", false
	}
	return s.Verdict, true
}

func TestC1307_001_TailAnchoredSelectionOnLiveCycle1298Fixture(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, fixtureRelPath))
	if err != nil {
		t.Fatalf("regression fixture %s unreadable: %v", fixtureRelPath, err)
	}
	content := string(raw)

	s, ok := phasecontract.ParseVerdictSentinelFull(content)
	if !ok {
		t.Fatalf("ParseVerdictSentinelFull(cycle-1298 fixture): ok=false — quoted decoys blanked the real tail verdict")
	}
	if s.Verdict != "FAIL" {
		t.Errorf("verdict = %q, want %q (the tail sentinel is the producer's real verdict)", s.Verdict, "FAIL")
	}
	if s.Failure == nil || s.Failure.Class != "gate_bypass" {
		t.Errorf("failure block = %+v, want class %q from the tail sentinel", s.Failure, "gate_bypass")
	}

	if v, ok := firstMatchVerdict(content); ok && v == s.Verdict {
		t.Errorf("first-match selection also returns %q on this fixture — the fixture no longer discriminates tail-anchored from first-match selection", v)
	}
}

func TestC1307_002_MalformedEarlierDecoyDoesNotBlankTail(t *testing.T) {
	doc := strings.Join([]string{
		"# Adversarial Review",
		"Prose quoting the shape with elided JSON:",
		"`<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"WARN\",…} -->`",
		"",
		phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL",
			&phasecontract.FailureBlock{Class: "gate_bypass", Defects: []string{"real defect"}}),
	}, "\n")

	s, ok := phasecontract.ParseVerdictSentinelFull(doc)
	if !ok {
		t.Fatalf("ok=false — an unparseable earlier decoy blanked the real tail sentinel")
	}
	if s.Verdict != "FAIL" {
		t.Errorf("verdict = %q, want FAIL", s.Verdict)
	}
}

func TestC1307_003_AllCandidatesInvalidReturnsNotOK(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"all malformed", "<!-- evolve-verdict: {\"verdict\":\"FAIL\",…} -->\n<!-- evolve-verdict: {not json} -->"},
		{"verdict-less", "<!-- evolve-verdict: {\"phase\":\"audit\",\"schema_version\":1} -->"},
		{"no candidate at all", "# Report\nNo sentinel anywhere.\n"},
		{"placeholder echo only", "<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\",\"schema_version\":2,\"failure\":{\"class\":\"x\",\"defects\":[\"<one line per defect>\"]}} -->"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if s, ok := phasecontract.ParseVerdictSentinelFull(tc.doc); ok {
				t.Errorf("ok=true (verdict %q) — no valid candidate exists; the parser must report not-found, not invent one", s.Verdict)
			}
		})
	}
}

func TestC1307_004_DeliverableGateReachesTailAnchoredParser(t *testing.T) {
	decoy := "Quoted contract example in prose: " +
		phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", nil)

	for _, tc := range []struct {
		name        string
		tail        string
		wantMissing bool
	}{
		{
			name:        "tail PASS beats quoted blockless-FAIL decoy",
			tail:        phasecontract.RenderVerdictSentinel("audit", "PASS"),
			wantMissing: false,
		},
		{
			name:        "genuine tail FAIL without a failure block is still caught",
			tail:        phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", nil),
			wantMissing: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			report := strings.Join([]string{"# Audit Report", decoy, "", tc.tail, ""}, "\n")
			if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(report), 0o644); err != nil {
				t.Fatalf("write audit-report.md: %v", err)
			}

			res, err := deliverable.Verify("audit", phasecontract.Roots{Workspace: ws, Worktree: ws, EvolveDir: ws})
			if err != nil {
				t.Fatalf("deliverable.Verify: %v", err)
			}
			got := false
			for _, v := range res.Violations {
				if v.Code == deliverable.CodeFailureContextMissing {
					got = true
				}
			}
			if got != tc.wantMissing {
				t.Errorf("failure_context_missing = %v, want %v — the contract gate is reading the wrong sentinel candidate (violations: %+v)", got, tc.wantMissing, res.Violations)
			}
		})
	}
}

// acs-predicate: config-check — the deliverable of this criterion IS operator
func TestC1307_005_ContractDocDocumentsTailAnchoring(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, docRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s unreadable: %v", docRelPath, err)
	}
	doc := strings.ToLower(string(raw))

	for _, want := range []struct {
		needle string
		why    string
	}{
		{"tail-anchor", "the rule itself (tail-anchored selection)"},
		{"cycle-1298", "the source incident whose quoted decoys circuit-opened the contract gate"},
		{strings.ToLower(fixtureRelPath), "the regression fixture an operator can run"},
	} {
		if !strings.Contains(doc, want.needle) {
			t.Errorf("%s does not mention %q — missing %s", docRelPath, want.needle, want.why)
		}
	}
}
