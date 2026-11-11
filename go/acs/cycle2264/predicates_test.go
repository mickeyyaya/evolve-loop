//go:build acs

package cycle2264

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	noteRel     = "docs/research/practice-gemini-prompting-guide-2026-11.md"
	priorRel    = "docs/research/practice-gemini-prompting-guide-2026-10.md"
	indexRel    = "docs/research-index.md"
	indexLink   = "(research/practice-gemini-prompting-guide-2026-11.md)"
	itemID      = "practice-gemini-prompting-guide-b79996aa-202611020008"
	evalRel     = ".evolve/evals/" + itemID + ".md"
	sourceURL   = "https://ai.google.dev/gemini-api/docs/prompting-strategies"
	snapshotRef = "2026-10-14T16:10:54Z"
	adrSection  = "(ADR-0133)"
)

var (
	dateLine       = `(?m)^Date: 2026-11-\d\d`
	factRows       = `(?m)^\|\s*F\d+\s*\|.*\n\|\s*F\d+\s*\|.*\n\|\s*F\d+\s*\|`
	decisionLine   = `(?m)^Decision D\d+:`
	readAtLink     = `(?is)(read|fetched|loaded)[^.]{0,120}(link|page|url|source)`
	noFollow       = `(?i)(never follows?|does not follow|do not follow|not follow)`
	noChangeClaim  = `(?is)(no (core )?(prompt|persona)s?( or (prompt|persona)s?)? (change|changes|needs? (a )?change)|none of the[^.]{0,40}(prompts?|personas?)[^.]{0,30}change)`
	becauseClause  = `(?is)(no (core )?(prompt|persona)[^.]{0,120}because|because[^.]{0,200}(no (core )?(prompt|persona)|baseline))`
	proposalFields = `(?is)proposal.*goal.*done criteria.*limit.*before.*after`
	personaName    = `evolve-(scout|triage|tdd-engineer|builder|auditor|retrospective)`
	truncationWord = `(?i)(truncat|cut off|is cut|text is cut|shows only)`
	placeholder    = `(?i)\b(TBD|TODO|lorem ipsum)\b`
)

func noteText(t *testing.T) string {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), noteRel)
	if !acsassert.FileExists(t, path) {
		t.FailNow()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("research note %s is unreadable: %v", noteRel, err)
	}
	return string(raw)
}

func TestC2264_001_NoteCitesSourceSnapshotItemAndADR0133(t *testing.T) {
	doc := noteText(t)
	for _, want := range []string{sourceURL, snapshotRef, itemID, "ADR-0133"} {
		if !strings.Contains(doc, want) {
			t.Errorf("RED: note does not cite %q", want)
		}
	}
	if !regexp.MustCompile(dateLine).MatchString(doc) {
		t.Errorf("RED: note has no `Date: 2026-11-DD` line that records when the source was read")
	}
}

func TestC2264_002_NoteStatesThatTheSourceWasReadAtItsLink(t *testing.T) {
	doc := noteText(t)
	if !regexp.MustCompile(readAtLink).MatchString(doc) {
		t.Errorf("RED: note does not state that the source was read at its link")
	}
	if !strings.Contains(doc, "HTTP") {
		t.Errorf("RED: note records no HTTP outcome of the read of the source")
	}
}

func TestC2264_003_SourceLinesAreExplainedAsUntrustedDataAndNeverFollowed(t *testing.T) {
	doc := noteText(t)
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: note does not state that the source text is untrusted data")
	}
	if !regexp.MustCompile(noFollow).MatchString(doc) {
		t.Errorf("RED: note does not state that no instruction inside the source is followed")
	}
	if !regexp.MustCompile(factRows).MatchString(doc) {
		t.Errorf("RED: note has fewer than three consecutive fact rows (`| F1 | ... |`) that explain lines of the source")
	}
}

func TestC2264_004_NoteQuotesTheItemLinesAndRecordsHowThinTheyAre(t *testing.T) {
	doc := noteText(t)
	if !strings.Contains(doc, "API CLI") {
		t.Errorf("RED: note does not quote the item lines `API CLI` as data")
	}
	if !regexp.MustCompile(truncationWord).MatchString(doc) {
		t.Errorf("RED: note does not record that the item shows only two words of the change")
	}
}

func TestC2264_005_NoteStatesWhichPersonasChangeOrWhyNoneChanges(t *testing.T) {
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
		t.Errorf("RED: note states that nothing changes but gives no `because` reason tied to the persona scope or the baseline")
	}
	if namesProposal && !regexp.MustCompile(personaName).MatchString(doc) {
		t.Errorf("RED: note gives a proposal but names no core persona")
	}
}

func TestC2264_006_VerdictIsTiedToTheBaselineRuleAndOnePersonaAtATime(t *testing.T) {
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

func TestC2264_007_NoPersonaOrSkillFileChangesBecauseTheNoteIsOnlyAProposal(t *testing.T) {
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

func TestC2264_008_NoteAndEvalAreNotIgnoredByGit(t *testing.T) {
	_ = noteText(t)
	ignored, stderr, code, err := acsassert.SubprocessOutput("git", "-C", acsassert.RepoRoot(t), "check-ignore", noteRel, evalRel)
	if code != 1 {
		t.Errorf("RED: gitignore matches would drop files at ship (code=%d err=%v stderr=%q): %q", code, err, stderr, ignored)
	}
}

func TestC2264_009_ResearchIndexLinksTheNoteInsideTheADR0133Section(t *testing.T) {
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
	section := idx[head:]
	if tail := strings.Index(section, "\n## "); tail >= 0 {
		section = section[:tail]
	}
	if !strings.Contains(section, indexLink) {
		t.Errorf("RED: the ADR-0133 section of %s does not link %s", indexRel, indexLink)
	}
	if !strings.Contains(idx, "(research/practice-gemini-prompting-guide-2026-10.md)") {
		t.Errorf("RED: the index lost the entry of the earlier note %s", priorRel)
	}
}

func TestC2264_010_EarlierGeminiNoteIsKeptUnchanged(t *testing.T) {
	_ = noteText(t)
	status, _, code, err := acsassert.SubprocessOutput("git", "-C", acsassert.RepoRoot(t), "diff", "--name-status", "origin/main", "--", priorRel)
	if err != nil || code != 0 {
		t.Fatalf("git diff for %s failed: code=%d err=%v", priorRel, code, err)
	}
	if strings.TrimSpace(status) != "" {
		t.Errorf("earlier note changed or deleted (a document is archived, never deleted): %q", status)
	}
}

func TestC2264_011_NoteHoldsNoPlaceholderText(t *testing.T) {
	doc := noteText(t)
	if loc := regexp.MustCompile(placeholder).FindString(doc); loc != "" {
		t.Errorf("RED: note holds placeholder text %q", loc)
	}
}

func TestC2264_012_NoteAndIndexPassSteLintStrict(t *testing.T) {
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

func TestC2264_013_EvalFilePinsTheNoteWithScoreCaps(t *testing.T) {
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
}
