package skillcheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const neutralRepoRoot = "../../.."

var (
	toolTableRow      = regexp.MustCompile("(?m)^\\| `([A-Z][A-Za-z]+)` \\|")
	neutralSections   = []string{"## Per-CLI runtime details", "### 1. Tool Hygiene"}
	sectionHeadingEnd = regexp.MustCompile(`(?m)^#{2,3} `)
)

func toolNames(t *testing.T, root string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "skills", "loop", "reference", "claude-tools.md"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, m := range toolTableRow.FindAllStringSubmatch(string(body), -1) {
		names = append(names, m[1])
	}
	if len(names) < 5 {
		t.Fatalf("found %d tool names; the table moved", len(names))
	}
	return names
}

func sectionOf(t *testing.T, doc, heading string) string {
	t.Helper()
	start := strings.Index(doc, "\n"+heading)
	if start < 0 {
		t.Fatalf("heading %q is gone from AGENTS.md", heading)
	}
	rest := doc[start+1+len(heading):]
	if end := sectionHeadingEnd.FindStringIndex(rest); end != nil {
		rest = rest[:end[0]]
	}
	return rest
}

func familyToolsIn(section string, names []string) []string {
	var found []string
	for _, n := range names {
		span := "`" + n + "`"
		midSentence := regexp.MustCompile(`[a-z,] ` + n + `\b`)
		if strings.Contains(section, span) || midSentence.MatchString(section) {
			found = append(found, n)
		}
	}
	return found
}

func TestAgentsMd_PerCliAndToolHygieneSectionsNameNoToolOfOneFamily(t *testing.T) {
	t.Parallel()
	doc, err := os.ReadFile(filepath.Join(neutralRepoRoot, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	names := toolNames(t, neutralRepoRoot)
	for _, heading := range neutralSections {
		if found := familyToolsIn(sectionOf(t, string(doc), heading), names); len(found) > 0 {
			t.Errorf("%s names tools of one family: %v; move them to skills/loop/reference/<cli>-tools.md", heading, found)
		}
	}
}

func TestFamilyToolsIn_CatchesACodeSpanAndAMidSentenceName(t *testing.T) {
	t.Parallel()
	names := []string{"Read", "Bash"}
	if got := familyToolsIn("After each `Read`, summarize.", names); len(got) != 1 || got[0] != "Read" {
		t.Errorf("code span: got %v", got)
	}
	if got := familyToolsIn("then use Bash for the call", names); len(got) != 1 || got[0] != "Bash" {
		t.Errorf("mid sentence: got %v", got)
	}
	if got := familyToolsIn("Read the file. Bash is not named here.", names); len(got) != 0 {
		t.Errorf("a sentence-initial verb must pass: got %v", got)
	}
}
