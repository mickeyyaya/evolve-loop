//go:build acs

package cycle1903

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	researchDoc      = "docs/research/practice-claude-model-release-notes-2026-10.md"
	indexDoc         = "docs/research-index.md"
	snapshotMarker   = "2026-10-15T23:06:21Z"
	researchIndexRef = "research/practice-claude-model-release-notes-2026-10.md"
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

func TestC1903_001_FastModeRateLimitTiersExplainedAsData(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if !strings.Contains(doc, snapshotMarker) {
		t.Fatalf("RED: doc does not cite the snapshot %s that this update follows", snapshotMarker)
	}
	for _, want := range []string{"Start", "Build", "Scale", "rate limit", "Opus 5.5", "Opus 4.8", "research preview"} {
		if !strings.Contains(doc, want) {
			t.Errorf("RED: doc does not cover release-notes fact %q", want)
		}
	}
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: doc does not state the release-notes text is untrusted data")
	}
}

func TestC1903_002_ResearchDocIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !strings.Contains(readRepoFile(t, researchDoc), snapshotMarker) {
		t.Fatalf("RED: doc lacks the new snapshot, so tracking proves nothing new")
	}
	_, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", researchDoc)
	if err != nil || code != 0 {
		t.Errorf("RED: %s untracked, may be gitignored and dropped at ship (code=%d err=%v)", researchDoc, code, err)
	}
}

func TestC1903_003_EveryCorePersonaHasNoChangeVerdictForThisUpdate(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if !strings.Contains(doc, "ADR-0133") {
		t.Fatalf("RED: doc does not cite ADR-0133")
	}
	if !strings.Contains(doc, snapshotMarker) {
		t.Fatalf("RED: doc has no review of snapshot %s, so its persona verdicts do not cover this update", snapshotMarker)
	}
	section := doc[strings.Index(doc, snapshotMarker):]
	for _, p := range corePersonas {
		idx := strings.Index(section, p)
		if idx < 0 {
			t.Errorf("RED: persona %s not named in the section for this update", p)
			continue
		}
		window := strings.ToLower(section[idx:])
		if len(window) > 600 {
			window = window[:600]
		}
		if !strings.Contains(window, "no change") && !strings.Contains(window, "does not change") {
			t.Errorf("RED: persona %s has no stated no-change reason for this update", p)
		}
	}
}

func TestC1903_004_BaselineRequiredBeforeAnyPersonaChange(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if !strings.Contains(doc, snapshotMarker) {
		t.Fatalf("RED: doc lacks the new snapshot")
	}
	section := strings.ToLower(doc[strings.Index(doc, snapshotMarker):])
	for _, want := range []string{"baseline", "m2", "d8"} {
		if !strings.Contains(section, want) {
			t.Errorf("RED: update section does not tie its verdict to %q", want)
		}
	}
}

func TestC1903_005_ResearchIndexLinksDocUnderADR0133(t *testing.T) {
	idx := readRepoFile(t, indexDoc)
	pos := strings.Index(idx, "("+researchIndexRef+")")
	if pos < 0 {
		t.Fatalf("RED: %s does not link %s", indexDoc, researchIndexRef)
	}
	adr := strings.Index(idx, "ADR-0133")
	if adr < 0 || pos < adr {
		t.Fatalf("RED: index entry sits before the ADR-0133 section")
	}
	if next := strings.Index(idx[adr:], "\n## 20"); next > 0 && pos > adr+next {
		t.Errorf("RED: index entry sits outside the ADR-0133 section")
	}
}

func TestC1903_006_NoPersonaFileChanges(t *testing.T) {
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

func TestC1903_007_DocAndIndexPassSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !strings.Contains(readRepoFile(t, researchDoc), snapshotMarker) {
		t.Fatalf("RED: doc lacks the new snapshot, so a lint pass would prove nothing new")
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

func TestC1903_008_PracticewatchSuiteStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !strings.Contains(readRepoFile(t, researchDoc), snapshotMarker) {
		t.Fatalf("RED: deliverable lacks the new snapshot")
	}
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/practicewatch")
	if err != nil || code != 0 {
		t.Errorf("practicewatch tests rc=%d err=%v out=%s", code, err, out)
	}
}
