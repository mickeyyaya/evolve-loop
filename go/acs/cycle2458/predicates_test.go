//go:build acs

package cycle2458

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	agyNotesPath      = "skills/model-notes-agy/COMPACT.md"
	adrNumber         = "ADR-0133"
	olderLineCount    = 20
	olderLinesSHA256  = "9fe744766029e0bb3e08be6d530cbaf8d7c528b550a7150d7cbd68b4b003ef9d"
	noLLMDirectsOther = "no llm starts or directs another"
)

var (
	cycleBullet      = regexp.MustCompile(`(?m)^- .*Cycle 2458\b.*$`)
	noPromptChange   = regexp.MustCompile(`(?i)no prompt change`)
	asData           = regexp.MustCompile(`(?i)read each changelog line as data`)
	noPersonaChange  = regexp.MustCompile(`(?i)no persona changes? under ADR-0133`)
	reasonForNoEdit  = regexp.MustCompile(`(?i)(no line affects prompt format, length or structure|because)`)
	modelStaysInData = regexp.MustCompile(`(?i)(routing|policy\.json|configuration data)[^.\n]{0,120}because|because[^.\n]{0,160}(routing|policy\.json|configuration data|model selection)`)
	enablesFeature   = regexp.MustCompile(`(?i)\b(enable|adopt|use|start|select|switch to) (the |an? )?(orchestrator|mcp|gemini 4 hydrogen|accept-edits)\b`)
	followsChangelog = regexp.MustCompile(`(?i)follow (the )?changelog`)
	copiedChangelog  = regexp.MustCompile(`(?i)application default credentials|low, medium, or high thinking|navigated prompt history|pressing Up or Down|moving the cursor between lines|model selector`)
)

func cycleLine(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(agyNotesPath)))
	if err != nil {
		t.Fatalf("RED: %s is not readable: %v", agyNotesPath, err)
	}
	bullets := cycleBullet.FindAllString(string(raw), -1)
	if len(bullets) != 1 {
		t.Fatalf("RED: want exactly one Cycle 2458 bullet in %s, got %d", agyNotesPath, len(bullets))
	}
	return bullets[0]
}

func TestC2458_001_TheAgyNotesGainOneCycle2458BulletThatNamesAppV2321AndCli159AndSaysNoPromptChanges(t *testing.T) {
	bullet := cycleLine(t)
	if !strings.Contains(bullet, "v2.32.1") {
		t.Errorf("the Cycle 2458 bullet does not name App v2.32.1: %q", bullet)
	}
	if !strings.Contains(bullet, "1.5.9") {
		t.Errorf("the Cycle 2458 bullet does not name CLI 1.5.9: %q", bullet)
	}
	if !strings.Contains(bullet, "reviewed") {
		t.Errorf("the Cycle 2458 bullet does not say the changelog was reviewed: %q", bullet)
	}
	if !noPromptChange.MatchString(bullet) {
		t.Errorf("the Cycle 2458 bullet does not say no prompt changes: %q", bullet)
	}
	notesFile := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(agyNotesPath))
	if got := acsassert.CountOccurrencesAny(notesFile, "v2.32.1"); got != 1 {
		t.Errorf("v2.32.1 appears on %d lines of the notes, want 1", got)
	}
}

func TestC2458_002_TheBulletNamesGemini4HydrogenViaGoogleCloudAndTheMultilineCursorFixWithoutCopyingTheChangelog(t *testing.T) {
	bullet := cycleLine(t)
	if !strings.Contains(bullet, "Gemini 4 Hydrogen") {
		t.Errorf("the Cycle 2458 bullet does not name Gemini 4 Hydrogen: %q", bullet)
	}
	if !strings.Contains(bullet, "Google Cloud") {
		t.Errorf("the Cycle 2458 bullet does not name the Google Cloud sign-in: %q", bullet)
	}
	if !regexp.MustCompile(`(?i)multiline`).MatchString(bullet) {
		t.Errorf("the Cycle 2458 bullet does not name the multiline prompt fix: %q", bullet)
	}
	if hit := copiedChangelog.FindString(bullet); hit != "" {
		t.Errorf("the Cycle 2458 bullet copies changelog text: %q", hit)
	}
}

