//go:build acs

package cycle2274

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	agyNotesPath = "skills/model-notes-agy/COMPACT.md"
	itemMarker   = "1b0c9251"
	itemID       = "practice-agy-cli-changelog-page-1b0c9251-202611100841"
	sourceURL    = "https://antigravity.google/docs/changelog?tab=cli"
	snapshotMark = "2026-11-04T00:37:48Z"
	adrNumber    = "ADR-0133"
	notesCeiling = 12
)

var originalBullets = []string{
	"- Put the task and the deliverable path first, then the constraints. Keep the prompt short and direct.",
	"- Name the exact output format that the deliverable needs, with one example.",
	"- Give one instruction for each step, and avoid long chains of nested conditions.",
	"- Run a check command and read its output before you claim done.",
	"- Start the deliverable skeleton yourself, as a response prefix, when the format is strict.",
}

var corePersonas = []string{
	"evolve-scout",
	"evolve-triage",
	"evolve-tdd-engineer",
	"evolve-builder",
	"evolve-auditor",
	"evolve-retrospective",
}

var releaseNames = []string{"v2.30.0", "v2.29.1", "v2.29.0"}

var (
	versionBullet = regexp.MustCompile(`^- .*v2\.30\.0`)
	findingRow    = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	copiedImport  = regexp.MustCompile(`(?i)(import your setup|ask the agent to|switch to a simpler)`)
)

func readRepo(t *testing.T, rel string) string {
	t.Helper()
	file, err := os.Open(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("RED: open %s: %v", rel, err)
	}
	defer file.Close()
	body, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("RED: read %s: %v", rel, err)
	}
	return string(body)
}

func requireSteClean(t *testing.T, label, text string) {
	t.Helper()
	words, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	var reports []string
	for _, finding := range stelint.Check([]byte(text), stelint.Options{Words: words}) {
		if finding.IsSkipNotice() {
			continue
		}
		reports = append(reports, fmt.Sprintf("line %d %s: %s: %q", finding.Line, finding.Rule, finding.Message, finding.Excerpt))
	}
	if len(reports) > 0 {
		t.Errorf("%s: %d STE findings:\n%s", label, len(reports), strings.Join(reports, "\n"))
	}
}

func versionBullets(text string) []string {
	var found []string
	for _, line := range strings.Split(text, "\n") {
		if versionBullet.MatchString(line) {
			found = append(found, line)
		}
	}
	return found
}

func proposalNote(t *testing.T) (rel, text string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(acsassert.RepoRoot(t), "docs", "research", "*.md"))
	if err != nil {
		t.Fatalf("glob docs/research: %v", err)
	}
	named := map[string]string{}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(body), itemMarker) {
			named[filepath.Base(path)] = string(body)
		}
	}
	if len(named) != 1 {
		t.Fatalf("RED: want exactly one note in docs/research that names %s, got %d", itemID, len(named))
	}
	for name, body := range named {
		rel, text = "docs/research/"+name, body
	}
	return rel, text
}

func TestC2274_001_TheAgyNotesGainOneV2300BulletThatSaysNoPromptChangeInSteEnglish(t *testing.T) {
	text := readRepo(t, agyNotesPath)
	bullets := versionBullets(text)
	if len(bullets) != 1 {
		t.Fatalf("RED: want exactly one bullet that names v2.30.0 in %s, got %d", agyNotesPath, len(bullets))
	}
	lower := strings.ToLower(bullets[0])
	if !strings.Contains(lower, "prompt") || !regexp.MustCompile(`\bno\b`).MatchString(lower) {
		t.Errorf("the v2.30.0 bullet does not say that no prompt changes: %q", bullets[0])
	}
	if hit := copiedImport.FindString(bullets[0]); hit != "" {
		t.Errorf("the v2.30.0 bullet copies changelog text: %q", hit)
	}
	requireSteClean(t, agyNotesPath, text)
}

