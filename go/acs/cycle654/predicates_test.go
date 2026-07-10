//go:build acs

// Package cycle654 materialises the acceptance criteria for the single
// triage-committed top_n task of cycle 654, infra-classifier-echo-veto (weight
// 0.95, operator-injected). It is the fix-of-record for lesson
// cycle-641-infra-incident-classifier-matches-echoed-prompt-keywords, which
// recurred byte-for-byte in cycle-642: a phase whose driver exited 0 and whose
// deliverable carried a PASS sentinel was discarded FAIL because the infra /
// escalation classifiers keyword-matched the agent's OWN echoed prompt text
// ("...missing rate limits.", the reviewer checklist) as a runtime rate_limit
// signal. The fix has three layers, one predicate each, plus a negative guard:
//
//   - AC1 (C654_001) cycleclassify source-of-truth veto gate: a PASS deliverable
//   - driver-exit-0 + prompt-echo infra_failure must NOT classify infrastructure.
//   - AC3 (C654_002) negative axis: a genuine runtime infra signal (non-zero exit,
//     non-echo excerpt) MUST still classify infrastructure — the fix is not a
//     blanket disable. GREEN today; the AC1 fix must keep it green.
//   - AC1' (C654_003) normalizer emit gate: the Classifier must not emit an
//     infra_failure INCIDENT for a line that is a verbatim echo of the injected
//     prompt; a genuine line absent from the prompt must still emit.
//   - AC2 (C654_004) escalation auto-responder: echoed prompt exhaustion text is
//     stripped before the exhaustion match; a genuine CLI quota banner survives.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…653 precedent).
// Each predicate shells `go test -run` over one RED regression test authored this
// cycle in the real system-under-test package, so every predicate EXERCISES the
// SUT (Classify over an on-disk workspace; Classifier.Stderr emit path;
// stripPromptEchoLines + matchExhausted) and asserts on behaviour — none is a
// source-grep. RED now: cycleclassify vetoes the echo (001); phasestream and
// bridge fail to compile (SetInjectedPrompt / stripPromptEchoLines absent — 003,
// 004). 002 is the currently-green guard. GREEN once Builder lands the three-layer
// fix. The Acceptance-Criteria-Summary line "go test -race on touched packages
// PASS; apicover clean" is dispositioned manual+checklist in test-report.md (a
// repo-wide toolchain gate the cycle audit already runs), not predicated here.
package cycle654

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cycleclassifyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	phasestreamPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	bridgePkg        = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. code<0 is a genuine
// launch failure (binary missing / killed by signal), never a test verdict —
// that fails loudly rather than being misread as a RED behavioral result.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC654_001_SourceOfTruthVetoGate — AC1: a PASS deliverable + driver-exit-0 +
// a prompt-echo infra_failure event must NOT be classified infrastructure.
func TestC654_001_SourceOfTruthVetoGate(t *testing.T) {
	ok, out := runGoTest(t, cycleclassifyPkg, "TestC654_001_PassDeliverableExit0EchoNotInfraVeto")
	if !ok {
		t.Errorf("cycleclassify vetoes a PASS/exit-0 phase on a prompt-echo infra_failure (source-of-truth gate missing):\n%s", out)
	}
}

// TestC654_002_GenuineInfraStillVetoes — AC3 (negative axis): a genuine runtime
// infra signal must still classify infrastructure after the AC1 fix.
func TestC654_002_GenuineInfraStillVetoes(t *testing.T) {
	ok, out := runGoTest(t, cycleclassifyPkg, "TestC654_002_GenuineInfraStillVetoes")
	if !ok {
		t.Errorf("genuine runtime infra no longer classifies infrastructure — the echo-veto fix over-corrected:\n%s", out)
	}
}

// TestC654_003_NormalizerPromptEchoGate — AC1': the normalizer must not emit an
// infra_failure INCIDENT for a verbatim echo of the injected prompt.
func TestC654_003_NormalizerPromptEchoGate(t *testing.T) {
	ok, out := runGoTest(t, phasestreamPkg, "TestC654_003_PromptEchoNotEmittedGenuineEmitted")
	if !ok {
		t.Errorf("normalizer still emits infra_failure for echoed prompt text (SetInjectedPrompt gate missing):\n%s", out)
	}
}

// TestC654_004_EscalationPromptEchoGate — AC2: the exhaustion/escalation matcher
// must not fire on echoed prompt text; a genuine CLI banner must survive.
func TestC654_004_EscalationPromptEchoGate(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestC654_004_EchoedExhaustionStrippedGenuineSurvives")
	if !ok {
		t.Errorf("escalation matcher still fires on echoed prompt exhaustion text (stripPromptEchoLines missing):\n%s", out)
	}
}
