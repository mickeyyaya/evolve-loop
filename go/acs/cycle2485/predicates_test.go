//go:build acs

package cycle2485

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
	itemMarker  = "5cd04824"
	itemID      = "practice-claude-model-release-notes-5cd04824-202611171847"
	sourceURL   = "https://platform.claude.com/docs/en/release-notes/overview"
	snapshotRef = "2026-11-16T22:41:47Z"
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

var factTopics = []string{
	"linked billing",
	"user-profiles-billing",
	"billing_policy",
	"/v1/messages",
	"/v1/chat/completions",
	"spend cap",
	"claude organization",
	"anthropic-managed",
}

var (
	factRow        = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	readAtLink     = regexp.MustCompile(`(?i)\bread\b[^\n]{0,60}\bat (its|the) link`)
	cutOffWord     = regexp.MustCompile(`(?i)\b(truncated|cut off|incomplete)\b`)
	copiedSentence = regexp.MustCompile(`(?i)(your customers can now pay for the claude usage in your app|billing works for customers whose organization pays anthropic directly)`)
	placeholder    = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	followsAsRule  = regexp.MustCompile(`(?i)follow (the )?(release notes|page text|item text) as (instructions|rules)`)
	changeVerdict  = regexp.MustCompile(`(?i)\|\s*(change|trim|diet|edit|update)\s*\|`)
	decisionReason = regexp.MustCompile(`(?mi)^.*\bdecision\b.*\bbecause\b.*$`)
	notesUnchanged = regexp.MustCompile(`(?mi)^.*COMPACT\.md.*\b(unchanged|no change|leave|leaves|stays)\b.*\bbecause\b.*$`)
	notesAddsLine  = regexp.MustCompile(`(?mi)^.*(COMPACT\.md.*\b(add|adds|append|appends|one bullet)\b|\b(add|adds|append|appends|one bullet)\b.*COMPACT\.md).*$`)
	noPersonaSets  = regexp.MustCompile(`(?mi)^.*\bno (phase prompt or )?(persona|prompt)\b[^\n]*\b(sets|reads|uses|calls)\b[^\n]*billing_policy.*$`)
	noPersonaWord  = regexp.MustCompile(`(?i)(no persona|change no persona|no prompt)`)
)

func readRepo(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("RED: cannot read %s (%v)", rel, err)
	}
	return string(raw)
}

func noteForItem(t *testing.T) (rel, text string) {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "docs", "research")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("RED: docs/research cannot be listed: %v", err)
	}
	var hits []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr == nil && strings.Contains(string(raw), itemMarker) {
			hits = append(hits, "docs/research/"+entry.Name())
		}
	}
	if len(hits) != 1 {
		t.Fatalf("RED: want one docs/research note that names %s, found %d: %v", itemMarker, len(hits), hits)
	}
	return hits[0], readRepo(t, hits[0])
}

func changedUnder(t *testing.T, scope string) []string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	base := "HEAD"
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "rev-parse", "--verify", "--quiet", "origin/main"); code == 0 {
		if out, _, mbCode, mbErr := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "origin/main"); mbErr == nil && mbCode == 0 && strings.TrimSpace(out) != "" {
			base = strings.TrimSpace(out)
		}
	}
	tracked, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", base, "--", scope)
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	untracked, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "--", scope)
	return strings.Fields(tracked + "\n" + untracked)
}

func verdictRow(text, persona string) (verdict, reason string) {
	for _, line := range strings.Split(text, "\n") {
		row := strings.TrimSpace(line)
		if !strings.HasPrefix(row, "|") || !strings.Contains(row, persona) {
			continue
		}
		cells := strings.Split(strings.Trim(row, "|"), "|")
		if len(cells) < 3 {
			continue
		}
		return strings.TrimSpace(cells[1]), strings.TrimSpace(cells[len(cells)-1])
	}
	return "", ""
}

