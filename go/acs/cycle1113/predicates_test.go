//go:build acs

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

	reviewerSrc = "go/internal/topngate/reviewer.go"

	enforceTest = "TestNewReviewer_TDDEnforceBlocksEmptyTopN"
	shadowTest  = "TestNewReviewer_TDDShadowApprovesEmptyTopN"
	newTestsRun = "^TestNewReviewer_TDD"

	orphanSlug     = "orphan-task-cycle-1113"
	orphanTestFile = "go/acs/cycle1113/predicates_test.go"
)

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

func TestC1113_002_reviewer_approves_empty_topn_at_shadow(t *testing.T) {
	ws := writeOrphanFixture(t)
	res := topngate.NewReviewer(config.StageShadow).Review(
		context.Background(), core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if !res.Approve {
		t.Fatalf("shadow must approve even the FATAL case (stage-gating is the whole rollout control); got Approve=false reason=%q", res.Reason)
	}
}

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

func TestC1113_004_enforce_test_dies_when_gate_is_unwired(t *testing.T) {
	overlay := mutateReviewer(t,
		"gates: []gate{topNBindingGate{}, tddScopeGate{}},",
		"gates: []gate{topNBindingGate{}},")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-overlay", overlay, "-run", newTestsRun, topngatePkg)
	assertMutantKills(t, enforceTest, "tddScopeGate unwired from the gates slice", stdout, stderr, code)
}

func TestC1113_005_shadow_test_dies_when_stage_guard_is_dropped(t *testing.T) {
	overlay := mutateReviewer(t,
		"if block && r.stage == config.StageEnforce {",
		"if block {")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-overlay", overlay, "-run", newTestsRun, topngatePkg)
	assertMutantKills(t, shadowTest, "the StageEnforce guard dropped so shadow blocks too", stdout, stderr, code)
}

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
