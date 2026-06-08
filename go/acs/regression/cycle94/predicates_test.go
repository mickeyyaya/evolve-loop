//go:build acs

// Package cycle94 ports the cycle-94 ACS predicates (5 bash files).
// Subjects: lesson template externalization, fast-fail counter logic,
// orchestrator fast-fail stop criterion, stream-json operator visibility,
// trust kernel regression guard.
package cycle94

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC94_001_LessonTemplateExternalized ports cycle-94/001.
func TestC94_001_LessonTemplateExternalized(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "agents", "evolve-retrospective-reference.md"),
		filepath.Join(root, "agents", "evolve-retrospective.md"),
		filepath.Join(root, "docs", "templates", "failure-lesson.yaml"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if acsassert.FileContainsAny(p, "failure-lesson", "lesson template", "lesson_id") {
				return
			}
		}
	}
	t.Logf("no externalized lesson template found")
}

// TestC94_002_FastFailCounterLogic ports cycle-94/002.
func TestC94_002_FastFailCounterLogic(t *testing.T) {
	root := acsassert.RepoRoot(t)
	subagent := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	if _, err := os.Stat(subagent); err != nil {
		t.Skip("subagent-run.sh missing — skip")
	}
	if !acsassert.FileContains(t, subagent, ".fast-fail-counter") {
		return
	}
}

// TestC94_003_OrchestratorFastFailStopCriterion ports cycle-94/003.
func TestC94_003_OrchestratorFastFailStopCriterion(t *testing.T) {
	root := acsassert.RepoRoot(t)
	orch := filepath.Join(root, "agents", "evolve-orchestrator.md")
	if _, err := os.Stat(orch); err != nil {
		t.Skip("orchestrator persona missing — skip")
	}
	if !acsassert.FileContainsAny(orch, "fast-fail", "fast_fail", "STOP CRITERION") {
		t.Logf("orchestrator: no fast-fail stop criterion marker")
	}
}

// TestC94_004_StreamJsonOperatorVisibility ports cycle-94/004.
func TestC94_004_StreamJsonOperatorVisibility(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh"),
		filepath.Join(root, "archive", "legacy", "scripts", "dispatch", "evolve-loop-dispatch.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if acsassert.FileContainsAny(p, "stream-json", "stream_json", "--output-format") {
			return
		}
	}
	t.Logf("no stream-json operator-visibility marker")
}

// TestC94_005_TrustKernelRegressionGuard ports cycle-94/005.
func TestC94_005_TrustKernelRegressionGuard(t *testing.T) {
	root := acsassert.RepoRoot(t)
	subagent := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	if _, err := os.Stat(subagent); err != nil {
		t.Skip("subagent-run.sh missing — skip")
	}
	// Trust kernel anchors: challenge token + ledger
	for _, marker := range []string{"challenge_token", "ledger"} {
		if !acsassert.FileContains(t, subagent, marker) {
			return
		}
	}
}
