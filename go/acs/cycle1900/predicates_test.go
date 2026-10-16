//go:build acs

package cycle1900

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	researchDoc  = "docs/research/practice-agy-cli-changelog-2026-10.md"
	researchIdx  = "docs/research-index.md"
	adr0133Title = "the-loop-watches-cli-practice-and-trims-its-scaffolding"
)

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

func TestC1900_001_DocIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: %s missing on disk", researchDoc)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", researchDoc)
	if code != 0 {
		t.Errorf("RED: %s not tracked by git (gitignored files drop at ship)", researchDoc)
	}
}

func TestC1900_002_ChangelogVersionsExplainedAsData(t *testing.T) {
	doc := readDoc(t)
	for _, v := range []string{"1.3.4", "1.3.5", "1.3.6", "1.4.0"} {
		if !strings.Contains(doc, v) {
			t.Errorf("RED: doc does not cover agy changelog version %s", v)
		}
	}
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: doc does not state the changelog text is untrusted data")
	}
}

func TestC1900_003_EveryCorePersonaEvaluatedUnderADR0133(t *testing.T) {
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

func TestC1900_004_NoPersonaFileChanges(t *testing.T) {
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

func TestC1900_005_DocPassesSteLint(t *testing.T) {
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

func TestC1900_006_PracticewatchSuiteStaysGreenWithDocPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: deliverable absent")
	}
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/practicewatch")
	if err != nil || code != 0 {
		t.Errorf("practicewatch tests rc=%d err=%v out=%s", code, err, out)
	}
}

func TestC1900_007_ResearchIndexLinksDocUnderADR0133(t *testing.T) {
	root := acsassert.RepoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, researchIdx))
	if err != nil {
		t.Fatalf("%s unreadable: %v", researchIdx, err)
	}
	idx := string(b)
	adrAt := strings.Index(idx, adr0133Title)
	linkAt := strings.Index(idx, "](research/practice-agy-cli-changelog-2026-10.md)")
	if linkAt < 0 {
		t.Fatalf("RED: %s does not link research/practice-agy-cli-changelog-2026-10.md", researchIdx)
	}
	if adrAt < 0 || linkAt < adrAt {
		t.Errorf("RED: link is not placed in the ADR-0133 section (adr at %d, link at %d)", adrAt, linkAt)
	}
	next := strings.Index(idx[adrAt:], "\n## ")
	if next >= 0 && linkAt > adrAt+next {
		t.Errorf("RED: link sits after the ADR-0133 section ends")
	}
}

func TestC1900_008_DocCoversNewestReleaseBehaviours(t *testing.T) {
	doc := readDoc(t)
	for _, token := range []string{"OSC 7501", "alt+m", "GIT_EDITOR", "/compact", "--strict"} {
		if !strings.Contains(doc, token) {
			t.Errorf("RED: doc does not explain the changelog item %q", token)
		}
	}
}

func TestC1900_009_DocStatesAdoptionNeedsMeasuredBaseline(t *testing.T) {
	doc := strings.ToLower(readDoc(t))
	for _, token := range []string{"10-cycle", "baseline", "m2", "m3"} {
		if !strings.Contains(doc, token) {
			t.Errorf("RED: doc does not tie the verdict to the %q measurement rule of ADR-0133", token)
		}
	}
}

func TestC1900_010_IndexEntryPassesSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, researchIdx))
	if err != nil || !strings.Contains(string(b), "research/practice-agy-cli-changelog-2026-10.md") {
		t.Fatalf("RED: index entry absent; nothing to lint")
	}
	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, researchIdx)
	if runErr != nil {
		t.Fatalf("ste-lint did not run: %v", runErr)
	}
	if code != 0 {
		t.Errorf("ste-lint rc=%d out=%q stderr=%q", code, out, errOut)
	}
}
