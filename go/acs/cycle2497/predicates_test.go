//go:build acs

package cycle2497

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
	changelogLink     = "https://antigravity.google/docs/changelog?tab=cli"
	adrNumber         = "ADR-0133"
	olderLineCount    = 22
	olderLinesSHA256  = "7fd8633f33473145f6d3d364a540cb056f70eaf3f48c14d15963d9d1fff81ad0"
	noLLMDirectsOther = "no llm starts or directs another"
)

var (
	cycleBullet      = regexp.MustCompile(`(?m)^- .*Cycle 2497\b.*$`)
	noPromptChange   = regexp.MustCompile(`(?i)no prompt change`)
	asData           = regexp.MustCompile(`(?i)read each (changelog )?line as data`)
	noPersonaChange  = regexp.MustCompile(`(?i)no persona changes? under ADR-0133`)
	reasonForNoEdit  = regexp.MustCompile(`(?i)(no line affects prompt format, length or structure|because)`)
	swarmStaysUnused = regexp.MustCompile(`(?i)/swarm[^.\n]{0,80}stays unused[^.\n]{0,40}because`)
	prLine           = regexp.MustCompile(`(?i)\b(pull requests?|PR)\b`)
	browserSearchRe  = regexp.MustCompile(`(?i)browser search`)
	enablesFeature   = regexp.MustCompile(`(?i)\b(enable|adopt|use|start|select|switch to) (the |an? )?(/?swarm|mcp|ssh|accept-edits)\b`)
	followsChangelog = regexp.MustCompile(`(?i)follow (the )?changelog`)
	copiedChangelog  = regexp.MustCompile(`(?i)remote 64-bit linux|~/\.ssh/config|hover cards|nord dark|syntax highlighting|16 improvements|21 fixes|cloudflare|grafana|sonarqube|200 MB|\.amr`)
)

func notesLines(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(agyNotesPath)))
	if err != nil {
		t.Fatalf("RED: %s is not readable: %v", agyNotesPath, err)
	}
	return strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
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
		t.Fatalf("RED: want exactly one Cycle 2497 bullet in %s, got %d", agyNotesPath, len(bullets))
	}
	return bullets[0]
}

func TestC2497_001_TheAgyNotesGainOneCycle2497BulletThatNamesAppV2330ReadsTheSourceLinkAndSaysNoPromptChanges(t *testing.T) {
	bullet := cycleLine(t)
	if !strings.Contains(bullet, "v2.33.0") {
		t.Errorf("the Cycle 2497 bullet does not name App v2.33.0: %q", bullet)
	}
	if !strings.Contains(bullet, changelogLink) {
		t.Errorf("the Cycle 2497 bullet does not name the source %s: %q", changelogLink, bullet)
	}
	if !strings.Contains(bullet, "reviewed") {
		t.Errorf("the Cycle 2497 bullet does not say the changelog was reviewed: %q", bullet)
	}
	if !noPromptChange.MatchString(bullet) {
		t.Errorf("the Cycle 2497 bullet does not say no prompt changes: %q", bullet)
	}
}

func TestC2497_002_TheBulletNamesTheSshPullRequestBrowserSearchAndSwarmLinesWithoutCopyingTheChangelog(t *testing.T) {
	bullet := cycleLine(t)
	if !strings.Contains(bullet, "SSH") {
		t.Errorf("the Cycle 2497 bullet does not name the SSH host connections: %q", bullet)
	}
	if !prLine.MatchString(bullet) {
		t.Errorf("the Cycle 2497 bullet does not name the pull request status lines: %q", bullet)
	}
	if !browserSearchRe.MatchString(bullet) {
		t.Errorf("the Cycle 2497 bullet does not name the browser search line: %q", bullet)
	}
	if !strings.Contains(bullet, "/swarm") {
		t.Errorf("the Cycle 2497 bullet does not name the /swarm command: %q", bullet)
	}
	if hit := copiedChangelog.FindString(bullet); hit != "" {
		t.Errorf("the Cycle 2497 bullet copies changelog text: %q", hit)
	}
}

func TestC2497_003_TheBulletExplainsTheLinesAsDataAndNeverTellsAnAgentToFollowThem(t *testing.T) {
	bullet := cycleLine(t)
	if !asData.MatchString(bullet) {
		t.Errorf("the Cycle 2497 bullet does not say to read each changelog line as data: %q", bullet)
	}
	if followsChangelog.MatchString(bullet) {
		t.Errorf("the Cycle 2497 bullet tells an agent to follow the changelog: %q", bullet)
	}
	if hits := enablesFeature.FindAllString(bullet, -1); len(hits) > 0 {
		t.Errorf("the Cycle 2497 bullet tells an agent to enable or use a changelog feature: %q", hits)
	}
}

func TestC2497_004_TheProposalCitesAdr0133StatesNoPersonaChangeAndGivesTheReason(t *testing.T) {
	bullet := cycleLine(t)
	if !strings.Contains(bullet, adrNumber) {
		t.Errorf("the Cycle 2497 bullet does not cite %s: %q", adrNumber, bullet)
	}
	if !noPersonaChange.MatchString(bullet) {
		t.Errorf("the Cycle 2497 bullet does not state that no persona changes under %s: %q", adrNumber, bullet)
	}
	if !reasonForNoEdit.MatchString(bullet) {
		t.Errorf("the Cycle 2497 bullet gives no reason for the no-persona-change verdict: %q", bullet)
	}
}

func TestC2497_005_TheSwarmCommandStaysUnusedWithAReasonThatTiesToTheNoLLMRule(t *testing.T) {
	bullet := cycleLine(t)
	if !swarmStaysUnused.MatchString(bullet) {
		t.Errorf("RED: the Cycle 2497 bullet does not say the /swarm command stays unused, with a reason: %q", bullet)
	}
	if !strings.Contains(strings.ToLower(bullet), noLLMDirectsOther) {
		t.Errorf("the Cycle 2497 bullet does not tie the verdict to the rule %q: %q", noLLMDirectsOther, bullet)
	}
}

func TestC2497_006_TheNotesKeepTheTwentyTwoOlderLinesUnchangedAndGrowByExactlyOneLine(t *testing.T) {
	cycleLine(t)
	lines := notesLines(t)
	if len(lines) != olderLineCount+1 {
		t.Fatalf("%s has %d lines, want %d (one new bullet)", agyNotesPath, len(lines), olderLineCount+1)
	}
	sum := sha256.Sum256([]byte(strings.Join(lines[:olderLineCount], "\n") + "\n"))
	if got := hex.EncodeToString(sum[:]); got != olderLinesSHA256 {
		t.Errorf("the first %d lines of the notes changed: sha256 %s, want %s", olderLineCount, got, olderLinesSHA256)
	}
	if !strings.HasPrefix(lines[olderLineCount], "- Cycle 2497 ") {
		t.Errorf("the new bullet is not the last line of the notes: %q", lines[olderLineCount])
	}
}

func TestC2497_007_TheNotesPassTheStrictSentenceLintWithZeroFindings(t *testing.T) {
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

func TestC2497_008_NoPersonaFileUnderAgentsChangesBecauseTheProposalChangesNone(t *testing.T) {
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
