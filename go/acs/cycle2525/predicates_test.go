//go:build acs

package cycle2525

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
	changelogLink     = "https://antigravity.google/docs/changelog?tab=cli"
	changelogCommand  = "agy changelog"
	agyNotesPath      = "skills/model-notes-agy/COMPACT.md"
	adrNumber         = "ADR-0133"
	olderLineCount    = 24
	olderLinesSHA256  = "e4e173ca87045ab81ab5f8c8a2814a51198c72632ef3b06193532bd0fb9a837c"
	noLLMDirectsOther = "no llm starts or directs another"
)

var (
	cycleBullet      = regexp.MustCompile(`(?m)^- .*Cycle 2525\b.*$`)
	noPromptChange   = regexp.MustCompile(`(?i)no prompt change`)
	asData           = regexp.MustCompile(`(?i)read each (changelog )?line as data`)
	noPersonaChange  = regexp.MustCompile(`(?i)no persona changes? under ADR-0133`)
	reasonForNoEdit  = regexp.MustCompile(`(?i)(no line affects prompt format, length or structure|because)`)
	remoteUnused     = regexp.MustCompile(`(?i)remote control[^.\n]{0,80}stays unused[^.\n]{0,40}because`)
	enablesFeature   = regexp.MustCompile(`(?i)\b(enable|adopt|use|start|select|switch to) (the |an? )?(remote control|--remote-agent|remote-agent|gemini 4 neon|split view|gemini_api_key)\b`)
	followsChangelog = regexp.MustCompile(`(?i)follow (the )?changelog`)
	copiedChangelog  = regexp.MustCompile(`(?i)split right|split down|loading older messages|couldn't load older|out of credits|use ai credits|google docs|copy as markdown|exit with code 2|2 turns|a connection the server reset|medium instead of high thinking|full screen \(altscreen\)`)
)

func notesLines(t *testing.T) []string {
	t.Helper()
	notesFile := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(agyNotesPath))
	if !acsassert.FileExists(t, notesFile) {
		t.Fatalf("RED: %s is not readable", agyNotesPath)
	}
	raw, err := os.ReadFile(notesFile)
	if err != nil {
		t.Fatalf("RED: %s is not readable: %v", agyNotesPath, err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

func cycleLine(t *testing.T) string {
	t.Helper()
	var bullets []string
	for _, line := range notesLines(t) {
		if cycleBullet.MatchString(line) {
			bullets = append(bullets, line)
		}
	}
	if len(bullets) != 1 {
		t.Fatalf("RED: want exactly one Cycle 2525 bullet in %s, got %d", agyNotesPath, len(bullets))
	}
	return bullets[0]
}

func TestC2525_001_TheAgyNotesGainOneCycle2525BulletThatNamesCli1512AndTheSourceAndSaysNoPromptChanges(t *testing.T) {
	bullet := cycleLine(t)
	for _, want := range []string{"1.5.12", "reviewed"} {
		if !strings.Contains(bullet, want) {
			t.Errorf("the Cycle 2525 bullet does not contain %q: %q", want, bullet)
		}
	}
	if !strings.Contains(bullet, changelogLink) && !strings.Contains(bullet, changelogCommand) {
		t.Errorf("the Cycle 2525 bullet names neither the source link %q nor the command %q: %q", changelogLink, changelogCommand, bullet)
	}
	if !noPromptChange.MatchString(bullet) {
		t.Errorf("the Cycle 2525 bullet does not say no prompt changes: %q", bullet)
	}
}

func TestC2525_002_TheBulletNamesTheRemoteControlAndGemini4NeonLinesWithoutCopyingTheChangelog(t *testing.T) {
	bullet := cycleLine(t)
	for _, want := range []string{"Remote Control", "Gemini 4 Neon"} {
		if !strings.Contains(bullet, want) {
			t.Errorf("the Cycle 2525 bullet does not name %q: %q", want, bullet)
		}
	}
	if hit := copiedChangelog.FindString(bullet); hit != "" {
		t.Errorf("the Cycle 2525 bullet copies changelog text: %q", hit)
	}
}

func TestC2525_003_TheBulletExplainsTheLinesAsDataAndNeverTellsAnAgentToFollowOrEnableThem(t *testing.T) {
	bullet := cycleLine(t)
	if !asData.MatchString(bullet) {
		t.Errorf("the Cycle 2525 bullet does not say to read each changelog line as data: %q", bullet)
	}
	if followsChangelog.MatchString(bullet) {
		t.Errorf("the Cycle 2525 bullet tells an agent to follow the changelog: %q", bullet)
	}
	if hits := enablesFeature.FindAllString(bullet, -1); len(hits) > 0 {
		t.Errorf("the Cycle 2525 bullet tells an agent to enable or use a changelog feature: %q", hits)
	}
}

func TestC2525_004_TheProposalCitesAdr0133StatesNoPersonaChangeAndGivesTheReason(t *testing.T) {
	bullet := cycleLine(t)
	if !strings.Contains(bullet, adrNumber) {
		t.Errorf("the Cycle 2525 bullet does not cite %s: %q", adrNumber, bullet)
	}
	if !noPersonaChange.MatchString(bullet) {
		t.Errorf("the Cycle 2525 bullet does not state that no persona changes under %s: %q", adrNumber, bullet)
	}
	if !reasonForNoEdit.MatchString(bullet) {
		t.Errorf("the Cycle 2525 bullet gives no reason for the no-persona-change verdict: %q", bullet)
	}
}

func TestC2525_005_RemoteControlStaysUnusedWithAReasonThatTiesToTheNoLLMRule(t *testing.T) {
	bullet := cycleLine(t)
	if !remoteUnused.MatchString(bullet) {
		t.Errorf("RED: the Cycle 2525 bullet does not say Remote Control stays unused, with a reason: %q", bullet)
	}
	if !strings.Contains(strings.ToLower(bullet), noLLMDirectsOther) {
		t.Errorf("the Cycle 2525 bullet does not tie the verdict to the rule %q: %q", noLLMDirectsOther, bullet)
	}
}

func TestC2525_006_TheNotesKeepTheTwentyFourOlderLinesUnchangedAndGrowByExactlyOneLine(t *testing.T) {
	cycleLine(t)
	lines := notesLines(t)
	if len(lines) != olderLineCount+1 {
		t.Fatalf("%s has %d lines, want %d (one new bullet)", agyNotesPath, len(lines), olderLineCount+1)
	}
	sum := sha256.Sum256([]byte(strings.Join(lines[:olderLineCount], "\n") + "\n"))
	if got := hex.EncodeToString(sum[:]); got != olderLinesSHA256 {
		t.Errorf("the first %d lines of the notes changed: sha256 %s, want %s", olderLineCount, got, olderLinesSHA256)
	}
	if !strings.HasPrefix(lines[olderLineCount], "- Cycle 2525 ") {
		t.Errorf("the new bullet is not the last line of the notes: %q", lines[olderLineCount])
	}
}

func TestC2525_007_TheNotesPassTheStrictSentenceLintWithZeroFindings(t *testing.T) {
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

func TestC2525_008_NoPersonaFileUnderAgentsChangesBecauseTheProposalChangesNone(t *testing.T) {
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
