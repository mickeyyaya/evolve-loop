//go:build acs

package cycle2258

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	noteRel     = "docs/research/agy-cli-changelog-2026-11.md"
	indexRel    = "docs/research-index.md"
	indexLink   = "(research/agy-cli-changelog-2026-11.md)"
	evalRel     = ".evolve/evals/practice-agy-cli-changelog-page-a64d2284-202611040037.md"
	sourceURL   = "https://antigravity.google/docs/changelog?tab=cli"
	snapshotRef = "2026-11-02T00:08:04Z"
	adrSection  = "(ADR-0133)"
)

var earlierNotes = []string{
	"docs/research/practice-agy-cli-changelog-2026-10.md",
	"docs/research/practice-agy-cli-changelog-page-2026-10.md",
	"docs/research/practice-agy-cli-changelog-2026-11.md",
	"docs/research/practice-agy-cli-changelog-page-2026-11.md",
}

var (
	dateLine        = `(?m)^Date: 2026-11-\d\d`
	factRows        = `(?m)^\|\s*F\d+\s*\|.*\n\|\s*F\d+\s*\|.*\n\|\s*F\d+\s*\|`
	decisionLine    = `(?m)^Decision D\d+:`
	noChangeClaim   = `(?is)(no (core )?(prompt|persona)s?( or (prompt|persona)s?)? (change|changes|needs? (a )?change)|none of the[^.]{0,40}(prompts?|personas?)[^.]{0,30}change)`
	becauseClause   = `(?is)(no (core )?(prompt|persona)[^.]{0,120}because|because[^.]{0,200}(no (core )?(prompt|persona)|baseline))`
	proposalFields  = `(?is)proposal.*goal.*done criteria.*limit.*before.*after`
	placeholderText = `(?i)\b(TBD|TODO|lorem ipsum)\b`
	cliOwnsAppClaim = `(?i)(CLI binary (already )?handles|as internal CLI binary behavior|the agy CLI (now )?(ships|adds) (voice mode|remote agents))`
)

func noteText(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), noteRel))
	if err != nil {
		t.Fatalf("RED: research note %s is not readable: %v", noteRel, err)
	}
	return string(raw)
}

func requireNotIgnored(t *testing.T, rel string) {
	t.Helper()
	_, stderr, code, err := acsassert.SubprocessOutput("git", "-C", acsassert.RepoRoot(t), "check-ignore", "-q", rel)
	if code != 0 && code != 1 {
		t.Fatalf("git check-ignore did not run for %s: code=%d err=%v stderr=%q", rel, code, err, stderr)
	}
	if code == 0 {
		t.Errorf("RED: %s is gitignored and would be dropped at ship", rel)
	}
}

func TestC2258_001_NoteCitesTheSourceLinkTheSnapshotAndADR0133(t *testing.T) {
	doc := noteText(t)
	for _, want := range []string{sourceURL, snapshotRef, "ADR-0133"} {
		if !strings.Contains(doc, want) {
			t.Errorf("RED: note does not cite %q", want)
		}
	}
	if !regexp.MustCompile(dateLine).MatchString(doc) {
		t.Errorf("RED: note has no `Date: 2026-11-DD` line that records when the source was read")
	}
}

func TestC2258_002_PageLinesAreExplainedAsUntrustedDataAndNeverFollowed(t *testing.T) {
	doc := noteText(t)
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: note does not state that the page text is untrusted data")
	}
	if !regexp.MustCompile(`(?i)(never follows?|does not follow|do not follow|not follow)`).MatchString(doc) {
		t.Errorf("RED: note does not state that no instruction inside the source is followed")
	}
	if !regexp.MustCompile(factRows).MatchString(doc) {
		t.Errorf("RED: note has fewer than three consecutive fact rows (`| F1 | ... |`) that explain lines of the source")
	}
}

func TestC2258_003_NoteExplainsEachLineOfTheItemAndRecordsTheTruncation(t *testing.T) {
	doc := noteText(t)
	for _, want := range []string{"Voice mode", "Remote Agents", "Agent Client Protocol", "ACP", "v1.4.12", "v1.4.11"} {
		if !strings.Contains(doc, want) {
			t.Errorf("RED: note does not explain the item line %q", want)
		}
	}
	if !strings.Contains(strings.ToLower(doc), "truncat") {
		t.Errorf("RED: note does not record that the added text of the item is cut")
	}
	if !strings.Contains(doc, "Added support for re") {
		t.Errorf("RED: note does not quote the cut line `Added support for re` as data")
	}
}

func TestC2258_004_NoteSeparatesTheTabsAndDoesNotClaimAppFeaturesAsCliBehavior(t *testing.T) {
	doc := noteText(t)
	if loc := regexp.MustCompile(cliOwnsAppClaim).FindString(doc); loc != "" {
		t.Errorf("RED: note claims an app feature as CLI behavior: %q", loc)
	}
	if !regexp.MustCompile(`(?i)\b(app|cli) tab\b`).MatchString(doc) {
		t.Errorf("RED: note does not say which page tab (App or CLI) each line belongs to")
	}
}

