//go:build acs

package cycle2276

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
	itemMarker      = "9cd2c891"
	sourceURL       = "https://platform.claude.com/docs/en/release-notes/overview"
	snapshotMark    = "2026-11-02T17:20:05Z"
	adrNumber       = "ADR-0133"
	notesCeiling    = 12
)

var originalBullets = []string{
	"- State the goal, the done criteria and the limits in the first lines. Claude models follow explicit scope closely.",
	"- Give the reason with each rule. A rule with a reason generalizes better than a bare order.",
	"- Use one clear instruction for each step. Do not stack contradictory rules.",
	"- Run independent tool calls in one turn.",
	"- Verify with a command before you claim done, and report only what the command printed.",
}

var corePersonas = []string{
	"evolve-scout",
	"evolve-triage",
	"evolve-tdd-engineer",
	"evolve-builder",
	"evolve-auditor",
	"evolve-retrospective",
}

var factTopics = []string{"prefix", "advisor", "allowlist", "watermark", "fast mode"}

var (
	modelBullet = regexp.MustCompile(`^- .*Fable 5\.2`)
	factRow     = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	copiedText  = regexp.MustCompile(`(?i)(whatever its age|always-on|cyber verification|2\.5x|sign a member)`)
	placeholder = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
)

func fileText(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("RED: %s is unreadable: %v", rel, err)
	}
	return string(data)
}

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
	for _, f := range found {
		if !f.IsSkipNotice() {
			lines = append(lines, rel+": "+f.Rule+": "+f.Message+": "+f.Excerpt)
		}
	}
	return lines
}

func modelBullets(text string) []string {
	var hits []string
	for _, line := range strings.Split(text, "\n") {
		if modelBullet.MatchString(line) {
			hits = append(hits, line)
		}
	}
	return hits
}

func itemNote(t *testing.T) (rel, text string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	names, err := acsassert.WalkModule(filepath.Join(root, "docs", "research"), func(name string) bool {
		if strings.Contains(name, "/") || !strings.HasSuffix(name, ".md") {
			return false
		}
		body, readErr := os.ReadFile(filepath.Join(root, "docs", "research", name))
		return readErr == nil && strings.Contains(string(body), itemMarker)
	})
	if err != nil {
		t.Fatalf("walk docs/research: %v", err)
	}
	if len(names) != 1 {
		t.Fatalf("RED: want exactly one note in docs/research that names the item %s, got %d", itemMarker, len(names))
	}
	rel = "docs/research/" + names[0]
	return rel, fileText(t, rel)
}

func TestC2276_001_TheClaudeNotesGainOneFable52BulletThatNamesMythos52ThinkingAndTheErrorInSteEnglish(t *testing.T) {
	text := fileText(t, claudeNotesPath)
	bullets := modelBullets(text)
	if len(bullets) != 1 {
		t.Fatalf("RED: want exactly one bullet that names Fable 5.2 in %s, got %d", claudeNotesPath, len(bullets))
	}
	bullet := bullets[0]
	lower := strings.ToLower(bullet)
	for _, need := range []string{"mythos 5.2", "thinking", "400"} {
		if !strings.Contains(lower, need) {
			t.Errorf("the Fable 5.2 bullet does not name %q: %q", need, bullet)
		}
	}
	if !strings.Contains(lower, "unchanged") {
		t.Errorf("the Fable 5.2 bullet does not tell the reader to send the history back unchanged: %q", bullet)
	}
	if hit := copiedText.FindString(bullet); hit != "" {
		t.Errorf("the Fable 5.2 bullet copies release-note text: %q", hit)
	}
	if findings := steFindings(t, claudeNotesPath); len(findings) != 0 {
		t.Errorf("the notes fail the sentence lint:\n%s", strings.Join(findings, "\n"))
	}
}

func TestC2276_002_TheNotesKeepTheirFiveBulletsAndStayUnderTheLineCeiling(t *testing.T) {
	text := fileText(t, claudeNotesPath)
	if len(modelBullets(text)) != 1 {
		t.Fatalf("RED: the notes have no single Fable 5.2 bullet to check around")
	}
	for _, bullet := range originalBullets {
		if n := strings.Count(text, bullet+"\n"); n != 1 {
			t.Errorf("the original bullet appears %d times, want 1: %q", n, bullet)
		}
	}
	if lines := len(strings.Split(strings.TrimRight(text, "\n"), "\n")); lines > notesCeiling {
		t.Errorf("%s has %d lines, the ceiling is %d", claudeNotesPath, lines, notesCeiling)
	}
}

func TestC2276_003_TheNoteReadsTheSourceAtTheLinkAndExplainsTheLinesAsUntrustedData(t *testing.T) {
	rel, text := itemNote(t)
	root := acsassert.RepoRoot(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1)", rel, code)
	}
	for _, need := range []string{sourceURL, snapshotMark, "Fable 5.2", "Mythos 5.2"} {
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
	if findings := steFindings(t, rel); len(findings) != 0 {
		t.Errorf("the note fails the sentence lint:\n%s", strings.Join(findings, "\n"))
	}
}

func TestC2276_004_TheProposalCitesTheAdrNamesTheChangedFileAndGivesEachCorePersonaANoChangeVerdictWithAReason(t *testing.T) {
	_, text := itemNote(t)
	if !strings.Contains(text, adrNumber) {
		t.Errorf("the note does not cite %s", adrNumber)
	}
	if !strings.Contains(text, claudeNotesPath) {
		t.Errorf("the note does not name the file that changes: %s", claudeNotesPath)
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

func TestC2276_005_TheResearchIndexHasAnEntryThatLinksTheNote(t *testing.T) {
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

func TestC2276_006_NoPersonaFileUnderAgentsChangesBecauseTheProposalChangesNone(t *testing.T) {
	_, text := itemNote(t)
	if !regexp.MustCompile(`(?i)(no persona|change no persona|no prompt)`).MatchString(text) {
		t.Fatalf("RED: the note does not state that no persona or prompt changes")
	}
	root := acsassert.RepoRoot(t)
	base := "origin/main"
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "rev-parse", "--verify", "--quiet", base); code != 0 {
		base = "HEAD"
	}
	changed, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", base, "--", "agents")
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	if strings.TrimSpace(changed) != "" {
		t.Errorf("persona files changed although the proposal changes none:\n%s", changed)
	}
	added, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "agents")
	if strings.TrimSpace(added) != "" {
		t.Errorf("untracked files appeared under agents:\n%s", added)
	}
}

func TestC2276_007_NeitherTheNoteNorTheNotesHoldPlaceholderText(t *testing.T) {
	_, text := itemNote(t)
	for name, body := range map[string]string{"the note": text, claudeNotesPath: fileText(t, claudeNotesPath)} {
		if hit := placeholder.FindString(body); hit != "" {
			t.Errorf("%s holds placeholder text %q", name, hit)
		}
	}
}
