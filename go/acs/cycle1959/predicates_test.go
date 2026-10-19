//go:build acs

package cycle1959

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	researchDoc   = "docs/research/practice-claude-code-changelog-2026-10.md"
	researchIdx   = "docs/research-index.md"
	adr0133Title  = "the-loop-watches-cli-practice-and-trims-its-scaffolding"
	newestHeading = "### Version 2.1.303"
	olderRelease  = "### Version 2.1.302"
	sourceURL     = "https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md"
	snapshotLine  = "Bug fixes and reliability improvements"
	snapshotHash  = "c6b0a656"
)

var corePersonas = []string{
	"evolve-scout", "evolve-triage", "evolve-tdd-engineer",
	"evolve-builder", "evolve-auditor", "evolve-retrospective",
}

var backtickSpan = regexp.MustCompile("`([^`]+)`")

var emphaticRule = regexp.MustCompile(`\b(MUST|NEVER|CRITICAL|ALWAYS)\b`)

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

func TestC1959_001_DocIsGitTrackedAndHasNewestSection(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: %s missing on disk", researchDoc)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", researchDoc)
	if code != 0 {
		t.Errorf("RED: %s not tracked by git", researchDoc)
	}
	if !strings.Contains(readFile(t, researchDoc), newestHeading+"\n") {
		t.Errorf("RED: doc has no %q section", newestHeading)
	}
}

func TestC1959_002_DocCitesTheSourceLinkAndSnapshotAsUntrustedData(t *testing.T) {
	doc := readFile(t, researchDoc)
	if !strings.Contains(doc, sourceURL) {
		t.Errorf("RED: doc does not cite the source link %s", sourceURL)
	}
	if !strings.Contains(doc, snapshotHash) {
		t.Errorf("RED: doc does not name the snapshot hash %s", snapshotHash)
	}
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: doc does not state the changelog text is untrusted data")
	}
	if !strings.Contains(strings.SplitN(doc, "\n## ", 2)[0], "2.1.303") {
		t.Errorf("RED: the doc summary does not name release 2.1.303 in its scope")
	}
}

func TestC1959_003_NewestSectionExplainsTheMaintenanceLineAsData(t *testing.T) {
	section := sectionAfter(t, readFile(t, researchDoc), newestHeading)
	if !strings.Contains(section, snapshotLine) {
		t.Errorf("RED: the 2.1.303 section does not quote the changelog line %q", snapshotLine)
	}
	lower := strings.ToLower(section)
	if !strings.Contains(lower, "no ") || !(strings.Contains(lower, "flag") || strings.Contains(lower, "command") || strings.Contains(lower, "behavior") || strings.Contains(lower, "behaviour")) {
		t.Errorf("RED: the 2.1.303 section does not state that the line names no flag, command or behavior change")
	}
}

func TestC1959_004_NewestSectionInventsNoIdentifierOrFeatureClaim(t *testing.T) {
	section := sectionAfter(t, readFile(t, researchDoc), newestHeading)
	if spans := backtickSpan.FindAllStringSubmatch(section, -1); len(spans) != 0 {
		t.Errorf("RED: the 2.1.303 section cites %d code identifiers, but the 2.1.303 changelog snapshot contains none: %v", len(spans), spans)
	}
	if strings.TrimSpace(section) == "" {
		t.Errorf("RED: the 2.1.303 section is empty")
	}
	if emphaticRule.MatchString(section) {
		t.Errorf("RED: the 2.1.303 section adds an emphatic capital rule")
	}
}

func TestC1959_005_DocKeepsOlderReleaseSections(t *testing.T) {
	doc := readFile(t, researchDoc)
	if !strings.Contains(doc, newestHeading+"\n") {
		t.Fatalf("RED: doc has no 2.1.303 section; preservation of the older sections is not meaningful")
	}
	for _, heading := range []string{"### Version 2.1.299", "### Version 2.1.300", "### Version 2.1.301", olderRelease} {
		if !strings.Contains(doc, heading+"\n") {
			t.Errorf("RED: doc drops the %q section", heading)
		}
	}
	for _, sourced := range []string{
		"$.prompt.media", "virtualText", "CLAUDE_CODE_RETRY_WATCHDOG_MAX_USAGE_LIMIT_WAIT_MS",
		"CLAUDE_CODE_AUTO_MODE_MODEL", "Idle compaction", "classifier_category",
	} {
		if !strings.Contains(doc, sourced) {
			t.Errorf("RED: doc drops sourced changelog item %q", sourced)
		}
	}
}

func TestC1959_006_EveryCorePersonaWeighsRelease2_1_303(t *testing.T) {
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
		if !strings.Contains(body, "2.1.303") {
			t.Errorf("RED: persona %s entry does not weigh the 2.1.303 release", p)
		}
	}
}

func TestC1959_007_NoPersonaFileChanges(t *testing.T) {
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

func TestC1959_008_ResearchIndexLinksDocOnceInsideADR0133SectionAndNamesNewRelease(t *testing.T) {
	idx := readFile(t, researchIdx)
	link := "](research/practice-claude-code-changelog-2026-10.md)"
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
	row := idx[linkAt:]
	if end := strings.Index(row, "\n"); end >= 0 {
		row = row[:end]
	}
	if !strings.Contains(row, "2.1.303") {
		t.Errorf("RED: the research-index row for the doc does not name release 2.1.303")
	}
}

func TestC1959_009_ConclusionTiesVerdictToMeasuredBaseline(t *testing.T) {
	doc := readFile(t, researchDoc)
	lower := strings.ToLower(doc)
	for _, token := range []string{"10-cycle", "baseline", "m2", "m3", "d7", "d8"} {
		if !strings.Contains(lower, token) {
			t.Errorf("RED: doc does not tie the verdict to the %q rule of ADR-0133", token)
		}
	}
	at := strings.LastIndex(doc, "\n## 4.")
	if at < 0 {
		t.Fatalf("RED: doc has no conclusion section")
	}
	conclusion := doc[at:]
	if !strings.Contains(conclusion, "2.1.303") {
		t.Errorf("RED: the conclusion does not state the scope through 2.1.303")
	}
	if !strings.Contains(strings.ToLower(conclusion), "no persona file changes") {
		t.Errorf("RED: the conclusion does not state that no persona file changes")
	}
}

func TestC1959_010_DocAddsNoEmphaticCapitalRule(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !strings.Contains(readFile(t, researchDoc), newestHeading+"\n") {
		t.Fatalf("RED: doc has no 2.1.303 section; the diff check is not meaningful")
	}
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "-U0", "origin/main", "--", researchDoc, researchIdx)
	if err != nil || code != 0 {
		t.Fatalf("git diff failed: code=%d err=%v", code, err)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") && emphaticRule.MatchString(line) {
			t.Errorf("RED: added line carries an emphatic capital rule: %q", line)
		}
	}
}

func TestC1959_011_DocAndIndexPassSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !strings.Contains(readFile(t, researchDoc), newestHeading) {
		t.Fatalf("RED: doc has no 2.1.303 section; lint of the unchanged doc is not meaningful")
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

func TestC1959_012_PracticewatchSuiteStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/practicewatch")
	if err != nil || code != 0 {
		t.Errorf("practicewatch tests rc=%d err=%v out=%s", code, err, out)
	}
}
