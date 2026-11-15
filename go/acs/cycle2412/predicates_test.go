//go:build acs

package cycle2412

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
	itemMarker   = "c433b054"
	sourceURL    = "https://platform.claude.com/docs/en/release-notes/overview"
	snapshotRef  = "2026-11-13T19:47:32Z"
	adrNumber    = "ADR-0133"
	claudeNotes  = "skills/model-notes-claude/COMPACT.md"
	indexPath    = "docs/research-index.md"
	emptyExcerpt = "no added line"
)

var corePersonas = []string{
	"evolve-scout",
	"evolve-triage",
	"evolve-tdd-engineer",
	"evolve-builder",
	"evolve-auditor",
	"evolve-retrospective",
}

var findingTopics = []string{
	emptyExcerpt,
	"november 12",
	"managed agents",
	"self-hosted",
}

var (
	findingRow       = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	copiedLine       = regexp.MustCompile(`(?i)you run the whole managed agents service|not only self-hosted sandboxes|contact your anthropic account team|store agents, sessions, and session history there`)
	placeholder      = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	adoptsService    = regexp.MustCompile(`(?i)\b(adopt|enable|use|start|add|run) (the |a |an )?(self-hosted )?(Claude )?Managed Agents\b`)
	negatedLine      = regexp.MustCompile(`(?i)\b(not|never|no|without)\b|rule[sd]? out`)
	followsAsRule    = regexp.MustCompile(`(?i)follow (the )?release notes as (instructions|rules)`)
	decisionBecause  = regexp.MustCompile(`(?mi)^.*\bdecision\b.*\bbecause\b.*$`)
	noPersonaChange  = regexp.MustCompile(`(?i)(no persona|change no persona|no prompt)`)
	personaChanges   = regexp.MustCompile(`(?i)\|\s*(change|trim|diet|edit|update)\s*\|`)
	notesUnchanged   = regexp.MustCompile(`(?i)(leave|leaves|keep|keeps|stay|stays|remain|remains)[^\n]*unchanged|unchanged[^\n]*(leave|keep|stay|remain)|no change[^\n]*COMPACT|COMPACT[^\n]*no change`)
	itemIDInNoteBody = regexp.MustCompile(`practice-claude-model-release-notes-c433b054-\d+`)
)

func repoText(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("RED: %s cannot be read: %v", rel, err)
	}
	return string(body)
}

func steFindings(t *testing.T, rel string) []string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("the STE standard did not load: %v", err)
	}
	var report []string
	for _, finding := range stelint.Check([]byte(repoText(t, rel)), stelint.Options{Words: words}) {
		if finding.IsSkipNotice() {
			continue
		}
		report = append(report, fmt.Sprintf("%s:%d [%s] %s", rel, finding.Line, finding.Rule, finding.Message))
	}
	return report
}

func researchNote(t *testing.T) (rel, text string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "docs", "research", "*.md"))
	if err != nil {
		t.Fatalf("RED: the docs/research glob failed: %v", err)
	}
	var holders []string
	for _, abs := range matches {
		if acsassert.FileContainsAny(abs, itemMarker) {
			holders = append(holders, "docs/research/"+filepath.Base(abs))
		}
	}
	if len(holders) != 1 {
		t.Fatalf("RED: want exactly one docs/research note naming %s, found %d: %v", itemMarker, len(holders), holders)
	}
	return holders[0], repoText(t, holders[0])
}

func personaVerdict(text, persona string) (verdict, reason string) {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || !strings.Contains(trimmed, persona) {
			continue
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		if len(cells) < 3 {
			continue
		}
		last := len(cells) - 1
		return strings.TrimSpace(cells[1]), strings.TrimSpace(cells[last])
	}
	return "", ""
}

func TestC2412_001_TheNoteNamesTheItemTheSourceTheSnapshotAndCallsThePageUntrustedData(t *testing.T) {
	rel, text := researchNote(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", acsassert.RepoRoot(t), "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1)", rel, code)
	}
	if !itemIDInNoteBody.MatchString(text) {
		t.Errorf("the note does not name the full item id practice-claude-model-release-notes-%s-<stamp>", itemMarker)
	}
	for _, need := range []string{sourceURL, snapshotRef, adrNumber} {
		if !strings.Contains(text, need) {
			t.Errorf("the note does not name %s", need)
		}
	}
	if !strings.Contains(strings.ToLower(text), "untrusted data") {
		t.Errorf("the note does not say that the release-note text is untrusted data")
	}
}

