//go:build acs

package cycle2371

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
	claudeNotesPath = "skills/model-notes-claude/COMPACT.md"
	itemMarker      = "18278f9c"
	itemID          = "practice-claude-model-release-notes-18278f9c-202611131947"
	snapshotRef     = "2026-11-13T00:01:23Z"
	sourceURL       = "https://platform.claude.com/docs/en/release-notes/overview"
	adrNumber       = "ADR-0133"
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
	findingRow      = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	copiedLine      = regexp.MustCompile(`Sessions can be filtered by status, and events can be filtered by type`)
	decisionBecause = regexp.MustCompile(`(?mi)^.*\bdecision\b.*\bbecause\b.*$`)
	placeholder     = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	noPersonaChange = regexp.MustCompile(`(?i)(no persona|change no persona|no prompt|none changes)`)
	managedUnused   = regexp.MustCompile(`(?is)managed agents[^|\n]{0,200}(does not use|do not use|not use[ds]?|unused|no use)`)
)

func noteHolders(t *testing.T) map[string]string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	rels, err := acsassert.WalkModule(filepath.Join(root, "docs", "research"), func(rel string) bool {
		return !strings.Contains(rel, "/") && strings.HasSuffix(rel, ".md")
	})
	if err != nil {
		t.Fatalf("RED: %v", err)
	}
	holders := map[string]string{}
	for _, rel := range rels {
		raw, readErr := os.ReadFile(filepath.Join(root, "docs", "research", rel))
		if readErr != nil {
			t.Fatalf("read %s: %v", rel, readErr)
		}
		if strings.Contains(string(raw), itemMarker) {
			holders["docs/research/"+rel] = string(raw)
		}
	}
	return holders
}

func proposalNote(t *testing.T) (rel, text string) {
	t.Helper()
	holders := noteHolders(t)
	if len(holders) != 1 {
		t.Fatalf("RED: want exactly one note in docs/research that names %s, got %d", itemID, len(holders))
	}
	for rel, text = range holders {
	}
	return rel, text
}

func steFindings(t *testing.T, rel, text string) []string {
	t.Helper()
	standard, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	var lines []string
	for _, f := range stelint.Check([]byte(text), stelint.Options{Words: standard}) {
		if f.IsSkipNotice() {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s:%d [%s] %s", rel, f.Line, f.Rule, f.Message))
	}
	return lines
}

func personaVerdict(text, persona string) (verdict, reason string) {
	row := regexp.MustCompile(`(?m)^\|[^\n]*` + regexp.QuoteMeta(persona) + `[^\n]*$`).FindString(text)
	cells := strings.Split(strings.Trim(row, "| \t"), "|")
	if row == "" || len(cells) < 3 {
		return "", ""
	}
	return strings.TrimSpace(cells[1]), strings.TrimSpace(cells[len(cells)-1])
}

func diffBase(t *testing.T, root string) string {
	t.Helper()
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "rev-parse", "--verify", "--quiet", "origin/main"); code == 0 {
		return "origin/main"
	}
	return "HEAD"
}

func TestC2371_001_TheNoteReadsTheSourceAtItsLinkAndSaysTheReleaseNoteTextIsUntrustedData(t *testing.T) {
	rel, text := proposalNote(t)
	root := acsassert.RepoRoot(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1)", rel, code)
	}
	for _, need := range []string{sourceURL, snapshotRef, itemMarker} {
		if !strings.Contains(text, need) {
			t.Errorf("the note does not name %s", need)
		}
	}
	if !strings.Contains(strings.ToLower(text), "untrusted data") {
		t.Errorf("the note does not say that the release-note text is untrusted data")
	}
	if placeholder.MatchString(text) {
		t.Errorf("the note holds a placeholder word")
	}
	if hit := copiedLine.FindString(text); hit != "" {
		t.Errorf("the note copies release-note text: %q", hit)
	}
}

func TestC2371_002_EachQuotedLineGetsAnFRowThatExplainsItAsData(t *testing.T) {
	_, text := proposalNote(t)
	rows := findingRow.FindAllString(text, -1)
	if len(rows) < 4 {
		t.Fatalf("RED: the note explains %d release-note lines in F rows, want at least 4", len(rows))
	}
	joined := strings.ToLower(strings.Join(rows, "\n"))
	for _, topic := range []string{"webhook", "session", "event", "dreams"} {
		if !strings.Contains(joined, topic) {
			t.Errorf("no F row explains the %s line of the release notes", topic)
		}
	}
	if !strings.Contains(joined, "truncat") {
		t.Errorf("no F row records that the Dreams line is cut off in the item text")
	}
}

func TestC2371_003_TheNoteStatesWhyManagedAgentsFeaturesTouchNoPhasePrompt(t *testing.T) {
	_, text := proposalNote(t)
	if !managedUnused.MatchString(text) {
		t.Errorf("RED: the note does not say that the pipeline does not use Claude Managed Agents")
	}
	if !strings.Contains(text, "evolve agent run") {
		t.Errorf("the note does not name evolve agent run as the way the pipeline runs a seat")
	}
	if !decisionBecause.MatchString(text) {
		t.Errorf("no Decision line states its reason with because")
	}
}

func TestC2371_004_TheProposalCitesTheAdrAndGivesEachCorePersonaANoChangeVerdictWithAReason(t *testing.T) {
	_, text := proposalNote(t)
	if !strings.Contains(text, adrNumber) {
		t.Errorf("the note does not cite %s", adrNumber)
	}
	if !noPersonaChange.MatchString(text) {
		t.Errorf("RED: the note does not state that no persona or prompt changes")
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
}

func TestC2371_005_TheNotePassesTheSteLintWithZeroFindings(t *testing.T) {
	rel, text := proposalNote(t)
	for _, line := range steFindings(t, rel, text) {
		t.Error(line)
	}
}

func TestC2371_006_TheNoteIsInTheResearchIndexEntries(t *testing.T) {
	rel, _ := proposalNote(t)
	dir := filepath.Join(acsassert.RepoRoot(t), "docs", "research-index")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read docs/research-index: %v", err)
	}
	link := "(" + strings.TrimPrefix(rel, "docs/") + ")"
	for _, entry := range entries {
		raw, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}
		if strings.Contains(string(raw), link) {
			return
		}
	}
	t.Errorf("RED: no entry file in docs/research-index links %s", rel)
}

func TestC2371_007_NoPersonaFileAndNoClaudeNotesFileChangeBecauseTheProposalChangesNone(t *testing.T) {
	_, text := proposalNote(t)
	if !noPersonaChange.MatchString(text) {
		t.Fatalf("RED: the note does not state that no persona or prompt changes")
	}
	root := acsassert.RepoRoot(t)
	base := diffBase(t, root)
	changed, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", base, "--", "agents", claudeNotesPath)
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	if strings.TrimSpace(changed) != "" {
		t.Errorf("files changed although the proposal changes none:\n%s", changed)
	}
	added, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "agents")
	if strings.TrimSpace(added) != "" {
		t.Errorf("untracked files appeared under agents:\n%s", added)
	}
}