func TestC2485_001_TheNoteNamesTheItemTheSourceTheSnapshotAndTheReadAtTheLinkAndCallsThePageUntrustedData(t *testing.T) {
	rel, text := noteForItem(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", acsassert.RepoRoot(t), "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1)", rel, code)
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

func TestC2485_002_TheNoteExplainsEachReleaseNoteLineAsAFactRowAndFlagsTheCutOffLine(t *testing.T) {
	_, text := noteForItem(t)
	rows := factRow.FindAllString(text, -1)
	if len(rows) < 5 {
		t.Fatalf("RED: the note explains %d lines in F rows, want at least 5", len(rows))
	}
	joined := strings.Join(rows, "\n")
	lowered := strings.ToLower(joined)
	for _, topic := range factTopics {
		if !strings.Contains(lowered, topic) {
			t.Errorf("no F row covers %q", topic)
		}
	}
	if !cutOffWord.MatchString(joined) {
		t.Errorf("no F row says that the last release-note line is cut off and so is not a rule")
	}
}

func TestC2485_003_TheNoteCopiesNoReleaseNoteSentenceAndHoldsNoPlaceholderAndFollowsNoPageTextAsARule(t *testing.T) {
	_, text := noteForItem(t)
	if hit := copiedSentence.FindString(text); hit != "" {
		t.Errorf("the note copies release-note text instead of explaining it: %q", hit)
	}
	if placeholder.MatchString(text) {
		t.Errorf("the note holds a placeholder word")
	}
	if hit := followsAsRule.FindString(text); hit != "" {
		t.Errorf("the note treats page text as a rule: %q", hit)
	}
}

func TestC2485_004_EachCorePersonaGetsANoChangeVerdictWithAReasonAndNoRowProposesAChange(t *testing.T) {
	_, text := noteForItem(t)
	if !noPersonaWord.MatchString(text) {
		t.Fatalf("RED: the note does not state that no persona or prompt changes")
	}
	for _, persona := range corePersonas {
		verdict, reason := verdictRow(text, persona)
		if verdict == "" {
			t.Errorf("no table row gives a verdict for %s", persona)
			continue
		}
		if !strings.Contains(strings.ToLower(verdict), "no change") {
			t.Errorf("the verdict of %s is not a no-change verdict: %q", persona, verdict)
		}
		if len(strings.Fields(reason)) < 3 {
			t.Errorf("the verdict row of %s has no real reason: %q", persona, reason)
		}
	}
	if changeVerdict.MatchString(text) {
		t.Errorf("a verdict row proposes a persona change although linked billing gives no prompt rule")
	}
}

func TestC2485_005_TheDecisionStatesItsReasonAndWhyNoPersonaChangesBecauseNoPersonaSetsBillingPolicy(t *testing.T) {
	_, text := noteForItem(t)
	if !decisionReason.MatchString(text) {
		t.Errorf("no Decision line states its reason with because")
	}
	if !noPersonaSets.MatchString(text) {
		t.Errorf("no line states that no persona or phase prompt sets billing_policy")
	}
}

func TestC2485_006_TheDecisionLeavesTheClaudeNotesUnchangedWithAReasonAndTheFileIsUnchanged(t *testing.T) {
	_, text := noteForItem(t)
	if !notesUnchanged.MatchString(text) {
		t.Errorf("no line states that %s stays unchanged, with because", claudeNotes)
	}
	if notesAddsLine.MatchString(text) {
		t.Errorf("a line says that %s gets a new bullet, but linked billing is no prompt-behavior fact", claudeNotes)
	}
	for _, file := range changedUnder(t, claudeNotes) {
		t.Errorf("%s changed, but the decision leaves it unchanged", file)
	}
}

func TestC2485_007_TheNotePassesStrictSteLintWithZeroFindings(t *testing.T) {
	rel, text := noteForItem(t)
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

func TestC2485_008_TheNoteIsInAResearchIndexEntryAndTheGeneratedIndex(t *testing.T) {
	rel, _ := noteForItem(t)
	root := acsassert.RepoRoot(t)
	link := "(" + strings.TrimPrefix(rel, "docs/") + ")"
	entries, err := os.ReadDir(filepath.Join(root, "docs", "research-index"))
	if err != nil {
		t.Fatalf("read docs/research-index: %v", err)
	}
	linked := false
	for _, entry := range entries {
		raw, readErr := os.ReadFile(filepath.Join(root, "docs", "research-index", entry.Name()))
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}
		linked = linked || strings.Contains(string(raw), link)
	}
	if !linked {
		t.Errorf("RED: no entry file in docs/research-index links %s", rel)
	}
	if !strings.Contains(readRepo(t, indexPath), link) {
		t.Errorf("RED: %s does not link %s, so the index was not regenerated", indexPath, rel)
	}
}

func TestC2485_009_NoPersonaNoModelNotesFileAndNoGoSourceChangeBecauseTheProposalChangesNone(t *testing.T) {
	for _, scope := range []string{"agents", "skills/model-notes-claude", "skills/model-notes-agy", "go/internal", "go/cmd", "go/pkg"} {
		for _, file := range changedUnder(t, scope) {
			t.Errorf("%s changed, but the proposal changes no persona, no notes file and no Go source", file)
		}
	}
}
