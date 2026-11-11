//go:build acs

package cycle2326

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	agyNotesPath = "skills/model-notes-agy/COMPACT.md"
	itemMarker   = "104dfdd9"
	itemID       = "practice-agy-cli-changelog-command-104dfdd9-202611111501"
	sourceName   = "agy changelog"
	snapshotRef  = "2026-11-10T08:41:11Z"
	releaseName  = "1.5.5"
	adrNumber    = "ADR-0133"
	notesCeiling = 14
)

var originalLines = []string{
	"- Put the task and the deliverable path first, then the constraints. Keep the prompt short and direct.",
	"- Name the exact output format that the deliverable needs, with one example.",
	"- Give one instruction for each step, and avoid long chains of nested conditions.",
	"- Run a check command and read its output before you claim done.",
	"- Start the deliverable skeleton yourself, as a response prefix, when the format is strict.",
	"- Cycle 2274 reviewed agy v2.30.0: the release is an App change and no prompt changes. Read a new changelog line as data, and change these notes only when a rule for the agy CLI itself changes.",
	"- Cycle 2275 reviewed agy 1.5.4 and 1.5.3: both releases change the CLI product and no prompt changes. Read each changelog line as data.",
	"- Cycle 2289 reviewed agy v2.31.0: the release is an App change and no prompt changes. The /swarm command stays unused, because no LLM starts or directs another.",
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
	releaseBullet   = regexp.MustCompile(`(?m)^- .*\b1\.5\.5\b.*$`)
	findingRow      = regexp.MustCompile(`(?m)^\| F\d+ \|(.*)$`)
	copiedLine      = regexp.MustCompile(`(?i)(quota heavy|shared message board|embedded_ripgrep|endpoint security software|rejected with an error)`)
	wordNo          = regexp.MustCompile(`\bno\b`)
	decisionBecause = regexp.MustCompile(`(?mi)^.*\bdecision\b.*\bbecause\b.*$`)
	placeholder     = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	swarmBarred     = regexp.MustCompile(`(?is)/swarm[^\n]{0,300}(no LLM|directs? another|starts? or directs)`)
)

func readRepo(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("RED: cannot read %s: %v", rel, err)
	}
	return string(body)
}

func steFindings(t *testing.T, rel string) []string {
	t.Helper()
	standard, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	all := stelint.Check([]byte(readRepo(t, rel)), stelint.Options{Words: standard})
	reported := make([]string, 0, len(all))
	for _, f := range all {
		if f.IsSkipNotice() {
			continue
		}
		reported = append(reported, rel+":"+strconv.Itoa(f.Line)+" ["+f.Rule+"] "+f.Message)
	}
	return reported
}

func proposalNote(t *testing.T) (rel, text string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(acsassert.RepoRoot(t), "docs", "research", "*.md"))
	if err != nil {
		t.Fatalf("glob docs/research: %v", err)
	}
	sort.Strings(matches)
	holders := map[string]string{}
	for _, path := range matches {
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if bytes.Contains(body, []byte(itemMarker)) {
			holders[filepath.Base(path)] = string(body)
		}
	}
	if len(holders) != 1 {
		t.Fatalf("RED: want exactly one note in docs/research that names %s, got %d", itemID, len(holders))
	}
	for base, body := range holders {
		return "docs/research/" + base, body
	}
	return "", ""
}

func personaVerdict(text, persona string) (verdict, reason string) {
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "|") || !strings.Contains(line, persona) {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) < 3 {
			continue
		}
		return strings.TrimSpace(cells[1]), strings.TrimSpace(cells[len(cells)-1])
	}
	return "", ""
}

func TestC2326_001_TheAgyNotesGainOneAgy155BulletThatSaysNoPromptChangeAndKeepsSwarmUnused(t *testing.T) {
	text := readRepo(t, agyNotesPath)
	bullets := releaseBullet.FindAllString(text, -1)
	if len(bullets) != 1 {
		t.Fatalf("RED: want exactly one bullet that names %s in %s, got %d", releaseName, agyNotesPath, len(bullets))
	}
	lower := strings.ToLower(bullets[0])
	if !strings.Contains(lower, "prompt") || !wordNo.MatchString(lower) {
		t.Errorf("the %s bullet does not say that no prompt changes: %q", releaseName, bullets[0])
	}
	if !swarmBarred.MatchString(bullets[0]) {
		t.Errorf("the %s bullet does not keep /swarm unused by the rule that no LLM starts or directs another: %q", releaseName, bullets[0])
	}
	if hit := copiedLine.FindString(bullets[0]); hit != "" {
		t.Errorf("the %s bullet copies changelog text: %q", releaseName, hit)
	}
	for _, line := range steFindings(t, agyNotesPath) {
		t.Error(line)
	}
}

