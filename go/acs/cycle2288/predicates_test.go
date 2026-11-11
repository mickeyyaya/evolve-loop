//go:build acs

package cycle2288

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	consoleSkillRel = "skills/loop/SKILL.md"
	inboxRankRel    = "docs/architecture/packages/internal-inboxrank.md"
	inboxBatchRel   = "docs/architecture/packages/internal-inboxbatch.md"
)

var exitThree = regexp.MustCompile("(?i)exit (code )?`?3`?")

func paragraphs(t *testing.T, rel string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", rel, err)
	}
	return strings.Split(string(raw), "\n\n")
}

func exitThreeParagraph(t *testing.T, rel string, needles ...string) bool {
	t.Helper()
	for _, p := range paragraphs(t, rel) {
		if !exitThree.MatchString(p) || !strings.Contains(p, "inbox add") {
			continue
		}
		lower := strings.ToLower(p)
		all := true
		for _, n := range needles {
			all = all && strings.Contains(lower, strings.ToLower(n))
		}
		if all {
			return true
		}
	}
	return false
}

// acs-predicate: config-check
func TestC2288_001_ConsoleSkillTellsConsoleToAskOperatorOnExitThree(t *testing.T) {
	if !exitThreeParagraph(t, consoleSkillRel, "ask the operator", "--allow-overlap", "--reason", "withdraw") {
		t.Errorf("RED: %s has no one paragraph that ties `inbox add` exit 3 to: ask the operator, withdraw, --allow-overlap, --reason", consoleSkillRel)
	}
}

// acs-predicate: config-check
func TestC2288_002_ConsoleSkillNamesTheRecordedJustificationAndIds(t *testing.T) {
	if !exitThreeParagraph(t, consoleSkillRel, "ids", "reason") {
		t.Errorf("RED: %s exit 3 paragraph does not mention the printed ids and the recorded reason", consoleSkillRel)
	}
}

// acs-predicate: config-check
func TestC2288_003_InboxRankNoteDescribesOriginAndTheIntakeRefusal(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileMatchesRegex(t, filepath.Join(root, inboxRankRel), "(?i)origin") {
		t.Errorf("RED: %s does not describe the origin field", inboxRankRel)
	}
	if !exitThreeParagraph(t, inboxRankRel, "--allow-overlap") {
		t.Errorf("RED: %s does not describe the exit 3 overlap refusal of `inbox add` with --allow-overlap", inboxRankRel)
	}
}

// acs-predicate: config-check
func TestC2288_004_InboxBatchNoteKeepsOriginAndAddsTheIntakeRefusal(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileContains(t, filepath.Join(root, inboxBatchRel), "An item records its origin") {
		t.Errorf("RED: %s lost its origin note", inboxBatchRel)
	}
	if !exitThreeParagraph(t, inboxBatchRel, "--allow-overlap") {
		t.Errorf("RED: %s does not describe the exit 3 overlap refusal of `inbox add` with --allow-overlap", inboxBatchRel)
	}
}

// acs-predicate: config-check
func TestC2288_005_NoDocClaimsTheSupersededResolveVerb(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{consoleSkillRel, inboxRankRel, inboxBatchRel} {
		acsassert.FileNotContains(t, filepath.Join(root, rel), "inbox resolve")
	}
}

var checkedFiles = regexp.MustCompile(`(\d+) file\(s\) checked`)

func TestC2288_006_AddedDocLinesPassSteLintStrictAndTheDocsAreChanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--changed", "refs/remotes/origin/main", "--added", "--project-root", root)
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
		t.Fatalf("ste-lint rc=%d out=%q stderr=%q", code, acsassert.OutputTail(out, 20), acsassert.OutputTail(errOut, 5))
	}
	m := checkedFiles.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("RED: ste-lint printed no checked-file count: %q", out)
	}
	if n, _ := strconv.Atoi(m[1]); n < 3 {
		t.Errorf("RED: ste-lint checked %d changed file(s), want at least 3 (skill and two package notes)", n)
	}
}
