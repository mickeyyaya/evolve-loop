//go:build acs

package cycle1901

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	researchDoc  = "docs/research/practice-claude-code-changelog-2026-10.md"
	researchIdx  = "docs/research-index.md"
	adr0133Title = "the-loop-watches-cli-practice-and-trims-its-scaffolding"
)

var corePersonas = []string{
	"evolve-scout", "evolve-triage", "evolve-tdd-engineer",
	"evolve-builder", "evolve-auditor", "evolve-retrospective",
}

func readFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", rel, err)
	}
	return string(b)
}

func TestC1901_001_DocIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: %s missing on disk", researchDoc)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", researchDoc)
	if code != 0 {
		t.Errorf("RED: %s not tracked by git", researchDoc)
	}
}

func TestC1901_002_DocExplainsNewestReleaseItemsAsData(t *testing.T) {
	doc := readFile(t, researchDoc)
	for _, token := range []string{
		"CLAUDE_CODE_RETRY_WATCHDOG_MAX_USAGE_LIMIT_WAIT_MS",
		"classifier_verdict",
		"WorktreeCreate",
		"full disk",
		"fable",
		"Grep",
	} {
		if !strings.Contains(doc, token) {
			t.Errorf("RED: doc does not explain changelog item %q", token)
		}
	}
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: doc does not state the changelog text is untrusted data")
	}
}

func TestC1901_003_DocKeepsSourcedOlderReleaseItems(t *testing.T) {
	doc := readFile(t, researchDoc)
	for _, sourced := range []string{
		"$.prompt.media",
		"$.session.identity()",
		"virtualText",
		"prompt.change",
		"tracing guidance",
	} {
		if !strings.Contains(doc, sourced) {
			t.Errorf("RED: doc drops changelog item %q that the snapshot lists", sourced)
		}
	}
}

func TestC1901_010_DocMakesNoUnsourcedClaims(t *testing.T) {
	lower := strings.ToLower(readFile(t, researchDoc))
	if strings.Contains(lower, "subshell") {
		t.Errorf("RED: doc claims a subshell fix, which no changelog line states")
	}
	if !strings.Contains(lower, "background command") {
		t.Errorf("RED: doc does not state the 2.1.300 background command polling fix")
	}
}

func TestC1901_011_ExplanationDoesNotOverstateIndexChange(t *testing.T) {
	root := acsassert.RepoRoot(t)
	explanations, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1901-*.md"))
	if err != nil || len(explanations) == 0 {
		t.Fatalf("RED: no cycle 1901 build explanation found: %v", err)
	}
	for _, path := range explanations {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("RED: %s unreadable: %v", path, err)
		}
		for _, line := range strings.Split(strings.ToLower(string(b)), "\n") {
			if strings.Contains(line, "research-index") && strings.Contains(line, "align") {
				t.Errorf("RED: %s claims the index diff realigns entries, but it adds one row: %q", filepath.Base(path), line)
			}
		}
	}
}

func TestC1901_004_EveryCorePersonaHasEntryWithReason(t *testing.T) {
	doc := readFile(t, researchDoc)
	if !strings.Contains(doc, "ADR-0133") {
		t.Errorf("RED: doc does not cite ADR-0133")
	}
	for _, p := range corePersonas {
		heading := "### " + p + "\n"
		at := strings.Index(doc, heading)
		if at < 0 {
			t.Errorf("RED: no entry heading for persona %s", p)
			continue
		}
		body := doc[at+len(heading):]
		if next := strings.Index(body, "\n#"); next >= 0 {
			body = body[:next]
		}
		lower := strings.ToLower(body)
		if !strings.Contains(lower, "does not change") || strings.Count(body, "\n") < 3 {
			t.Errorf("RED: persona %s entry lacks a reasoned no-change verdict", p)
		}
	}
}

func TestC1901_005_NoPersonaFileChanges(t *testing.T) {
	root := acsassert.RepoRoot(t)
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

func TestC1901_006_ResearchIndexLinksDocInsideADR0133Section(t *testing.T) {
	idx := readFile(t, researchIdx)
	linkAt := strings.Index(idx, "](research/practice-claude-code-changelog-2026-10.md)")
	if linkAt < 0 {
		t.Fatalf("RED: %s does not link research/practice-claude-code-changelog-2026-10.md", researchIdx)
	}
	adrAt := strings.Index(idx, adr0133Title)
	if adrAt < 0 || linkAt < adrAt {
		t.Fatalf("RED: link precedes the ADR-0133 entry (adr at %d, link at %d)", adrAt, linkAt)
	}
	if next := strings.Index(idx[adrAt:], "\n## "); next >= 0 && linkAt > adrAt+next {
		t.Errorf("RED: link sits after the ADR-0133 section ends")
	}
}

func TestC1901_007_DocStatesAdoptionNeedsMeasuredBaseline(t *testing.T) {
	doc := strings.ToLower(readFile(t, researchDoc))
	for _, token := range []string{"10-cycle", "baseline", "m2", "m3", "d7", "d8"} {
		if !strings.Contains(doc, token) {
			t.Errorf("RED: doc does not tie the verdict to the %q rule of ADR-0133", token)
		}
	}
}

func TestC1901_008_DocAndIndexPassSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{researchDoc, researchIdx} {
		if !strings.Contains(readFile(t, researchIdx), "practice-claude-code-changelog-2026-10.md") {
			t.Fatalf("RED: index entry absent; lint of %s is not meaningful", rel)
		}
		out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
			"docs", "ste-lint", "--strict", "--project-root", root, rel)
		if err != nil {
			t.Fatalf("ste-lint did not run: %v", err)
		}
		if code != 0 {
			t.Errorf("ste-lint %s rc=%d out=%q stderr=%q", rel, code, out, errOut)
		}
	}
}

func TestC1901_009_PracticewatchSuiteStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/practicewatch")
	if err != nil || code != 0 {
		t.Errorf("practicewatch tests rc=%d err=%v out=%s", code, err, out)
	}
}
