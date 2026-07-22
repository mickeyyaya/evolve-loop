//go:build acs

// Package cycle1034 materializes the cycle-1034 acceptance criteria for this
// fleet lane's sole assigned item, failure-disposition-router (scout selected
// slices S1 failure-digest-assembler + S2 disposition-contract-gate; S3/S4
// deferred). Per R9.3 predicates bind ONLY to these two committed slices.
//
// Predicate strategy (cycle-85 non-degenerate rule): every AC here is a
// BEHAVIORAL contract on new go/internal/core code, so each predicate SHELLS the
// system-under-test — it runs the corresponding default-suite unit test as a
// subprocess and requires an explicit `--- PASS: <name>` line. A bare exit-0 is
// insufficient: `go test -run` on a pattern matching no test exits 0 with "no
// tests to run", so asserting on the PASS line (not the exit code) is what makes
// a still-unlanded / renamed / build-tag-hidden test RED instead of false-GREEN.
// -count=1 defeats the test cache so current source is always exercised. No
// source-grep predicate appears in this file.
//
// AC map (1:1 with the two eval files' [code] ACs):
//
//	S1 failure-digest-assembler:
//	  001 → AC1 pre-class buckets from real artifacts
//	  002 → AC2 fingerprint stable + phase-composed
//	  003 → AC3 recurrence read through the ledger
//	  004 → AC4 (negative) missing artifacts degrade to unknown, no abort
//	  005 → AC5 (edge) digest written atomically as valid JSON
//	S2 disposition-contract-gate:
//	  006 → AC1 retro fails loud without a valid disposition
//	  007 → AC2 fingerprint cross-checked against the digest
//	  008 → AC3 (negative) out-of-vocabulary enums rejected
//	  009 → AC4 (edge) salvage pointer floor
//	  010 → AC5 (wiring) gate invoked on the composed retro-completion path
//
// RED today: the internal/core surface (AssembleFailureDigest / VerifyDisposition
// / finalizeRetroCompletion) does not exist, so its test package fails to
// COMPILE — every subprocess below produces no `--- PASS:` line and each
// predicate fails.
package cycle1034

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -v -count=1 pkg`
// in the DEFAULT build suite (no -tags) and requires EVERY name to have printed a
// `--- PASS: <name>` line. Asserting on the PASS line rather than the exit code
// is essential: a pattern matching no test exits 0, so a missing/renamed test
// would otherwise false-GREEN.
func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		// -1 = the subprocess never launched (toolchain/module resolution failure),
		// a genuine harness error rather than a test verdict.
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("default-suite test %s did NOT pass in %s "+
				"(unlanded, failing, or the package failed to compile). exit=%d\ncombined output:\n%s",
				name, pkg, code, out)
		}
	}
}

// --- S1: failure-digest-assembler ---

func TestC1034_001_PreClassBucketsFromRealArtifacts(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_PreClassBucketsFromRealArtifacts")
}

func TestC1034_002_FingerprintStableAndPhaseComposed(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_FingerprintComposition")
}

func TestC1034_003_RecurrenceReadThroughLedger(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_RecurrenceFromLedger")
}

func TestC1034_004_MissingArtifactsDegradeToUnknown(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_MissingArtifactsDegradeToUnknown")
}

func TestC1034_005_DigestWrittenAsValidJSON(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_WritesDigestArtifact")
}

// --- S2: disposition-contract-gate ---

func TestC1034_006_RetroFailsLoudWithoutValidDisposition(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestDispositionGate_RetroFailsLoudWithoutValidDisposition")
}

func TestC1034_007_FingerprintCrossCheckedAgainstDigest(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestDispositionGate_CrossChecksFingerprintAgainstDigest")
}

func TestC1034_008_RejectsInvalidEnums(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestDispositionGate_RejectsInvalidEnums")
}

func TestC1034_009_SalvagePointerRequiredWhenValue(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestDispositionGate_SalvagePointerRequiredWhenValue")
}

// C1034_010 binds BOTH the orchestrator-level wiring test (the composed
// recordFailureLearning path invokes the gate) AND the isolated seam contract,
// so a Builder that implements the gate but forgets to wire it fails here.
func TestC1034_010_GateWiredIntoRetroCompletion(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestDispositionGate_WiredIntoRetroCompletion",
		"TestFinalizeRetroCompletion_SeamContract")
}
