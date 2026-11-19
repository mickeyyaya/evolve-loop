//go:build acs

package cycle2526

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
	itemMarker  = "7177779e"
	itemID      = "practice-claude-model-release-notes-7177779e-202611190048"
	sourceURL   = "https://platform.claude.com/docs/en/release-notes/overview"
	snapshotRef = "2026-11-17T18:47:38Z"
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

var quotedLineTopics = []string{
	"security posture",
	"endpoint",
	"claude api",
}

var untouchedScopes = []string{
	"go/internal",
	"go/cmd",
	"go/pkg",
	"go/acs/regression",
	"skills/model-notes-agy",
	"skills/model-notes-codex",
}

var (
	factRow         = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	readAtLink      = regexp.MustCompile(`(?i)\bread\b[^\n]{0,60}\bat (its|the) link`)
	cutOffWord      = regexp.MustCompile(`(?i)\b(truncated|cut off|cut|incomplete)\b`)
	copiedFragments = regexp.MustCompile(`(?i)(returns an organization.s current security posture results|bringing the claude api to anthro)`)
	placeholder     = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	pageAsRule      = regexp.MustCompile(`(?i)follow (the )?(release notes|page text|item text) as (instructions|rules)`)
	decisionReason  = regexp.MustCompile(`(?mi)^.*\bdecision\b.*\bbecause\b.*$`)
	rejectedOption  = regexp.MustCompile(`(?mi)\brejected option\b`)
	notesLine       = regexp.MustCompile(`(?mi)^.*COMPACT\.md.*$`)
	notesStays      = regexp.MustCompile(`(?i)\b(unchanged|no change|stays|leave|leaves)\b`)
	notesGrows      = regexp.MustCompile(`(?i)\b(add|adds|append|appends|one bullet|new bullet)\b`)
	noPersonaStated = regexp.MustCompile(`(?i)(no persona|change no persona|no prompt)`)
)

func repoText(t *testing.T, rel string) string {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel))
	if !acsassert.FileExists(t, path) {
		t.FailNow()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("RED: %v", err)
	}
	return string(raw)
}

func itemNote(t *testing.T) (string, string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(acsassert.RepoRoot(t), "docs", "research"))
	if err != nil {
		t.Fatalf("RED: docs/research cannot be listed: %v", err)
	}
	var found []string
	for _, entry := range entries {
		rel := "docs/research/" + entry.Name()
		if !entry.IsDir() && strings.HasSuffix(rel, ".md") && strings.Contains(repoText(t, rel), itemMarker) {
			found = append(found, rel)
		}
	}
	if len(found) != 1 {
		t.Fatalf("RED: want exactly one docs/research note that names %s, found %d: %v", itemMarker, len(found), found)
	}
	return found[0], repoText(t, found[0])
}

func diffBase(t *testing.T) string {
	t.Helper()
	out, _, code, _ := acsassert.SubprocessOutput("git", "-C", acsassert.RepoRoot(t), "merge-base", "HEAD", "origin/main")
	if base := strings.TrimSpace(out); code == 0 && base != "" {
		return base
	}
	return "HEAD"
}

func changedPaths(t *testing.T, scope string) []string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	tracked, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", diffBase(t), "--", scope)
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	fresh, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "--", scope)
	return strings.Fields(tracked + "\n" + fresh)
}

func personaVerdicts(text string) map[string][2]string {
	verdicts := map[string][2]string{}
	for _, line := range strings.Split(text, "\n") {
		row := strings.TrimSpace(line)
		cells := strings.Split(strings.Trim(row, "|"), "|")
		if !strings.HasPrefix(row, "|") || len(cells) < 3 {
			continue
		}
		for _, persona := range corePersonas {
			if strings.Contains(cells[0], persona) {
				verdicts[persona] = [2]string{strings.ToLower(strings.TrimSpace(cells[1])), strings.TrimSpace(cells[len(cells)-1])}
			}
		}
	}
	return verdicts
}

func changedPersonas(verdicts map[string][2]string) []string {
	var changed []string
	for persona, v := range verdicts {
		if !strings.Contains(v[0], "no change") {
			changed = append(changed, persona)
		}
	}
	return changed
}

func TestC2526_001_TheNoteNamesTheItemTheSourceTheSnapshotAndTheReadAtTheLinkAndCallsThePageUntrustedData(t *testing.T) {
	rel, text := itemNote(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", acsassert.RepoRoot(t), "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and ship would drop it (check-ignore exit code %d, want 1)", rel, code)
	}
	for _, need := range []string{itemID, sourceURL, snapshotRef, adrNumber} {
		if !strings.Contains(text, need) {
			t.Errorf("the note does not name %s", need)
		}
	}
	if !readAtLink.MatchString(text) {
		t.Errorf("the note does not state that the source was read at its link")
	}
	if !strings.Contains(strings.ToLower(text), "untrusted data") {
		t.Errorf("the note does not call the release-note text untrusted data")
	}
}

func TestC2526_002_TheNoteExplainsEachQuotedLineInAFactRowAndFlagsTheCutOffLine(t *testing.T) {
	_, text := itemNote(t)
	rows := factRow.FindAllString(text, -1)
	if len(rows) < 3 {
		t.Fatalf("RED: the note holds %d F rows, want at least 3", len(rows))
	}
	joined := strings.Join(rows, "\n")
	for _, topic := range quotedLineTopics {
		if !strings.Contains(strings.ToLower(joined), topic) {
			t.Errorf("no F row covers the quoted topic %q", topic)
		}
	}
	if !cutOffWord.MatchString(joined) {
		t.Errorf("no F row says that the last quoted line is cut off and so is not a rule")
	}
}