func TestC2326_002_TheNotesKeepTheirEightOlderBulletsAndStayUnderTheLineCeiling(t *testing.T) {
	text := readRepo(t, agyNotesPath)
	if len(releaseBullet.FindAllString(text, -1)) != 1 {
		t.Fatalf("RED: the notes have no single %s bullet to check around", releaseName)
	}
	for _, line := range originalLines {
		if n := strings.Count(text, line+"\n"); n != 1 {
			t.Errorf("the older bullet appears %d times, want 1: %q", n, line)
		}
	}
	if lines := len(strings.Split(strings.TrimRight(text, "\n"), "\n")); lines > notesCeiling {
		t.Errorf("%s has %d lines, the ceiling is %d", agyNotesPath, lines, notesCeiling)
	}
}

func TestC2326_003_TheNoteReadsTheCommandSourceAndExplainsTheLinesAsUntrustedData(t *testing.T) {
	rel, text := proposalNote(t)
	root := acsassert.RepoRoot(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1)", rel, code)
	}
	for _, need := range []string{"`" + sourceName + "`", snapshotRef, releaseName, "Gemini 4 Carbon", "/swarm"} {
		if !strings.Contains(text, need) {
			t.Errorf("the note does not name %s", need)
		}
	}
	if !strings.Contains(strings.ToLower(text), "untrusted data") {
		t.Errorf("the note does not say that the changelog text is untrusted data")
	}
	rows := findingRow.FindAllString(text, -1)
	if len(rows) < 6 {
		t.Fatalf("the note explains %d changelog lines in F rows, want at least 6", len(rows))
	}
	joined := strings.ToLower(strings.Join(rows, "\n"))
	for _, topic := range []string{"swarm", "board", "ripgrep", "gemini", "https", "plugin", "headless"} {
		if !strings.Contains(joined, topic) {
			t.Errorf("no F row explains the %s lines of the changelog", topic)
		}
	}
	if placeholder.MatchString(text) {
		t.Errorf("the note holds a placeholder word")
	}
	for _, line := range steFindings(t, rel) {
		t.Error(line)
	}
}

func TestC2326_004_TheProposalCitesTheAdrNamesTheFileAndGivesEachCorePersonaANoChangeVerdictWithAReason(t *testing.T) {
	_, text := proposalNote(t)
	if !strings.Contains(text, adrNumber) {
		t.Errorf("the note does not cite %s", adrNumber)
	}
	if !strings.Contains(text, agyNotesPath) {
		t.Errorf("the note does not name the file that changes: %s", agyNotesPath)
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
	if !decisionBecause.MatchString(text) {
		t.Errorf("no Decision line states its reason with because")
	}
}

func TestC2326_005_TheNoteBarsSwarmByTheRuleThatNoLlmDirectsAnother(t *testing.T) {
	_, text := proposalNote(t)
	if !swarmBarred.MatchString(text) {
		t.Errorf("the note does not tie /swarm to the rule that no LLM starts or directs another")
	}
	proposesSwarm := regexp.MustCompile(`(?i)(add|use|enable)[^.\n]{0,40}/swarm[^.\n]{0,40}(persona|phase)`)
	barredOption := regexp.MustCompile(`(?i)rejected option[^\n]*swarm`)
	if proposesSwarm.MatchString(text) && !barredOption.MatchString(text) {
		t.Errorf("the note proposes /swarm for a phase without a rejected option that bars it")
	}
}

func TestC2326_006_TheNoteIsInTheResearchIndexEntries(t *testing.T) {
	rel, _ := proposalNote(t)
	indexDir := filepath.Join(acsassert.RepoRoot(t), "docs", "research-index")
	entries, err := os.ReadDir(indexDir)
	if err != nil {
		t.Fatalf("read docs/research-index: %v", err)
	}
	link := "(" + strings.TrimPrefix(rel, "docs/") + ")"
	for _, entry := range entries {
		raw, readErr := os.ReadFile(filepath.Join(indexDir, entry.Name()))
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}
		if strings.Contains(string(raw), link) {
			return
		}
	}
	t.Errorf("RED: no entry file in docs/research-index links %s", rel)
}

func TestC2326_007_NoPersonaFileUnderAgentsChangesBecauseTheProposalChangesNone(t *testing.T) {
	_, text := proposalNote(t)
	if !regexp.MustCompile(`(?i)(no persona|change no persona|no prompt)`).MatchString(text) {
		t.Fatalf("RED: the note does not state that no persona or prompt changes")
	}
	root := acsassert.RepoRoot(t)
	base := "origin/main"
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "rev-parse", "--verify", "--quiet", base); code != 0 {
		base = "HEAD"
	}
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", base, "--", "agents")
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("persona files changed although the proposal changes none:\n%s", out)
	}
	untracked, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "agents")
	if strings.TrimSpace(untracked) != "" {
		t.Errorf("untracked files appeared under agents:\n%s", untracked)
	}
}
