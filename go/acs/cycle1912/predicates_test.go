//go:build acs

package cycle1912

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
	newestHeading = "### Version 2.1.302"
)

var corePersonas = []string{
	"evolve-scout", "evolve-triage", "evolve-tdd-engineer",
	"evolve-builder", "evolve-auditor", "evolve-retrospective",
}

var release2_1_302SnapshotIdentifiers = map[string]bool{
	"--permission-mode auto": true, "--plugin-dir": true, ".git": true, ".tmp": true,
	"/autocompact": true, "/config": true, "/design consent": true, "/design revoke": true,
	"/design-login": true, "/design-sync": true, "/effort": true, "/feedback": true,
	"/mcp": true, "/plugin": true, "@browser": true, "CLAUDE_CODE_AUTO_MODE_MODEL": true,
	"CLAUDE_CODE_FORK_SUBAGENT=0": true, "Idle compaction": true,
	"OTEL_LOGRECORD_ATTRIBUTE_COUNT_LIMIT": true, "Select-Object": true,
	"anthropic/claude-opus-4-1": true, "bin/": true, "cacheScope": true,
	"classifier_category": true, "claude -p": true, "claude mcp serve": true,
	"claude plugin": true, "claude plugin details <name>@builtin": true,
	"claude plugin list": true, "claude remote-control --help": true, "default": true,
	"resultType": true, `subagent_type: "fork"`: true, "tool_decision": true,
	"tool_parameters": true, "tool_source": true, "tool_use_id": true, "ttlMs": true,
	"Read": true, "ToolSearch": true, "PowerShell": true, "rg": true, "grep": true,
	"findstr": true, "git grep": true, "fork": true, "Prompt is too long": true,
	"diagnostics: Extra inputs are not permitted": true,
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

func TestC1912_001_DocIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: %s missing on disk", researchDoc)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", researchDoc)
	if code != 0 {
		t.Errorf("RED: %s not tracked by git", researchDoc)
	}
}

func TestC1912_002_DocExplainsRelease2_1_302AsData(t *testing.T) {
	doc := readFile(t, researchDoc)
	section := sectionAfter(t, doc, newestHeading)
	for _, item := range []string{
		"CLAUDE_CODE_AUTO_MODE_MODEL",
		"Idle compaction",
		"128 MB",
		"reftable",
		"classifier_category",
		"Prompt is too long",
	} {
		if !strings.Contains(section, item) {
			t.Errorf("RED: the 2.1.302 section does not explain changelog item %q", item)
		}
	}
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: doc does not state the changelog text is untrusted data")
	}
	if !strings.Contains(strings.SplitN(doc, "\n## ", 2)[0], "2.1.302") {
		t.Errorf("RED: the doc summary does not name release 2.1.302 in its scope")
	}
}

func TestC1912_003_DocKeepsSourcedOlderReleaseItems(t *testing.T) {
	doc := readFile(t, researchDoc)
	for _, heading := range []string{"### Version 2.1.299", "### Version 2.1.300", "### Version 2.1.301"} {
		if !strings.Contains(doc, heading+"\n") {
			t.Errorf("RED: doc drops the %q section", heading)
		}
	}
	for _, sourced := range []string{
		"$.prompt.media",
		"$.session.identity()",
		"virtualText",
		"prompt.change",
		"CLAUDE_CODE_RETRY_WATCHDOG_MAX_USAGE_LIMIT_WAIT_MS",
		"tracing guidance",
	} {
		if !strings.Contains(doc, sourced) {
			t.Errorf("RED: doc drops changelog item %q that the snapshot lists", sourced)
		}
	}
}

func TestC1912_004_NewestSectionCitesOnlySnapshotIdentifiers(t *testing.T) {
	section := sectionAfter(t, readFile(t, researchDoc), newestHeading)
	spans := backtickSpan.FindAllStringSubmatch(section, -1)
	if len(spans) < 3 {
		t.Fatalf("RED: the 2.1.302 section cites %d code identifiers, want at least 3", len(spans))
	}
	for _, m := range spans {
		if !release2_1_302SnapshotIdentifiers[m[1]] {
			t.Errorf("RED: the 2.1.302 section cites %q, which the 2.1.302 changelog snapshot does not contain", m[1])
		}
	}
	if strings.Contains(strings.ToLower(section), "subshell") {
		t.Errorf("RED: the 2.1.302 section claims a subshell change, which no changelog line states")
	}
}

func TestC1912_005_EveryCorePersonaHasReasonedNoChangeVerdict(t *testing.T) {
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
		if !strings.Contains(body, "2.1.302") {
			t.Errorf("RED: persona %s entry does not weigh the 2.1.302 release", p)
		}
	}
}

func TestC1912_006_NoPersonaFileChanges(t *testing.T) {
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

func TestC1912_007_ResearchIndexLinksDocInsideADR0133Section(t *testing.T) {
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
}

func TestC1912_008_DocTiesVerdictToMeasuredBaseline(t *testing.T) {
	doc := strings.ToLower(readFile(t, researchDoc))
	for _, token := range []string{"10-cycle", "baseline", "m2", "m3", "d7", "d8"} {
		if !strings.Contains(doc, token) {
			t.Errorf("RED: doc does not tie the verdict to the %q rule of ADR-0133", token)
		}
	}
}

func TestC1912_009_DocAndIndexPassSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !strings.Contains(readFile(t, researchDoc), newestHeading) {
		t.Fatalf("RED: doc has no 2.1.302 section; lint of the unchanged doc is not meaningful")
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

func TestC1912_010_PracticewatchSuiteStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/practicewatch")
	if err != nil || code != 0 {
		t.Errorf("practicewatch tests rc=%d err=%v out=%s", code, err, out)
	}
}
