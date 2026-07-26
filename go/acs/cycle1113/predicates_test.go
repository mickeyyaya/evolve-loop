//go:build acs

// Package cycle1113 materializes the cycle-1113 acceptance criteria for the
// sole committed item of this fleet lane, tdd-topn-binding-gate
// (triage-report.md ## top_n; fleet_scope pins this lane to that one todo-id,
// so per R9.3 no predicate here binds to a deferred or other-lane item).
//
// Task nature: COVERAGE GAP. internal/topngate is fully implemented and wired
// (cmd_cycle.go -> NewReviewer(cfg.TopNGate), default StageEnforce), and its
// unit suite is green. What does not exist is proof that tddScopeGate's ONE
// fatal path (gate.go:117-120, empty committed ## top_n + a non-empty authored
// set) survives the COMPOSITION: NewReviewer(stage).Review(Phase: PhaseTDD) —
// the exact code path `evolve loop` executes. Every reviewer_test.go case
// today drives PhaseBuild or PhaseAudit, so an appliesTo typo, a dispatch
// reordering, or a stage-comparison inversion could silently disarm the TDD
// gate with the whole suite still green.
//
// AC map (1:1, from scout-report.md ## Acceptance Criteria Summary + both
// Selected Tasks' verifiableBy):
//
//	AC1 "StageEnforce + PhaseTDD + empty top_n + authored files => Approve
//	     false, Reason non-empty" (Task 1 verifiableBy)
//	    -> C1113_001 exercises the composed reviewer directly (production
//	       behaviour today: PRE-EXISTING GREEN, bound so a regression in
//	       reviewer.go's dispatch is caught by audit regardless of what the
//	       package's own tests do) and C1113_003 (the named reviewer-level
//	       test must exist and PASS: RED until Builder writes it).
//	AC2 "StageShadow on the identical fixture => Approve true" (Task 2)
//	    -> C1113_002 (direct, PRE-EXISTING GREEN) + C1113_003 (named test).
//	AC3 "the new tests are load-bearing, not tautological — they fail on a
//	     deliberately reverted reviewer.go"
//	    -> C1113_004 (mutation: tddScopeGate dropped from the gates slice =>
//	       the enforce test MUST fail) and C1113_005 (mutation: the
//	       stage-comparison guard removed so shadow blocks too => the shadow
//	       test MUST fail). Both RED today (no such tests to kill).
//	AC4 "go test ./internal/topngate/... remains green (no regression)"
//	    -> C1113_006 counts an explicit verbose PASS for all 15 pre-existing
//	       test funcs; a bare exit 0 would hide a deleted or renamed one.
//	AC5 "no production behaviour change (gate.go/reviewer.go untouched)"
//	    -> manual+checklist in test-report.md (a diff-shape judgement, not a
//	       behavioural assertion; C1113_001/002/006 pin the behaviour that a
//	       production edit would have to preserve anyway).
//
// Adversarial axes: NEGATIVE — C1113_002 asserts the gate does NOT block at
// shadow, and C1113_004/005 assert the new tests DO fail under mutation (the
// anti-no-op signal: a test asserting `true` passes C1113_003 and dies here).
// EDGE — C1113_003 rejects the "no tests to run" exit-0 hole; C1113_006
// rejects exit-0-with-a-test-deleted. SEMANTIC — enforce blocking, shadow
// approving, mutation sensitivity and suite health are four distinct
// behaviours, not one restated.
//
// No source-grep predicates (cycle-85 rule): every predicate below either
// calls the system under test in-process (001/002) or runs it as a subprocess
// and asserts on real emitted output (003/004/005/006).
package cycle1113

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/topngate"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	topngatePkg = "github.com/mickeyyaya/evolve-loop/go/internal/topngate"

	// reviewerSrc is the production file the mutation predicates rewrite via
	// `go test -overlay` (never on disk — the real tree stays untouched).
	reviewerSrc = "go/internal/topngate/reviewer.go"

	// The reviewer-level test names this cycle contracts. They are part of the
	// deliverable, not an implementation detail: C1113_004/005 must be able to
	// name the individual test a mutation is required to kill, which a bare
	// "some test failed" check cannot do.
	enforceTest = "TestNewReviewer_TDDEnforceBlocksEmptyTopN"
	shadowTest  = "TestNewReviewer_TDDShadowApprovesEmptyTopN"
	newTestsRun = "^TestNewReviewer_TDD"

	// orphanSlug is the slug the fixture's test-report.md claims while triage
	// committed nothing — the shape gate.go:117-120 calls FATAL.
	orphanSlug     = "orphan-task-cycle-1113"
	orphanTestFile = "go/acs/cycle1113/predicates_test.go"
)

