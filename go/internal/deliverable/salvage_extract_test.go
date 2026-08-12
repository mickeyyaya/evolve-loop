package deliverable

// salvage_extract_test.go — names and exercises SalvageVerdict and
// SalvageSummaryLine directly (apicover naming floor: house rule 1). The
// behavioral acceptance criteria live in go/acs/cycle1392/predicates_test.go
// (real Reviewer wiring, all three recoverable shapes, ambiguity refusal);
// these unit tests only need to prove the exported symbols are named and
// executed by a real assertion in the package's own (non-acs-tagged) suite.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// unpairedQuoteAmbiguityBypass reproduces cycle-1424 audit defect
// d4982b388c4982275303ee68529b9313d (CRITICAL) through the production seam.
//
// One unpaired `"` in prose is the whole exploit. It leaves the string-aware
// scan believing every byte after it is inside a string literal, so BOTH
// verdict objects below become invisible to candidateCount and the
// `candidateCount > 1` ambiguity guard reads a two-candidate report as
// unambiguous. Classification is unaffected, because step 2 computes quote
// parity FENCE-LOCALLY (verdictObjSpan runs over the fence body alone) and
// still qualifies the fenced PASS. Salvage therefore repairs the stray PASS
// while the report's own genuine — and malformed — FAIL stays unparseable, and
// ParseVerdictSentinelFull reads the repaired PASS: a report whose intended
// verdict is FAIL reaches Approve=true.
const unpairedQuoteAmbiguityBypass = "## Verdict\n\n" +
	`The failing phase said "the run was inconclusive.` + "\n\n" +
	"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n\n" +
	`<!-- evolve-verdict: {"phase":"audit","verdict":"FAIL","schema_version":2,} -->` + "\n"

// reviewFixture drives the REAL production caller (Reviewer.Review, the seam
// core.DeliverableReviewer is wired to) rather than SalvageVerdict directly —
// the defect being closed is a gate DECISION defect, and only the gate's own
// decision can prove it closed.
func reviewFixture(t *testing.T, content string) core.ReviewResult {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	r := NewReviewerWithCatalogStageReportSize(
		config.StageEnforce, phasespec.Catalog{}, config.StageEnforce, config.StageOff, 0)
	return r.Review(context.Background(), core.ReviewInput{Phase: "audit", Workspace: ws, ProjectRoot: t.TempDir()})
}

