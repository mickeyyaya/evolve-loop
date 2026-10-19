//go:build acs

package cycle1973

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1973_001_FixtureCyclesClassifyByTheirCause(t *testing.T) {
	protectedFloor := []retryladder.FloorFailure{{Path: "go/internal/core/guarded.go", Protected: true}}
	insideScopeFloor := []retryladder.FloorFailure{{Path: "go/internal/handoff/testdata/predicates_test.go"}}
	scope := []string{"go/internal/core", "go/internal/handoff", "go/acs"}
	tests := []struct {
		name  string
		facts retryladder.Facts
		want  retryladder.Class
	}{
		{"cycle 1946 protected coverage floor", retryladder.Facts{AuditClass: "code-build-fail", LaneScope: scope, Floors: protectedFloor}, retryladder.Spec},
		{"cycle 1950 protected coverage floor", retryladder.Facts{AuditClass: "code-build-fail", LaneScope: scope, Floors: []retryladder.FloorFailure{{Path: "go/internal/cliroute/table.go", Protected: true}}}, retryladder.Spec},
		{"cycle 1953 intent-malformed", retryladder.Facts{AuditClass: "intent-malformed"}, retryladder.Spec},
		{"cycle 1965 copied predicate under testdata", retryladder.Facts{AuditClass: "code-build-fail", LaneScope: scope, Floors: insideScopeFloor}, retryladder.Capability},
		{"cycle 1966 red predicate stays red", retryladder.Facts{AuditClass: "code-audit-fail", LaneScope: scope, Floors: []retryladder.FloorFailure{{Path: "go/acs/cycle1966/predicates_test.go"}}}, retryladder.Capability},
		{"the 1946 floor off the protected list and inside the scope is capability", retryladder.Facts{AuditClass: "code-build-fail", LaneScope: scope, Floors: []retryladder.FloorFailure{{Path: "go/internal/core/guarded.go"}}}, retryladder.Capability},
	}
	for _, tt := range tests {
		if got := retryladder.Classify(tt.facts).Class; got != tt.want {
			t.Errorf("%s: Classify = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestC1973_002_HaltExitsAndTheUnbenchedUnknownPromptExit(t *testing.T) {
	for _, code := range []int{2, 3, 10, 86, 99} {
		if got := retryladder.Classify(retryladder.Facts{ExitCode: code, AuditClass: "code-build-fail", DeliverableMissing: true}).Class; got != retryladder.Halt {
			t.Errorf("exit %d: Classify = %q, want %q", code, got, retryladder.Halt)
		}
	}
	if got := retryladder.Classify(retryladder.Facts{ExitCode: 85}).Class; got != retryladder.Infra {
		t.Errorf("exit 85 without a bench: Classify = %q, want %q", got, retryladder.Infra)
	}
	for _, code := range []int{1, 4, 11, 84, 100} {
		if got := retryladder.Classify(retryladder.Facts{ExitCode: code}).Class; got == retryladder.Halt {
			t.Errorf("exit %d: Classify = %q, only the five halt codes may halt", code, got)
		}
	}
}

func TestC1973_003_PackageDoesNotImportCore(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, stderr, code, err := acsassert.SubprocessOutput("go", "list", "-C", filepath.Join(root, "go"), "-deps", "-test", "./internal/retryladder")
	if err != nil || code != 0 {
		t.Fatalf("go list exit=%d err=%v: %s", code, err, stderr)
	}
	if !strings.Contains(out, "/go/internal/retryladder") {
		t.Fatalf("go list did not report the package itself:\n%s", out)
	}
	for _, dep := range strings.Fields(out) {
		if strings.HasSuffix(dep, "/go/internal/core") || strings.Contains(dep, "/go/internal/core/") {
			t.Errorf("retryladder depends on %s", dep)
		}
	}
}

func TestC1973_004_BenchedQuotaWallAndTheUnknownResidue(t *testing.T) {
	if got := retryladder.Classify(retryladder.Facts{ExitCode: 85, QuotaBench: true}).Class; got != retryladder.Quota {
		t.Errorf("exit 85 with a bench: Classify = %q, want %q", got, retryladder.Quota)
	}
	if got := retryladder.Classify(retryladder.Facts{ExitCode: 1, QuotaBench: true}).Class; got == retryladder.Quota {
		t.Errorf("a bench without exit 85 classified as quota")
	}
	for _, facts := range []retryladder.Facts{
		{},
		{ExitCode: 1},
		{ExitCode: 1, Reason: "the model said something odd"},
		{AuditClass: "infrastructure-transient"},
	} {
		if got := retryladder.Classify(facts).Class; got != retryladder.Unknown {
			t.Errorf("Classify(%+v) = %q, want %q", facts, got, retryladder.Unknown)
		}
	}
}

func TestC1973_005_FingerprintReusesTheCodeSetIdentity(t *testing.T) {
	a := retryladder.Fingerprint("[missing_section] heading [bad_verdict] verdict")
	b := retryladder.Fingerprint("[bad_verdict] other prose [missing_section] more prose")
	if a == "" || a != b {
		t.Errorf("same code set, different order and prose: %q vs %q", a, b)
	}
	if c := retryladder.Fingerprint("[missing_section] heading"); c == a {
		t.Errorf("a code subset shares the fingerprint of the full set: %q", c)
	}
	x := retryladder.Fingerprint("coverage floor failed in cycle 1946 after 0.31s")
	y := retryladder.Fingerprint("coverage floor failed in cycle-1950 after 12.80s")
	if x == "" || x != y {
		t.Errorf("text identity must ignore cycle numbers and durations: %q vs %q", x, y)
	}
}

// acs-predicate: config-check
func TestC1973_006_PackageIsEnrolledInTheApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileContains(t, filepath.Join(root, "go", ".apicover-enforce"), "\n./internal/retryladder\n") {
		t.Errorf("go/.apicover-enforce lacks the line ./internal/retryladder")
	}
}
