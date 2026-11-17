//go:build acs

package cycle2484

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
	olderLineCount    = 21
	olderLinesSHA256  = "6d9f31368663b9bcb2dd4b9514a571ad552c82f39cb614a5ffcc2d0b69a21ee3"
	noLLMDirectsOther = "no llm starts or directs another"
)

var (
	cycleBullet      = regexp.MustCompile(`(?m)^- .*Cycle 2484\b.*$`)
	noPromptChange   = regexp.MustCompile(`(?i)no prompt change`)
	asData           = regexp.MustCompile(`(?i)read each changelog line as data`)
	noPersonaChange  = regexp.MustCompile(`(?i)no persona changes? under ADR-0133`)
	reasonForNoEdit  = regexp.MustCompile(`(?i)(no line affects prompt format, length or structure|because)`)
	swarmStaysUnused = regexp.MustCompile(`(?i)/swarm[^.\n]{0,80}stays unused[^.\n]{0,40}because`)
	enablesFeature   = regexp.MustCompile(`(?i)\b(enable|adopt|use|start|select|switch to) (the |an? )?(/?swarm|mcp|ssh|accept-edits)\b`)
	followsChangelog = regexp.MustCompile(`(?i)follow (the )?changelog`)
	copiedChangelog  = regexp.MustCompile(`(?i)remote 64-bit linux|~/\.ssh/config|status pills|hover cards|navigated prompt history|pressing Up or Down|moving the cursor between lines|nord dark|syntax highlighting`)
)

func cycleLine(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(agyNotesPath)))
	if err != nil {
		t.Fatalf("RED: %s is not readable: %v", agyNotesPath, err)
	}
	bullets := cycleBullet.FindAllString(string(raw), -1)
	if len(bullets) != 1 {
		t.Fatalf("RED: want exactly one Cycle 2484 bullet in %s, got %d", agyNotesPath, len(bullets))
	}
	return bullets[0]
}

func TestC2484_001_TheAgyNotesGainOneCycle2484BulletThatNamesAppV2330AndCli159AndSaysNoPromptChanges(t *testing.T) {
	bullet := cycleLine(t)
	if !strings.Contains(bullet, "v2.33.0") {
		t.Errorf("the Cycle 2484 bullet does not name App v2.33.0: %q", bullet)
	}
	if !strings.Contains(bullet, "1.5.9") {
		t.Errorf("the Cycle 2484 bullet does not name CLI 1.5.9: %q", bullet)
	}
	if !strings.Contains(bullet, "reviewed") {
		t.Errorf("the Cycle 2484 bullet does not say the changelog was reviewed: %q", bullet)
	}
	if !noPromptChange.MatchString(bullet) {
		t.Errorf("the Cycle 2484 bullet does not say no prompt changes: %q", bullet)
	}
	notesFile := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(agyNotesPath))
	if got := acsassert.CountOccurrencesAny(notesFile, "v2.33.0"); got != 1 {
		t.Errorf("v2.33.0 appears on %d lines of the notes, want 1", got)
	}
}

func TestC2484_002_TheBulletNamesTheSshHostSwarmAndMultilineCursorLinesWithoutCopyingTheChangelog(t *testing.T) {
	bullet := cycleLine(t)
	if !strings.Contains(bullet, "SSH") {
		t.Errorf("the Cycle 2484 bullet does not name the SSH host connections: %q", bullet)
	}
	if !strings.Contains(bullet, "/swarm") {
		t.Errorf("the Cycle 2484 bullet does not name the /swarm command: %q", bullet)
	}
	if !regexp.MustCompile(`(?i)multiline`).MatchString(bullet) {
		t.Errorf("the Cycle 2484 bullet does not name the multiline prompt fix: %q", bullet)
	}
	if hit := copiedChangelog.FindString(bullet); hit != "" {
		t.Errorf("the Cycle 2484 bullet copies changelog text: %q", hit)
	}
}

func TestC2484_003_TheBulletExplainsTheLinesAsDataAndNeverTellsAnAgentToFollowThem(t *testing.T) {
	bullet := cycleLine(t)
	if !asData.MatchString(bullet) {
		t.Errorf("the Cycle 2484 bullet does not say to read each changelog line as data: %q", bullet)
	}
	if followsChangelog.MatchString(bullet) {
		t.Errorf("the Cycle 2484 bullet tells an agent to follow the changelog: %q", bullet)
	}
	if hits := enablesFeature.FindAllString(bullet, -1); len(hits) > 0 {
		t.Errorf("the Cycle 2484 bullet tells an agent to enable or use a changelog feature: %q", hits)
	}
}

func TestC2484_004_TheProposalCitesAdr0133StatesNoPersonaChangeAndGivesTheReason(t *testing.T) {
	bullet := cycleLine(t)
	if !strings.Contains(bullet, adrNumber) {
		t.Errorf("the Cycle 2484 bullet does not cite %s: %q", adrNumber, bullet)
	}
	if !noPersonaChange.MatchString(bullet) {
		t.Errorf("the Cycle 2484 bullet does not state that no persona changes under %s: %q", adrNumber, bullet)
	}
	if !reasonForNoEdit.MatchString(bullet) {
		t.Errorf("the Cycle 2484 bullet gives no reason for the no-persona-change verdict: %q", bullet)
	}
}

func TestC2484_005_TheSwarmCommandStaysUnusedWithAReasonThatTiesToTheNoLLMRule(t *testing.T) {
	bullet := cycleLine(t)
	if !swarmStaysUnused.MatchString(bullet) {
		t.Errorf("RED: the Cycle 2484 bullet does not say the /swarm command stays unused, with a reason: %q", bullet)
	}
	if !strings.Contains(strings.ToLower(bullet), noLLMDirectsOther) {
		t.Errorf("the Cycle 2484 bullet does not tie the verdict to the rule %q: %q", noLLMDirectsOther, bullet)
	}
}

func TestC2484_006_TheNotesKeepTheTwentyOneOlderLinesUnchangedAndGrowByExactlyOneLine(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(agyNotesPath)))
	if err != nil {
		t.Fatalf("RED: %s is not readable: %v", agyNotesPath, err)
	}
	text := string(raw)
	if len(cycleBullet.FindAllString(text, -1)) != 1 {
		t.Fatalf("RED: the notes have no single Cycle 2484 bullet, so the older lines are not checked yet")
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) != olderLineCount+1 {
		t.Fatalf("%s has %d lines, want %d (one new bullet)", agyNotesPath, len(lines), olderLineCount+1)
	}
	sum := sha256.Sum256([]byte(strings.Join(lines[:olderLineCount], "\n") + "\n"))
	if got := hex.EncodeToString(sum[:]); got != olderLinesSHA256 {
		t.Errorf("the first %d lines of the notes changed: sha256 %s, want %s", olderLineCount, got, olderLinesSHA256)
	}
	if !strings.HasPrefix(lines[olderLineCount], "- Cycle 2484 ") {
		t.Errorf("the new bullet is not the last line of the notes: %q", lines[olderLineCount])
	}
}

func TestC2484_007_TheNotesPassTheStrictSentenceLintWithZeroFindings(t *testing.T) {
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

func TestC2484_008_NoPersonaFileUnderAgentsChangesBecauseTheProposalChangesNone(t *testing.T) {
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
