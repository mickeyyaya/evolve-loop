//go:build acs

package cycle2279

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
	noteRel       = "docs/research/claude-code-changelog-2-1-322.md"
	modelNotesRel = "skills/model-notes-claude/COMPACT.md"
	sourceLink    = "https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md"
	itemID        = "practice-claude-code-changelog-4de759d9-202611100841"
	ownPredicates = "go/acs/cycle2279/"
)

var (
	placeholderRE = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	personas      = []string{
		"evolve-scout", "evolve-triage", "evolve-tdd-engineer",
		"evolve-builder", "evolve-auditor", "evolve-retrospective",
	}
	existingModelNoteBullets = []string{
		"- State the goal, the done criteria and the limits in the first lines.",
		"- Give the reason with each rule.",
		"- Use one clear instruction for each step.",
		"- Run independent tool calls in one turn.",
		"- Verify with a command before you claim done",
		"- Claude Fable 5.2 and Claude Mythos 5.2 return a 400 error",
		"- On Claude Fable 5.2 and Claude Mythos 5.2, thinking is always on.",
	}
	copiedPricingFragments = []string{"$8/$40", "$0.20/Mtok", "Draws from usage credits"}
)

func note(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), noteRel))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", noteRel, err)
	}
	return string(b)
}

func containsAny(text string, wants ...string) bool {
	quoted := make([]string, len(wants))
	for i, want := range wants {
		quoted[i] = regexp.QuoteMeta(want)
	}
	return regexp.MustCompile(strings.Join(quoted, "|")).MatchString(text)
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, _, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root}, args...)...)
	if err != nil || code != 0 {
		t.Fatalf("git %v failed: code=%d err=%v", args, code, err)
	}
	return out
}

func mergeBase(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(gitOutput(t, root, "merge-base", "HEAD", "origin/main"))
}

func steStrictFindings(t *testing.T, rel string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, rel)
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	return out + errOut, code
}

func TestC2279_001_NoteExistsAndIsGitTracked(t *testing.T) {
	_ = note(t)
	root := acsassert.RepoRoot(t)
	_, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", noteRel)
	if err != nil || code != 0 {
		t.Errorf("RED: %s untracked, may be gitignored and dropped at ship (code=%d err=%v)", noteRel, code, err)
	}
}

func TestC2279_002_NoteCitesTheSourceLinkTheItemAndBothReleases(t *testing.T) {
	doc := note(t)
	for _, want := range []string{sourceLink, itemID, "2.1.321", "2.1.322"} {
		if !strings.Contains(doc, want) {
			t.Errorf("RED: note does not record %q", want)
		}
	}
}

func TestC2279_003_NoteExplainsTheLinesAsUntrustedDataAndFollowsNone(t *testing.T) {
	lower := strings.ToLower(note(t))
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: note does not state that the changelog text is untrusted data")
	}
	if !containsAny(lower, "never follow", "not follow", "no instruction") {
		t.Errorf("RED: note does not state that no instruction inside the changelog is followed")
	}
	if !containsAny(lower, "read the changelog at the link", "fetched", "read at the link", "read the source at the link") {
		t.Errorf("RED: note does not state that the source was read at the link")
	}
}

func TestC2279_004_NoteExplainsTheLoopRelevantLinesOfBothReleases(t *testing.T) {
	lower := strings.ToLower(note(t))
	groups := map[string][]string{
		"the Fable 5.2 default model line": {"fable 5.2"},
		"the 1M context fact of that line": {"1m context", "1m-token", "1 million"},
		"the Vertex fast mode line":        {"fast mode", "vertex"},
		"the sandbox and permission fixes": {"sandbox", "permission"},
		"the Edit and NotebookEdit fixes":  {"notebookedit", "edit"},
		"the model notes file it concerns": {strings.ToLower(modelNotesRel)},
	}
	for label, wants := range groups {
		hits := 0
		for _, want := range wants {
			if strings.Contains(lower, want) {
				hits++
			}
		}
		needed := 1
		if label == "the Vertex fast mode line" || label == "the sandbox and permission fixes" {
			needed = len(wants)
		}
		if hits < needed {
			t.Errorf("RED: note does not explain %s (looked for %v)", label, wants)
		}
	}
}