// soleViolationIsBadVerdict proves through the production Verify entry point
// that a fixture reaches the salvage path at all, so a blocked result can never
// be credited to some second violation the fixture drifted into.
func soleViolationIsBadVerdict(t *testing.T, label, content string) {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	res, err := VerifyWithStage("audit", phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
	if err != nil {
		t.Fatalf("verify %s: %v", label, err)
	}
	if len(res.Violations) != 1 || res.Violations[0].Code != CodeBadVerdict {
		t.Fatalf("precondition (%s): fixture must fail for bad_verdict ALONE so salvage is reached; got %+v", label, res.Violations)
	}
}

// TestReview_RefusesSalvage_WhenAnUnpairedQuoteHidesASecondCandidate is the
// regression pin for the ambiguity guard's vacuity.
//
// The guard's contract is "refuse when the content carries more than one
// verdict-bearing candidate ANYWHERE". A counter that can only ever UNDERCOUNT
// under attacker-controlled quoting does not implement that contract: it
// implements "refuse when the attacker permits". The two preconditions below
// fence the assertion so a green result cannot come from salvage simply having
// stopped working — the fixture must still reach salvage (sole bad_verdict) and
// must still classify recoverable.
func TestReview_RefusesSalvage_WhenAnUnpairedQuoteHidesASecondCandidate(t *testing.T) {
	soleViolationIsBadVerdict(t, "unpairedQuoteAmbiguityBypass", unpairedQuoteAmbiguityBypass)
	if cls := ClassifyBadVerdict(unpairedQuoteAmbiguityBypass); !cls.Recoverable {
		t.Fatalf("precondition: fence-local parity must still qualify the fenced PASS, so a refusal is attributable to the ambiguity guard and not to classification; got %+v", cls)
	}

	if got := reviewFixture(t, unpairedQuoteAmbiguityBypass); got.Approve {
		t.Errorf("CONTRACT-GATE BYPASS: the report carries TWO verdict-bearing candidates (a fenced PASS and the phase's own malformed FAIL) yet was APPROVED. One unpaired quote in prose flips candidateCount to 0, so the `> 1` ambiguity guard never fires while fence-local parity still qualifies the PASS — the phase's FAIL is laundered into an approval (cycle-1424 audit d4982b388c4982275303ee68529b9313d, CRITICAL). want Approve=false, got Approve=true (reason=%q)", got.Reason)
	}
}

// TestReview_StillSalvages_TheGenuineSoleCandidate is the anti-over-correction
// control. The cheapest way to make the predicate above green is to make
// candidateCount pessimistic enough that nothing ever salvages; this fixture —
// one fenced verdict object, no stray quote, no second candidate — must keep
// salvaging for the layer to still exist.
func TestReview_StillSalvages_TheGenuineSoleCandidate(t *testing.T) {
	const genuineFencedPass = "## Verdict\n" +
		"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"

	soleViolationIsBadVerdict(t, "genuineFencedPass", genuineFencedPass)
	if got := reviewFixture(t, genuineFencedPass); !got.Approve {
		t.Errorf("over-correction: the canonical single-candidate fenced-json shape — the one salvage exists for — must still be recovered; want Approve=true, got false (reason=%q)", got.Reason)
	}
}

func TestSalvageVerdict_RecoversFencedJSON(t *testing.T) {
	res := Result{
		Phase: "audit",
		// The "## Verdict" heading is REQUIRED in the fixture: salvage now
		// re-verifies the repaired bytes against the audit contract, and a
		// headingless report is a missing_section failure that repair cannot
		// fix. (Its absence also made the hand-built Violations list unreal —
		// the real Verify would have reported missing_section alongside
		// bad_verdict, which is precisely the multi-violation shape salvage
		// must refuse.)
		Content: "## Verdict\n```json\n{\"phase\":\"audit\",\"verdict\":\"PASS\"}\n```\n",
		Violations: []Violation{
			{Code: CodeBadVerdict, Message: "no parseable verdict"},
		},
	}

	got, applied := SalvageVerdict(res)
	if !applied {
		t.Fatalf("want applied=true for a fenced-JSON recoverable verdict, got false")
	}
	if !got.OK || len(got.Violations) != 0 {
		t.Errorf("want salvaged Result approved with zero Violations, got OK=%v Violations=%v", got.OK, got.Violations)
	}
	// CONTRACT CHANGE (cycle-1441 audit H1, HIGH): this assertion used to demand
	// Content come back byte-identical to the malformed input. That is precisely
	// the defect — salvage re-verified `repaired` and then returned the ORIGINAL,
	// so the gate reported OK=true over bytes it had never approved. An approved
	// salvage now returns the bytes it verified; the "changes nothing" invariant
	// survives intact where it belongs, on the REFUSAL path
	// (TestSalvageVerdict_RefusesGenuinelyAbsent, below).
	if got.Content == res.Content {
		t.Errorf("an approved salvage must return the repaired bytes it re-verified, not the malformed original")
	}
	if s, ok := phasecontract.ParseVerdictSentinelFull(got.Content); !ok || s.Verdict != "PASS" {
		t.Errorf("the salvaged Content must parse as a canonical verdict sentinel; got ok=%v sentinel=%+v content=%q", ok, s, got.Content)
	}
}

// TestSalvageVerdict_RecoversSentinelTrailingComma covers the OTHER recoverable
// sentinel shape: a canonical evolve-verdict comment whose payload is JSON with
// a trailing comma. It drives repairVerdict's SalvagePatternTrailingComma branch
// (salvage_extract.go:376-379), which shipped at zero executions because every
// existing test drives the fenced-json path (cycle-1441 audit H2, HIGH) — in a
// transform with cycle-1406/cycle-1399 CRITICAL history for mis-repaired spans.
func TestSalvageVerdict_RecoversSentinelTrailingComma(t *testing.T) {
	const malformed = "## Verdict\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",} -->\n"
	res := Result{
		Phase:      "audit",
		Content:    malformed,
		Violations: []Violation{{Code: CodeBadVerdict, Message: "no parseable verdict"}},
	}

	// Precondition: these bytes really are unparseable today, so the recovery
	// below is load-bearing rather than a restatement of the input.
	if _, ok := phasecontract.ParseVerdictSentinelFull(malformed); ok {
		t.Fatalf("precondition: the trailing-comma payload must NOT parse before salvage")
	}
	if cls := ClassifyBadVerdict(malformed); !cls.Recoverable || cls.Pattern != SalvagePatternTrailingComma {
		t.Fatalf("precondition: fixture must classify as trailing-comma; got recoverable=%v pattern=%q", cls.Recoverable, cls.Pattern)
	}

	got, applied := SalvageVerdict(res)
	if !applied {
		t.Fatalf("want applied=true for a trailing-comma sentinel payload, got false")
	}
	s, ok := phasecontract.ParseVerdictSentinelFull(got.Content)
	if !ok {
		t.Fatalf("the salvaged Content must parse; got %q", got.Content)
	}
	// Reformatting only: the comma is dropped and every field the agent wrote is
	// carried across verbatim — the payload is relocated, never re-authored.
	if s.Phase != "audit" || s.Verdict != "PASS" {
		t.Errorf("repair must preserve the agent's own field values; got %+v", s)
	}
	if strings.Contains(got.Content, ",}") {
		t.Errorf("the trailing comma must be gone from the repaired bytes; got %q", got.Content)
	}
}

func TestSalvageVerdict_RefusesGenuinelyAbsent(t *testing.T) {
	res := Result{
		Phase:   "audit",
		Content: "no verdict of any kind here",
		Violations: []Violation{
			{Code: CodeBadVerdict, Message: "no parseable verdict"},
		},
	}

	got, applied := SalvageVerdict(res)
	if applied {
		t.Errorf("want applied=false for a genuinely absent verdict, got true (result=%+v)", got)
	}
	if got.OK != res.OK || got.Content != res.Content || len(got.Violations) != len(res.Violations) {
		t.Errorf("a refused salvage must return res UNCHANGED; got %+v want %+v", got, res)
	}
}

func TestSalvageSummaryLine_SurfacesAndIsSilentAtZero(t *testing.T) {
	dir := t.TempDir()

	if line := SalvageSummaryLine(dir); line != "" {
		t.Errorf("want empty string when the sidecar is absent (no zero-noise), got %q", line)
	}

	sidecar := filepath.Join(dir, salvageAppliedFile)
	record := `{"event_type":"salvage_applied","phase":"audit","pattern":"fenced-json"}` + "\n"
	if err := os.WriteFile(sidecar, []byte(record), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	line := SalvageSummaryLine(dir)
	if line == "" {
		t.Fatalf("want a non-empty summary line for 1 salvage-applied record, got empty")
	}
	if want := "1"; !strings.Contains(line, want) {
		t.Errorf("want the total count %q present in %q", want, line)
	}
	if !strings.Contains(line, "fenced-json") {
		t.Errorf("want the pattern breakdown present in %q", line)
	}
}
