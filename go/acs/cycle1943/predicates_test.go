//go:build acs

package cycle1943

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

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", rel, err)
	}
	return string(b)
}

func steLint(t *testing.T, rel string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, rel)
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
		t.Errorf("ste-lint rc=%d out=%q stderr=%q", code, out, errOut)
	}
}

func TestC1943_001_DocIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, researchDoc)) {
		t.Fatalf("RED: %s missing on disk", researchDoc)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", researchDoc)
	if code != 0 {
		t.Errorf("RED: %s not tracked by git (gitignored files drop at ship)", researchDoc)
	}
}

func TestC1943_002_NewVersionsExplainedAsUntrustedData(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	for _, v := range []string{"1.4.1", "1.4.2"} {
		if !strings.Contains(doc, v) {
			t.Errorf("RED: doc does not cover agy changelog version %s", v)
		}
	}
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: doc does not state the changelog text is untrusted data")
	}
}

func TestC1943_003_DocCoversEachNewReleaseItem(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	for _, token := range []string{
		"plugin validate", "24 KB", "AGY_ADC_AUTH",
		"/title", "/remote-control", "Remote Control", "broken pipe",
	} {
		if !strings.Contains(doc, token) {
			t.Errorf("RED: doc does not explain the changelog item %q", token)
		}
	}
}

func TestC1943_004_EarlierVersionsStayCovered(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	for _, v := range []string{"1.3.4", "1.3.5", "1.3.6", "1.4.0"} {
		if !strings.Contains(doc, v) {
			t.Errorf("RED: doc lost coverage of agy changelog version %s", v)
		}
	}
	if !strings.Contains(doc, "1.4.1") {
		t.Errorf("RED: doc has no 1.4.1 coverage beside the earlier versions")
	}
}

func TestC1943_005_EveryCorePersonaHasVerdictInEntry(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if !strings.Contains(doc, "ADR-0133") {
		t.Errorf("RED: doc does not cite ADR-0133")
	}
	if !strings.Contains(doc, "1.4.2") {
		t.Fatalf("RED: doc is not refreshed for 1.4.2; persona verdicts are stale")
	}
	for _, p := range corePersonas {
		idx := strings.Index(doc, "### "+p)
		if idx < 0 {
			t.Errorf("RED: persona %s has no entry heading", p)
			continue
		}
		window := strings.ToLower(doc[idx:min(len(doc), idx+600)])
		if !strings.Contains(window, "does not change") && !strings.Contains(window, "no change") {
			t.Errorf("RED: persona %s has no stated no-change verdict in its entry", p)
		}
	}
}

