//go:build acs

package cycle2527

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
	itemMarker  = "2c5f9a4d"
	itemID      = "practice-claude-prompting-guide-2c5f9a4d-202611190048"
	sourceURL   = "https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices"
	snapshotRef = "2026-11-15T11:10:47Z"
	reviewDate  = "2026-11-19"
	adrNumber   = "ADR-0133"
	claudeNotes = "skills/model-notes-claude/COMPACT.md"
	indexPath   = "docs/research-index.md"
)

var corePersonas = []string{
	"evolve-scout",
	"evolve-triage",
	"evolve-tdd-engineer",
	"evolve-builder",
	"evolve-auditor",
	"evolve-retrospective",
}

var untouchedScopes = []string{
	"go/internal",
	"go/cmd",
	"go/pkg",
	"go/acs/regression",
	"skills/model-notes-agy",
	"skills/model-notes-codex",
}

var changeProposalTerms = []string{"goal", "done criteria", "limits", "before measure"}

var (
	factRow        = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	readAtLink     = regexp.MustCompile(`(?i)\bread\b[^\n]{0,60}\bat (its|the) link`)
	chromeWords    = regexp.MustCompile(`(?i)\b(search|shortcut|keyboard|command key)\b`)
	chromeKind     = regexp.MustCompile(`(?i)\b(chrome|shell|glyph|symbol|key cap|button|icon)\b`)
	notARule       = regexp.MustCompile(`(?i)\b(not a rule|no rule|not an instruction|gives no rule|carries no rule|holds no rule)\b`)
	placeholder    = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	pageAsRule     = regexp.MustCompile(`(?i)follow (the )?(guide|page text|item text|quoted lines?) as (instructions|rules)`)
	decisionReason = regexp.MustCompile(`(?mi)^.*\bdecision\b.*\bbecause\b.*$`)
	rejectedReason = regexp.MustCompile(`(?mi)\brejected option\b[^\n]*(\bbecause\b|\brules? out\b)`)
	notesLine      = regexp.MustCompile(`(?mi)^.*COMPACT\.md.*$`)
	notesStays     = regexp.MustCompile(`(?i)\b(unchanged|no change|stays|leave|leaves)\b`)
	notesGrows     = regexp.MustCompile(`(?i)\b(add|adds|append|appends|one bullet|new bullet|edit|edits|rewrite|rewrites)\b`)
	becauseWord    = regexp.MustCompile(`(?i)\bbecause\b`)
	noPersonaWords = regexp.MustCompile(`(?i)(no persona|change no persona|no prompt)`)
)

type personaRow struct {
	verdict string
	reason  string
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	full := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel))
	if !acsassert.FileExists(t, full) {
		t.FailNow()
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("RED: %v", err)
	}
	return string(raw)
}

func locateItemNote(t *testing.T) (string, string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(acsassert.RepoRoot(t), "docs", "research"))
	if err != nil {
		t.Fatalf("RED: docs/research cannot be listed: %v", err)
	}
	var hits []string
	for _, entry := range entries {
		rel := "docs/research/" + entry.Name()
		if entry.IsDir() || !strings.HasSuffix(rel, ".md") {
			continue
		}
		if strings.Contains(readRepoFile(t, rel), itemMarker) {
			hits = append(hits, rel)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("RED: want exactly one docs/research note that names %s, found %d: %v", itemMarker, len(hits), hits)
	}
	return hits[0], readRepoFile(t, hits[0])
}

func pathsChangedUnder(t *testing.T, scope string) []string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	tracked, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "HEAD", "--", scope)
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	fresh, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "--", scope)
	if err != nil || code != 0 {
		t.Fatalf("git ls-files exit=%d: %v", code, err)
	}
	return strings.Fields(tracked + "\n" + fresh)
}

func personaTable(text string) map[string][]personaRow {
	table := map[string][]personaRow{}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		if len(cells) < 3 {
			continue
		}
		for _, persona := range corePersonas {
			if strings.Contains(cells[0], persona) {
				table[persona] = append(table[persona], personaRow{
					verdict: strings.ToLower(strings.TrimSpace(cells[1])),
					reason:  strings.TrimSpace(cells[len(cells)-1]),
				})
			}
		}
	}
	return table
}

