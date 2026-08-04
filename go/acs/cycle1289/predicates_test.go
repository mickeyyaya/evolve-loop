//go:build acs

// Package cycle1289 materializes the cycle-1289 acceptance criteria for the two
// committed top_n tasks (triage-report.md), both from inbox item
// contract-block-cli-escalation (2026-08-04, weight 0.96, P1):
//
//	T1  task-1-fingerprint-gate — the landed contract-block CLI escalation
//	    (PR #390) triggers on a RAW BLOCK COUNT: cyclerun_review.go escalates on
//	    `rr.Blocks >= contractEscalateAtBlock` and never asks whether block 2 is
//	    the SAME violation as block 1. Two genuinely different contract defects on
//	    one phase therefore read as one incapable-CLI signature and spend round 2's
//	    budget on a different family for nothing. Fix: gate the trigger on failure
//	    IDENTITY, reusing failure_digest.go's normalizeReasonForFingerprint (the
//	    blocker breaker's own primitive) — never a second hashing scheme.
//	T2  task-2-doc-addendum — record the gating rule in the research doc the
//	    strategic evaluation that ranked this item created
//	    (kb/research/deliverable-alignment-2026-08/README.md, append-only).
//
// The T1 predicates are BEHAVIORAL (cycle-85 lesson). The trigger site, the
// escalation helpers and the normalization primitive are ALL unexported in
// package core, so an in-package white-box test driven by subprocess is the only
// way to exercise them. Those tests drive the real Orchestrator through RunCycle
// with real .evolve/profiles/*.json on disk — a magic string in a source file can
// neither suppress a re-dispatch's ModelRoutingCLI override nor produce a named
// `--- PASS:` line for a trigger that still counts blocks blindly.
//
// Predicate-shape note (flaky-predicate-shape, Gate D): every `go test`
// invocation below names ONE package and carries a selective -run, so no
// recursive sweep and no whole-suite run of the known-slow internal/core suite
// happens here.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.negative  differing block reasons do NOT escalate     → C1289_001 (named PASS line)
//	T1.positive  normalization-equal reasons DO escalate     → C1289_002 (named PASS line)
//	T1.regress   the escalation family/policy path is intact → C1289_003 (no FAIL line, incl. hot-breaker edge)
//	T2.doc       research doc records the fingerprint gate   → C1289_004 (doc content)
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

