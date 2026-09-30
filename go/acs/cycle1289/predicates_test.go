//go:build acs

package cycle1289

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg              = "./internal/core/"
	escalationRunPattern = "TestContractCorrection_|TestChainReviewers_|TestFormatContractGateDemotionWarn|TestUniversalContractFallbackMatchesLLMRouteDefault"

	negativeTest = "TestContractCorrection_DifferingBlockReasonsDoNotEscalate"
	positiveTest = "TestContractCorrection_NormalizedIdenticalReasonsEscalate"
	hotBreaker   = "TestContractCorrection_HotBreakerEscalatesOnFirstCorrection"

	researchDoc = "docs/research/deliverable-alignment-2026-08/README.md"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

func topLevelPassed(out, name string) bool {
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

var (
	escalationOnce sync.Once
	escalationOut  string
)

func runEscalationLadder(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	escalationOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", escalationRunPattern, corePkg)
		escalationOut = stdout + "\n" + stderr
	})
	return escalationOut
}

func TestC1289_001_DifferingBlockReasonsDoNotEscalate(t *testing.T) {
	out := runEscalationLadder(t)
	if !topLevelPassed(out, negativeTest) {
		t.Errorf("RED: %s did not PASS — the escalation trigger still fires on a raw "+
			"block count, so two DIFFERENT contract violations on one phase escalate as if "+
			"they were one incapable-CLI signature. Gate the trigger on failure identity "+
			"(normalizeReasonForFingerprint over the prior block's reason).\n%s",
			negativeTest, tail(out, 30))
	}
}

func TestC1289_002_NormalizedIdenticalReasonsStillEscalate(t *testing.T) {
	out := runEscalationLadder(t)
	if !topLevelPassed(out, positiveTest) {
		t.Errorf("RED/REGRESSION: %s did not PASS — two blocks that are the same defect "+
			"under normalizeReasonForFingerprint (differing only in a duration token) must "+
			"still escalate. A raw string-equality gate fails exactly here; reuse the "+
			"failure_digest.go primitive instead of inventing a second identity scheme.\n%s",
			positiveTest, tail(out, 30))
	}
}

func TestC1289_003_EscalationLadderSuiteGreen(t *testing.T) {
	out := runEscalationLadder(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a contract-escalation ladder test FAILs — the identity gate "+
			"must not change family selection, the policy guardrail, or the hot-breaker path:\n%s",
			tail(out, 40))
	}
	if !topLevelPassed(out, hotBreaker) {
		t.Errorf("RED/REGRESSION: %s did not PASS — with the breaker already hot there is no "+
			"prior block reason on this ladder, and escalation must still get its shot before "+
			"the third strike opens the circuit.\n%s", hotBreaker, tail(out, 30))
	}
}

func TestC1289_004_ResearchDocRecordsFingerprintGate(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), researchDoc)
	if !acsassert.FileExists(t, doc) {
		t.Fatalf("%s is missing — this cycle APPENDS to the doc PR #409 created, it must not be deleted or moved", researchDoc)
	}
	for _, want := range []string{
		"normalizeReasonForFingerprint",
		"contract_escalation.go",
	} {
		if !acsassert.FileContains(t, doc, want) {
			t.Errorf("%s does not mention %q — the addendum must name the primitive it reuses and cross-reference the landed escalation mechanism", researchDoc, want)
		}
	}
	body := strings.ToLower(readDoc(t, doc))
	if !strings.Contains(body, "escalat") || !strings.Contains(body, "fingerprint") {
		t.Errorf("%s does not document the fingerprint-gated ESCALATION rule at all", researchDoc)
	}
	if !strings.Contains(body, "differ") {
		t.Errorf("%s states no rule for DIFFERING consecutive block reasons — the suppression half (two distinct defects are not one incapable CLI) is the behavior this cycle added and is what a future reader needs", researchDoc)
	}
	if !strings.Contains(body, "hot") && !strings.Contains(body, "no prior") {
		t.Errorf("%s does not record the no-prior-reason (hot breaker) edge — the gate is 'prior known AND differing ⇒ suppress', and omitting that is how the escape hatch gets deleted by the next reader", researchDoc)
	}
}

func readDoc(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