func personasMarkedChange(table map[string][]personaRow) []string {
	var marked []string
	for persona, rows := range table {
		if len(rows) == 1 && !strings.Contains(rows[0].verdict, "no change") {
			marked = append(marked, persona)
		}
	}
	return marked
}

func TestC2527_001_TheNoteNamesTheItemTheSourceTheSnapshotTheReviewDateAndTheReadAtTheLinkAndCallsTheGuideUntrustedData(t *testing.T) {
	rel, text := locateItemNote(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", acsassert.RepoRoot(t), "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and ship would drop it (check-ignore exit code %d, want 1)", rel, code)
	}
	for _, need := range []string{itemID, sourceURL, snapshotRef, reviewDate, adrNumber} {
		if !strings.Contains(text, need) {
			t.Errorf("the note does not name %s", need)
		}
	}
	if !readAtLink.MatchString(text) {
		t.Errorf("the note does not state that the guide was read at its link")
	}
	if !strings.Contains(strings.ToLower(text), "untrusted data") {
		t.Errorf("the note does not call the guide text untrusted data")
	}
}

func TestC2527_002_AFactRowExplainsTheQuotedLinesAsSearchShortcutChromeThatIsNotARuleAndOtherRowsCoverTheGuide(t *testing.T) {
	_, text := locateItemNote(t)
	rows := factRow.FindAllString(text, -1)
	if len(rows) < 3 {
		t.Fatalf("RED: the note holds %d F rows, want at least 3", len(rows))
	}
	quotedLineRows := 0
	guideRows := 0
	for _, row := range rows {
		if chromeWords.MatchString(row) && chromeKind.MatchString(row) {
			quotedLineRows++
			if !notARule.MatchString(row) {
				t.Errorf("the F row on the quoted lines does not say that they are not a rule: %s", row)
			}
			continue
		}
		if strings.Contains(strings.ToLower(row), "guide") {
			guideRows++
		}
	}
	if quotedLineRows == 0 {
		t.Errorf("no F row explains that the quoted lines are search shortcut symbols from the page shell")
	}
	if guideRows < 2 {
		t.Errorf("only %d F rows explain a point of the guide itself, want at least 2", guideRows)
	}
}

func TestC2527_003_TheNoteHoldsNoPlaceholderAndTreatsNoPageTextAsARule(t *testing.T) {
	_, text := locateItemNote(t)
	if hit := placeholder.FindString(text); hit != "" {
		t.Errorf("the note holds a placeholder word: %q", hit)
	}
	if hit := pageAsRule.FindString(text); hit != "" {
		t.Errorf("the note treats page text as a rule: %q", hit)
	}
}

func TestC2527_004_EachCorePersonaGetsExactlyOneVerdictRowWithARealReasonAndAnAllNoChangeNoteSaysSo(t *testing.T) {
	_, text := locateItemNote(t)
	table := personaTable(text)
	for _, persona := range corePersonas {
		rows := table[persona]
		if len(rows) != 1 {
			t.Errorf("want exactly one verdict row for %s, found %d", persona, len(rows))
			continue
		}
		if !strings.Contains(rows[0].verdict, "change") {
			t.Errorf("the verdict of %s is neither change nor no change: %q", persona, rows[0].verdict)
		}
		if len(strings.Fields(rows[0].reason)) < 3 {
			t.Errorf("the verdict row of %s has no real reason: %q", persona, rows[0].reason)
		}
	}
	if len(personasMarkedChange(table)) == 0 && !noPersonaWords.MatchString(text) {
		t.Errorf("RED: every persona is no change, but the note never states that no persona or prompt changes")
	}
}

func TestC2527_005_TheDecisionStatesItsReasonAndTheRejectedOptionStatesWhatRulesItOut(t *testing.T) {
	_, text := locateItemNote(t)
	if !decisionReason.MatchString(text) {
		t.Errorf("no Decision line states its reason with because")
	}
	if !rejectedReason.MatchString(text) {
		t.Errorf("no rejected option line states the reason that rules the option out")
	}
}

