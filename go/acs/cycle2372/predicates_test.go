//go:build acs

package cycle2372

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	itemMarker    = "526431cf"
	itemID        = "practice-claude-prompting-guide-526431cf-202611131947"
	snapshotRef   = "2026-11-13T00:01:22Z"
	sourceURL     = "https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices"
	adrNumber     = "ADR-0133"
	maxQuoteWords = 25
)

var corePersonas = []string{
	"evolve-scout",
	"evolve-triage",
	"evolve-tdd-engineer",
	"evolve-builder",
	"evolve-auditor",
	"evolve-retrospective",
}

var (
	guideRow      = regexp.MustCompile(`(?m)^\| H\d+ \|.*$`)
	quotedFrag    = regexp.MustCompile(`"([^"\n]{8,})"`)
	markerWord    = regexp.MustCompile(`\b(CRITICAL|MUST)\b`)
	placeholder   = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	becauseLine   = regexp.MustCompile(`(?mi)^.*\bdecision\b.*\bbecause\b.*$`)
	guessWord     = regexp.MustCompile(`(?i)\bguess`)
	verdictVocab  = regexp.MustCompile(`^(change now|change later|no change)$`)
	sectionHeader = regexp.MustCompile(`(?m)^## .*$`)
)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func itemNote(t *testing.T) (rel, text string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	names, err := acsassert.WalkModule(filepath.Join(root, "docs", "research"), func(r string) bool {
		return !strings.Contains(r, "/") && strings.HasSuffix(r, ".md")
	})
	if err != nil {
		t.Fatalf("walk docs/research: %v", err)
	}
	var holders []string
	for _, name := range names {
		if strings.Contains(readRepoFile(t, "docs/research/"+name), itemMarker) {
			holders = append(holders, "docs/research/"+name)
		}
	}
	if len(holders) != 1 {
		t.Fatalf("RED: want exactly one note in docs/research that names %s, got %v", itemID, holders)
	}
	return holders[0], readRepoFile(t, holders[0])
}

func itemSection(t *testing.T) string {
	t.Helper()
	_, text := itemNote(t)
	headers := sectionHeader.FindAllStringIndex(text, -1)
	for i, h := range headers {
		if !strings.Contains(text[h[0]:h[1]], itemMarker) {
			continue
		}
		if i+1 < len(headers) {
			return text[h[0]:headers[i+1][0]]
		}
		return text[h[0]:]
	}
	t.Fatalf("RED: no level-2 heading in the note names %s", itemMarker)
	return ""
}

func tableCells(row string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func personaRow(section, persona string) []string {
	row := regexp.MustCompile("(?m)^\\|[^\\n]*`" + regexp.QuoteMeta(persona) + "`[^\\n]*$").FindString(section)
	if row == "" {
		return nil
	}
	return tableCells(row)
}

func markersIn(t *testing.T, persona string) int {
	t.Helper()
	return len(markerWord.FindAllString(readRepoFile(t, "agents/"+persona+".md"), -1))
}

func TestC2372_001_TheSectionReadsTheSourceAtItsLinkAndCallsTheGuideTextUntrustedData(t *testing.T) {
	rel, _ := itemNote(t)
	section := itemSection(t)
	root := acsassert.RepoRoot(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1)", rel, code)
	}
	for _, need := range []string{sourceURL, snapshotRef} {
		if !strings.Contains(section, need) {
			t.Errorf("the section of %s does not name %s", itemMarker, need)
		}
	}
	if !strings.Contains(strings.ToLower(section), "untrusted data") {
		t.Errorf("the section does not say that the guide text is untrusted data")
	}
	if placeholder.MatchString(section) {
		t.Errorf("the section holds a placeholder word")
	}
}

func TestC2372_002_TheSectionRecordsThatTheStoredExcerptHoldsNoAddedLineAndGuessesNoChange(t *testing.T) {
	section := itemSection(t)
	if !strings.Contains(strings.ToLower(section), "no added line") {
		t.Errorf("RED: the section does not record that the stored excerpt has no added line")
	}
	if !guessWord.MatchString(section) {
		t.Errorf("RED: the section does not say that the note guesses no changed line")
	}
	if !becauseLine.MatchString(section) {
		t.Errorf("no Decision line states its reason with because")
	}
}

func TestC2372_003_EachExplainedGuideLineIsAShortQuoteInAnHRowWithAMeaning(t *testing.T) {
	section := itemSection(t)
	rows := guideRow.FindAllString(section, -1)
	if len(rows) < 4 {
		t.Fatalf("RED: the section explains %d guide lines in H rows, want at least 4", len(rows))
	}
	seen := map[string]bool{}
	for _, row := range rows {
		cells := tableCells(row)
		if len(cells) < 3 {
			t.Errorf("row has fewer than 3 cells: %s", row)
			continue
		}
		quotes := quotedFrag.FindAllStringSubmatch(cells[1], -1)
		if len(quotes) == 0 {
			t.Errorf("%s has no quoted guide line", cells[0])
		}
		for _, q := range quotes {
			if words := len(strings.Fields(q[1])); words > maxQuoteWords {
				t.Errorf("%s quotes %d words, want at most %d", cells[0], words, maxQuoteWords)
			}
		}
		meaning := cells[len(cells)-1]
		if len(strings.Fields(meaning)) < 5 {
			t.Errorf("%s has no meaning for the loop: %q", cells[0], meaning)
		}
		if seen[cells[1]] {
			t.Errorf("%s repeats an earlier quote", cells[0])
		}
		seen[cells[1]] = true
	}
}

