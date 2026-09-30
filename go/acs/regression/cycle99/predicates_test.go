//go:build acs

package cycle99

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestC99_001_PsmasABVerificationDocumented(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "architecture", "psmas-phase-scheduling.md")
	if !fixtures.FilePresent(doc) {
		t.Skip("psmas-phase-scheduling.md missing — skip cycle-99-001")
	}
	if !acsassert.FileMatchesRegex(t, doc, `cycle-\d+`) {
		return
	}
	if count := acsassert.CountOccurrencesAny(doc, "cycle-"); count < 5 {
		t.Errorf("psmas doc references only %d cycle-NN identifiers (need ≥5)", count)
	}
	if !acsassert.FileMatchesRegex(t, doc, `\d+(\.\d+)?\s*%`) {
		return
	}
	if !acsassert.FileMatchesRegex(t, doc, `(≥|>=|at least)\s*20\s*%|20%\s*(threshold|target|criterion)`) {
		return
	}
	if !acsassert.FileMatchesRegex(t, doc, `\b(FLIP|DEFER|REJECT)\b`) {
		return
	}
	if !acsassert.FileContains(t, doc, "EVOLVE_PSMAS_SKIP") {
		return
	}
}

func TestC99_002_GitignoreReachabilityGuardFunctional(t *testing.T) {
	root := acsassert.RepoRoot(t)
	guard := filepath.Join(root, "legacy", "scripts", "guards", "gitignore-reachability-check.sh")
	if !fixtures.FilePresent(guard) {
		t.Skip("gitignore-reachability-check.sh missing — skip cycle-99-002")
	}

	claudemd := filepath.Join(root, "CLAUDE.md")
	if acsassert.FileExists(t, claudemd) {
		_, _, code, _ := acsassert.SubprocessOutput("bash", guard, claudemd)
		if code != 0 {
			t.Errorf("guard rc=%d on reachable path %s (expected 0)", code, claudemd)
		}
	}
}

func TestC99_003_TurnOverrunIncidentAnalysisComplete(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "docs", "operations", "incidents", "cycle-95-turn-overrun.md"),
		filepath.Join(root, "knowledge-base", "research", "cycle-95-turn-overrun.md"),
		filepath.Join(root, "knowledge-base", "research", "turn-overrun-cycle-95.md"),
	}
	var report string
	for _, p := range candidates {
		if acsassert.FileExists(t, p) {
			report = p
			break
		}
	}
	if report == "" {
		t.Skip("no cycle-95 turn-overrun incident report at accepted paths")
	}
	labels := []string{"happened", "research", "reasoning", "fix", "lessons", "references"}
	found := 0
	for _, label := range labels {
		if acsassert.FileMatchesRegex(t, report, "(?im)^#{1,6}.*"+strings.ToLower(label)) {
			found++
		}
	}
	if found < 6 {
		t.Errorf("%s: only %d/6 incident-structure section labels found", report, found)
	}
	for _, token := range []string{"abnormal-turn-overrun-c95", "abnormal-events.jsonl"} {
		if !acsassert.FileContains(t, report, token) {
			return
		}
	}
	if !acsassert.FileMatchesRegex(t, report, `(?i)cycle[- ]?95|c95`) {
		return
	}
}
