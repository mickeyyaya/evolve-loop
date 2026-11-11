//go:build acs

package cycle2275

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
	itemMarker   = "5a49deca"
	itemID       = "practice-agy-cli-changelog-command-5a49deca-202611100841"
	sourceName   = "agy changelog"
	snapshotMark = "2026-11-03T02:36:34Z"
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

var releaseNames = []string{"1.5.4", "1.5.3"}

var (
	releaseBullet = regexp.MustCompile(`^- .*1\.5\.4`)
	findingRow    = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	copiedLine    = regexp.MustCompile(`(?i)(lighter-thinking|windows credential manager|editors can switch a session)`)
	wordNo        = regexp.MustCompile(`\bno\b`)
)

func readRepo(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("RED: read %s: %v", rel, err)
	}
	return string(body)
}

func requireSteClean(t *testing.T, rel string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	findings, err := stelint.LintFile(filepath.Join(root, filepath.FromSlash(rel)), stelint.Options{Words: words})
	if err != nil {
		t.Fatalf("lint %s: %v", rel, err)
	}
	for _, finding := range findings {
		if !finding.IsSkipNotice() {
			t.Errorf("%s line %d %s: %s: %q", rel, finding.Line, finding.Rule, finding.Message, finding.Excerpt)
		}
	}
}

func releaseBullets(text string) []string {
	var found []string
	for _, line := range strings.Split(text, "\n") {
		if releaseBullet.MatchString(line) {
			found = append(found, line)
		}
	}
	return found
}

func proposalNote(t *testing.T) (rel, text string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	notes, err := acsassert.WalkModule(filepath.Join(root, "docs", "research"), func(name string) bool {
		if strings.Contains(name, "/") || !strings.HasSuffix(name, ".md") {
			return false
		}
		body, readErr := os.ReadFile(filepath.Join(root, "docs", "research", name))
		return readErr == nil && strings.Contains(string(body), itemMarker)
	})
	if err != nil {
		t.Fatalf("walk docs/research: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("RED: want exactly one note in docs/research that names %s, got %d", itemID, len(notes))
	}
	rel = "docs/research/" + notes[0]
	return rel, readRepo(t, rel)
}

func TestC2275_001_TheAgyNotesGainOneAgy154BulletThatSaysNoPromptChangeInSteEnglish(t *testing.T) {
	text := readRepo(t, agyNotesPath)
	bullets := releaseBullets(text)
	if len(bullets) != 1 {
		t.Fatalf("RED: want exactly one bullet that names 1.5.4 in %s, got %d", agyNotesPath, len(bullets))
	}
	lower := strings.ToLower(bullets[0])
	if !strings.Contains(bullets[0], "1.5.3") {
		t.Errorf("the 1.5.4 bullet does not also name 1.5.3: %q", bullets[0])
	}
	if !strings.Contains(lower, "prompt") || !wordNo.MatchString(lower) {
		t.Errorf("the 1.5.4 bullet does not say that no prompt changes: %q", bullets[0])
	}
	if hit := copiedLine.FindString(bullets[0]); hit != "" {
		t.Errorf("the 1.5.4 bullet copies changelog text: %q", hit)
	}
	requireSteClean(t, agyNotesPath)
}

func TestC2275_002_TheNotesKeepTheirFiveBulletsAndStayUnderTheLineCeiling(t *testing.T) {
	text := readRepo(t, agyNotesPath)
	if len(releaseBullets(text)) != 1 {
		t.Fatalf("RED: the notes have no single 1.5.4 bullet to check around")
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

func TestC2275_003_TheNoteReadsTheCommandSourceAndExplainsTheLinesAsUntrustedData(t *testing.T) {
	rel, text := proposalNote(t)
	root := acsassert.RepoRoot(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1)", rel, code)
	}
	for _, need := range append([]string{"`" + sourceName + "`", snapshotMark}, releaseNames...) {
		if !strings.Contains(text, need) {
			t.Errorf("the note does not name %s", need)
		}
	}
	if !strings.Contains(strings.ToLower(text), "untrusted data") {
		t.Errorf("the note does not say that the changelog text is untrusted data")
	}
	rows := findingRow.FindAllString(text, -1)
	if len(rows) < 4 {
		t.Fatalf("the note explains %d changelog lines in F rows, want at least 4", len(rows))
	}
	joined := strings.ToLower(strings.Join(rows, "\n"))
	for _, topic := range []string{"auto mode", "hook", "subagent", "sign-in"} {
		if !strings.Contains(joined, topic) {
			t.Errorf("no F row explains the %s lines of the changelog", topic)
		}
	}
	requireSteClean(t, rel)
}

func TestC2275_004_TheProposalCitesTheAdrNamesTheFileAndGivesEachCorePersonaANoChangeVerdictWithAReason(t *testing.T) {
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
		cells := strings.Split(strings.Trim(row, "| "), "|")
		if len(cells) < 3 || strings.TrimSpace(cells[len(cells)-1]) == "" {
			t.Errorf("the verdict row of %s has no reason: %q", persona, row)
			continue
		}
		if !strings.Contains(strings.ToLower(cells[1]), "no change") {
			t.Errorf("the verdict of %s is not a no-change verdict: %q", persona, cells[1])
		}
	}
	if !regexp.MustCompile(`(?mi)^.*\bdecision\b.*\bbecause\b.*$`).MatchString(text) {
		t.Errorf("no Decision line states its reason with because")
	}
}

func TestC2275_005_TheNoteIsInTheResearchIndexEntries(t *testing.T) {
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

func TestC2275_006_NoPersonaFileUnderAgentsChangesBecauseTheProposalChangesNone(t *testing.T) {
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