func TestC2258_005_NoteStatesWhichPersonasChangeOrWhyNoneChanges(t *testing.T) {
	doc := noteText(t)
	if !regexp.MustCompile(decisionLine).MatchString(doc) {
		t.Errorf("RED: note records no `Decision D<n>:` line for the effect on prompts and personas")
	}
	if !strings.Contains(doc, "Rejected option") {
		t.Errorf("RED: note names no rejected option with the fact that rules it out")
	}
	statesNone := regexp.MustCompile(noChangeClaim).MatchString(doc)
	namesProposal := regexp.MustCompile(proposalFields).MatchString(doc)
	if !statesNone && !namesProposal {
		t.Fatalf("RED: note neither states that no prompt or persona changes nor gives a proposal with goal, done criteria, limits and before and after measures")
	}
	if statesNone && !regexp.MustCompile(becauseClause).MatchString(doc) {
		t.Errorf("RED: note states that nothing changes but gives no reason (a `because` clause tied to the persona scope or the baseline)")
	}
}

func TestC2258_006_VerdictIsTiedToTheBaselineRuleAndOnePersonaAtATime(t *testing.T) {
	doc := noteText(t)
	lower := strings.ToLower(doc)
	for _, want := range []string{"baseline", "m2", "d7", "d8"} {
		if !strings.Contains(lower, want) {
			t.Errorf("RED: note does not tie its verdict to %q", want)
		}
	}
	if !strings.Contains(lower, "one persona at a time") && !strings.Contains(lower, "trims one persona") {
		t.Errorf("RED: note does not state that a diet changes one persona at a time")
	}
}

func TestC2258_007_NoteIsNotIgnoredByGit(t *testing.T) {
	_ = noteText(t)
	requireNotIgnored(t, noteRel)
}

func TestC2258_008_ResearchIndexLinksTheNoteInsideTheADR0133Section(t *testing.T) {
	_ = noteText(t)
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), indexRel))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", indexRel, err)
	}
	idx := string(raw)
	head := strings.Index(idx, adrSection)
	if head < 0 {
		t.Fatalf("RED: %s has no section headed %s", indexRel, adrSection)
	}
	tail := strings.Index(idx[head:], "\n## ")
	section := idx[head:]
	if tail >= 0 {
		section = idx[head : head+tail]
	}
	if !strings.Contains(section, indexLink) {
		t.Errorf("RED: the ADR-0133 section of %s does not link %s", indexRel, indexLink)
	}
	for _, kept := range earlierNotes {
		if !strings.Contains(idx, "("+strings.TrimPrefix(kept, "docs/")+")") {
			t.Errorf("RED: the index lost the entry of the earlier note %s", kept)
		}
	}
}

func TestC2258_009_NoPersonaOrPromptFileChangesBecauseTheNoteIsOnlyAProposal(t *testing.T) {
	_ = noteText(t)
	root := acsassert.RepoRoot(t)
	changed, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "origin/main", "--", "agents", "skills")
	if err != nil || code != 0 {
		t.Fatalf("git diff for agents/ and skills/ failed: code=%d err=%v", code, err)
	}
	if strings.TrimSpace(changed) != "" {
		t.Errorf("a persona or skill file changed although the deliverable is a proposal:\n%s", changed)
	}
	added, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "agents", "skills")
	if err != nil || code != 0 || strings.TrimSpace(added) != "" {
		t.Errorf("untracked files under agents/ or skills/ (code=%d err=%v): %q", code, err, added)
	}
}

func TestC2258_010_EarlierNotesAreKeptUnchanged(t *testing.T) {
	_ = noteText(t)
	root := acsassert.RepoRoot(t)
	for _, prior := range earlierNotes {
		status, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-status", "origin/main", "--", prior)
		if err != nil || code != 0 {
			t.Fatalf("git diff for %s failed: code=%d err=%v", prior, code, err)
		}
		if strings.TrimSpace(status) != "" {
			t.Errorf("earlier note changed or deleted (a document is archived, never deleted): %q", status)
		}
	}
}

func TestC2258_011_NoteHoldsNoPlaceholderText(t *testing.T) {
	doc := noteText(t)
	if loc := regexp.MustCompile(placeholderText).FindString(doc); loc != "" {
		t.Errorf("RED: note holds placeholder text %q", loc)
	}
}

func TestC2258_012_NoteAndIndexPassSteLintStrict(t *testing.T) {
	_ = noteText(t)
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, noteRel, indexRel)
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
		t.Errorf("ste-lint rc=%d out=%q stderr=%q", code, acsassert.OutputTail(out, 20), acsassert.OutputTail(errOut, 5))
	}
}

func TestC2258_013_EvalFilePinsTheNoteAndIsNotIgnored(t *testing.T) {
	_ = noteText(t)
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), evalRel))
	if err != nil {
		t.Fatalf("RED: eval file %s unreadable: %v", evalRel, err)
	}
	eval := string(raw)
	if !strings.HasPrefix(eval, "---\nscore_cap:") {
		t.Fatalf("RED: %s has no score_cap frontmatter", evalRel)
	}
	if n := strings.Count(eval, "max_if_missing:"); n < 3 {
		t.Errorf("RED: eval has %d score_cap entries, want at least 3", n)
	}
	if !strings.Contains(eval, "ADR-0133") || !strings.Contains(eval, noteRel) {
		t.Errorf("RED: eval does not name the note or ADR-0133")
	}
	requireNotIgnored(t, evalRel)
}
