//go:build acs

package cycle2272

import (
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
	itemMarker   = "1945a6f2"
	itemID       = "practice-gemini-prompting-guide-1945a6f2-202611100841"
	sourceURL    = "https://ai.google.dev/gemini-api/docs/prompting-strategies"
	adrNumber    = "ADR-0133"
	notesCeiling = 12
)

var originalBullets = []string{
	"- Put the task and the deliverable path first, then the constraints. Keep the prompt short and direct.",
	"- Name the exact output format that the deliverable needs, with one example.",
	"- Give one instruction for each step, and avoid long chains of nested conditions.",
	"- Run a check command and read its output before you claim done.",
}

var corePersonas = []string{
	"evolve-scout",
	"evolve-triage",
	"evolve-tdd-engineer",
	"evolve-builder",
	"evolve-auditor",
	"evolve-retrospective",
}

var (
	responsePrefixBullet = regexp.MustCompile(`(?i)^- .*response prefix`)
	blanketFewShot       = regexp.MustCompile(`(?i)(\balways\b[^\n]*\bexamples?\b|few-shot[^\n]*\b(always|every)\b|\bevery prompt\b)`)
	findingRow           = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
)

func repoText(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("RED: read %s: %v", rel, err)
	}
	return string(raw)
}

func bulletsMatching(text string, re *regexp.Regexp) []string {
	var found []string
	for _, line := range strings.Split(text, "\n") {
		if re.MatchString(line) {
			found = append(found, line)
		}
	}
	return found
}

func requireCleanSte(t *testing.T, label, text string) {
	t.Helper()
	words, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for _, finding := range stelint.Check([]byte(text), stelint.Options{Words: words}) {
		if !finding.IsSkipNotice() {
			t.Errorf("%s: STE finding %s at line %d: %s: %q", label, finding.Rule, finding.Line, finding.Message, finding.Excerpt)
		}
	}
}

func proposalNote(t *testing.T) (rel, text string) {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "docs", "research")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read docs/research: %v", err)
	}
	var matches []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if strings.Contains(string(raw), itemMarker) {
			matches = append(matches, entry.Name())
			text = string(raw)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("RED: want exactly one note in docs/research that names %s, got %d: %v", itemID, len(matches), matches)
	}
	return "docs/research/" + matches[0], text
}

func TestC2272_001_TheAgyNotesGainOneResponsePrefixBulletInSteEnglish(t *testing.T) {
	text := repoText(t, agyNotesPath)
	bullets := bulletsMatching(text, responsePrefixBullet)
	if len(bullets) != 1 {
		t.Fatalf("RED: want exactly one response prefix bullet in %s, got %d", agyNotesPath, len(bullets))
	}
	for _, word := range []string{"skeleton", "deliverable", "format"} {
		if !strings.Contains(strings.ToLower(bullets[0]), word) {
			t.Errorf("the response prefix bullet does not say %q: %q", word, bullets[0])
		}
	}
	requireCleanSte(t, agyNotesPath, text)
}

func TestC2272_002_TheNotesKeepTheirFourBulletsAndGainNoBlanketFewShotRuleOrLength(t *testing.T) {
	text := repoText(t, agyNotesPath)
	if len(bulletsMatching(text, responsePrefixBullet)) != 1 {
		t.Fatalf("RED: the notes have no single response prefix bullet to check around")
	}
	for _, bullet := range originalBullets {
		if n := strings.Count(text, bullet+"\n"); n != 1 {
			t.Errorf("the original bullet appears %d times, want 1: %q", n, bullet)
		}
	}
	if lines := len(strings.Split(strings.TrimRight(text, "\n"), "\n")); lines > notesCeiling {
		t.Errorf("%s has %d lines, the ceiling is %d", agyNotesPath, lines, notesCeiling)
	}
	if hit := blanketFewShot.FindString(text); hit != "" {
		t.Errorf("the notes carry a blanket few-shot rule: %q", hit)
	}
}

func TestC2272_003_TheProposalNoteReadsTheSourceAndExplainsTheLinesAsUntrustedData(t *testing.T) {
	rel, text := proposalNote(t)
	root := acsassert.RepoRoot(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1 for not ignored)", rel, code)
	}
	if !strings.Contains(text, sourceURL) {
		t.Errorf("the note does not give the source link %s", sourceURL)
	}
	if !strings.Contains(strings.ToLower(text), "untrusted data") {
		t.Errorf("the note does not say that the guide text is untrusted data")
	}
	rows := findingRow.FindAllString(text, -1)
	if len(rows) < 3 {
		t.Fatalf("the note explains %d guide lines in F rows, want at least 3", len(rows))
	}
	joined := strings.ToLower(strings.Join(rows, "\n"))
	for _, topic := range []string{"completion", "few-shot"} {
		if !strings.Contains(joined, topic) {
			t.Errorf("no F row explains the %s line of the guide", topic)
		}
	}
	requireCleanSte(t, rel, text)
}

func TestC2272_004_TheProposalNamesTheChangedFileAndGivesAPersonaVerdictWithAReason(t *testing.T) {
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

func TestC2272_005_TheProposalNoteIsInTheResearchIndexEntries(t *testing.T) {
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
