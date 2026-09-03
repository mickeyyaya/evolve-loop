package reportdoc

// review_fields_test.go — the explanation-review section as auditors actually
// write it (cycles 1604, 1605, 1606 verbatim). Five of eleven consecutive
// FAILs were this section's FORMAT, not its substance: several `- Evidence:`
// lines, a line range in a citation, backticks around a value, prose under
// other field names. The parser reads what a careful reviewer wrote; the
// substance gates (every material path cited at a line) are unchanged.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// section loads a verbatim audit-report section from testdata (raw strings
// cannot hold the backticks auditors write).
func section(t *testing.T, cycle string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "explanation-review-cycle-"+cycle+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const reviewKeys = "Status,Build status,Document,Document SHA256,Evidence"

func reviewFields(t *testing.T, section string) (string, map[string]string) {
	t.Helper()
	body, found, err := Section(section, "Explanation Documentation")
	if err != nil || !found {
		t.Fatalf("section: found=%v err=%v", found, err)
	}
	fields, err := Fields(body, strings.Split(reviewKeys, ",")...)
	if err != nil {
		t.Fatalf("Fields: %v", err)
	}
	return body, fields
}

func TestFields_EvidenceIsListValued_Cycle1604(t *testing.T) {
	t.Parallel()
	_, fields := reviewFields(t, section(t, "1604"))
	ev := fields["evidence"]
	if strings.Count(ev, "docs/explain/builds/cycle-1604-01m1fd688vm2a4y7kz2nqyt870.md:") != 3 {
		t.Fatalf("four Evidence lines must be kept as one list-valued field, got %q", ev)
	}
	if err := RequirePathLineEvidence(ev, "docs/explain/builds/cycle-1604-01m1fd688vm2a4y7kz2nqyt870.md", "go/internal/phaseio/digest.go"); err != nil {
		t.Fatalf("the document and digest.go are cited at a line across the Evidence lines and must pass: %v", err)
	}
	// The substance rule is untouched: 1604 never cited tdd.go at a line, so the
	// ONE remaining correction is a real one, not "duplicate evidence fields".
	if err := RequirePathLineEvidence(ev, "go/internal/phases/tdd/tdd.go"); err == nil || !strings.Contains(err.Error(), "tdd.go with path:line") {
		t.Fatalf("tdd.go is not cited at a line in 1604 and must still be required: %v", err)
	}
	if fields["status"] != "VERIFIED" {
		t.Fatalf("status = %q", fields["status"])
	}
}

func TestFields_SingleValuedKeysStillRejectDuplicates(t *testing.T) {
	t.Parallel()
	_, err := Fields("- Status: VERIFIED\n- Status: NEEDS_CORRECTION\n- Evidence: a.go:1 twenty characters here\n", "Status", "Evidence")
	if err == nil || !strings.Contains(err.Error(), "duplicate status") {
		t.Fatalf("a repeated single-valued field is ambiguous and must still be rejected, got %v", err)
	}
}

func TestCitation_AcceptsLineRangesAndColumns_Cycle1605(t *testing.T) {
	t.Parallel()
	_, fields := reviewFields(t, section(t, "1605"))
	if err := RequirePathLineEvidence(fields["evidence"],
		"docs/explain/builds/cycle-1605-01m1fd689e6f9m7sz7gj985tz1.md",
		"go/.apicover-enforce", "go/internal/decisionsample/sampler.go", "go/internal/policy/policy.go"); err != nil {
		t.Fatalf("cycle-1605 cites sampler.go:23-48 (a range) and must pass: %v", err)
	}
	for _, ev := range []string{"see pkg/x.go:23-48 for the seam", "see `pkg/x.go:12:3` col cite", "see pkg/x.go#L7-L9 too"} {
		if err := RequirePathLineEvidence(ev, "pkg/x.go"); err != nil {
			t.Errorf("%q must count as a path:line citation: %v", ev, err)
		}
	}
	if err := RequirePathLineEvidence("mentions pkg/x.go:abc and pkg/x.go without a line", "pkg/x.go"); err == nil {
		t.Error("a path without a line is still not a citation")
	}
}

func TestFields_StripsBackticksAroundValues_Cycle1606(t *testing.T) {
	t.Parallel()
	_, fields := reviewFields(t, section(t, "1606"))
	if fields["document"] != "docs/explain/builds/cycle-1606-01m1fd689x791deg8axtk4kr9p.md" {
		t.Fatalf("backticked Document must parse to the bare path, got %q", fields["document"])
	}
	if fields["document sha256"] != "14ba1079c7e4b145a8bc37c250192cab72a59cc7211db702a700d03bbf4f1660" || fields["status"] != "NEEDS_CORRECTION" {
		t.Fatalf("sha=%q status=%q", fields["document sha256"], fields["status"])
	}
}

func TestEvidenceOrBody_FallsBackToTheSectionProse_Cycle1606(t *testing.T) {
	t.Parallel()
	body, fields := reviewFields(t, section(t, "1606"))
	if fields["evidence"] != "" {
		t.Fatalf("1606 has no Evidence field, got %q", fields["evidence"])
	}
	ev := EvidenceOrBody(body, fields["evidence"])
	if !strings.Contains(ev, "build-explanation.json:9") {
		t.Fatalf("the section prose is the evidence when no Evidence field exists, got %q", ev)
	}
	if got := EvidenceOrBody(body, "explicit a.go:1 evidence"); got != "explicit a.go:1 evidence" {
		t.Fatalf("an explicit Evidence field wins, got %q", got)
	}
}

// TestReviewFields_ParsesOnceForBothGates — the single home both phase gates
// call: Fields over the allowed keys with the evidence text resolved once.
func TestReviewFields_ParsesOnceForBothGates(t *testing.T) {
	t.Parallel()
	body, _, _ := Section(section(t, "1606"), "Explanation Documentation")
	fields, err := ReviewFields(body, strings.Split(reviewKeys, ",")...)
	if err != nil {
		t.Fatal(err)
	}
	if fields["status"] != "NEEDS_CORRECTION" || !strings.Contains(fields["evidence"], "build-explanation.json:9") {
		t.Fatalf("ReviewFields must resolve the prose evidence: %+v", fields)
	}
	if _, err := ReviewFields("- Status: A\n- Status: B\n", "Status"); err == nil {
		t.Fatal("ReviewFields must surface Fields errors")
	}
}