func TestC2526_003_TheNoteCopiesNoQuotedFragmentAndHoldsNoPlaceholderAndFollowsNoPageTextAsARule(t *testing.T) {
	_, text := itemNote(t)
	if hit := copiedFragments.FindString(text); hit != "" {
		t.Errorf("the note copies item text instead of explaining it: %q", hit)
	}
	if placeholder.MatchString(text) {
		t.Errorf("the note holds a placeholder word")
	}
	if hit := pageAsRule.FindString(text); hit != "" {
		t.Errorf("the note treats page text as a rule: %q", hit)
	}
}

func TestC2526_004_EachCorePersonaGetsOneVerdictWithAReasonAndAChangeNamesThePersonaFile(t *testing.T) {
	_, text := itemNote(t)
	verdicts := personaVerdicts(text)
	for _, persona := range corePersonas {
		v, ok := verdicts[persona]
		if !ok {
			t.Errorf("no table row gives a verdict for %s", persona)
			continue
		}
		if len(strings.Fields(v[1])) < 3 {
			t.Errorf("the verdict row of %s has no real reason: %q", persona, v[1])
		}
	}
	if len(changedPersonas(verdicts)) == 0 && !noPersonaStated.MatchString(text) {
		t.Errorf("RED: every persona is no change, but the note never states that no persona or prompt changes")
	}
}

func TestC2526_005_TheDecisionStatesItsReasonAndNamesTheRejectedOption(t *testing.T) {
	_, text := itemNote(t)
	if !decisionReason.MatchString(text) {
		t.Errorf("no Decision line states its reason with because")
	}
	if !rejectedOption.MatchString(text) {
		t.Errorf("the note names no rejected option")
	}
}

func TestC2526_006_TheClaudeNotesDecisionMatchesTheFileDiffInBothDirections(t *testing.T) {
	_, text := itemNote(t)
	lines := notesLine.FindAllString(text, -1)
	if len(lines) == 0 {
		t.Fatalf("RED: the note never states the fate of %s", claudeNotes)
	}
	statement := strings.Join(lines, "\n")
	diff := changedPaths(t, claudeNotes)
	switch {
	case notesGrows.MatchString(statement) && !notesStays.MatchString(statement):
		if len(diff) == 0 {
			t.Errorf("the note says %s gets a change, but the file is unchanged", claudeNotes)
		}
	case notesStays.MatchString(statement):
		if len(diff) != 0 {
			t.Errorf("the note says %s stays, but it changed: %v", claudeNotes, diff)
		}
		if !regexp.MustCompile(`(?i)because`).MatchString(statement) {
			t.Errorf("the note leaves %s alone without a reason", claudeNotes)
		}
	default:
		t.Errorf("the note neither keeps nor changes %s in clear words", claudeNotes)
	}
}

func TestC2526_007_ThePersonaFilesThatChangeAreExactlyThePersonasTheNoteMarksAsChange(t *testing.T) {
	_, text := itemNote(t)
	marked := map[string]bool{}
	for _, persona := range changedPersonas(personaVerdicts(text)) {
		marked["agents/"+persona+".md"] = true
	}
	for _, rel := range changedPaths(t, "agents") {
		if !marked[rel] {
			t.Errorf("%s changed, but the note gives its persona a no-change verdict", rel)
		}
		delete(marked, rel)
	}
	for rel := range marked {
		t.Errorf("the note proposes a change to %s, but the file is unchanged", rel)
	}
}

func TestC2526_008_TheNotePassesStrictSteLintWithZeroFindings(t *testing.T) {
	rel, text := itemNote(t)
	words, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("the STE standard did not load: %v", err)
	}
	for _, finding := range stelint.Check([]byte(text), stelint.Options{Words: words}) {
		if !finding.IsSkipNotice() {
			t.Error(fmt.Sprintf("%s:%d [%s] %s", rel, finding.Line, finding.Rule, finding.Message))
		}
	}
}

func TestC2526_009_TheNoteIsInAResearchIndexEntryAndTheGeneratedIndex(t *testing.T) {
	rel, _ := itemNote(t)
	link := "(" + strings.TrimPrefix(rel, "docs/") + ")"
	entries, err := os.ReadDir(filepath.Join(acsassert.RepoRoot(t), "docs", "research-index"))
	if err != nil {
		t.Fatalf("read docs/research-index: %v", err)
	}
	linked := false
	for _, entry := range entries {
		linked = linked || strings.Contains(repoText(t, "docs/research-index/"+entry.Name()), link)
	}
	if !linked {
		t.Errorf("RED: no entry file in docs/research-index links %s", rel)
	}
	if !strings.Contains(repoText(t, indexPath), link) {
		t.Errorf("RED: %s does not link %s, so the index was not regenerated", indexPath, rel)
	}
}

func TestC2526_010_NoGoSourceNoRegressionPredicateAndNoOtherFamilyNotesChangeBecauseTheTaskIsAReview(t *testing.T) {
	for _, scope := range untouchedScopes {
		for _, rel := range changedPaths(t, scope) {
			t.Errorf("%s changed, but a model-release-notes review changes no Go source and no other family notes", rel)
		}
	}
}
