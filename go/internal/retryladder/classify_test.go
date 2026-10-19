package retryladder_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func TestClassify_ExitCodes(t *testing.T) {
	tests := []struct {
		name  string
		facts retryladder.Facts
		want  retryladder.Class
	}{
		{"safety gate", retryladder.Facts{ExitCode: 2}, retryladder.Halt},
		{"cost leak", retryladder.Facts{ExitCode: 3}, retryladder.Halt},
		{"bad flags", retryladder.Facts{ExitCode: 10}, retryladder.Halt},
		{"respond loop guard", retryladder.Facts{ExitCode: 86}, retryladder.Halt},
		{"require full unmet", retryladder.Facts{ExitCode: 99}, retryladder.Halt},
		{"repl boot timeout", retryladder.Facts{ExitCode: 80}, retryladder.Infra},
		{"artifact timeout", retryladder.Facts{ExitCode: 81}, retryladder.Infra},
		{"model mismatch", retryladder.Facts{ExitCode: 87}, retryladder.Infra},
		{"command timeout", retryladder.Facts{ExitCode: 124}, retryladder.Infra},
		{"missing binary", retryladder.Facts{ExitCode: 127}, retryladder.Infra},
		{"85 without a bench", retryladder.Facts{ExitCode: 85}, retryladder.Infra},
		{"85 with a bench", retryladder.Facts{ExitCode: 85, QuotaBench: true}, retryladder.Quota},
		{"bench without 85 is not a quota failure", retryladder.Facts{ExitCode: 1, QuotaBench: true}, retryladder.Unknown},
		{"teardown error", retryladder.Facts{TeardownError: true}, retryladder.Infra},
		{"halt outranks a spec signal", retryladder.Facts{ExitCode: 2, AuditClass: "intent-malformed"}, retryladder.Halt},
		{"halt outranks a teardown error", retryladder.Facts{ExitCode: 99, TeardownError: true}, retryladder.Halt},
		{"infra outranks a capability signal", retryladder.Facts{ExitCode: 124, DeliverableMissing: true}, retryladder.Infra},
		{"quota outranks a capability signal", retryladder.Facts{ExitCode: 85, QuotaBench: true, AuditClass: "code-build-fail"}, retryladder.Quota},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryladder.Classify(tt.facts).Class; got != tt.want {
				t.Fatalf("Classify(%+v).Class = %q, want %q", tt.facts, got, tt.want)
			}
		})
	}
}

func TestClassify_FixtureCycles(t *testing.T) {
	tests := []struct {
		name  string
		facts retryladder.Facts
		want  retryladder.Class
	}{
		{
			name: "cycle 1946 coverage of unreachable branches in a protected file",
			facts: retryladder.Facts{
				AuditClass: "code-build-fail",
				LaneScope:  []string{"go/internal/core"},
				Floors:     []retryladder.FloorFailure{{Path: "go/internal/core/guarded.go", Protected: true}},
			},
			want: retryladder.Spec,
		},
		{
			name: "cycle 1950 coverage of unreachable branches in a protected file",
			facts: retryladder.Facts{
				AuditClass: "code-build-fail",
				LaneScope:  []string{"go/internal/cliroute"},
				Floors:     []retryladder.FloorFailure{{Path: "go/internal/cliroute/table.go", Protected: true}},
			},
			want: retryladder.Spec,
		},
		{
			name:  "cycle 1953 acceptance needs a protected writer",
			facts: retryladder.Facts{AuditClass: "intent-malformed"},
			want:  retryladder.Spec,
		},
		{
			name: "cycle 1965 copied predicate under testdata that the floor runs",
			facts: retryladder.Facts{
				AuditClass: "code-build-fail",
				LaneScope:  []string{"go/internal/handoff", "go/acs/cycle1965"},
				Floors:     []retryladder.FloorFailure{{Path: "go/internal/handoff/testdata/predicates_test.go"}},
			},
			want: retryladder.Capability,
		},
		{
			name: "cycle 1966 red predicate never turned green",
			facts: retryladder.Facts{
				AuditClass: "code-audit-fail",
				LaneScope:  []string{"go/internal/handoff", "go/acs/cycle1966"},
				Floors:     []retryladder.FloorFailure{{Path: "go/acs/cycle1966/predicates_test.go"}},
			},
			want: retryladder.Capability,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryladder.Classify(tt.facts).Class; got != tt.want {
				t.Fatalf("Classify(%+v).Class = %q, want %q", tt.facts, got, tt.want)
			}
		})
	}
}

