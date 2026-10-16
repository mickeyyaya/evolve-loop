//go:build acs

package cycle1902

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	researchDoc = "docs/research/practice-agy-cli-changelog-page-2026-10.md"
	indexDoc    = "docs/research-index.md"
)

var corePersonas = []string{
	"evolve-scout", "evolve-triage", "evolve-tdd-engineer",
	"evolve-builder", "evolve-auditor", "evolve-retrospective",
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", rel, err)
	}
	return string(b)
}

func TestC1902_001_ChangelogV2231EntriesExplainedAsData(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	for _, want := range []string{"v2.23.1", "license picker", "Gemini 4 Argon"} {
		if !strings.Contains(doc, want) {
			t.Errorf("RED: doc does not cover changelog entry %q", want)
		}
	}
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: doc does not state the changelog text is untrusted data")
	}
}

func TestC1902_002_EveryCorePersonaHasNoChangeVerdictForV2231(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if !strings.Contains(doc, "ADR-0133") {
		t.Fatalf("RED: doc does not cite ADR-0133")
	}
	if !strings.Contains(doc, "v2.23.1") {
		t.Fatalf("RED: doc has no v2.23.1 review, so its persona verdicts do not cover this update")
	}
	for _, p := range corePersonas {
		idx := strings.Index(doc, p)
		if idx < 0 {
			t.Errorf("RED: persona %s not named", p)
			continue
		}
		window := strings.ToLower(doc[idx:])
		if len(window) > 600 {
			window = window[:600]
		}
		if !strings.Contains(window, "no change") && !strings.Contains(window, "does not change") {
			t.Errorf("RED: persona %s has no stated no-change reason in its entry", p)
		}
	}
}

func TestC1902_003_BaselineRequiredBeforeAnyPersonaChange(t *testing.T) {
	doc := strings.ToLower(readRepoFile(t, researchDoc))
	if !strings.Contains(doc, "v2.23.1") || !strings.Contains(doc, "baseline") {
		t.Errorf("RED: doc must tie the v2.23.1 verdict to the measured baseline rule")
	}
}

func TestC1902_004_ResearchIndexLinksDocUnderADR0133(t *testing.T) {
	idx := readRepoFile(t, indexDoc)
	link := "research/practice-agy-cli-changelog-page-2026-10.md"
	pos := strings.Index(idx, "("+link+")")
	if pos < 0 {
		t.Fatalf("RED: %s does not link %s", indexDoc, link)
	}
	adr := strings.Index(idx, "ADR-0133")
	next := strings.Index(idx[adr:], "\n## 20")
	if next > 0 {
		next = adr + next
		if pos > next {
			t.Errorf("RED: index entry sits outside the ADR-0133 section")
		}
	} else if pos < adr {
		t.Errorf("RED: index entry sits before the ADR-0133 section")
	}
}

func TestC1902_005_NoPersonaFileChanges(t *testing.T) {
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

func TestC1902_006_DocAndIndexPassSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !strings.Contains(readRepoFile(t, researchDoc), "v2.23.1") {
		t.Fatalf("RED: doc lacks the v2.23.1 update, so a lint pass would prove nothing new")
	}
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, researchDoc, indexDoc)
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
		t.Errorf("ste-lint rc=%d out=%q stderr=%q", code, out, errOut)
	}
}

func TestC1902_007_PracticewatchSuiteStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !strings.Contains(readRepoFile(t, researchDoc), "v2.23.1") {
		t.Fatalf("RED: deliverable lacks the v2.23.1 update")
	}
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/practicewatch")
	if err != nil || code != 0 {
		t.Errorf("practicewatch tests rc=%d err=%v out=%s", code, err, out)
	}
}