// corePkg is the ONE package every predicate below runs, and escalationRunPattern
// is the ONE -run selector. Held as consts on the same file as the invocation so
// the shape lint resolves both without a helper hop.
const (
	corePkg = "./internal/core/"
	// escalationRunPattern selects the contract-escalation ladder tests plus the
	// chain-reviewer demotion-propagation tests that carry ReviewResult.Blocks /
	// .Reason to the trigger — the exact surface this cycle's change touches.
	escalationRunPattern = "TestContractCorrection_|TestChainReviewers_|TestFormatContractGateDemotionWarn|TestUniversalContractFallbackMatchesLLMRouteDefault"

	negativeTest = "TestContractCorrection_DifferingBlockReasonsDoNotEscalate"
	positiveTest = "TestContractCorrection_NormalizedIdenticalReasonsEscalate"
	hotBreaker   = "TestContractCorrection_HotBreakerEscalatesOnFirstCorrection"

	// researchDoc is the append-only target of T2. It already exists (created by
	// PR #409); this cycle adds the fingerprint-gate addendum to it.
	researchDoc = "kb/research/deliverable-alignment-2026-08/README.md"
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or from go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

// topLevelPassed reports whether a `--- PASS: <name>` line names exactly `name`.
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

// runEscalationLadder runs the contract-escalation white-box tests (which drive
// the real Orchestrator correction ladder over real profiles on disk), verbose,
// ONCE per predicate process. Narrowed by -run so an unrelated core change
// cannot false-RED these gates.
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

// ============ T1 — fingerprint-gated contract-block CLI escalation ============

// --- C1289_001 (T1.negative): differing violations must NOT escalate ---------
//
// THE anti-no-op axis of this cycle. Block 1 misses a section heading, block 2
// misses the verdict sentinel — two honest defects, not one CLI that cannot
// format. Correction 2 must therefore re-dispatch on the phase's OWN routing
// (ModelRoutingCLI ""), exactly as the ladder behaved before PR #390.
//
// RED baseline: the trigger reads only rr.Blocks, so the second block escalates
// to codex-tmux regardless of what it says, and the white-box test FAILs with
// `correction 2 dispatched on ModelRoutingCLI="codex-tmux", want ""`. No source
// string can make the orchestrator stop overriding ModelRoutingCLI.
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

// --- C1289_002 (T1.positive): normalization-equal violations DO escalate -----
//
// The discriminating half. The two block reasons name the SAME defect and differ
// only in a go-test duration token — precisely the identity noise
// failure_digest.go's normalizeReasonForFingerprint folds to "<dur>". This
// predicate is what separates the required fix (reuse the breaker's identity
// primitive) from the cheap one (raw `block1.Reason == block2.Reason`), which
// would suppress this escalation and leave the mis-formatting CLI to demote the
// gate — the very outcome PR #390 exists to prevent.
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

// --- C1289_003 (T1.regress): the rest of the escalation contract is intact ---
//
// Anti-no-op regression gate for the surface the change touches. Two properties
// are load-bearing and easy to break while adding an identity check:
//
//   - the HOT-BREAKER edge: a breaker left hot by an earlier cycle arrives at
//     Blocks>=2 on this ladder's FIRST block, so there is NO prior reason to
//     compare. The gate must be "prior reason known AND differing ⇒ suppress",
//     not "equal ⇒ escalate" — the latter silently deletes the escape hatch.
//   - family selection + the policy.ValidatePin guardrail, which PR #390's
//     review established and this cycle must not touch.
//
// Asserting no `--- FAIL:` line over the whole narrowed set covers both.
func TestC1289_003_EscalationLadderSuiteGreen(t *testing.T) {
	out := runEscalationLadder(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a contract-escalation ladder test FAILs — the identity gate "+
			"must not change family selection, the policy guardrail, or the hot-breaker path:\n%s",
			tail(out, 40))
	}
	// The hot-breaker edge is the one this change is most likely to regress, so
	// it is named explicitly rather than left to the no-FAIL sweep (a test that
	// silently stops RUNNING produces no FAIL line either).
	if !topLevelPassed(out, hotBreaker) {
		t.Errorf("RED/REGRESSION: %s did not PASS — with the breaker already hot there is no "+
			"prior block reason on this ladder, and escalation must still get its shot before "+
			"the third strike opens the circuit.\n%s", hotBreaker, tail(out, 30))
	}
}

// ===================== T2 — research-doc addendum ============================

// --- C1289_004 (T2.doc): the research doc records the gating rule ------------
//
// The deliverable IS documentation, so its content is the criterion, not a proxy
// for one (the cycle-85 degenerate-predicate ban targets source-file magic
// strings standing in for behavior; here the prose is the behavior under test).
// Four independent content requirements, so a one-word "fingerprint" sprinkle
// cannot satisfy it: the identity primitive by name, the mechanism file it gates,
// and BOTH directions of the rule.
func TestC1289_004_ResearchDocRecordsFingerprintGate(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), researchDoc)
	if !acsassert.FileExists(t, doc) {
		t.Fatalf("%s is missing — this cycle APPENDS to the doc PR #409 created, it must not be deleted or moved", researchDoc)
	}
	for _, want := range []string{
		"normalizeReasonForFingerprint", // the primitive reused, named
		"contract_escalation.go",        // cross-reference to the mechanism it gates
	} {
		if !acsassert.FileContains(t, doc, want) {
			t.Errorf("%s does not mention %q — the addendum must name the primitive it reuses and cross-reference the landed escalation mechanism", researchDoc, want)
		}
	}
	// Both directions of the rule must be stated, not just the headline.
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

// readDoc returns the doc body, failing the predicate rather than silently
// reading "" when the file cannot be read.
func readDoc(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
