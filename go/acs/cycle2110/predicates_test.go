//go:build acs

package cycle2110

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	researchDoc      = "docs/research/practice-claude-model-release-notes-2026-11.md"
	priorDoc         = "docs/research/practice-claude-model-release-notes-2026-10.md"
	indexDoc         = "docs/research-index.md"
	researchIndexRef = "research/practice-claude-model-release-notes-2026-11.md"
	evalFile         = ".evolve/evals/practice-claude-model-release-notes-8403ed85-202610270220.md"
	sourceURL        = "https://platform.claude.com/docs/en/release-notes/overview"
	snapshotMarker   = "2026-10-26T12:00:01Z"
	verdictWindow    = 900
)

var corePersonas = []string{
	"evolve-scout", "evolve-triage", "evolve-tdd-engineer",
	"evolve-builder", "evolve-auditor", "evolve-retrospective",
}

var (
	dateLineRE   = regexp.MustCompile(`(?m)^Date: 2026-11-\d\d`)
	factRowRE    = regexp.MustCompile(`(?m)^\|\s*F\d+\s*\|`)
	placeholders = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
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
	for _, want := range []string{"goal", "done criteria", "limit", "before", "after"} {
		if !strings.Contains(section, want) {
			return false
		}
	}
	return true
}

func TestC2110_001_NoteReadsTheSourceAtTheLinkAndCitesTheSnapshotItFollows(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	for _, want := range []string{sourceURL, snapshotMarker} {
		if !strings.Contains(doc, want) {
			t.Errorf("RED: note does not cite %q", want)
		}
	}
	if !dateLineRE.MatchString(doc) {
		t.Errorf("RED: note has no `Date: 2026-11-DD` line recording when the source was read")
	}
	if !strings.Contains(doc, "ADR-0133") {
		t.Errorf("RED: note does not cite ADR-0133")
	}
}

func TestC2110_002_ReleaseNoteLinesAreExplainedAsUntrustedData(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	lower := strings.ToLower(doc)
	if !strings.Contains(lower, "untrusted") || !strings.Contains(lower, "data") {
		t.Errorf("RED: note does not state the release-notes text is untrusted data")
	}
	if !strings.Contains(lower, "never follow") && !strings.Contains(lower, "does not follow") && !strings.Contains(lower, "not follow") {
		t.Errorf("RED: note does not state that no instruction inside the source is followed")
	}
	rows := factRowRE.FindAllString(doc, -1)
	if len(rows) < 1 {
		t.Errorf("RED: note has no fact table row (`| F1 | ... |`) explaining a line of the source")
	}
}

func TestC2110_003_NoteDecidesImpactOnTheLoopWithARejectedOption(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if !regexp.MustCompile(`(?m)^Decision D\d+:`).MatchString(doc) {
		t.Errorf("RED: note records no `Decision D<n>:` line for the impact on the loop")
	}
	if !strings.Contains(doc, "Rejected option") {
		t.Errorf("RED: note names no rejected option with the fact that rules it out")
	}
}

func TestC2110_004_NoteIsGitTracked(t *testing.T) {
	_ = readRepoFile(t, researchDoc)
	requireTracked(t, researchDoc)
}

func TestC2110_005_EveryCorePersonaHasAVerdictAfterTheSnapshotMarker(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if !strings.Contains(doc, "ADR-0133") {
		t.Fatalf("RED: note does not cite ADR-0133")
	}
	for _, p := range corePersonas {
		section := personaSection(doc, p)
		if section == "" {
			t.Errorf("RED: persona %s is not named in the note", p)
			continue
		}
		if !hasNoChangeVerdict(section) && !hasChangeProposal(section) {
			t.Errorf("RED: persona %s has neither a no-change verdict nor a proposal with goal, done criteria, limits and before/after measures", p)
		}
	}
}

func TestC2110_006_PersonaVerdictsAreTiedToTheBaselineRule(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	lower := strings.ToLower(doc)
	for _, want := range []string{"baseline", "m2", "d8", "d7"} {
		if !strings.Contains(lower, want) {
			t.Errorf("RED: note does not tie its verdicts to %q", want)
		}
	}
	if !strings.Contains(lower, "one persona at a time") && !strings.Contains(lower, "trims one persona") {
		t.Errorf("RED: note does not state that a diet changes one persona at a time")
	}
}

func TestC2110_007_ResearchIndexLinksTheNoteInsideTheADR0133Section(t *testing.T) {
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
	if !strings.Contains(idx, "(research/practice-claude-model-release-notes-2026-10.md)") {
		t.Errorf("RED: the earlier note lost its index entry; archive keeps every document")
	}
}

func TestC2110_008_NoPersonaFileChangesBecauseTheNoteIsOnlyAProposal(t *testing.T) {
	_ = readRepoFile(t, researchDoc)
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "origin/main", "--", "agents")
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

func TestC2110_009_EarlierNoteIsKeptUnchanged(t *testing.T) {
	_ = readRepoFile(t, researchDoc)
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-status", "origin/main", "--", priorDoc)
	if err != nil || code != 0 {
		t.Fatalf("git diff for %s failed: code=%d err=%v", priorDoc, code, err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("earlier note changed or deleted (a document is archived, never deleted): %q", out)
	}
}

func TestC2110_010_NoteHoldsNoPlaceholderText(t *testing.T) {
	doc := readRepoFile(t, researchDoc)
	if loc := placeholders.FindString(doc); loc != "" {
		t.Errorf("RED: note holds placeholder text %q", loc)
	}
}

func TestC2110_011_NoteAndIndexPassSteLintStrict(t *testing.T) {
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

func TestC2110_012_EvalFilePinsTheDeliverableAndIsTracked(t *testing.T) {
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
