//go:build acs

package cycle2259

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	noteRel        = "docs/research/practice-agy-cli-changelog-2026-11.md"
	itemID         = "practice-agy-cli-changelog-command-184aa36b-202611030236"
	snapshotMarker = "2026-10-27T07:28:00Z"
	lastRelease    = "1.4.12"
	ownPredicates  = "go/acs/cycle2259/"
)

var (
	headingMarker = "\n## "
	placeholderRE = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	reasonMarkers = []string{"m2", "baseline", "already cover", "covered"}
)

func addendum(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), noteRel))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", noteRel, err)
	}
	doc := string(b)
	at := strings.Index(doc, itemID)
	if at < 0 {
		t.Fatalf("RED: %s does not record item id %s, so the re-detection is not dispositioned", noteRel, itemID)
	}
	start := strings.LastIndex(doc[:at], headingMarker)
	if start < 0 {
		t.Fatalf("RED: item id %s sits above every ## heading, so it has no addendum section of its own", itemID)
	}
	section := doc[start+1:]
	if next := strings.Index(section[len(headingMarker)-1:], headingMarker); next >= 0 {
		section = section[:next+len(headingMarker)-1]
	}
	return section
}

func containsAny(text string, wants ...string) bool {
	for _, want := range wants {
		if strings.Contains(text, want) {
			return true
		}
	}
	return false
}

func TestC2259_001_AddendumRecordsTheSourceCommandSnapshotAndNewestRelease(t *testing.T) {
	section := addendum(t)
	for _, want := range []string{itemID, snapshotMarker, "agy changelog", lastRelease} {
		if !strings.Contains(section, want) {
			t.Errorf("RED: addendum does not record %q", want)
		}
	}
}

func TestC2259_002_AddendumExplainsTheSourceLinesAsUntrustedDataAndFollowsNone(t *testing.T) {
	lower := strings.ToLower(addendum(t))
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: addendum does not state that the source text is untrusted data")
	}
	if !containsAny(lower, "never follow", "not follow", "no instruction") {
		t.Errorf("RED: addendum does not state that no instruction inside the source is followed")
	}
}

func TestC2259_003_AddendumStatesThatNoPromptOrPersonaChangesUnderADR0133AndWhy(t *testing.T) {
	lower := strings.ToLower(addendum(t))
	if !strings.Contains(lower, "adr-0133") {
		t.Errorf("RED: addendum does not cite ADR-0133")
	}
	if !containsAny(lower, "no persona", "no prompt", "none of the persona") {
		t.Errorf("RED: addendum does not state that no persona or prompt changes")
	}
	if !containsAny(lower, reasonMarkers...) {
		t.Errorf("RED: addendum gives no reason (one of %v) for the no-change decision", reasonMarkers)
	}
}

func TestC2259_004_NegativeAddendumHoldsNoPlaceholderAndNoPersonaOrCodeFileChanges(t *testing.T) {
	section := addendum(t)
	if loc := placeholderRE.FindString(section); loc != "" {
		t.Errorf("RED: addendum holds placeholder text %q", loc)
	}
	root := acsassert.RepoRoot(t)
	base, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "origin/main")
	if err != nil || code != 0 {
		t.Fatalf("merge-base failed: code=%d err=%v", code, err)
	}
	changed, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", strings.TrimSpace(base))
	if err != nil || code != 0 {
		t.Fatalf("git diff failed: code=%d err=%v", code, err)
	}
	untracked, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard")
	if err != nil || code != 0 {
		t.Fatalf("git ls-files failed: code=%d err=%v", code, err)
	}
	for _, path := range strings.Fields(changed + "\n" + untracked) {
		forbidden := strings.HasPrefix(path, "agents/") || strings.HasPrefix(path, "skills/") ||
			(strings.HasPrefix(path, "go/") && !strings.HasPrefix(path, ownPredicates))
		if forbidden {
			t.Errorf("a no-change addendum must not touch %s", path)
		}
	}
}

func TestC2259_005_EdgeEarlierSectionsAndPersonaVerdictsSurviveTheAppend(t *testing.T) {
	_ = addendum(t)
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, noteRel)
	for _, want := range []string{
		"## 1. Context and source", "## 2. Changelog analysis as data", "## 3. Core persona evaluation under ADR-0133",
		"## 4. Conclusion", "### Versions 1.4.3 and 1.4.4", "### Versions 1.4.11 and 1.4.12",
		"### evolve-scout", "### evolve-triage", "### evolve-tdd-engineer",
		"### evolve-builder", "### evolve-auditor", "### evolve-retrospective",
	} {
		if !acsassert.FileContains(t, note, want) {
			t.Errorf("RED: the append removed %q from the earlier note", want)
		}
	}
}

func TestC2259_006_NoteStaysAsteStrictCleanAndGitTracked(t *testing.T) {
	_ = addendum(t)
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, noteRel)
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
		t.Errorf("ste-lint rc=%d out=%q stderr=%q", code, acsassert.OutputTail(out, 20), acsassert.OutputTail(errOut, 5))
	}
	_, _, code, err = acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", noteRel)
	if err != nil || code != 0 {
		t.Errorf("RED: %s untracked, may be gitignored and dropped at ship (code=%d err=%v)", noteRel, code, err)
	}
}