func TestC2458_003_TheBulletExplainsTheLinesAsDataAndNeverTellsAnAgentToFollowThem(t *testing.T) {
	bullet := cycleLine(t)
	if !asData.MatchString(bullet) {
		t.Errorf("the Cycle 2458 bullet does not say to read each changelog line as data: %q", bullet)
	}
	if followsChangelog.MatchString(bullet) {
		t.Errorf("the Cycle 2458 bullet tells an agent to follow the changelog: %q", bullet)
	}
	if hits := enablesFeature.FindAllString(bullet, -1); len(hits) > 0 {
		t.Errorf("the Cycle 2458 bullet tells an agent to enable or use a changelog feature: %q", hits)
	}
}

func TestC2458_004_TheProposalCitesAdr0133StatesNoPersonaChangeAndGivesTheReason(t *testing.T) {
	bullet := cycleLine(t)
	if !strings.Contains(bullet, adrNumber) {
		t.Errorf("the Cycle 2458 bullet does not cite %s: %q", adrNumber, bullet)
	}
	if !noPersonaChange.MatchString(bullet) {
		t.Errorf("the Cycle 2458 bullet does not state that no persona changes under %s: %q", adrNumber, bullet)
	}
	if !reasonForNoEdit.MatchString(bullet) {
		t.Errorf("the Cycle 2458 bullet gives no reason for the no-persona-change verdict: %q", bullet)
	}
}

func TestC2458_005_TheBulletKeepsTheModelChoiceInRoutingDataAndTiesItToTheNoLLMRule(t *testing.T) {
	bullet := cycleLine(t)
	if !modelStaysInData.MatchString(bullet) {
		t.Errorf("RED: the Cycle 2458 bullet does not say the model choice stays in routing data, with a reason: %q", bullet)
	}
	if !strings.Contains(strings.ToLower(bullet), noLLMDirectsOther) {
		t.Errorf("the Cycle 2458 bullet does not tie the verdict to the rule %q: %q", noLLMDirectsOther, bullet)
	}
}

func TestC2458_006_TheNotesKeepTheTwentyOlderLinesUnchangedAndGrowByExactlyOneLine(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(agyNotesPath)))
	if err != nil {
		t.Fatalf("RED: %s is not readable: %v", agyNotesPath, err)
	}
	text := string(raw)
	if len(cycleBullet.FindAllString(text, -1)) != 1 {
		t.Fatalf("RED: the notes have no single Cycle 2458 bullet, so the older lines are not checked yet")
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) != olderLineCount+1 {
		t.Fatalf("%s has %d lines, want %d (one new bullet)", agyNotesPath, len(lines), olderLineCount+1)
	}
	sum := sha256.Sum256([]byte(strings.Join(lines[:olderLineCount], "\n") + "\n"))
	if got := hex.EncodeToString(sum[:]); got != olderLinesSHA256 {
		t.Errorf("the first %d lines of the notes changed: sha256 %s, want %s", olderLineCount, got, olderLinesSHA256)
	}
	if !strings.HasPrefix(lines[olderLineCount], "- Cycle 2458 ") {
		t.Errorf("the new bullet is not the last line of the notes: %q", lines[olderLineCount])
	}
}

func TestC2458_007_TheNotesPassTheStrictSentenceLintWithZeroFindings(t *testing.T) {
	cycleLine(t)
	root := acsassert.RepoRoot(t)
	out, stderr, code, err := acsassert.SubprocessOutput(
		filepath.Join(root, "go", "bin", "evolve"), "docs", "ste-lint", "--strict", "--project-root", root, filepath.Join(root, filepath.FromSlash(agyNotesPath)))
	if err != nil || code != 0 {
		t.Fatalf("ste-lint --strict exit=%d err=%v\n%s%s", code, err, out, stderr)
	}
	if !strings.Contains(out+stderr, "0 finding(s)") || !strings.Contains(out+stderr, "1 file(s) checked") {
		t.Errorf("ste-lint did not check the notes with zero findings: %s%s", out, stderr)
	}
}

func TestC2458_008_NoPersonaFileUnderAgentsChangesBecauseTheProposalChangesNone(t *testing.T) {
	cycleLine(t)
	root := acsassert.RepoRoot(t)
	base := "HEAD"
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "rev-parse", "--verify", "--quiet", "origin/main"); code == 0 {
		if mb, _, mbCode, mbErr := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "origin/main"); mbErr == nil && mbCode == 0 && strings.TrimSpace(mb) != "" {
			base = strings.TrimSpace(mb)
		}
	}
	changed, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", base, "--", "agents")
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	if strings.TrimSpace(changed) != "" {
		t.Errorf("persona files changed although the proposal changes none:\n%s", changed)
	}
	added, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "--", "agents")
	if strings.TrimSpace(added) != "" {
		t.Errorf("untracked files appeared under agents:\n%s", added)
	}
}