// preExistingTests is the full test surface of internal/topngate before this
// cycle. C1113_006 requires a verbose PASS for every one (AC4).
var preExistingTests = []string{
	"TestNewReviewer_Named",
	"TestTopNBindingGate",
	"TestTopNBindingGate_AppliesToBuildOnly",
	"TestTDDScopeGate_LabelDriftIsAdvisory",
	"TestTDDScopeGate_EmptyTopNStillBlocks",
	"TestTDDScopeGate",
	"TestTDDScopeGate_AppliesToTDDOnly",
	"TestTDDScopeGate_FileScopeDriftIsAdvisory",
	"TestTDDScopeGate_FileScopeBinding",
	"TestBuilderPromptNamesTopNAsSoleTaskAuthority",
	"TestNewReviewer_EnforceApprovesLabelDrift",
	"TestNewReviewer_EnforceApprovesInLaneBuild",
	"TestNewReviewer_ShadowLogsButApproves",
	"TestNewReviewer_NonBuildPhaseApproves",
	"TestReplayCycle640Shape",
}

// writeOrphanFixture materialises the ONE fatal shape in a temp workspace:
// triage committed an EMPTY ## top_n while test-report.md claims a slug and
// declares authored test files. Report shapes match agents/evolve-triage.md
// Step 4 and agents/evolve-tdd.md Step 6 respectively.
func writeOrphanFixture(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	triage := "# Triage Decision — Cycle 1113\n\n" +
		"## top_n (commit to THIS cycle)\n\n" +
		"## deferred (carry to NEXT cycle's carryoverTodos)\n- something-else: deferred\n"
	tdd := "# TDD Report\n\n## Task: " + orphanSlug + "\n\n## RED Run Output\n\n```\nFAIL\n```\n\n" +
		"## Handoff to Builder\n\n```json\n{\"testFiles\": [\"" + orphanTestFile + "\"], \"redRunConfirmed\": true}\n```\n"
	for name, body := range map[string]string{"triage-report.md": triage, "test-report.md": tdd} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return ws
}

// TestC1113_001_reviewer_blocks_empty_topn_at_enforce is AC1's behavioural
// half: the FATAL verdict must reach the caller as a blocked ReviewResult
// through the real composition root constructor and phase dispatch, not merely
// as a (reason, block) tuple from the unexported gate. PRE-EXISTING GREEN.
func TestC1113_001_reviewer_blocks_empty_topn_at_enforce(t *testing.T) {
	ws := writeOrphanFixture(t)
	res := topngate.NewReviewer(config.StageEnforce).Review(
		context.Background(), core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if res.Approve {
		t.Fatalf("enforce must BLOCK orphan TDD authoring under an empty ## top_n; got Approve=true reason=%q", res.Reason)
	}
	if res.Reason == "" {
		t.Errorf("a blocked review must record a non-empty abort_reason (the operator's only evidence)")
	}
	if !strings.Contains(res.Reason, orphanSlug) {
		t.Errorf("abort reason must name the claimed slug %q; got %q", orphanSlug, res.Reason)
	}
	if !strings.Contains(res.Reason, orphanTestFile) {
		t.Errorf("abort reason must name the authored file(s) so the operator can find the orphan scaffold; got %q", res.Reason)
	}
}

// TestC1113_002_reviewer_approves_empty_topn_at_shadow is AC2's behavioural
// half and the NEGATIVE axis of 001: the identical fatal fixture must be
// logged-and-approved at shadow. A reviewer that blocks here would abort
// cycles during a rollout that promises observation only. PRE-EXISTING GREEN.
func TestC1113_002_reviewer_approves_empty_topn_at_shadow(t *testing.T) {
	ws := writeOrphanFixture(t)
	res := topngate.NewReviewer(config.StageShadow).Review(
		context.Background(), core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if !res.Approve {
		t.Fatalf("shadow must approve even the FATAL case (stage-gating is the whole rollout control); got Approve=false reason=%q", res.Reason)
	}
}

// TestC1113_003_reviewer_level_tdd_tests_exist_and_pass is AC1+AC2's coverage
// half: the two contracted reviewer-level tests must exist and PASS. The
// "no tests to run" guard is load-bearing — `go test -run <nonexistent>` exits
// 0, so an exit-code-only predicate would green on an empty test file.
// RED until Builder authors them.
func TestC1113_003_reviewer_level_tdd_tests_exist_and_pass(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", newTestsRun, topngatePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			newTestsRun, topngatePkg, code, err, stdout, stderr)
	}
	if strings.Contains(stdout, "no tests to run") || strings.Contains(stderr, "no tests to run") {
		t.Fatalf("no test matches %s — the reviewer-level TDD-phase coverage does not exist (exit 0 here is the vacuous pass this predicate rejects)\nstdout:\n%s", newTestsRun, stdout)
	}
	for _, name := range []string{enforceTest, shadowTest} {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("missing PASS for %s (renamed, skipped, or not run)\nstdout:\n%s", name, stdout)
		}
	}
}