func TestC2274_002_TheNotesKeepTheirFiveBulletsAndStayUnderTheLineCeiling(t *testing.T) {
	text := readRepo(t, agyNotesPath)
	if len(versionBullets(text)) != 1 {
		t.Fatalf("RED: the notes have no single v2.30.0 bullet to check around")
	}
	for _, bullet := range originalBullets {
		if n := strings.Count(text, bullet+"\n"); n != 1 {
			t.Errorf("the original bullet appears %d times, want 1: %q", n, bullet)
		}
	}
	if lines := len(strings.Split(strings.TrimRight(text, "\n"), "\n")); lines > notesCeiling {
		t.Errorf("%s has %d lines, the ceiling is %d", agyNotesPath, lines, notesCeiling)
	}
}

func TestC2274_003_TheNoteReadsTheSourceAtItsLinkAndExplainsTheLinesAsUntrustedData(t *testing.T) {
	rel, text := proposalNote(t)
	root := acsassert.RepoRoot(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1)", rel, code)
	}
	for _, need := range append([]string{sourceURL, snapshotMark}, releaseNames...) {
		if !strings.Contains(text, need) {
			t.Errorf("the note does not name %s", need)
		}
	}
	if !strings.Contains(strings.ToLower(text), "untrusted data") {
		t.Errorf("the note does not say that the page text is untrusted data")
	}
	rows := findingRow.FindAllString(text, -1)
	if len(rows) < 4 {
		t.Fatalf("the note explains %d page lines in F rows, want at least 4", len(rows))
	}
	joined := strings.ToLower(strings.Join(rows, "\n"))
	for _, topic := range []string{"knowledge work", "voice mode", "remote agents", "mcp"} {
		if !strings.Contains(joined, topic) {
			t.Errorf("no F row explains the %s line of the page", topic)
		}
	}
	requireSteClean(t, rel, text)
}

func TestC2274_004_TheProposalCitesTheAdrNamesTheFileAndGivesEachCorePersonaAVerdictWithAReason(t *testing.T) {
	_, text := proposalNote(t)
	if !strings.Contains(text, adrNumber) {
		t.Errorf("the note does not cite %s", adrNumber)
	}
	if !strings.Contains(text, agyNotesPath) {
		t.Errorf("the note does not name the file that changes: %s", agyNotesPath)
	}
	for _, persona := range corePersonas {
		verdict := regexp.MustCompile(`(?m)^\|[^\n]*` + regexp.QuoteMeta(persona) + `[^\n]*\|[^\n]*change[^\n]*\|`)
		row := verdict.FindString(text)
		if row == "" {
			t.Errorf("no table row gives a verdict for %s", persona)
			continue
		}
		if cells := strings.Split(strings.Trim(row, "| "), "|"); len(cells) < 3 || strings.TrimSpace(cells[len(cells)-1]) == "" {
			t.Errorf("the verdict row of %s has no reason: %q", persona, row)
		}
	}
	if !regexp.MustCompile(`(?mi)^.*\bdecision\b.*\bbecause\b.*$`).MatchString(text) {
		t.Errorf("no Decision line states its reason with because")
	}
}

func TestC2274_005_TheNoteIsInTheResearchIndexEntries(t *testing.T) {
	rel, _ := proposalNote(t)
	indexDir := filepath.Join(acsassert.RepoRoot(t), "docs", "research-index")
	entries, err := os.ReadDir(indexDir)
	if err != nil {
		t.Fatalf("read docs/research-index: %v", err)
	}
	link := "(" + strings.TrimPrefix(rel, "docs/") + ")"
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(indexDir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if strings.Contains(string(raw), link) {
			return
		}
	}
	t.Errorf("RED: no entry file in docs/research-index links %s", rel)
}

func TestC2274_006_NoPersonaFileUnderAgentsChangesBecauseTheProposalChangesNone(t *testing.T) {
	_, text := proposalNote(t)
	if !regexp.MustCompile(`(?i)(no persona|change no persona|no prompt)`).MatchString(text) {
		t.Fatalf("RED: the note does not state that no persona or prompt changes")
	}
	root := acsassert.RepoRoot(t)
	base := "origin/main"
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "rev-parse", "--verify", "--quiet", base); code != 0 {
		base = "HEAD"
	}
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", base, "--", "agents")
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("persona files changed although the proposal changes none:\n%s", out)
	}
	untracked, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "agents")
	if strings.TrimSpace(untracked) != "" {
		t.Errorf("untracked files appeared under agents:\n%s", untracked)
	}
}
