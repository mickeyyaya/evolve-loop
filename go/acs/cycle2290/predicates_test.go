//go:build acs

package cycle2290

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
	claudeNotesPath = "skills/model-notes-claude/COMPACT.md"
	itemMarker      = "2f02c519"
	sourceURL       = "https://platform.claude.com/docs/en/release-notes/overview"
	snapshotMark    = "2026-11-10T08:41:11Z"
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

var factTopics = []string{"security posture", "compliance api", "activity feed", "managed agents"}

var (
	factRow     = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	placeholder = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	changeVerb  = regexp.MustCompile(`(?i)(change no persona|no persona changes|no persona change|no prompt changes|no prompt change)`)
)

func steFindings(t *testing.T, rel string) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	found, err := stelint.LintFile(filepath.Join(root, rel), stelint.Options{Words: words})
	if err != nil {
		t.Fatalf("lint %s: %v", rel, err)
	}
	var report strings.Builder
	for _, f := range found {
		if f.IsSkipNotice() {
			continue
		}
		report.WriteString(f.Rule + ": " + f.Message + ": " + f.Excerpt + "\n")
	}
	return report.String()
}

func itemNote(t *testing.T) (rel, text string) {
	t.Helper()
	candidates, err := filepath.Glob(filepath.Join(acsassert.RepoRoot(t), "docs", "research", "*.md"))
	if err != nil {
		t.Fatalf("glob docs/research: %v", err)
	}
	var matches []string
	var bodies []string
	for _, path := range candidates {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if strings.Contains(string(raw), itemMarker) {
			matches = append(matches, filepath.Base(path))
			bodies = append(bodies, string(raw))
		}
	}
	if len(matches) != 1 {
		t.Fatalf("RED: want exactly one note in docs/research that names the item %s, got %d", itemMarker, len(matches))
	}
	return "docs/research/" + matches[0], bodies[0]
}

func diffBase(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "rev-parse", "--verify", "--quiet", "origin/main"); code != 0 {
		return "HEAD"
	}
	return "origin/main"
}

func TestC2290_001_TheNoteReadsTheSourceAtTheLinkAndExplainsTheSecurityPostureAndManagedAgentsLinesAsUntrustedData(t *testing.T) {
	rel, text := itemNote(t)
	root := acsassert.RepoRoot(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1)", rel, code)
	}
	for _, need := range []string{sourceURL, snapshotMark} {
		if !strings.Contains(text, need) {
			t.Errorf("the note does not name %s", need)
		}
	}
	if !strings.Contains(strings.ToLower(text), "untrusted data") {
		t.Errorf("the note does not say that the release-note text is untrusted data")
	}
	rows := factRow.FindAllString(text, -1)
	if len(rows) < len(factTopics) {
		t.Fatalf("the note explains %d release-note lines in F rows, want at least %d", len(rows), len(factTopics))
	}
	joined := strings.ToLower(strings.Join(rows, "\n"))
	for _, topic := range factTopics {
		if !strings.Contains(joined, topic) {
			t.Errorf("no F row explains the %s lines of the release notes", topic)
		}
	}
	if !strings.Contains(joined, "research preview") {
		t.Errorf("no F row says that Managed Agents is a research preview")
	}
	if !strings.Contains(joined, "/v1/compliance/organizations/{organization_id}/security_posture") {
		t.Errorf("no F row names the security_posture endpoint as data")
	}
}

func TestC2290_002_TheNoteGivesEachCorePersonaANoChangeVerdictWithAReasonUnderTheAdr(t *testing.T) {
	_, text := itemNote(t)
	if !strings.Contains(text, adrNumber) {
		t.Errorf("the note does not cite %s", adrNumber)
	}
	for _, persona := range corePersonas {
		verdictRow := regexp.MustCompile(`(?m)^\|[^\n]*` + regexp.QuoteMeta(persona) + `[^\n]*\|[^\n]*change[^\n]*\|`).FindString(text)
		if verdictRow == "" {
			t.Errorf("no table row gives a verdict for %s", persona)
			continue
		}
		cells := strings.Split(strings.Trim(verdictRow, "| "), "|")
		if len(cells) < 3 || strings.TrimSpace(cells[len(cells)-1]) == "" {
			t.Errorf("the verdict row of %s has no reason: %q", persona, verdictRow)
			continue
		}
		if !strings.Contains(strings.ToLower(cells[1]), "no change") {
			t.Errorf("the verdict of %s is not a no-change verdict: %q", persona, cells[1])
		}
	}
	if !regexp.MustCompile(`(?mi)^.*\bdecision\b.*\bbecause\b.*$`).MatchString(text) {
		t.Errorf("no Decision line states its reason with because")
	}
	if !strings.Contains(text, "Rejected option") {
		t.Errorf("the note names no rejected option with the fact that rules it out")
	}
}

func TestC2290_003_TheNoteStatesThatNoPersonaOrPromptChangesAndNoPersonaFileUnderAgentsChanges(t *testing.T) {
	_, text := itemNote(t)
	if !changeVerb.MatchString(text) {
		t.Fatalf("RED: the note does not state that no persona or prompt changes")
	}
	root := acsassert.RepoRoot(t)
	changed, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", diffBase(t), "--", "agents")
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	if strings.TrimSpace(changed) != "" {
		t.Errorf("persona files changed although the note changes none:\n%s", changed)
	}
	added, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "agents")
	if strings.TrimSpace(added) != "" {
		t.Errorf("untracked files appeared under agents:\n%s", added)
	}
}

func TestC2290_004_TheNoteTiesTheNoChangeVerdictToTheClaudeNotesFileThatStaysUnchanged(t *testing.T) {
	_, text := itemNote(t)
	if !strings.Contains(text, claudeNotesPath) {
		t.Fatalf("RED: the note does not name %s and its verdict", claudeNotesPath)
	}
	if !regexp.MustCompile(`(?i)` + regexp.QuoteMeta(claudeNotesPath) + `[^\n]*(unchanged|no change|does not change|stays)`).MatchString(text) {
		t.Errorf("the note does not say that %s stays unchanged", claudeNotesPath)
	}
	root := acsassert.RepoRoot(t)
	changed, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", diffBase(t), "--", claudeNotesPath)
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	if strings.TrimSpace(changed) != "" {
		t.Errorf("%s changed although no model-behavior line was found", claudeNotesPath)
	}
}

func TestC2290_005_TheResearchIndexHasAnEntryThatLinksTheNote(t *testing.T) {
	rel, _ := itemNote(t)
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

func TestC2290_006_TheNoteHoldsNoPlaceholderTextAndPassesTheSentenceLint(t *testing.T) {
	rel, text := itemNote(t)
	if hit := placeholder.FindString(text); hit != "" {
		t.Errorf("the note holds placeholder text %q", hit)
	}
	if findings := steFindings(t, rel); findings != "" {
		t.Errorf("the note fails the sentence lint:\n%s", findings)
	}
}
