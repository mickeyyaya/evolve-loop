//go:build acs

package cycle1904

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const researchDoc = "docs/research/practice-claude-prompting-guide-2026-10.md"

const inboxItemID = "practice-claude-prompting-guide-ef1aac80-202610161459"

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

func TestC1904_001_DocIsGitTrackedAndNamesThisInboxItem(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: %s missing on disk", researchDoc)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", researchDoc); code != 0 {
		t.Errorf("RED: %s not tracked by git", researchDoc)
	}
	doc := readRepoFile(t, researchDoc)
	if !strings.Contains(doc, inboxItemID) {
		t.Errorf("RED: doc does not name the inbox item %s it answers", inboxItemID)
	}
	if !strings.Contains(doc, "2026-10-14T16:10:51Z") {
		t.Errorf("RED: doc does not name the prior snapshot 2026-10-14T16:10:51Z the change is measured against")
	}
}

func TestC1904_002_MedicationDosingSectionExplainedAsData(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	lower := strings.ToLower(doc)
	if !strings.Contains(doc, "claude-prompting-best-practices") {
		t.Errorf("RED: doc does not name the claude-prompting-best-practices source")
	}
	for _, topic := range []string{"medication dosing", "dailymed", "label", "web search", "pharmacist"} {
		if !strings.Contains(lower, topic) {
			t.Errorf("RED: doc does not explain the new guide topic %q", topic)
		}
	}
	for _, model := range []string{"Sonnet 5.5", "Haiku 5.5", "Sonnet 4.6", "Haiku 4.5"} {
		if !strings.Contains(doc, model) {
			t.Errorf("RED: doc does not explain the dosing block for Claude %s", model)
		}
	}
	if !strings.Contains(lower, "emotional distress") {
		t.Errorf("RED: doc does not explain the Haiku 5.5 emotional-distress clause")
	}
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: doc does not state the guide text is untrusted data")
	}
}

func TestC1904_003_DocDoesNotRelayTheUpstreamInstruction(t *testing.T) {
	lower := strings.ToLower(readRepoFile(t, researchDoc))
	for _, relayed := range []string{
		"add the block below for the model your application calls to your system prompt, exactly as written",
		"claude doesn't give a number from memory and instead points the person",
	} {
		if strings.Contains(lower, relayed) {
			t.Errorf("doc copies the upstream instruction verbatim instead of explaining it as data: %q", relayed)
		}
	}
}

func TestC1904_004_EveryCorePersonaHasAVerdictOnTheDosingSection(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if !strings.Contains(doc, "ADR-0133") {
		t.Errorf("RED: doc does not cite ADR-0133")
	}
	for _, p := range corePersonas {
		heading := "### " + p
		idx := strings.Index(doc, heading)
		if idx < 0 {
			t.Errorf("RED: persona %s has no section heading %q", p, heading)
			continue
		}
		section := doc[idx+len(heading):]
		if next := strings.Index(section, "\n#"); next >= 0 {
			section = section[:next]
		}
		lowerSection := strings.ToLower(section)
		if !strings.Contains(lowerSection, "does not change") && !strings.Contains(lowerSection, "no change") {
			t.Errorf("RED: persona %s has no change verdict in its section", p)
		}
		if !strings.Contains(lowerSection, "dosing") && !strings.Contains(lowerSection, "medication") {
			t.Errorf("RED: persona %s section gives no reason tied to the new dosing guidance", p)
		}
	}
}

func TestC1904_005_NoPersonaFileChangedOrCarriesDosingText(t *testing.T) {
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
		lowerPersona := strings.ToLower(readRepoFile(t, path))
		if strings.Contains(lowerPersona, "dailymed") || strings.Contains(lowerPersona, "medication dosing") {
			t.Errorf("persona %s carries untrusted dosing text injected without measurement", p)
		}
	}
}

func TestC1904_006_ResearchIndexLinksTheDoc(t *testing.T) {
	index := readRepoFile(t, "docs/research-index.md")
	link := "(research/practice-claude-prompting-guide-2026-10.md)"
	if !strings.Contains(index, link) {
		t.Fatalf("RED: docs/research-index.md has no link %s", link)
	}
	root := acsassert.RepoRoot(t)
	target := filepath.Join(root, "docs", "research", "practice-claude-prompting-guide-2026-10.md")
	if !acsassert.FileExists(t, target) {
		t.Errorf("RED: index link target %s does not resolve", target)
	}
}

func TestC1904_007_DocPassesSteLint(t *testing.T) {
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

func TestC1904_008_PracticewatchSuiteStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/practicewatch")
	if err != nil || code != 0 {
		t.Errorf("practicewatch tests rc=%d err=%v out=%s", code, err, out)
	}
}