func TestC1943_006_NoPersonaFileChanges(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !strings.Contains(readRepoFile(t, researchDoc), "1.4.2") {
		t.Fatalf("RED: deliverable not refreshed; cannot claim a no-persona-change cycle")
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

func TestC1943_007_DocStatesAdoptionNeedsMeasuredBaseline(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if !strings.Contains(doc, "1.4.2") {
		t.Fatalf("RED: doc not refreshed for 1.4.2")
	}
	lower := strings.ToLower(doc)
	for _, token := range []string{"10-cycle", "baseline", "m2", "m3"} {
		if !strings.Contains(lower, token) {
			t.Errorf("RED: doc does not tie the verdict to the %q measurement rule of ADR-0133", token)
		}
	}
}

func TestC1943_008_ResearchIndexLinksDocUnderADR0133(t *testing.T) {
	idx := readRepoFile(t, researchIdx)
	adrAt := strings.Index(idx, adr0133Title)
	linkAt := strings.Index(idx, "](research/practice-agy-cli-changelog-2026-10.md)")
	if linkAt < 0 || adrAt < 0 || linkAt < adrAt {
		t.Fatalf("RED: index does not link the doc inside the ADR-0133 section (adr %d, link %d)", adrAt, linkAt)
	}
	if next := strings.Index(idx[adrAt:], "\n## "); next >= 0 && linkAt > adrAt+next {
		t.Errorf("RED: link sits after the ADR-0133 section ends")
	}
	line := idx[linkAt:]
	if nl := strings.Index(line, "\n"); nl >= 0 {
		line = line[:nl]
	}
	if !strings.Contains(line, "1.4.2") {
		t.Errorf("RED: index entry does not name the refreshed range through 1.4.2: %q", line)
	}
}

func TestC1943_009_DocPassesSteLint(t *testing.T) {
	if !strings.Contains(readRepoFile(t, researchDoc), "1.4.2") {
		t.Fatalf("RED: doc not refreshed; lint of the stale doc proves nothing")
	}
	steLint(t, researchDoc)
}

func TestC1943_010_IndexPassesSteLint(t *testing.T) {
	if !strings.Contains(readRepoFile(t, researchIdx), "through 1.4.2") {
		t.Fatalf("RED: index entry not refreshed; lint proves nothing")
	}
	steLint(t, researchIdx)
}

func TestC1943_011_PracticewatchSuiteStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !strings.Contains(readRepoFile(t, researchDoc), "1.4.2") {
		t.Fatalf("RED: deliverable not refreshed")
	}
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/practicewatch")
	if err != nil || code != 0 {
		t.Errorf("practicewatch tests rc=%d err=%v out=%s", code, err, out)
	}
}

var source141BacktickTokens = []string{
	";", "/config", "/diff", "/mcp", "/remote-control Off", "/remote-control", "/title ON", "/title",
	"Agent execution terminated due to error.", "agent.json", "AGENTS.md", "agents/", "agy plugin validate",
	"AGY_ADC_AUTH", "broken pipe", "client is closing: EOF", "description", "Enable Telemetry", "Error",
	"GEMINI.md", "glob", "globs", "http2: client connection lost", "model_decision", "name",
	"network is down", "off (disabled)", "off", "on", "README.md", "Ready", "rules/", "SKILL.md",
	"skills/", "software caused connection abort", "Terminal", "trigger",
}

func versionSection(t *testing.T, doc, version string) string {
	t.Helper()
	heading := "### Version " + version + "\n"
	start := strings.Index(doc, heading)
	if start < 0 {
		t.Fatalf("RED: doc has no %q heading", strings.TrimSpace(heading))
	}
	body := doc[start+len(heading):]
	if end := strings.Index(body, "\n#"); end >= 0 {
		body = body[:end]
	}
	return body
}

func backtickTokens(text string) []string {
	parts := strings.Split(text, "`")
	tokens := []string{}
	for i := 1; i < len(parts); i += 2 {
		tokens = append(tokens, parts[i])
	}
	return tokens
}

func isInSource(token string, source []string) bool {
	for _, s := range source {
		if strings.Contains(s, token) {
			return true
		}
	}
	return false
}

func TestC1943_012_Version141SectionCitesOnly141Items(t *testing.T) {
	section := versionSection(t, readRepoFile(t, researchDoc), "1.4.1")
	if strings.Contains(section, "print-timeout") {
		t.Errorf("RED: 1.4.1 section attributes the --print-timeout change to 1.4.1; the changelog lists it under 1.1.28")
	}
	for _, token := range backtickTokens(section) {
		if !isInSource(token, source141BacktickTokens) {
			t.Errorf("RED: 1.4.1 section cites %q, which the 1.4.1 changelog entry does not contain", token)
		}
	}
	for _, item := range []string{"plugin validate", "24 KB", "AGY_ADC_AUTH", "broken pipe", "/title", "/remote-control"} {
		if !strings.Contains(section, item) {
			t.Errorf("RED: 1.4.1 section dropped the real 1.4.1 item %q", item)
		}
	}
}

func TestC1943_013_Version142SectionClaimsOnlyTheHistoryFix(t *testing.T) {
	section := versionSection(t, readRepoFile(t, researchDoc), "1.4.2")
	lower := strings.ToLower(section)
	for _, invented := range []string{"replay", "reconnect", "event"} {
		if strings.Contains(lower, invented) {
			t.Errorf("RED: 1.4.2 section claims %q; the 1.4.2 entry only fixes conversation history in Remote Control daemon mode", invented)
		}
	}
	for _, fact := range []string{"conversation history", "remote control", "daemon"} {
		if !strings.Contains(lower, fact) {
			t.Errorf("RED: 1.4.2 section does not state the real fix item %q", fact)
		}
	}
	if tokens := backtickTokens(section); len(tokens) != 0 {
		t.Errorf("RED: 1.4.2 section cites %q; the 1.4.2 entry has no code tokens", tokens)
	}
}

func TestC1943_014_FaithfulnessCheckRejectsTheAuditedMisattributions(t *testing.T) {
	misattributedDoc := "### Version 1.4.1\nThe tool preserves partial output when `--print-timeout` triggers.\n\n### Version 1.4.2\nThe fix ensures full event replay across reconnections.\n"
	if isInSource("--print-timeout", source141BacktickTokens) {
		t.Fatalf("source141BacktickTokens wrongly holds --print-timeout")
	}
	if tokens := backtickTokens(versionSection(t, misattributedDoc, "1.4.1")); len(tokens) != 1 || tokens[0] != "--print-timeout" {
		t.Errorf("backtickTokens missed the misattributed token: %q", tokens)
	}
	if !strings.Contains(strings.ToLower(versionSection(t, misattributedDoc, "1.4.2")), "replay") {
		t.Errorf("versionSection did not isolate the 1.4.2 body")
	}
	if !isInSource("plugin validate", source141BacktickTokens) {
		t.Errorf("isInSource rejects a real 1.4.1 token")
	}
}
