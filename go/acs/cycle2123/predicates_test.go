//go:build acs

package cycle2123

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
	researchDoc      = "docs/research/practice-claude-prompting-guide-2026-11.md"
	priorDoc         = "docs/research/practice-claude-prompting-guide-2026-10.md"
	indexDoc         = "docs/research-index.md"
	researchIndexRef = "research/practice-claude-prompting-guide-2026-11.md"
	evalFile         = ".evolve/evals/practice-claude-prompting-guide-proposal.md"
	sourceURL        = "https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices"
	snapshotMarker   = "2026-10-26T11:59:59Z"
	verdictWindow    = 1200
)

var changedPersonas = []string{"evolve-builder", "evolve-auditor", "evolve-scout"}

var unchangedPersonas = []string{"evolve-triage", "evolve-tdd-engineer", "evolve-retrospective"}

var (
	dateLineRE   = regexp.MustCompile(`(?m)^Date: 2026-11-\d\d`)
	factRowRE    = regexp.MustCompile(`(?m)^\|\s*F\d+\s*\|`)
	decisionRE   = regexp.MustCompile(`(?m)^Decision D\d+:`)
	placeholders = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	markerRE     = regexp.MustCompile(`\b(CRITICAL|MUST)\b`)
)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("RED: %s unreadable: %v", rel, err)
	}
	return string(b)
}

func requireTracked(t *testing.T, rel string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	_, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel)
	if err != nil || code != 0 {
		t.Errorf("RED: %s untracked, may be gitignored and dropped at ship (code=%d err=%v)", rel, code, err)
	}
}

func personaSection(doc, persona string) string {
	idx := strings.Index(doc, "#### "+persona)
	if idx < 0 {
		idx = strings.Index(doc, persona)
	}
	if idx < 0 {
		return ""
	}
	window := doc[idx:]
	if len(window) > verdictWindow {
		window = window[:verdictWindow]
	}
	return strings.ToLower(window)
}

func hasNoChangeVerdict(section string) bool {
	return strings.Contains(section, "no change") || strings.Contains(section, "does not change") || strings.Contains(section, "requires no change")
}

func hasChangeProposal(section string) bool {
	for _, want := range []string{"goal", "done criteria", "limit", "before"} {
		if !strings.Contains(section, want) {
			return false
		}
	}
	return true
}

func markerCounts(t *testing.T, persona string) (occurrences, lines int) {
	t.Helper()
	body := readRepoFile(t, filepath.Join("agents", persona+".md"))
	occurrences = len(markerRE.FindAllString(body, -1))
	for _, line := range strings.Split(body, "\n") {
		if markerRE.MatchString(line) {
			lines++
		}
	}
	return occurrences, lines
}

func TestC2123_001_NoteReadsTheSourceAtTheLinkAndCitesTheSnapshotItFollows(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	for _, want := range []string{sourceURL, snapshotMarker, "ADR-0133"} {
		if !strings.Contains(doc, want) {
			t.Errorf("RED: note does not cite %q", want)
		}
	}
	if !dateLineRE.MatchString(doc) {
		t.Errorf("RED: note has no `Date: 2026-11-DD` line recording when the source was read")
	}
}

func TestC2123_002_GuideLinesAreExplainedAsUntrustedDataAndNeverFollowed(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: note does not state the guide text is untrusted data")
	}
	if !strings.Contains(lower, "never follow") && !strings.Contains(lower, "does not follow") && !strings.Contains(lower, "not follow") {
		t.Errorf("RED: note does not state that no instruction inside the source is followed")
	}
	if n := len(factRowRE.FindAllString(doc, -1)); n < 2 {
		t.Errorf("RED: note has %d fact table rows (`| F1 | ... |`), want at least 2 explaining lines of the guide", n)
	}
	for _, want := range []string{"overtrigger", "aggressive"} {
		if !strings.Contains(lower, want) {
			t.Errorf("RED: note does not explain the guide line about %q wording", want)
		}
	}
}

func TestC2123_003_NoteDecidesImpactWithARejectedOption(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if !decisionRE.MatchString(doc) {
		t.Errorf("RED: note records no `Decision D<n>:` line for the impact on the loop")
	}
	if !strings.Contains(doc, "Rejected option") {
		t.Errorf("RED: note names no rejected option with the fact that rules it out")
	}
}

func TestC2123_004_NoteIsGitTracked(t *testing.T) {
	_ = readRepoFile(t, researchDoc)
	requireTracked(t, researchDoc)
}

func TestC2123_005_EachPersonaToChangeHasGoalDoneCriteriaLimitsAndBeforeMeasure(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	for _, p := range changedPersonas {
		section := personaSection(doc, p)
		if section == "" {
			t.Errorf("RED: persona %s is not named in the note", p)
			continue
		}
		if !hasChangeProposal(section) {
			t.Errorf("RED: persona %s has no proposal with goal, done criteria, limits and before measure", p)
		}
	}
}

func TestC2123_006_EveryOtherCorePersonaHasANoChangeVerdict(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	for _, p := range unchangedPersonas {
		section := personaSection(doc, p)
		if section == "" {
			t.Errorf("RED: persona %s is not named in the note", p)
			continue
		}
		if !hasNoChangeVerdict(section) {
			t.Errorf("RED: persona %s has no no-change verdict with a reason", p)
		}
	}
}

