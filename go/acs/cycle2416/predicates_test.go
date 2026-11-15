//go:build acs

package cycle2416

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	noteRel       = "docs/research/practice-claude-prompting-guide-2026-11.md"
	itemMarker    = "7e8a56f7"
	sourceURL     = "https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices"
	priorSnapshot = "2026-11-13T19:47:31Z"
	adrNumber     = "ADR-0133"
	maxQuoteWords = 25
)

var (
	level2Heading = regexp.MustCompile(`(?m)^## .*$`)
	itemHeading   = regexp.MustCompile(`^## 9\. .*` + itemMarker)
	quotedFrag    = regexp.MustCompile(`"([^"\n]{8,})"`)
	placeholder   = regexp.MustCompile(`(?i)\b(TBD|TODO|lorem ipsum)\b`)
	noneChanges   = regexp.MustCompile(`(?i)(no persona (file )?(change|changes)|no persona changes|no prompt changes|none changes|no change)`)
	reasonLine    = regexp.MustCompile(`(?mi)^.*\b(decision|verdict)\b.*\bbecause\b.*$`)
	guessWord     = regexp.MustCompile(`(?i)\bguess`)
	uiWord        = regexp.MustCompile(`(?i)\b(navigation|menu|label|button|interface|UI)\b`)
	iconWord      = regexp.MustCompile(`(?i)\b(icon|glyph|symbol)\b`)
)

func noteText(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), noteRel))
	if err != nil {
		t.Fatalf("read %s: %v", noteRel, err)
	}
	return string(raw)
}

func itemSection(t *testing.T) string {
	t.Helper()
	text := noteText(t)
	headers := level2Heading.FindAllStringIndex(text, -1)
	for i, h := range headers {
		if !itemHeading.MatchString(text[h[0]:h[1]]) {
			continue
		}
		if i+1 < len(headers) {
			return text[h[0]:headers[i+1][0]]
		}
		return text[h[0]:]
	}
	t.Fatalf("RED: %s has no level-2 heading that starts with %q and names %s", noteRel, "## 9.", itemMarker)
	return ""
}

func TestC2416_001_TheNoteHasASectionNineForTheItemThatReadsTheSourceAtItsLinkAsUntrustedData(t *testing.T) {
	section := itemSection(t)
	root := acsassert.RepoRoot(t)
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", noteRel); code != 1 {
		t.Errorf("%s is ignored by git and would be dropped at ship (check-ignore exit code %d, want 1)", noteRel, code)
	}
	for _, need := range []string{sourceURL, priorSnapshot} {
		if !strings.Contains(section, need) {
			t.Errorf("section 9 does not name %s", need)
		}
	}
	lower := strings.ToLower(section)
	if !strings.Contains(lower, "untrusted data") {
		t.Errorf("section 9 does not say that the guide text is untrusted data")
	}
	if !strings.Contains(lower, "follows no instruction") {
		t.Errorf("section 9 does not say that the loop follows no instruction inside the guide")
	}
	if placeholder.MatchString(section) {
		t.Errorf("section 9 holds a placeholder word")
	}
	if got := strings.Count(section, "."); got < 5 {
		t.Errorf("section 9 has %d sentence ends, want at least 5", got)
	}
}

func TestC2416_002_TheSectionExplainsTheConsoleLineAndTheIconGlyphAsPageChromeAndGuessesNoChange(t *testing.T) {
	section := itemSection(t)
	if !strings.Contains(section, "Console") {
		t.Errorf("RED: section 9 does not explain the Console line of the excerpt")
	}
	if !iconWord.MatchString(section) {
		t.Errorf("RED: section 9 does not explain the icon glyph line of the excerpt")
	}
	if !uiWord.MatchString(section) {
		t.Errorf("RED: section 9 does not say that the lines are page navigation or interface text")
	}
	if !guessWord.MatchString(section) {
		t.Errorf("RED: section 9 does not say that the note guesses no changed guide line")
	}
	if !strings.Contains(strings.ToLower(section), "no guidance") && !strings.Contains(strings.ToLower(section), "no prompting rule") && !strings.Contains(strings.ToLower(section), "no added line") {
		t.Errorf("RED: section 9 does not say that the excerpt holds no prompting rule")
	}
}

