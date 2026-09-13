package explanationdocs

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

// TestValidateReviewedHandoff_ContractCore names the single home of the
// review contract the audit and retro gates both delegate to (architecture
// review 2026-09-01). Since 2026-09-13 (ADR-0102, operator decision) the
// reviewer's REASONING is the gate and the section's SHAPE is advisory: the
// status enum, the build-status echo, the required/not_applicable document
// match and the path:line citations are reported as advisories on the phase
// record, never as a blocking error. The reviewer's own NEEDS_CORRECTION is a
// judgment, not a shape finding. The one loud error left is a host-side
// defect — an unknown handoff status, a missing view — which no reviewer
// wrote and which must not pass through both phase gates silently.
func TestValidateReviewedHandoff_ContractCore(t *testing.T) {
	required := &phaseio.ExplanationView{Status: "required", DocumentPath: "d.md", DocumentSHA256: "abc", MaterialPaths: []string{"go/app.go"}}
	na := &phaseio.ExplanationView{Status: "not_applicable"}
	cited := "compared d.md:1 with go/app.go:3 line by line against the diff"
	full := func(status string) map[string]string {
		return map[string]string{"status": status, "build status": "required", "document": "d.md", "document sha256": "abc", "evidence": cited}
	}
	cases := []struct {
		name         string
		fields       map[string]string
		view         *phaseio.ExplanationView
		wantStatus   string
		wantAdvisory string // "" = a clean review
	}{
		{name: "a fully cited VERIFIED review is clean", fields: full("VERIFIED"), view: required, wantStatus: "VERIFIED"},
		{name: "NEEDS_CORRECTION is the reviewer's judgment, not a shape finding", fields: full("NEEDS_CORRECTION"), view: required, wantStatus: "NEEDS_CORRECTION"},
		{name: "an unknown review status is advisory", fields: map[string]string{"status": "MAYBE", "build status": "required", "document": "d.md", "document sha256": "abc", "evidence": cited}, view: required, wantAdvisory: "must be VERIFIED or NEEDS_CORRECTION"},
		{name: "a build-status echo mismatch is advisory", fields: map[string]string{"status": "VERIFIED", "build status": "not_applicable", "document": "d.md", "document sha256": "abc", "evidence": cited}, view: required, wantStatus: "VERIFIED", wantAdvisory: "does not match the host Build handoff"},
		{name: "a required document mismatch is advisory", fields: map[string]string{"status": "VERIFIED", "build status": "required", "document": "other.md", "document sha256": "abc", "evidence": cited}, view: required, wantStatus: "VERIFIED", wantAdvisory: "document does not match the host Build handoff"},
		{name: "a not-applicable review with a document is advisory", fields: map[string]string{"status": "VERIFIED", "build status": "not_applicable", "document": "d.md", "evidence": "verified the base-bound diff contains no material changes"}, view: na, wantStatus: "VERIFIED", wantAdvisory: "must omit Document and Document SHA256"},
		{name: "path-only citations are advisory", fields: map[string]string{"status": "VERIFIED", "build status": "required", "document": "d.md", "document sha256": "abc", "evidence": "reviewed d.md and go/app.go against the implementation"}, view: required, wantStatus: "VERIFIED", wantAdvisory: "path:line"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ValidateReviewedHandoff(context.Background(), tc.fields, tc.view, "", "")
			if err != nil {
				t.Fatalf("a review's shape never blocks: %v", err)
			}
			if r.Status != tc.wantStatus {
				t.Errorf("status %q, want %q", r.Status, tc.wantStatus)
			}
			if tc.wantAdvisory == "" {
				if len(r.Advisories) != 0 {
					t.Errorf("a clean review carries no advisories: %v", r.Advisories)
				}
				return
			}
			if !strings.Contains(strings.Join(r.Advisories, "\n"), tc.wantAdvisory) {
				t.Errorf("want an advisory containing %q, got %v", tc.wantAdvisory, r.Advisories)
			}
		})
	}
}

func TestValidateReviewedHandoff_HostDefectsStayLoud(t *testing.T) {
	fields := map[string]string{"status": "VERIFIED", "build status": "weird", "evidence": "compared d.md:1 with go/app.go:3 line by line"}
	if _, err := ValidateReviewedHandoff(context.Background(), fields, &phaseio.ExplanationView{Status: "weird"}, "", ""); err == nil || !strings.Contains(err.Error(), "unknown status") {
		t.Fatalf("an unknown handoff status must fail loudly: %v", err)
	}
	if _, err := ValidateReviewedHandoff(context.Background(), fields, nil, "", ""); err == nil {
		t.Fatal("a missing handoff view must fail loudly")
	}
}

// Review collects findings in the order they were made; a fresh Review is
// clean.
func TestReview_AdvisoriesKeepTheirOrder(t *testing.T) {
	var r Review
	if r.Status != "" || len(r.Advisories) != 0 {
		t.Fatalf("a fresh Review is clean: %+v", r)
	}
	r.advise("first")
	r.advise("second")
	if want := []string{"first", "second"}; strings.Join(r.Advisories, ",") != strings.Join(want, ",") {
		t.Errorf("advisories %v, want %v", r.Advisories, want)
	}
}

// AdvisoryPrefix is the one marker both phases stamp on a record; HostReason
// renders the host's account of a lost handoff, or nothing.
func TestAdvisoryPrefixAndHostReason_AreTheSharedVocabulary(t *testing.T) {
	if !strings.HasPrefix(AdvisoryPrefix, "explanation documentation review (advisory, ADR-0102)") || !strings.HasSuffix(AdvisoryPrefix, ": ") {
		t.Fatalf("the prefix is the documented grep key: %q", AdvisoryPrefix)
	}
	if got := HostReason("typed handoff does not match verified host snapshot"); got != " (host: typed handoff does not match verified host snapshot)" {
		t.Errorf("HostReason renders the host's account: %q", got)
	}
	if got := HostReason(""); got != "" {
		t.Errorf("no host account renders nothing: %q", got)
	}
}