func TestC2412_002_TheNoteExplainsTheNovember12EntryAndTheEmptyExcerptAsFactRows(t *testing.T) {
	_, text := researchNote(t)
	rows := findingRow.FindAllString(text, -1)
	if len(rows) < 3 {
		t.Fatalf("RED: the note explains %d lines in F rows, want at least 3", len(rows))
	}
	joined := strings.ToLower(strings.Join(rows, "\n"))
	for _, topic := range findingTopics {
		if !strings.Contains(joined, topic) {
			t.Errorf("no F row covers %q", topic)
		}
	}
	if hit := copiedLine.FindString(text); hit != "" {
		t.Errorf("the note copies release-note text instead of explaining it: %q", hit)
	}
}

func TestC2412_003_TheNoteHoldsNoPlaceholderAndNeverTellsTheLoopToAdoptManagedAgentsOrFollowTheText(t *testing.T) {
	_, text := researchNote(t)
	if placeholder.MatchString(text) {
		t.Errorf("the note holds a placeholder word")
	}
	if hit := followsAsRule.FindString(text); hit != "" {
		t.Errorf("the note treats page text as a rule: %q", hit)
	}
	for _, line := range strings.Split(text, "\n") {
		if adoptsService.MatchString(line) && !negatedLine.MatchString(line) {
			t.Errorf("the note tells the loop to use the hosted service: %q", line)
		}
	}
}

func TestC2412_004_EachCorePersonaGetsANoChangeVerdictWithAReasonAndTheDecisionStatesWhyNonePersonaChanges(t *testing.T) {
	_, text := researchNote(t)
	if !noPersonaChange.MatchString(text) {
		t.Fatalf("RED: the note does not state that no persona or prompt changes")
	}
	for _, persona := range corePersonas {
		verdict, reason := personaVerdict(text, persona)
		if verdict == "" {
			t.Errorf("no table row gives a verdict for %s", persona)
			continue
		}
		if !strings.Contains(strings.ToLower(verdict), "no change") {
			t.Errorf("the verdict of %s is not a no-change verdict: %q", persona, verdict)
		}
		if reason == "" {
			t.Errorf("the verdict row of %s has no reason", persona)
		}
	}
	if personaChanges.MatchString(text) {
		t.Errorf("a verdict row proposes a persona change although the proposal changes none")
	}
	if !decisionBecause.MatchString(text) {
		t.Errorf("no Decision line states its reason with because")
	}
	if !strings.Contains(text, claudeNotes) || !notesUnchanged.MatchString(text) {
		t.Errorf("the note does not say that %s stays unchanged", claudeNotes)
	}
}

func TestC2412_005_TheNotePassesStrictSteLintWithZeroFindings(t *testing.T) {
	rel, _ := researchNote(t)
	for _, line := range steFindings(t, rel) {
		t.Error(line)
	}
}

func TestC2412_006_TheNoteIsInAResearchIndexEntryAndTheGeneratedIndex(t *testing.T) {
	rel, _ := researchNote(t)
	root := acsassert.RepoRoot(t)
	link := "(" + strings.TrimPrefix(rel, "docs/") + ")"
	entries, err := os.ReadDir(filepath.Join(root, "docs", "research-index"))
	if err != nil {
		t.Fatalf("read docs/research-index: %v", err)
	}
	found := false
	for _, entry := range entries {
		raw, readErr := os.ReadFile(filepath.Join(root, "docs", "research-index", entry.Name()))
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}
		if strings.Contains(string(raw), link) {
			found = true
		}
	}
	if !found {
		t.Errorf("RED: no entry file in docs/research-index links %s", rel)
	}
	if !strings.Contains(repoText(t, indexPath), link) {
		t.Errorf("RED: %s does not link %s, so the index was not regenerated", indexPath, rel)
	}
}

func TestC2412_007_NoPersonaFileAndNoClaudeModelNotesFileChangeBecauseTheProposalChangesNone(t *testing.T) {
	_, text := researchNote(t)
	if !noPersonaChange.MatchString(text) {
		t.Fatalf("RED: the note does not state that no persona or prompt changes, so the unchanged paths are not checked yet")
	}
	root := acsassert.RepoRoot(t)
	for _, scope := range []string{"agents", claudeNotes} {
		changed, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "HEAD", "--", scope)
		if err != nil || code != 0 {
			t.Fatalf("git diff exit=%d: %v", code, err)
		}
		if strings.TrimSpace(changed) != "" {
			t.Errorf("files changed under %s although the proposal changes none:\n%s", scope, changed)
		}
		added, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", scope)
		if strings.TrimSpace(added) != "" {
			t.Errorf("untracked files appeared under %s:\n%s", scope, added)
		}
	}
}