// TestC1113_004_enforce_test_dies_when_gate_is_unwired is AC3's first mutation
// and the strongest anti-tautology signal: with tddScopeGate removed from
// reviewer.go's gates slice the TDD phase is ungated, so the enforce test MUST
// fail. A test that asserts nothing about the gate survives this and is
// rejected here. The mutation exists only in a `go test -overlay` mapping —
// the real reviewer.go is never written. RED until the test exists.
func TestC1113_004_enforce_test_dies_when_gate_is_unwired(t *testing.T) {
	overlay := mutateReviewer(t,
		"gates: []gate{topNBindingGate{}, tddScopeGate{}},",
		"gates: []gate{topNBindingGate{}},")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-overlay", overlay, "-run", newTestsRun, topngatePkg)
	assertMutantKills(t, enforceTest, "tddScopeGate unwired from the gates slice", stdout, stderr, code)
}

// TestC1113_005_shadow_test_dies_when_stage_guard_is_dropped is AC3's second
// mutation, aimed at the other half of reviewer.go:47: with the
// `r.stage == config.StageEnforce` comparison dropped, shadow blocks like
// enforce, so the shadow test MUST fail. This is what distinguishes a real
// stage-gating assertion from one that would pass at any stage.
// RED until the test exists.
func TestC1113_005_shadow_test_dies_when_stage_guard_is_dropped(t *testing.T) {
	overlay := mutateReviewer(t,
		"if block && r.stage == config.StageEnforce {",
		"if block {")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-overlay", overlay, "-run", newTestsRun, topngatePkg)
	assertMutantKills(t, shadowTest, "the StageEnforce guard dropped so shadow blocks too", stdout, stderr, code)
}

// TestC1113_006_topngate_suite_stays_green is AC4: the whole package must stay
// green, and every pre-existing test must still be there. Counting named PASS
// markers rejects the exit-0-after-deleting-an-inconvenient-test shape that a
// bare `go test` check cannot see.
func TestC1113_006_topngate_suite_stays_green(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", topngatePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test %s exited %d (err=%v) — this cycle is test-only and must not regress the package\nstdout:\n%s\nstderr:\n%s",
			topngatePkg, code, err, stdout, stderr)
	}
	for _, name := range preExistingTests {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("pre-existing test %s no longer reports PASS (deleted, renamed, or skipped) — a test-only cycle may add coverage, never remove it", name)
		}
	}
}

// mutateReviewer writes a copy of reviewer.go with old replaced by new and
// returns the path of a `go test -overlay` file mapping the real source at the
// mutant. It fails loudly when old is absent: a silently-unapplied mutation
// would make 004/005 pass for the wrong reason (the mutant would be the
// pristine source, whose tests are green).
func mutateReviewer(t *testing.T, old, new string) string {
	t.Helper()
	src := filepath.Join(acsassert.RepoRoot(t), reviewerSrc)
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	body := string(raw)
	if !strings.Contains(body, old) {
		t.Fatalf("mutation target %q absent from %s — reviewer.go's shape changed; update this predicate's mutation instead of deleting it", old, reviewerSrc)
	}
	dir := t.TempDir()
	mutant := filepath.Join(dir, "reviewer_mutant.go")
	if err := os.WriteFile(mutant, []byte(strings.Replace(body, old, new, 1)), 0o644); err != nil {
		t.Fatalf("write mutant: %v", err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	doc, err := json.Marshal(map[string]map[string]string{"Replace": {src: mutant}})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	if err := os.WriteFile(overlay, doc, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	return overlay
}

// assertMutantKills requires the named test to have FAILED under the mutation.
// A build failure is rejected too: the mutant must compile, or the "failure"
// proves nothing about the test's assertions.
func assertMutantKills(t *testing.T, name, mutation, stdout, stderr string, code int) {
	t.Helper()
	if strings.Contains(stderr, "build failed") || strings.Contains(stderr, "cannot use") || strings.Contains(stderr, "undefined:") {
		t.Fatalf("mutant (%s) failed to COMPILE — a non-zero exit from a broken build is not evidence the test is load-bearing\nstderr:\n%s", mutation, stderr)
	}
	if code == 0 {
		t.Fatalf("%s still PASSES with %s — the test is tautological (it does not depend on the gate it claims to cover)\nstdout:\n%s", name, mutation, stdout)
	}
	if !strings.Contains(stdout, "--- FAIL: "+name+" ") {
		t.Errorf("expected %s to FAIL under mutation (%s); it did not appear as a failure\nstdout:\n%s\nstderr:\n%s", name, mutation, stdout, stderr)
	}
}
