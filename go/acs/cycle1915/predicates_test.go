//go:build acs

package cycle1915

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	researchDoc   = "docs/research/practice-agy-cli-changelog-page-2026-10.md"
	researchIdx   = "docs/research-index.md"
	adr0133Title  = "the-loop-watches-cli-practice-and-trims-its-scaffolding"
	newestHeading = "### Version v2.24.0"
)

var corePersonas = []string{
	"evolve-scout", "evolve-triage", "evolve-tdd-engineer",
	"evolve-builder", "evolve-auditor", "evolve-retrospective",
}

var release2_24_0SnapshotIdentifiers = map[string]bool{
	"/compact": true, "Cmd+Shift+M": true, "Ctrl+Shift+M": true,
	"Cmd+Shift+V": true, "Ctrl+Shift+V": true, "Flash Lite": true, "Flash": true, "Pro": true,
	"Sort Conversations": true, "Sort Projects": true, "Auto Pin": true,
	"Application Default Credentials": true, "Claude Opus 5.5": true, "Claude Sonnet 5.5": true,
	"Log back in": true, "Display Options": true, "Review permissions": true,
	"@": true, "WebM": true, "PowerShell": true, "Vertex AI": true,
}

var backtickSpan = regexp.MustCompile("`([^`]+)`")

func readFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", rel, err)
	}
	return string(b)
}

func sectionAfter(t *testing.T, doc, heading string) string {
	t.Helper()
	at := strings.Index(doc, heading+"\n")
	if at < 0 {
		t.Fatalf("RED: doc has no %q section", heading)
	}
	body := doc[at+len(heading)+1:]
	if next := strings.Index(body, "\n#"); next >= 0 {
		body = body[:next]
	}
	return body
}

func TestC1915_001_DocIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: %s missing on disk", researchDoc)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", researchDoc)
	if code != 0 {
		t.Errorf("RED: %s not tracked by git", researchDoc)
	}
}

func TestC1915_002_DocExplainsV2_24_0AsData(t *testing.T) {
	doc := readFile(t, researchDoc)
	section := sectionAfter(t, doc, newestHeading)
	for _, item := range []string{
		"/compact",
		"side panel",
		"Flash Lite",
		"Auto Pin",
		"Application Default Credentials",
		"Claude Opus 5.5",
		"WebM",
		"Vertex AI",
	} {
		if !strings.Contains(section, item) {
			t.Errorf("RED: the v2.24.0 section does not explain changelog item %q", item)
		}
	}
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: doc does not state the changelog text is untrusted data")
	}
	if !strings.Contains(strings.SplitN(doc, "\n## ", 2)[0], "v2.24.0") {
		t.Errorf("RED: the doc summary does not name release v2.24.0 in its scope")
	}
}

func TestC1915_003_DocKeepsSourcedOlderReleaseItems(t *testing.T) {
	doc := readFile(t, researchDoc)
	for _, heading := range []string{"### Version v2.23.0", "### Version v2.23.1"} {
		if !strings.Contains(doc, heading+"\n") {
			t.Errorf("RED: doc drops the %q section", heading)
		}
	}
	for _, sourced := range []string{"Permissions picker", ".zip", "license picker", "Gemini 4 Argon"} {
		if !strings.Contains(doc, sourced) {
			t.Errorf("RED: doc drops changelog item %q that earlier releases list", sourced)
		}
	}
}

func TestC1915_004_NewestSectionCitesOnlySnapshotIdentifiers(t *testing.T) {
	section := sectionAfter(t, readFile(t, researchDoc), newestHeading)
	spans := backtickSpan.FindAllStringSubmatch(section, -1)
	if len(spans) < 3 {
		t.Fatalf("RED: the v2.24.0 section cites %d code identifiers, want at least 3", len(spans))
	}
	for _, m := range spans {
		if !release2_24_0SnapshotIdentifiers[m[1]] {
			t.Errorf("RED: the v2.24.0 section cites %q, which the v2.24.0 changelog snapshot does not contain", m[1])
		}
	}
}

func TestC1915_005_EveryCorePersonaHasReasonedNoChangeVerdict(t *testing.T) {
	doc := readFile(t, researchDoc)
	if !strings.Contains(doc, "ADR-0133") {
		t.Errorf("RED: doc does not cite ADR-0133")
	}
	for _, p := range corePersonas {
		heading := "### " + p
		if !strings.Contains(doc, heading+"\n") {
			t.Errorf("RED: no entry heading for persona %s", p)
			continue
		}
		body := sectionAfter(t, doc, heading)
		if !strings.Contains(strings.ToLower(body), "does not change") || strings.Count(body, "\n") < 3 {
			t.Errorf("RED: persona %s entry lacks a reasoned no-change verdict", p)
		}
		if !strings.Contains(body, "v2.24.0") {
			t.Errorf("RED: persona %s entry does not weigh the v2.24.0 release", p)
		}
	}
}

func TestC1915_006_NoPersonaFileChanges(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, p := range corePersonas {
		path := "agents/" + p + ".md"
		out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "origin/main", "--", path)
		if err != nil || code != 0 {
			t.Fatalf("git diff for %s failed: code=%d err=%v", path, code, err)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("persona %s changed although the proposal states none changes", p)
		}
	}
}

func TestC1915_007_ResearchIndexLinksDocInsideADR0133Section(t *testing.T) {
	idx := readFile(t, researchIdx)
	link := "](research/practice-agy-cli-changelog-page-2026-10.md)"
	if strings.Count(idx, link) != 1 {
		t.Fatalf("RED: %s links the doc %d times, want exactly 1", researchIdx, strings.Count(idx, link))
	}
	linkAt := strings.Index(idx, link)
	adrAt := strings.Index(idx, adr0133Title)
	if adrAt < 0 || linkAt < adrAt {
		t.Fatalf("RED: link precedes the ADR-0133 entry (adr at %d, link at %d)", adrAt, linkAt)
	}
	if next := strings.Index(idx[adrAt:], "\n## "); next >= 0 && linkAt > adrAt+next {
		t.Errorf("RED: link sits after the ADR-0133 section ends")
	}
	line := idx[strings.LastIndex(idx[:linkAt], "\n")+1:]
	line = line[:strings.Index(line, "\n")]
	if !strings.Contains(line, "v2.24.0") {
		t.Errorf("RED: the index line for the doc does not name v2.24.0: %q", line)
	}
}

func TestC1915_008_DocTiesVerdictToMeasuredBaseline(t *testing.T) {
	doc := strings.ToLower(readFile(t, researchDoc))
	for _, token := range []string{"10 cycles", "baseline", "m2", "m3", "d7", "d8"} {
		if !strings.Contains(doc, token) {
			t.Errorf("RED: doc does not tie the verdict to the %q rule of ADR-0133", token)
		}
	}
}

func TestC1915_009_DocAndIndexPassSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !strings.Contains(readFile(t, researchDoc), newestHeading) {
		t.Fatalf("RED: doc has no v2.24.0 section; lint of the unchanged doc is not meaningful")
	}
	for _, rel := range []string{researchDoc, researchIdx} {
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

func TestC1915_010_PracticewatchSuiteStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/practicewatch")
	if err != nil || code != 0 {
		t.Errorf("practicewatch tests rc=%d err=%v out=%s", code, err, out)
	}
}