func TestC2372_004_EachCorePersonaHasAVerdictRowWithATrueMarkerCountAndAReason(t *testing.T) {
	section := itemSection(t)
	if !strings.Contains(section, adrNumber) {
		t.Errorf("the section does not cite %s", adrNumber)
	}
	for _, persona := range corePersonas {
		cells := personaRow(section, persona)
		if len(cells) < 4 {
			t.Errorf("no table row with persona, verdict, marker count and reason for %s", persona)
			continue
		}
		if !verdictVocab.MatchString(strings.ToLower(cells[1])) {
			t.Errorf("the verdict of %s is %q, want change now, change later or no change", persona, cells[1])
		}
		want := markersIn(t, persona)
		got, err := strconv.Atoi(cells[2])
		if err != nil || got != want {
			t.Errorf("the marker count of %s is %q, the file holds %d", persona, cells[2], want)
		}
		if len(strings.Fields(cells[len(cells)-1])) < 4 {
			t.Errorf("the verdict row of %s has no reason: %q", persona, cells[len(cells)-1])
		}
	}
}

func TestC2372_005_OnePersonaAtATimeAndEveryChangeNamesGoalDoneCriteriaAndLimits(t *testing.T) {
	section := itemSection(t)
	now, later := 0, 0
	for _, persona := range corePersonas {
		cells := personaRow(section, persona)
		if len(cells) < 2 {
			t.Errorf("no verdict row for %s", persona)
			continue
		}
		switch strings.ToLower(cells[1]) {
		case "change now":
			now++
		case "change later":
			later++
		}
	}
	if now > 1 {
		t.Errorf("%d personas have the verdict change now, ADR-0133 D7 allows one at a time", now)
	}
	if now+later == 0 {
		if !regexp.MustCompile(`(?i)(no persona|no prompt|none changes)`).MatchString(section) {
			t.Errorf("RED: no persona changes and the section does not state that, with its reason")
		}
		return
	}
	for _, label := range []string{"Goal", "Done criteria", "Limits", "Before measure"} {
		if got := strings.Count(section, label+":"); got < now+later {
			t.Errorf("the section has %d %q lines for %d persona changes", got, label, now+later)
		}
	}
}

func TestC2372_006_NoPersonaFileChangesBeyondThePersonaWithTheVerdictChangeNow(t *testing.T) {
	section := itemSection(t)
	allowed := map[string]bool{}
	for _, persona := range corePersonas {
		if cells := personaRow(section, persona); len(cells) > 1 && strings.EqualFold(cells[1], "change now") {
			allowed["agents/"+persona+".md"] = true
		}
	}
	root := acsassert.RepoRoot(t)
	base := "HEAD"
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "rev-parse", "--verify", "--quiet", "origin/main"); code == 0 {
		base = "origin/main"
	}
	changed, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", base, "--", "agents")
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	untracked, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "agents")
	for _, file := range strings.Fields(changed + "\n" + untracked) {
		if !allowed[file] {
			t.Errorf("%s changed, but the section gives its persona no change now verdict", file)
		}
	}
}

func TestC2372_007_TheEarlierSectionsKeepTheirRowsAndTheClaudeNotesFileKeepsItsHeader(t *testing.T) {
	_, text := itemNote(t)
	for _, want := range []struct {
		pattern string
		min     int
	}{{`(?m)^\| F\d+ \|`, 6}, {`(?m)^\| G\d+ \|`, 4}} {
		if got := len(regexp.MustCompile(want.pattern).FindAllString(text, -1)); got < want.min {
			t.Errorf("rows matching %s: %d, want at least %d (an earlier section was cut)", want.pattern, got, want.min)
		}
	}
	for _, id := range []string{"6080fed9", "8d950f1e"} {
		if !strings.Contains(text, id) {
			t.Errorf("the note lost the earlier item %s", id)
		}
	}
	notes := readRepoFile(t, "skills/model-notes-claude/COMPACT.md")
	if !strings.HasPrefix(notes, "# Model notes: claude family\n") {
		t.Errorf("COMPACT.md lost its header")
	}
}

func TestC2372_008_TheNotePassesTheSteLintWithZeroFindings(t *testing.T) {
	rel, text := itemNote(t)
	standard, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for _, f := range stelint.Check([]byte(text), stelint.Options{Words: standard}) {
		if !f.IsSkipNotice() {
			t.Error(fmt.Sprintf("%s:%d [%s] %s", rel, f.Line, f.Rule, f.Message))
		}
	}
}

func TestC2372_009_AnEntryFileInTheResearchIndexLinksTheNote(t *testing.T) {
	rel, _ := itemNote(t)
	dir := filepath.Join(acsassert.RepoRoot(t), "docs", "research-index")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read docs/research-index: %v", err)
	}
	link := "(" + strings.TrimPrefix(rel, "docs/") + ")"
	for _, entry := range entries {
		raw, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr == nil && strings.Contains(string(raw), link) {
			return
		}
	}
	t.Errorf("RED: no entry file in docs/research-index links %s", rel)
}