func TestClassify_SpecAndCapabilitySignals(t *testing.T) {
	inScope := retryladder.FloorFailure{Path: "go/internal/foo/a.go"}
	outOfScope := retryladder.FloorFailure{Path: "go/internal/bar/b.go"}
	tests := []struct {
		name  string
		facts retryladder.Facts
		want  retryladder.Class
	}{
		{"intent-rejected", retryladder.Facts{AuditClass: "intent-rejected"}, retryladder.Spec},
		{"proven blocker", retryladder.Facts{ProvenBlocker: true, AuditClass: "code-build-fail"}, retryladder.Spec},
		{"proven blocker outranks an in-scope floor", retryladder.Facts{ProvenBlocker: true, LaneScope: []string{"go/internal/foo"}, Floors: []retryladder.FloorFailure{inScope}}, retryladder.Spec},
		{"floor outside the lane scope", retryladder.Facts{LaneScope: []string{"go/internal/foo"}, Floors: []retryladder.FloorFailure{outOfScope}}, retryladder.Spec},
		{"floor with an empty lane scope is outside it", retryladder.Facts{Floors: []retryladder.FloorFailure{inScope}}, retryladder.Spec},
		{"one outside floor among inside floors", retryladder.Facts{LaneScope: []string{"go/internal/foo"}, Floors: []retryladder.FloorFailure{inScope, outOfScope}}, retryladder.Spec},
		{"protected floor inside the lane scope", retryladder.Facts{LaneScope: []string{"go/internal/foo"}, Floors: []retryladder.FloorFailure{{Path: "go/internal/foo/a.go", Protected: true}}}, retryladder.Spec},
		{"sibling directory sharing a name prefix is outside", retryladder.Facts{LaneScope: []string{"go/internal/foo"}, Floors: []retryladder.FloorFailure{{Path: "go/internal/foobar/a.go"}}}, retryladder.Spec},
		{"floor inside the lane scope", retryladder.Facts{LaneScope: []string{"go/internal/foo"}, Floors: []retryladder.FloorFailure{inScope}}, retryladder.Capability},
		{"scope entry with a trailing slash", retryladder.Facts{LaneScope: []string{"go/internal/foo/"}, Floors: []retryladder.FloorFailure{inScope}}, retryladder.Capability},
		{"scope entry naming the file itself", retryladder.Facts{LaneScope: []string{"go/internal/foo/a.go"}, Floors: []retryladder.FloorFailure{inScope}}, retryladder.Capability},
		{"unclean floor path inside the scope", retryladder.Facts{LaneScope: []string{"go/internal/foo"}, Floors: []retryladder.FloorFailure{{Path: "go/internal/foo/../foo/a.go"}}}, retryladder.Capability},
		{"unclean floor path escaping the scope", retryladder.Facts{LaneScope: []string{"go/internal/foo"}, Floors: []retryladder.FloorFailure{{Path: "go/internal/foo/../bar/a.go"}}}, retryladder.Spec},
		{"missing deliverable", retryladder.Facts{DeliverableMissing: true}, retryladder.Capability},
		{"code-build-fail audit class", retryladder.Facts{AuditClass: "code-build-fail"}, retryladder.Capability},
		{"code-audit-fail audit class", retryladder.Facts{AuditClass: "code-audit-fail"}, retryladder.Capability},
		{"no signal", retryladder.Facts{}, retryladder.Unknown},
		{"generic exit with no signal", retryladder.Facts{ExitCode: 1}, retryladder.Unknown},
		{"unrelated audit class", retryladder.Facts{AuditClass: "infrastructure-transient"}, retryladder.Unknown},
		{"reason text alone is no signal", retryladder.Facts{Reason: "something broke"}, retryladder.Unknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryladder.Classify(tt.facts).Class; got != tt.want {
				t.Fatalf("Classify(%+v).Class = %q, want %q", tt.facts, got, tt.want)
			}
		})
	}
}

func TestFingerprint(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		other  string
		same   bool
	}{
		{"same code set in another order", "[b_code] second [a_code] first", "[a_code] first [b_code] second", true},
		{"same code set with different prose", "[a_code] one thing", "[a_code] another thing entirely", true},
		{"repeated code counts once", "[a_code] x [a_code] y", "[a_code] z", true},
		{"different code sets", "[a_code] x", "[b_code] x", false},
		{"subset of codes differs", "[a_code] x [b_code] y", "[a_code] x", false},
		{"text identity ignores the cycle number", "build failed in cycle 1946", "build failed in cycle-1950", true},
		{"text identity ignores go test durations", "FAIL pkg 0.42s", "FAIL pkg 13.07s", true},
		{"text identity ignores attempt denominators", "attempt 1/3 failed", "attempt 2 of 5 failed", true},
		{"text identity ignores the narrative verdict", "narrative=PASS but gates red", "narrative=FAIL but gates red", true},
		{"text identity ignores appended detail", cyclestate.WithDetail("gates red", "one"), cyclestate.WithDetail("gates red", "two"), true},
		{"different text differs", "gates red", "coverage low", false},
		{"codes and text never collide", "[a_code]", "a_code", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := retryladder.Fingerprint(tt.reason), retryladder.Fingerprint(tt.other)
			if a == "" || b == "" {
				t.Fatalf("Fingerprint of a non-empty reason is empty: %q, %q", a, b)
			}
			if (a == b) != tt.same {
				t.Fatalf("Fingerprint(%q) = %q, Fingerprint(%q) = %q, equal = %v, want %v", tt.reason, a, tt.other, b, a == b, tt.same)
			}
		})
	}
}

func TestFingerprint_EmptyReason(t *testing.T) {
	for _, reason := range []string{"", "   \n\t"} {
		if got := retryladder.Fingerprint(reason); got != "" {
			t.Fatalf("Fingerprint(%q) = %q, want empty", reason, got)
		}
	}
}

func TestClassify_VerdictCarriesTheFingerprintOfTheReason(t *testing.T) {
	reason := "[missing_section] heading absent"
	got := retryladder.Classify(retryladder.Facts{AuditClass: "code-build-fail", Reason: reason})
	if got.Fingerprint != retryladder.Fingerprint(reason) {
		t.Fatalf("Verdict.Fingerprint = %q, want %q", got.Fingerprint, retryladder.Fingerprint(reason))
	}
	if empty := retryladder.Classify(retryladder.Facts{ExitCode: 2}); empty.Fingerprint != "" {
		t.Fatalf("Verdict.Fingerprint without a reason = %q, want empty", empty.Fingerprint)
	}
}
