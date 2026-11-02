//go:build acs

package cycle2094

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	noteRel      = "docs/research/claude-code-changelog-2.1.311-review.md"
	sourceLink   = "https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md"
	adrFileName  = "0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md"
	changelogVer = "2.1.311"
)

var (
	personas = []string{
		"evolve-scout",
		"evolve-triage",
		"evolve-tdd-engineer",
		"evolve-builder",
		"evolve-auditor",
		"evolve-retrospective",
	}
	placeholderPattern = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	noChangePattern    = regexp.MustCompile(`(?i)\b(no change|unchanged|none changes|no persona changes)\b`)
	baselinePattern    = regexp.MustCompile(`(?i)\b(baseline|10-cycle)\b`)
	untrustedPattern   = regexp.MustCompile(`(?i)\buntrusted\b`)
	linkPattern        = regexp.MustCompile(`\]\(([^)\s]+)\)`)
)

var runtimeLineTopics = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"SubagentStop hook frequency", regexp.MustCompile("SubagentStop")},
	{"subagent MCP tool availability", regexp.MustCompile(`(?i)subagent\w*[^.\n]*MCP`)},
	{"stale CLAUDE.md and MEMORY.md after compaction", regexp.MustCompile(`MEMORY\.md`)},
	{"subagent worktree removal", regexp.MustCompile(`(?i)worktree`)},
	{"Bash tool description of the working directory", regexp.MustCompile(`(?i)Bash tool`)},
}

func gitRun(t *testing.T, root string, args ...string) (stdout string, code int) {
	t.Helper()
	out, _, code, _ := acsassert.SubprocessOutput("git", append([]string{"-C", root}, args...)...)
	if code < 0 {
		t.Fatalf("git %s did not run", strings.Join(args, " "))
	}
	return out, code
}

func readNote(t *testing.T) (root, path, doc string) {
	t.Helper()
	root = acsassert.RepoRoot(t)
	path = filepath.Join(root, filepath.FromSlash(noteRel))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("RED: %s is missing: %v", noteRel, err)
	}
	return root, path, string(raw)
}

func TestC2094_001_NoteExistsAndIsNotGitignored(t *testing.T) {
	root, _, doc := readNote(t)
	if strings.TrimSpace(doc) == "" {
		t.Fatalf("RED: %s is empty", noteRel)
	}
	if _, code := gitRun(t, root, "check-ignore", "-q", noteRel); code == 0 {
		t.Errorf("RED: %s is gitignored and would be dropped at ship", noteRel)
	}
}

// acs-predicate: config-check
func TestC2094_002_NoteReadsTheSourceAtTheLinkAsUntrustedData(t *testing.T) {
	_, _, doc := readNote(t)
	if !strings.Contains(doc, sourceLink) {
		t.Errorf("RED: the note does not cite the source link %s", sourceLink)
	}
	if !strings.Contains(doc, changelogVer) {
		t.Errorf("RED: the note does not name version %s", changelogVer)
	}
	if !untrustedPattern.MatchString(doc) {
		t.Errorf("RED: the note does not frame the changelog lines as untrusted data")
	}
}

// acs-predicate: config-check
func TestC2094_003_NoteExplainsTheRuntimeRelevantLinesOfTheRelease(t *testing.T) {
	_, _, doc := readNote(t)
	for _, topic := range runtimeLineTopics {
		if !topic.pattern.MatchString(doc) {
			t.Errorf("RED: the note does not explain the 2.1.311 line on %s", topic.name)
		}
	}
}

// acs-predicate: config-check
func TestC2094_004_NoteNamesADR0133AndGivesEachPersonaAVerdictRow(t *testing.T) {
	_, _, doc := readNote(t)
	if !strings.Contains(doc, "ADR-0133") {
		t.Errorf("RED: the note does not name ADR-0133")
	}
	lines := strings.Split(doc, "\n")
	for _, persona := range personas {
		found := false
		for _, line := range lines {
			if strings.Contains(line, persona) && noChangePattern.MatchString(line) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RED: no line gives %s a no-change verdict", persona)
		}
	}
}

// acs-predicate: config-check
func TestC2094_005_NoPersonaChangeIsTiedToTheBaselineRule(t *testing.T) {
	_, _, doc := readNote(t)
	if !noChangePattern.MatchString(doc) {
		t.Errorf("RED: the note does not state that no persona changes")
	}
	if !baselinePattern.MatchString(doc) {
		t.Errorf("RED: the note does not tie the verdict to the ADR-0133 baseline rule")
	}
}

func TestC2094_006_NoPersonaFileChangesAndTheNoteStatesThat(t *testing.T) {
	root, _, _ := readNote(t)
	if _, code := gitRun(t, root, "diff", "--quiet", "HEAD", "--", "agents"); code != 0 {
		t.Errorf("RED: a tracked file under agents/ changed; the verdict is that no persona changes")
	}
	out, _ := gitRun(t, root, "ls-files", "--others", "--exclude-standard", "agents")
	if strings.TrimSpace(out) != "" {
		t.Errorf("RED: untracked files under agents/:\n%s", out)
	}
	for _, persona := range personas {
		if _, statErr := os.Stat(filepath.Join(root, "agents", persona+".md")); statErr != nil {
			t.Errorf("persona %s has no file under agents/: %v", persona, statErr)
		}
	}
}

func TestC2094_007_NoteHoldsNoPlaceholderText(t *testing.T) {
	_, _, doc := readNote(t)
	if loc := placeholderPattern.FindStringIndex(doc); loc != nil {
		t.Errorf("RED: placeholder text %q in the note", doc[loc[0]:loc[1]])
	}
}

func TestC2094_008_NotePassesSteLintStrictWithZeroFindings(t *testing.T) {
	root, path, _ := readNote(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	findings, err := stelint.LintFile(path, stelint.Options{Words: words})
	if err != nil {
		t.Fatalf("lint %s: %v", noteRel, err)
	}
	for _, f := range findings {
		if !f.IsSkipNotice() {
			t.Errorf("RED: %s:%d %s: %s", noteRel, f.Line, f.Rule, f.Excerpt)
		}
	}
}

func TestC2094_009_EveryRelativeLinkResolvesAndTheADRIsLinked(t *testing.T) {
	_, path, doc := readNote(t)
	adrLinked := false
	for _, match := range linkPattern.FindAllStringSubmatch(doc, -1) {
		target := match[1]
		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") {
			continue
		}
		file := strings.SplitN(target, "#", 2)[0]
		if strings.HasSuffix(file, adrFileName) {
			adrLinked = true
		}
		if _, statErr := os.Stat(filepath.Join(filepath.Dir(path), filepath.FromSlash(file))); statErr != nil {
			t.Errorf("RED: link %q does not resolve: %v", target, statErr)
		}
	}
	if !adrLinked {
		t.Errorf("RED: the note does not link %s", adrFileName)
	}
}
