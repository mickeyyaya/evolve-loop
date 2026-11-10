//go:build acs

package cycle2278

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	claudeNotesPath = "skills/model-notes-claude/COMPACT.md"
	itemMarker      = "bc673fd8"
	sourceURL       = "https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices"
	snapshotMark    = "2026-11-06T02:47:04Z"
	adrNumber       = "ADR-0133"
	notesCeiling    = 12
)

var originalBullets = []string{
	"- State the goal, the done criteria and the limits in the first lines. Claude models follow explicit scope closely.",
	"- Give the reason with each rule. A rule with a reason generalizes better than a bare order.",
	"- Use one clear instruction for each step. Do not stack contradictory rules.",
	"- Run independent tool calls in one turn.",
	"- Verify with a command before you claim done, and report only what the command printed.",
	"- Claude Fable 5.2 and Claude Mythos 5.2 return a 400 error when you edit the system prompt, the tools or an earlier message. This holds when a thinking block follows the edit. Send the history back unchanged.",
}

var corePersonas = []string{
	"evolve-scout",
	"evolve-triage",
	"evolve-tdd-engineer",
	"evolve-builder",
	"evolve-auditor",
	"evolve-retrospective",
}

var factTopics = []string{"effort", "always on", "answer directly", "medication", "truncated"}

var (
	effortBullet   = regexp.MustCompile(`(?i)^- .*\beffort\b`)
	factRow        = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	copiedText     = regexp.MustCompile(`(?i)(dosing block|over-the-counter|DailyMed|pharmacist|exactly as written)`)
	placeholder    = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	noThinkOrder   = regexp.MustCompile(`(?i)(do not think|don't think|without thinking|without deliberat|skip (the )?thinking|no thinking)`)
	noChangeNotice = regexp.MustCompile(`(?i)(no persona|change no persona|no prompt)`)
)

func repoPath(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel))
}

func read(t *testing.T, rel string) string {
	t.Helper()
	text, stderr, code, err := acsassert.SubprocessOutput("cat", repoPath(t, rel))
	if err != nil || code != 0 {
		t.Fatalf("RED: %s is unreadable (exit %d): %v %s", rel, code, err, stderr)
	}
	return text
}

func lintReport(t *testing.T, rel string) string {
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
	var report []string
	for _, f := range found {
		if !f.IsSkipNotice() {
			report = append(report, f.Rule+" line "+strings.TrimSpace(f.Excerpt))
		}
	}
	return strings.Join(report, "\n")
}

func effortBullets(text string) []string {
	var hits []string
	for _, line := range strings.Split(text, "\n") {
		if effortBullet.MatchString(line) {
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
	return rel, read(t, rel)
}

func TestC2278_001_TheClaudeNotesGainOneEffortBulletThatNamesFable52Mythos52AndLoweringTheLevelInSteEnglish(t *testing.T) {
	text := read(t, claudeNotesPath)
	bullets := effortBullets(text)
	if len(bullets) != 1 {
		t.Fatalf("RED: want exactly one bullet that names effort in %s, got %d", claudeNotesPath, len(bullets))
	}
	bullet := bullets[0]
	lower := strings.ToLower(bullet)
	for _, need := range []string{"fable 5.2", "mythos 5.2", "thinking", "lower", "medium"} {
		if !strings.Contains(lower, need) {
			t.Errorf("the effort bullet does not name %q: %q", need, bullet)
		}
	}
	if hit := copiedText.FindString(bullet); hit != "" {
		t.Errorf("the effort bullet copies guide text: %q", hit)
	}
	if report := lintReport(t, claudeNotesPath); report != "" {
		t.Errorf("the notes fail the sentence lint:\n%s", report)
	}
}

func TestC2278_002_TheNotesKeepTheirSixBulletsAndStayUnderTheLineCeiling(t *testing.T) {
	text := read(t, claudeNotesPath)
	if len(effortBullets(text)) != 1 {
		t.Fatalf("RED: the notes have no single effort bullet to check around")
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

func TestC2278_003_TheNoteReadsTheSourceAtTheLinkAndExplainsEachGuideLineAsUntrustedData(t *testing.T) {
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
		t.Errorf("the note does not say that the guide text is untrusted data")
	}
	rows := factRow.FindAllString(text, -1)
	if len(rows) < len(factTopics) {
		t.Fatalf("the note explains %d guide lines in F rows, want at least %d", len(rows), len(factTopics))
	}
	joined := strings.ToLower(strings.Join(rows, "\n"))
	for _, topic := range factTopics {
		if !strings.Contains(joined, topic) {
			t.Errorf("no F row explains the %s lines of the guide", topic)
		}
	}
	if report := lintReport(t, rel); report != "" {
		t.Errorf("the note fails the sentence lint:\n%s", report)
	}
}

func TestC2278_004_TheProposalCitesTheAdrNamesTheChangedFileAndGivesEachCorePersonaANoChangeVerdictWithAReason(t *testing.T) {
	_, text := itemNote(t)
	if !strings.Contains(text, adrNumber) {
		t.Errorf("the note does not cite %s", adrNumber)
	}
	if !strings.Contains(text, claudeNotesPath) {
		t.Errorf("the note does not name the file that changes: %s", claudeNotesPath)
	}
	for _, persona := range corePersonas {
		row := regexp.MustCompile(`(?m)^\|[^\n]*` + regexp.QuoteMeta(persona) + `[^\n]*\|[^\n]*change[^\n]*\|`).FindString(text)
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
	if !strings.Contains(text, "Rejected option") {
		t.Errorf("the note names no rejected option with the fact that rules it out")
	}
}

func TestC2278_005_TheResearchIndexHasAnEntryThatLinksTheNote(t *testing.T) {
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

func TestC2278_006_NoPersonaFileChangesAndNoPersonaHoldsADoNotThinkOrderThatTheNoChangeVerdictWouldHide(t *testing.T) {
	_, text := itemNote(t)
	if !noChangeNotice.MatchString(text) {
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
	personas, err := filepath.Glob(filepath.Join(root, "agents", "evolve-*.md"))
	if err != nil || len(personas) == 0 {
		t.Fatalf("no persona files found under agents: %v", err)
	}
	for _, path := range personas {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if hit := noThinkOrder.FindString(string(raw)); hit != "" {
			t.Errorf("%s holds the order %q, so the no-change verdict is false", filepath.Base(path), hit)
		}
	}
}

func TestC2278_007_NeitherTheNoteNorTheNotesHoldPlaceholderText(t *testing.T) {
	_, text := itemNote(t)
	for name, body := range map[string]string{"the note": text, claudeNotesPath: read(t, claudeNotesPath)} {
		if hit := placeholder.FindString(body); hit != "" {
			t.Errorf("%s holds placeholder text %q", name, hit)
		}
	}
}

func TestC2278_008_NoPathTheCycleChangesIsAProtectedScopeSoNoAcNeedsAConsoleFollowup(t *testing.T) {
	rel, _ := itemNote(t)
	for _, path := range []string{claudeNotesPath, rel, "docs/research-index/", ".evolve/evals/"} {
		if guards.IsProtectedScope(path) {
			t.Errorf("%s is a protected scope, a lane builder cannot change it", path)
		}
	}
}