func TestC2416_003_TheSectionDoesNotPasteRawPrivateUseOrControlCharactersFromTheExcerpt(t *testing.T) {
	section := itemSection(t)
	for i, r := range section {
		if (r >= 0xE000 && r <= 0xF8FF) || (unicode.IsControl(r) && r != '\n' && r != '\t') {
			t.Errorf("section 9 holds the raw character U+%04X at byte %d; describe the glyph in words", r, i)
		}
	}
	for _, q := range quotedFrag.FindAllStringSubmatch(section, -1) {
		if words := len(strings.Fields(q[1])); words > maxQuoteWords {
			t.Errorf("section 9 quotes %d words, want at most %d", words, maxQuoteWords)
		}
	}
}

func TestC2416_004_TheSectionStatesThatNoPersonaChangesUnderTheAdrWithAReasonAndNamesTheNotesFile(t *testing.T) {
	section := itemSection(t)
	if !strings.Contains(section, adrNumber) {
		t.Errorf("RED: section 9 does not cite %s", adrNumber)
	}
	if !noneChanges.MatchString(section) {
		t.Errorf("RED: section 9 does not state that no persona changes")
	}
	if !reasonLine.MatchString(section) {
		t.Errorf("RED: no Decision or Verdict line in section 9 states its reason with because")
	}
	if !strings.Contains(section, "skills/model-notes-claude/COMPACT.md") {
		t.Errorf("RED: section 9 does not state that the claude model notes file stays unchanged")
	}
}

func TestC2416_005_NoPersonaFileAndNoClaudeModelNotesFileChangesInThisCycle(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base := "HEAD"
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "rev-parse", "--verify", "--quiet", "origin/main"); code == 0 {
		base = "origin/main"
	}
	guarded := []string{"agents", "skills/model-notes-claude"}
	changed, _, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root, "diff", "--name-only", base, "--"}, guarded...)...)
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	untracked, _, _, _ := acsassert.SubprocessOutput("git", append([]string{"-C", root, "ls-files", "--others", "--exclude-standard", "--"}, guarded...)...)
	for _, file := range strings.Fields(changed + "\n" + untracked) {
		t.Errorf("%s changed, but section 9 gives no persona or model-notes change", file)
	}
}

func TestC2416_006_TheEarlierSectionsAndTheirRowsStayAndSectionNineComesLast(t *testing.T) {
	text := noteText(t)
	for _, want := range []struct {
		pattern string
		min     int
	}{{`(?m)^\| F\d+ \|`, 6}, {`(?m)^\| G\d+ \|`, 4}, {`(?m)^\| H\d+ \|`, 6}} {
		if got := len(regexp.MustCompile(want.pattern).FindAllString(text, -1)); got < want.min {
			t.Errorf("rows matching %s: %d, want at least %d (an earlier section was cut)", want.pattern, got, want.min)
		}
	}
	for _, id := range []string{"6080fed9", "8d950f1e", "526431cf"} {
		if !strings.Contains(text, id) {
			t.Errorf("the note lost the earlier item %s", id)
		}
	}
	headers := level2Heading.FindAllString(text, -1)
	if len(headers) < 9 {
		t.Fatalf("RED: the note has %d level-2 headings, want at least 9", len(headers))
	}
	if last := headers[len(headers)-1]; !itemHeading.MatchString(last) {
		t.Errorf("the last level-2 heading is %q, want section 9 for %s", last, itemMarker)
	}
	if strings.Count(text, itemMarker) < 2 {
		t.Errorf("the note names %s %d times, want the heading and at least one more mention", itemMarker, strings.Count(text, itemMarker))
	}
}

func TestC2416_007_TheNotePassesTheSteLintWithZeroFindings(t *testing.T) {
	text := noteText(t)
	if !strings.Contains(text, itemMarker) {
		t.Fatalf("RED: the note does not name %s yet, so a lint pass proves nothing", itemMarker)
	}
	standard, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for _, f := range stelint.Check([]byte(text), stelint.Options{Words: standard}) {
		if !f.IsSkipNotice() {
			t.Error(fmt.Sprintf("%s:%d [%s] %s", noteRel, f.Line, f.Rule, f.Message))
		}
	}
}
