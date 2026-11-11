//go:build acs

package cycle2286

import (
	"fmt"
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
	itemMarker   = "297686f7"
	itemID       = "practice-gemini-prompting-guide-297686f7-202611110139"
	snapshotRef  = "2026-11-10T08:41:14Z"
	sourceURL    = "https://ai.google.dev/gemini-api/docs/prompting-strategies"
	adrNumber    = "ADR-0133"
	notesCeiling = 13
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

var (
	findingRow      = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	httpOutcome     = regexp.MustCompile(`HTTP \d{3}`)
	noAddedLine     = regexp.MustCompile(`(?i)no added line`)
	changedLineGone = regexp.MustCompile(`(?i)(changed line|exact line|which line)[^.\n]{0,80}(unknown|not known|cannot|can not)`)
	decisionBecause = regexp.MustCompile(`(?mi)^.*\bdecision\b.*\bbecause\b.*$`)
	placeholder     = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	rejectedOption  = regexp.MustCompile(`(?mi)^Rejected option`)
	notesVerdict    = regexp.MustCompile(`(?i)COMPACT\.md[^\n]*\b(no change|unchanged|stays|changes|adds?|updated?)\b`)
	proposalFields  = []string{"goal", "done criteria", "limit", "before", "after"}
)

func steFindings(t *testing.T, rel string) []string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	found, err := stelint.LintFile(filepath.Join(root, filepath.FromSlash(rel)), stelint.Options{Words: words})
	if err != nil {
		t.Fatalf("lint %s: %v", rel, err)
	}
	var lines []string
	for _, finding := range found {
		if !finding.IsSkipNotice() {
			lines = append(lines, fmt.Sprintf("%s line %d %s: %q", rel, finding.Line, finding.Rule, finding.Excerpt))
		}
	}
	return lines
}

func proposalNote(t *testing.T) (rel, text string) {
	t.Helper()
	files, err := acsassert.WalkModule(filepath.Join(acsassert.RepoRoot(t), "docs", "research"), func(name string) bool {
		return !strings.Contains(name, "/") && strings.HasSuffix(name, ".md")
	})
	if err != nil {
		t.Fatalf("walk docs/research: %v", err)
	}
	var named []string
	for _, name := range files {
		body, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "docs", "research", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(body), itemMarker) {
			named = append(named, name)
			text = string(body)
		}
	}
	if len(named) != 1 {
		t.Fatalf("RED: want exactly one note in docs/research that names %s, got %d: %v", itemID, len(named), named)
	}
	return "docs/research/" + named[0], text
}

func personaRow(text, persona string) string {
	return regexp.MustCompile(`(?m)^\|[^\n]*` + regexp.QuoteMeta(persona) + `[^\n]*\|[^\n]*\|[^\n]*\|`).FindString(text)
}

func rowCells(row string) []string {
	var cells []string
	for _, cell := range strings.Split(strings.Trim(row, "| "), "|") {
		cells = append(cells, strings.TrimSpace(cell))
	}
	return cells
}

func changedPersonaFiles(t *testing.T) []string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain", "--", "agents")
	if err != nil || code != 0 {
		t.Fatalf("git status exit=%d: %v", code, err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line != "" {
			files = append(files, strings.TrimSpace(line[2:]))
		}
	}
	return files
}

func TestC2286_001_TheNoteReadsTheSourceAtItsLinkAndExplainsTheItemLinesAsUntrustedData(t *testing.T) {
	rel, text := proposalNote(t)
	root := acsassert.RepoRoot(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1)", rel, code)
	}
	for _, want := range []string{sourceURL, itemID, snapshotRef} {
		if !strings.Contains(text, want) {
			t.Errorf("the note does not cite %q", want)
		}
	}
	if !strings.Contains(strings.ToLower(text), "untrusted data") {
		t.Errorf("the note does not say that the guide and item text are untrusted data")
	}
	if !httpOutcome.MatchString(text) {
		t.Errorf("the note does not record the HTTP outcome of the read at the link")
	}
	if !noAddedLine.MatchString(text) {
		t.Errorf("the note does not explain the item lines: the stored excerpt shows no added line")
	}
	if !changedLineGone.MatchString(text) {
		t.Errorf("the note does not state that the changed line is unknown")
	}
}

func TestC2286_002_TheNoteExplainsAtLeastThreeGuideLinesInFindingRows(t *testing.T) {
	_, text := proposalNote(t)
	rows := findingRow.FindAllString(text, -1)
	if len(rows) < 3 {
		t.Fatalf("RED: the note explains %d guide lines in F rows, want at least 3", len(rows))
	}
	for _, row := range rows {
		cells := rowCells(row)
		if len(cells) < 3 {
			t.Errorf("the F row has fewer than 3 cells: %q", row)
			continue
		}
		for _, cell := range cells {
			if cell == "" {
				t.Errorf("the F row has an empty cell: %q", row)
			}
		}
	}
}

