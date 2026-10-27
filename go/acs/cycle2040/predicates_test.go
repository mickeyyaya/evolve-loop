//go:build acs

package cycle2040

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func readLines(t *testing.T, rel string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return strings.Split(string(raw), "\n")
}

func linesContainingAll(lines []string, needles ...string) []string {
	var hits []string
nextLine:
	for _, line := range lines {
		for _, needle := range needles {
			if !strings.Contains(line, needle) {
				continue nextLine
			}
		}
		hits = append(hits, line)
	}
	return hits
}

const runtimeReference = "docs/operations/runtime-reference.md"

func TestC2040_001_PolicyLineNamesTheStaleKeyWithBothValuesAndExit21(t *testing.T) {
	hits := linesContainingAll(readLines(t, runtimeReference), "code_index", "`stale`", "warn", "refuse", "21")
	if len(hits) == 0 {
		t.Fatal("no line of runtime-reference names the code_index key `stale` with warn, refuse and exit 21")
	}
}

func TestC2040_002_TheRefreshVerbIsNamedWithItsJsonFlag(t *testing.T) {
	hits := linesContainingAll(readLines(t, runtimeReference), "evolve code refresh", "--json")
	if len(hits) == 0 {
		t.Fatal("no line of runtime-reference names `evolve code refresh` with --json")
	}
}

func TestC2040_003_SyncMainRefreshExitCodes22And23AreNamedWithTheirCauses(t *testing.T) {
	lines := readLines(t, runtimeReference)
	if len(linesContainingAll(lines, "sync-main", "22")) == 0 {
		t.Error("no line names sync-main with exit 22 (a change in the index worktree)")
	}
	if len(linesContainingAll(lines, "sync-main", "23")) == 0 {
		t.Error("no line names sync-main with exit 23 (git fetch origin main failed)")
	}
	if len(linesContainingAll(lines, "sync-main", "evolve code refresh")) == 0 {
		t.Error("no line ties the sync-main index refresh to `evolve code refresh`")
	}
}

func TestC2040_004_TheEntryLinksToSection4OfTheSpecOfRecord(t *testing.T) {
	linked := regexp.MustCompile(`\]\(\.\./architecture/code-index-cli\.md[^)]*\)`)
	sectionFour := regexp.MustCompile(`(?i)section 4|§ ?4`)
	for _, line := range readLines(t, runtimeReference) {
		if strings.Contains(line, "code_index") || strings.Contains(line, "evolve code refresh") {
			if linked.MatchString(line) && sectionFour.MatchString(line) {
				return
			}
		}
	}
	t.Fatal("no code_index or `evolve code refresh` line links code-index-cli.md with section 4")
}

func TestC2040_005_DocumentedExitCodesAgreeWithTheSourceConstants(t *testing.T) {
	root := acsassert.RepoRoot(t)
	want := map[string]string{
		"go/cmd/evolve/cmd_code_freshness.go": `codeExitStale\s*=\s*21`,
		"go/cmd/evolve/cmd_code_refresh.go":   `codeExitIndexDirty\s*=\s*22`,
	}
	for rel, pattern := range want {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !regexp.MustCompile(pattern).Match(raw) {
			t.Errorf("%s no longer defines %s: the documented code is stale", rel, pattern)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "go/cmd/evolve/cmd_code_refresh.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`codeExitIndexFetch\s*=\s*23`).Match(raw) {
		t.Error("codeExitIndexFetch is no longer 23")
	}
	doc := readLines(t, runtimeReference)
	for _, line := range linesContainingAll(doc, "code_index", "`stale`") {
		if strings.Contains(line, "exit 22") || strings.Contains(line, "exit 23") {
			t.Errorf("the stale key line attributes the refresh codes to a query: %q", line)
		}
	}
}