func TestC2527_006_TheClaudeNotesDecisionMatchesTheFileDiffInBothDirections(t *testing.T) {
	_, text := locateItemNote(t)
	lines := notesLine.FindAllString(text, -1)
	if len(lines) == 0 {
		t.Fatalf("RED: the note never states the fate of %s", claudeNotes)
	}
	statement := strings.Join(lines, "\n")
	diff := pathsChangedUnder(t, claudeNotes)
	switch {
	case notesGrows.MatchString(statement) && !notesStays.MatchString(statement):
		if len(diff) == 0 {
			t.Errorf("the note says %s gets a change, but the file is unchanged", claudeNotes)
		}
	case notesStays.MatchString(statement):
		if len(diff) != 0 {
			t.Errorf("the note says %s stays, but it changed: %v", claudeNotes, diff)
		}
		if !becauseWord.MatchString(statement) {
			t.Errorf("the note leaves %s alone without a reason", claudeNotes)
		}
	default:
		t.Errorf("the note neither keeps nor changes %s in clear words", claudeNotes)
	}
}

func TestC2527_007_ThePersonaFilesThatChangeAreExactlyThePersonasMarkedChangeAndAtMostOneChangesWithItsProposalTerms(t *testing.T) {
	_, text := locateItemNote(t)
	marked := personasMarkedChange(personaTable(text))
	wantFiles := map[string]bool{}
	for _, persona := range marked {
		wantFiles["agents/"+persona+".md"] = true
	}
	gotFiles := pathsChangedUnder(t, "agents")
	for _, rel := range gotFiles {
		if !wantFiles[rel] {
			t.Errorf("%s changed, but the note gives its persona a no-change verdict", rel)
		}
		delete(wantFiles, rel)
	}
	for rel := range wantFiles {
		t.Errorf("the note proposes a change to %s, but the file is unchanged", rel)
	}
	if len(marked) > 1 || len(gotFiles) > 1 {
		t.Errorf("%s changes one persona at a time: the note marks %v and the diff holds %v", adrNumber, marked, gotFiles)
	}
	if len(marked) > 0 {
		lower := strings.ToLower(text)
		for _, term := range changeProposalTerms {
			if !strings.Contains(lower, term) {
				t.Errorf("a persona change is proposed, but the note never states its %q", term)
			}
		}
	}
}

func TestC2527_008_TheNoteAndTheClaudeNotesPassStrictSteLintWithZeroFindings(t *testing.T) {
	rel, text := locateItemNote(t)
	words, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("the STE standard did not load: %v", err)
	}
	documents := map[string]string{rel: text, claudeNotes: readRepoFile(t, claudeNotes)}
	for name, body := range documents {
		for _, finding := range stelint.Check([]byte(body), stelint.Options{Words: words}) {
			if !finding.IsSkipNotice() {
				t.Error(fmt.Sprintf("%s:%d [%s] %s", name, finding.Line, finding.Rule, finding.Message))
			}
		}
	}
}

func TestC2527_009_TheNoteIsInAResearchIndexEntryAndTheGeneratedIndex(t *testing.T) {
	rel, _ := locateItemNote(t)
	link := "(" + strings.TrimPrefix(rel, "docs/") + ")"
	entries, err := os.ReadDir(filepath.Join(acsassert.RepoRoot(t), "docs", "research-index"))
	if err != nil {
		t.Fatalf("read docs/research-index: %v", err)
	}
	linkedBy := ""
	for _, entry := range entries {
		if strings.Contains(readRepoFile(t, "docs/research-index/"+entry.Name()), link) {
			linkedBy = entry.Name()
		}
	}
	if linkedBy == "" {
		t.Errorf("RED: no entry file in docs/research-index links %s", rel)
	}
	if !strings.Contains(readRepoFile(t, indexPath), link) {
		t.Errorf("RED: %s does not link %s, so the index was not regenerated", indexPath, rel)
	}
}

func TestC2527_010_NoGoSourceNoRegressionPredicateAndNoOtherFamilyNotesChangeBecauseTheTaskIsAReview(t *testing.T) {
	if _, text := locateItemNote(t); text == "" {
		t.Fatalf("RED: the review note is empty")
	}
	for _, scope := range untouchedScopes {
		for _, rel := range pathsChangedUnder(t, scope) {
			t.Errorf("%s changed, but a prompting guide review changes no Go source and no other family notes", rel)
		}
	}
}