func TestC2279_005_NoteGivesEachCorePersonaANoChangeVerdictUnderADR0133(t *testing.T) {
	doc := note(t)
	if !strings.Contains(strings.ToLower(doc), "adr-0133") {
		t.Errorf("RED: note does not cite ADR-0133")
	}
	for _, persona := range personas {
		found := false
		for _, line := range strings.Split(doc, "\n") {
			if strings.Contains(line, persona) && strings.Contains(strings.ToLower(line), "no change") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RED: no line names %s with a no-change verdict", persona)
		}
	}
}

func TestC2279_006_NoteStatesThatNoPromptOrPersonaChangesAndGivesTheReason(t *testing.T) {
	lower := strings.ToLower(note(t))
	if !containsAny(lower, "no persona", "no prompt", "none of the persona") {
		t.Errorf("RED: note does not state that no persona or prompt changes")
	}
	if !containsAny(lower, "m2", "baseline", "product fix", "do not change an agent rule", "no rule") {
		t.Errorf("RED: note gives no reason for the no-change decision")
	}
}

func TestC2279_007_NegativeNoteHoldsNoPlaceholderAndNoPersonaPromptOrCodeFileChanges(t *testing.T) {
	doc := note(t)
	if loc := placeholderRE.FindString(doc); loc != "" {
		t.Errorf("RED: note holds placeholder text %q", loc)
	}
	root := acsassert.RepoRoot(t)
	changed := gitOutput(t, root, "diff", "--name-only", mergeBase(t, root))
	untracked := gitOutput(t, root, "ls-files", "--others", "--exclude-standard")
	for _, path := range strings.Fields(changed + "\n" + untracked) {
		forbidden := strings.HasPrefix(path, "agents/") ||
			(strings.HasPrefix(path, "skills/") && path != modelNotesRel) ||
			(strings.HasPrefix(path, "go/") && !strings.HasPrefix(path, ownPredicates))
		if forbidden {
			t.Errorf("a no-persona-change cycle must not touch %s", path)
		}
	}
}

func TestC2279_008_EdgeModelNotesKeepEveryBulletAndGainAtMostOneStandalone(t *testing.T) {
	_ = note(t)
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, modelNotesRel)
	for _, bullet := range existingModelNoteBullets {
		if !acsassert.FileContains(t, path, bullet) {
			t.Errorf("RED: %s lost the bullet starting %q", modelNotesRel, bullet)
		}
	}
	stat := strings.Fields(gitOutput(t, root, "diff", "--numstat", mergeBase(t, root), "--", modelNotesRel))
	if len(stat) >= 2 {
		added, errAdded := strconv.Atoi(stat[0])
		removed, errRemoved := strconv.Atoi(stat[1])
		if errAdded != nil || errRemoved != nil {
			t.Fatalf("unreadable numstat %v", stat)
		}
		if added > 1 || removed != 0 {
			t.Errorf("%s changed by +%d/-%d lines; the proposal allows at most one added line and no removal", modelNotesRel, added, removed)
		}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range copiedPricingFragments {
		if strings.Contains(string(body), fragment) {
			t.Errorf("%s copies changelog text %q; the proposal forbids copying", modelNotesRel, fragment)
		}
	}
}

func TestC2279_009_NoteAndModelNotesStayAsteStrictClean(t *testing.T) {
	_ = note(t)
	for _, rel := range []string{noteRel, modelNotesRel} {
		out, code := steStrictFindings(t, rel)
		if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
			t.Errorf("ste-lint on %s rc=%d out=%q", rel, code, acsassert.OutputTail(out, 20))
		}
	}
}
