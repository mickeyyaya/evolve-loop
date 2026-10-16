//go:build acs

package cycle1896

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const researchDoc = "docs/research/practice-claude-code-changelog-2026-10.md"

var corePersonas = []string{
	"evolve-scout", "evolve-triage", "evolve-tdd-engineer",
	"evolve-builder", "evolve-auditor", "evolve-retrospective",
}

func readDoc(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, researchDoc))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", researchDoc, err)
	}
	return string(b)
}

func TestC1896_001_DocIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: %s missing on disk", researchDoc)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", researchDoc)
	if code != 0 {
		t.Errorf("RED: %s not tracked by git (gitignored files drop at ship)", researchDoc)
	}
}

func TestC1896_002_ChangelogVersionsExplainedAsData(t *testing.T) {
	doc := readDoc(t)
	for _, v := range []string{"2.1.299", "2.1.300", "2.1.301"} {
		if !strings.Contains(doc, v) {
			t.Errorf("RED: doc does not cover Claude Code changelog version %s", v)
		}
	}
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: doc does not state the changelog text is untrusted data")
	}
}

func TestC1896_003_EveryCorePersonaEvaluatedUnderADR0133(t *testing.T) {
	doc := readDoc(t)
	if !strings.Contains(doc, "ADR-0133") {
		t.Errorf("RED: doc does not cite ADR-0133")
	}
	for _, p := range corePersonas {
		idx := strings.Index(doc, p)
		if idx < 0 {
			t.Errorf("RED: persona %s not named", p)
			continue
		}
		window := doc[idx:]
		if len(window) > 600 {
			window = window[:600]
		}
		if !strings.Contains(strings.ToLower(window), "no change") && !strings.Contains(strings.ToLower(window), "does not change") && !strings.Contains(strings.ToLower(window), "no persona change") {
			t.Errorf("RED: persona %s has no stated reason for its no-change verdict within its entry", p)
		}
	}
}

func TestC1896_004_NoPersonaFileChanges(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: deliverable absent; cannot claim a no-persona-change cycle without it")
	}
	for _, p := range corePersonas {
		path := "agents/" + p + ".md"
		out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "origin/main...HEAD", "--", path)
		if err != nil || code != 0 {
			t.Fatalf("git diff for %s failed: code=%d err=%v", path, code, err)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("persona %s changed although the proposal states none changes", p)
		}
	}
}

func TestC1896_005_DocPassesSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: %s missing; nothing to lint", researchDoc)
	}
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, researchDoc)
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
		t.Errorf("ste-lint rc=%d out=%q stderr=%q", code, out, errOut)
	}
}

func TestC1896_006_PracticewatchSuiteStaysGreenWithDocPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: deliverable absent")
	}
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/practicewatch")
	if err != nil || code != 0 {
		t.Errorf("practicewatch tests rc=%d err=%v out=%s", code, err, out)
	}
}

func TestC1896_007_DocRejectsBuilderDietWithoutBaseline(t *testing.T) {
	doc := readDoc(t)
	lower := strings.ToLower(doc)
	for _, want := range []string{"d8", "baseline", "m2"} {
		if !strings.Contains(lower, want) {
			t.Errorf("RED: doc does not tie the no-change verdict to ADR-0133 %q", want)
		}
	}
	if strings.Contains(lower, "evolve-builder prompt will change") {
		t.Errorf("RED: doc promises a builder persona change this cycle")
	}
}
