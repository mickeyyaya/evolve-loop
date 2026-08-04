//go:build acs

// Package cycle1307 materialises the cycle-1307 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	sentinel-parse-tail-anchor → anchor evolve-verdict sentinel extraction at the
//	TAIL (last parseable candidate) in ONE shared implementation used by both
//	phasecontract sentinel parsing and deliverable verification, carrying the
//	cycle-1298 regression fixture (5 quoted decoys + 1 real tail sentinel), AND
//	documenting the rule in the operator-facing contract doc.
//
// State this cycle inherited (scout-report): the CODE half is already shipped on
// this branch — ParseVerdictSentinelFull walks candidates from the end
// (sentinel.go:90-98) and the fixture + unit tests are green. The predicates
// below therefore split into two groups on purpose:
//
//   - 001-004 are BEHAVIOURAL guards on the shipped semantics. They are expected
//     PRE-EXISTING GREEN and exist so a Builder that touches sentinel.go this
//     cycle (five sibling lanes received the identical inbox item — collision is
//     live) cannot silently regress first-match selection.
//   - 005 is the RED one: docs/architecture/deliverable-contract.md, the
//     connects_to target the inbox item named, has ZERO mention of tail-anchored
//     selection. That is this cycle's outstanding work.
//
// Predicate strategy — every behavioural predicate drives the real function or
// the real production entry point (deliverable.Verify) and asserts on its return
// value, never a source-grep of production code (the cycle-85 degenerate-
// predicate ban). 001 additionally carries an ANTI-VACUITY arm: it recomputes
// the OLD first-match selection over the same live fixture and asserts it gives
// the WRONG answer, so the predicate cannot pass on a repo where the fix was
// reverted. 004 is the wiring proof: it reaches the parser through
// deliverable.Verify — the entry the contract gate actually calls — not through
// phasecontract directly, so a tail-anchored parser with no production reader
// still fails it.
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

// fixtureRelPath is the cycle-1298 regression artifact the inbox item demanded
// be captured: the real adversarial-review report whose prose quotes the
// sentinel shape five times before emitting the genuine FAIL at the tail.
const fixtureRelPath = "go/internal/phasecontract/testdata/cycle1298-quoted-decoys.md"

// docRelPath is the operator-facing SSOT the inbox item named as connects_to.
const docRelPath = "docs/architecture/deliverable-contract.md"

// sentinelRE mirrors the production candidate regex. It exists ONLY to recompute
// the pre-fix FIRST-match selection for the anti-vacuity arm of 001 — never as a
// substitute for calling the real parser.
var sentinelRE = regexp.MustCompile(`<!--\s*evolve-verdict:\s*(\{.*?\})\s*-->`)

// firstMatchVerdict is the selection ParseVerdictSentinelFull used BEFORE the
// fix: take the first structural match and give up if it does not unmarshal.
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

// TestC1307_001_TailAnchoredSelectionOnLiveCycle1298Fixture is the crux
// behavioural predicate: the shared parser, run over the LIVE cycle-1298 report,
// must return the genuine tail verdict (FAIL, class gate_bypass) rather than any
// of the five decoys its prose quotes. The second arm proves the predicate is
// not vacuous — the retired first-match selection returns a DIFFERENT verdict on
// these exact bytes, so this test can only pass on a tail-anchored parser.
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

	// Anti-vacuity: the retired selection must disagree on these same bytes.
	if v, ok := firstMatchVerdict(content); ok && v == s.Verdict {
		t.Errorf("first-match selection also returns %q on this fixture — the fixture no longer discriminates tail-anchored from first-match selection", v)
	}
}

// TestC1307_002_MalformedEarlierDecoyDoesNotBlankTail is the elided-JSON shape
// that circuit-opened the gate in cycle-1298: an earlier candidate that does not
// unmarshal must be SKIPPED, not treated as fatal for the whole read.
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

// TestC1307_003_AllCandidatesInvalidReturnsNotOK is the NEGATIVE arm: skipping
// invalid candidates must not degrade into inventing a verdict. A document whose
// every candidate is malformed or verdict-less yields ok=false so the caller
// falls back to its legacy prose parser (tolerant reader).
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

// TestC1307_004_DeliverableGateReachesTailAnchoredParser is the WIRING PROOF.
// It never calls phasecontract: it drives deliverable.Verify — the entry the
// contract gate calls in production — over a real on-disk audit-report.md and
// asserts the failure-context check judged the TAIL sentinel.
//
// The fixture discriminates by construction: the earlier (quoted, decoy)
// sentinel is a FAIL with NO failure block, the tail sentinel is a clean PASS.
// A first-match reader judges the decoy and raises failure_context_missing; a
// tail-anchored reader judges the PASS and raises nothing. The inverse case then
// proves the check still BITES (a real FAIL without a block is still caught), so
// the first arm cannot pass merely because the check is dead.
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

// TestC1307_005_ContractDocDocumentsTailAnchoring is this cycle's RED predicate.
// The operator SSOT must explain the tail-anchoring rule, name the incident that
// motivated it, and point at the regression fixture — otherwise the next author
// of a sentinel reader re-derives first-match selection and re-breaks the gate.
//
// acs-predicate: config-check — the deliverable of this criterion IS operator
// prose, so document content is the system under test, not a proxy for it.
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
