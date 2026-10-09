//go:build acs

package cycle1852

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	resumeSectionStart = "**The resume.**"
	resumeSectionEnd   = "After the resume"
	settleTestName     = "TestLandingQueueCLI_ResumeSettlesLocalMainToOrigin"
)

var sentenceBoundary = regexp.MustCompile(`[.!?](\s|$)|\n`)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

func resumeSection(t *testing.T) string {
	t.Helper()
	spec := readRepoFile(t, "docs/architecture/fleet-landing-queue.md")
	start := strings.Index(spec, resumeSectionStart)
	if start < 0 {
		t.Fatalf("the spec has no %q section", resumeSectionStart)
	}
	rest := spec[start:]
	end := strings.Index(rest, resumeSectionEnd)
	if end < 0 {
		t.Fatalf("the resume section has no %q boundary", resumeSectionEnd)
	}
	return rest[:end]
}

func sentencesWithAll(text string, words ...string) []string {
	var hits []string
	for _, sentence := range sentenceBoundary.Split(text, -1) {
		lower := strings.ToLower(sentence)
		matched := true
		for _, w := range words {
			if !strings.Contains(lower, strings.ToLower(w)) {
				matched = false
				break
			}
		}
		if matched {
			hits = append(hits, sentence)
		}
	}
	return hits
}

func q17TestRow(t *testing.T) string {
	t.Helper()
	plan := readRepoFile(t, "docs/plans/concurrent-cycle-landing-2026-10.md")
	for _, line := range strings.Split(plan, "\n") {
		if strings.HasPrefix(line, "| Q17 |") && strings.Contains(line, "TestLandingQueueCLI_") {
			return line
		}
	}
	t.Fatal("the plan has no Q17 test-list row")
	return ""
}

func TestC1852_000_SentenceMatcherFindsOnlyTheSentenceThatHoldsEveryWord(t *testing.T) {
	text := "It fast-forwards `main` to `origin`. Then it clears the flag.\nThe `main` tip stays."
	if got := sentencesWithAll(text, "fast-forward", "`main`", "`origin`"); len(got) != 1 {
		t.Fatalf("want one settle sentence, got %q", got)
	}
	if got := sentencesWithAll("It clears the flag. It moves `main`.", "fast-forward", "`main`"); len(got) != 0 {
		t.Fatalf("a text with no fast-forward matched: %q", got)
	}
}

// acs-predicate: config-check
func TestC1852_001_ResumeFastForwardsLocalMainToOriginBeforeTheFlagClears(t *testing.T) {
	section := resumeSection(t)
	settle := sentencesWithAll(section, "fast-forward", "`main`", "`origin`")
	if len(settle) == 0 {
		t.Fatalf("RED: the resume section has no sentence that fast-forwards local `main` to `origin`; a stale local main makes each other candidate fail its fast-forward check\nsection:\n%s", section)
	}
	clear := strings.Index(section, "clears the flag")
	if clear < 0 {
		t.Fatalf("the resume section no longer says it clears the flag\nsection:\n%s", section)
	}
	if strings.Index(section, settle[0]) > clear && len(sentencesWithAll(section, "fast-forward", "before")) == 0 {
		t.Errorf("RED: the settle step comes after the flag clears and is not stated to run before it: %q", settle[0])
	}
}

// acs-predicate: config-check
func TestC1852_002_ResumeLeavesLocalMainWhenOriginLacksTheCommit(t *testing.T) {
	section := resumeSection(t)
	var keeps []string
	for _, phrase := range []string{"does not move", "not move", "leaves", "stays", "unchanged", "does not touch"} {
		keeps = append(keeps, sentencesWithAll(section, "`main`", phrase)...)
	}
	if len(keeps) == 0 {
		t.Errorf("RED: the resume section does not state that local `main` stays when `origin` lacks the intent commit\nsection:\n%s", section)
	}
	if len(sentencesWithAll(section, "becomes `parked`")) == 0 {
		t.Errorf("the origin-lacks branch no longer parks the record\nsection:\n%s", section)
	}
}

// acs-predicate: config-check
func TestC1852_003_Q17NamesTheSettleTestAndKeepsTheExistingTests(t *testing.T) {
	row := q17TestRow(t)
	if !strings.Contains(row, "`"+settleTestName+"`") {
		t.Errorf("RED: plan row Q17 does not name %s\nrow: %s", settleTestName, row)
	}
	for _, kept := range []string{
		"TestLandingQueueCLI_StatusPrintsEachCandidate",
		"TestLandingQueueCLI_ResumeRefusesWhileTheCauseHolds",
		"TestLandingQueueCLI_ResumeLandsAStrandedCommitThatOriginHolds",
		"TestLandingQueueCLI_ResumeParksAStrandedCommitThatOriginLacks",
		"TestLandingQueueCLI_EjectRoutesAsReviewUnsure",
		"TestLandingQueueCLI_EjectEndsAParkedRecord",
		"TestLandingQueueCLI_ShadowVerifyReportsEachEscape",
	} {
		if !strings.Contains(row, "`"+kept+"`") {
			t.Errorf("plan row Q17 lost %s\nrow: %s", kept, row)
		}
	}
}

// acs-predicate: config-check
func TestC1852_004_ResumeKeepsTheShipPushInvariant(t *testing.T) {
	section := resumeSection(t)
	if len(sentencesWithAll(section, "Ship moves `main` only after its push lands")) == 0 {
		t.Errorf("the resume section lost the ship push invariant that makes a local fast-forward safe\nsection:\n%s", section)
	}
}