func TestC2286_003_TheNoteCitesAdr0133AndGivesAVerdictWithAReasonForEveryCorePersona(t *testing.T) {
	_, text := proposalNote(t)
	if !strings.Contains(text, adrNumber) {
		t.Fatalf("RED: the note does not cite %s", adrNumber)
	}
	for _, persona := range corePersonas {
		row := personaRow(text, persona)
		if row == "" {
			t.Errorf("no table row gives a verdict for %s", persona)
			continue
		}
		cells := rowCells(row)
		if len(cells) < 3 {
			t.Errorf("the verdict row of %s has fewer than 3 cells: %q", persona, row)
			continue
		}
		if !strings.Contains(strings.ToLower(cells[1]), "change") {
			t.Errorf("the verdict cell of %s says neither change nor no change: %q", persona, cells[1])
		}
		if cells[len(cells)-1] == "" {
			t.Errorf("the verdict row of %s has no reason: %q", persona, row)
		}
	}
	if !decisionBecause.MatchString(text) {
		t.Errorf("no Decision line states its reason with because")
	}
	if !rejectedOption.MatchString(text) {
		t.Errorf("the note names no rejected option with its ruling fact")
	}
}

func TestC2286_004_TheNoteStatesWhetherTheAgyNotesFileChanges(t *testing.T) {
	_, text := proposalNote(t)
	if !strings.Contains(text, agyNotesPath) {
		t.Fatalf("RED: the note does not name %s", agyNotesPath)
	}
	if !notesVerdict.MatchString(text) {
		t.Errorf("the note does not say whether %s changes", agyNotesPath)
	}
}

func TestC2286_005_AProposedPersonaChangeStatesGoalDoneCriteriaLimitsAndBeforeAfterMeasures(t *testing.T) {
	_, text := proposalNote(t)
	lower := strings.ToLower(text)
	proposed := false
	for _, persona := range corePersonas {
		row := personaRow(text, persona)
		if row == "" {
			t.Fatalf("RED: no verdict row for %s", persona)
		}
		verdict := strings.ToLower(rowCells(row)[1])
		if strings.Contains(verdict, "change") && !strings.Contains(verdict, "no change") {
			proposed = true
		}
	}
	if !proposed {
		return
	}
	for _, field := range proposalFields {
		if !strings.Contains(lower, field) {
			t.Errorf("a persona change is proposed but the note lacks %q (ADR-0133 fix text)", field)
		}
	}
}

func TestC2286_006_ThePersonaFilesMatchTheVerdictTableOfTheNote(t *testing.T) {
	_, text := proposalNote(t)
	changed := changedPersonaFiles(t)
	for _, persona := range corePersonas {
		row := personaRow(text, persona)
		if row == "" {
			t.Fatalf("RED: no verdict row for %s", persona)
		}
		verdict := strings.ToLower(rowCells(row)[1])
		claimsChange := strings.Contains(verdict, "change") && !strings.Contains(verdict, "no change")
		fileChanged := false
		for _, file := range changed {
			if strings.HasSuffix(file, "agents/"+persona+".md") {
				fileChanged = true
			}
		}
		if claimsChange != fileChanged {
			t.Errorf("%s: the note says change=%v but the persona file changed=%v", persona, claimsChange, fileChanged)
		}
	}
	for _, file := range changed {
		base := strings.TrimSuffix(filepath.Base(file), ".md")
		known := false
		for _, persona := range corePersonas {
			if base == persona {
				known = true
			}
		}
		if !known {
			t.Errorf("an agents/ file outside the six core personas changed: %s", file)
		}
	}
}

func TestC2286_007_TheNoteHasNoPlaceholderTextAndPassesTheSentenceLint(t *testing.T) {
	rel, text := proposalNote(t)
	if hit := placeholder.FindString(text); hit != "" {
		t.Errorf("the note holds placeholder text: %q", hit)
	}
	for _, line := range steFindings(t, rel) {
		t.Errorf("STE finding: %s", line)
	}
}

func TestC2286_008_TheNoteIsInTheResearchIndexEntries(t *testing.T) {
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

func TestC2286_009_TheAgyNotesKeepTheirBulletsStaySteCleanAndStayUnderTheCeiling(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(agyNotesPath)))
	if err != nil {
		t.Fatalf("RED: read %s: %v", agyNotesPath, err)
	}
	text := string(raw)
	for _, bullet := range originalBullets {
		if n := strings.Count(text, bullet+"\n"); n != 1 {
			t.Errorf("the bullet appears %d times, want 1: %q", n, bullet)
		}
	}
	if lines := len(strings.Split(strings.TrimRight(text, "\n"), "\n")); lines > notesCeiling {
		t.Errorf("%s has %d lines, the ceiling is %d", agyNotesPath, lines, notesCeiling)
	}
	for _, line := range steFindings(t, agyNotesPath) {
		t.Errorf("STE finding: %s", line)
	}
}