func TestC2123_007_BeforeMeasuresMatchTheEmphaticMarkersOfThePersonaFilesToday(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	for _, p := range changedPersonas {
		section := personaSection(doc, p)
		occurrences, lines := markerCounts(t, p)
		if occurrences == 0 {
			t.Fatalf("RED: persona file %s holds no CRITICAL or MUST marker, the measure is vacuous", p)
		}
		hasOccurrences := strings.Contains(section, strconv.Itoa(occurrences))
		hasLines := strings.Contains(section, strconv.Itoa(lines))
		if !hasOccurrences && !hasLines {
			t.Errorf("RED: persona %s before measure is missing: file has %d occurrences on %d lines of CRITICAL or MUST", p, occurrences, lines)
		}
		if !strings.Contains(section, "marker") {
			t.Errorf("RED: persona %s section does not name the marker measure", p)
		}
	}
}

func TestC2123_008_VerdictsAreTiedToTheBaselineRuleAndOnePersonaAtATime(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	lower := strings.ToLower(doc)
	for _, want := range []string{"baseline", "m2", "d8", "d7"} {
		if !strings.Contains(lower, want) {
			t.Errorf("RED: note does not tie its verdicts to %q", want)
		}
	}
	if !strings.Contains(lower, "one persona at a time") && !strings.Contains(lower, "one persona per") {
		t.Errorf("RED: note does not state that a diet changes one persona at a time")
	}
}

func TestC2123_009_ResearchIndexLinksTheNoteAndKeepsTheEarlierEntry(t *testing.T) {
	_ = readRepoFile(t, researchDoc)
	idx := readRepoFile(t, indexDoc)
	pos := strings.Index(idx, "("+researchIndexRef+")")
	if pos < 0 {
		t.Fatalf("RED: %s does not link %s", indexDoc, researchIndexRef)
	}
	adr := strings.Index(idx, "ADR-0133")
	if adr < 0 || pos < adr {
		t.Fatalf("RED: index entry sits before the ADR-0133 entries")
	}
	if !strings.Contains(idx, "(research/practice-claude-prompting-guide-2026-10.md)") {
		t.Errorf("RED: the earlier note lost its index entry; archive keeps every document")
	}
}

func TestC2123_010_NoPersonaFileChangesBecauseTheNoteIsOnlyAProposal(t *testing.T) {
	_ = readRepoFile(t, researchDoc)
	root := acsassert.RepoRoot(t)
	base, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "origin/main")
	if err != nil || code != 0 {
		t.Fatalf("merge-base with origin/main failed: code=%d err=%v", code, err)
	}
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", strings.TrimSpace(base), "--", "agents")
	if err != nil || code != 0 {
		t.Fatalf("git diff for agents/ failed: code=%d err=%v", code, err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("persona files changed although the deliverable is a proposal:\n%s", out)
	}
	untracked, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "agents")
	if err != nil || code != 0 || strings.TrimSpace(untracked) != "" {
		t.Errorf("untracked files under agents/ (code=%d err=%v): %q", code, err, untracked)
	}
}

func TestC2123_011_EarlierNoteIsKeptUnchanged(t *testing.T) {
	_ = readRepoFile(t, researchDoc)
	root := acsassert.RepoRoot(t)
	base, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "origin/main")
	if err != nil || code != 0 {
		t.Fatalf("merge-base with origin/main failed: code=%d err=%v", code, err)
	}
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-status", strings.TrimSpace(base), "--", priorDoc)
	if err != nil || code != 0 {
		t.Fatalf("git diff for %s failed: code=%d err=%v", priorDoc, code, err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("earlier note changed or deleted (a document is archived, never deleted): %q", out)
	}
}

func TestC2123_012_NoteHoldsNoPlaceholderText(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if loc := placeholders.FindString(doc); loc != "" {
		t.Errorf("RED: note holds placeholder text %q", loc)
	}
}

func TestC2123_013_NoteAndIndexPassSteLintStrict(t *testing.T) {
	_ = readRepoFile(t, researchDoc)
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, researchDoc, indexDoc)
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
		t.Errorf("ste-lint rc=%d out=%q stderr=%q", code, acsassert.OutputTail(out, 20), acsassert.OutputTail(errOut, 5))
	}
}

func TestC2123_014_EvalFilePinsTheDeliverableAndIsTracked(t *testing.T) {
	_ = readRepoFile(t, researchDoc)
	eval := readRepoFile(t, evalFile)
	if !strings.HasPrefix(eval, "---\nscore_cap:") {
		t.Fatalf("RED: %s has no score_cap frontmatter", evalFile)
	}
	if n := strings.Count(eval, "max_if_missing:"); n < 3 {
		t.Errorf("RED: eval has %d score_cap entries, want at least 3", n)
	}
	if !strings.Contains(eval, "ADR-0133") || !strings.Contains(eval, researchDoc) {
		t.Errorf("RED: eval does not name the note or ADR-0133")
	}
	requireTracked(t, evalFile)
}
