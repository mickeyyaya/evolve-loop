//go:build acs

package cycle1272

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const changelogRelPath = "CHANGELOG.md"

var fleetScopeTodoIDs = []string{
	"infra-teardown-predicate-single-source",
	"retro-fleet-worktree-dispatch",
}

type citedTest struct {
	todoID   string
	testName string
	pkg      string
}

var citedTests = []citedTest{
	{
		todoID:   "infra-teardown-predicate-single-source",
		testName: "TestInfraTeardownUnion_SpelledExactlyOnce",
		pkg:      "./internal/core",
	},
	{
		todoID:   "retro-fleet-worktree-dispatch",
		testName: "TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate",
		pkg:      "./internal/phases/retro",
	},
}

var closureMarkers = []string{
	"already-implemented",
	"already implemented",
	"verified-closed",
	"verified closed",
	"already-landed",
	"already landed",
}

func changelogBlocks(t *testing.T) []string {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), changelogRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", changelogRelPath, err)
	}
	var blocks []string
	var cur []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "## ") {
			if len(cur) > 0 {
				blocks = append(blocks, strings.Join(cur, "\n"))
			}
			cur = []string{line}
			continue
		}
		if len(cur) > 0 {
			cur = append(cur, line)
		}
	}
	if len(cur) > 0 {
		blocks = append(blocks, strings.Join(cur, "\n"))
	}
	return blocks
}

func closureBlocks(t *testing.T) []string {
	t.Helper()
	var hits []string
	for _, b := range changelogBlocks(t) {
		joint := true
		for _, id := range fleetScopeTodoIDs {
			if !strings.Contains(b, id) {
				joint = false
				break
			}
		}
		if joint {
			hits = append(hits, b)
		}
	}
	return hits
}

// acs-predicate: doc-artifact — the deliverable IS the CHANGELOG text; the
func TestC1272_001_ChangelogRecordsBothFleetScopeIDsInOneEntry(t *testing.T) {
	hits := closureBlocks(t)
	if len(hits) != 1 {
		t.Fatalf("want exactly 1 CHANGELOG section naming both %v, got %d",
			fleetScopeTodoIDs, len(hits))
	}
	entry := hits[0]
	for _, ct := range citedTests {
		if !strings.Contains(entry, ct.testName) {
			t.Errorf("closure entry omits the proving test %s cited for %s", ct.testName, ct.todoID)
		}
	}
	if !strings.Contains(entry, "1272") {
		t.Errorf("closure entry does not identify the verifying cycle (1272); entry heading: %s",
			strings.SplitN(entry, "\n", 2)[0])
	}
	folded := strings.ToLower(entry)
	found := false
	for _, m := range closureMarkers {
		if strings.Contains(folded, m) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("closure entry never states the items were already-implemented/verified-closed (want one of %v) — a bare mention re-lists them as open work", closureMarkers)
	}
}

func goTestFuncsInTree(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join(acsassert.RepoRoot(t), "go")
	defs := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if !strings.HasPrefix(line, "func Test") {
				continue
			}
			name := strings.TrimPrefix(line, "func ")
			if i := strings.IndexByte(name, '('); i > 0 {
				defs[name[:i]] = path
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return defs
}

func TestC1272_002_CitedTestsAreNotFabricated(t *testing.T) {
	hits := closureBlocks(t)
	if len(hits) != 1 {
		t.Fatalf("want exactly 1 closure section naming both todo-ids, got %d", len(hits))
	}
	entry := hits[0]
	defs := goTestFuncsInTree(t)
	cited := 0
	for _, field := range strings.FieldsFunc(entry, func(r rune) bool {
		return !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		if !strings.HasPrefix(field, "Test") {
			continue
		}
		cited++
		if _, ok := defs[field]; !ok {
			t.Errorf("closure entry cites %s but no `func %s(` exists in the tree — fabricated evidence", field, field)
		}
	}
	if cited < len(citedTests) {
		t.Errorf("closure entry cites %d test names, want at least %d (one proof per todo-id)", cited, len(citedTests))
	}
}

func TestC1272_003_CitedTestsActuallyPass(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	for _, ct := range citedTests {
		ct := ct
		t.Run(ct.testName, func(t *testing.T) {
			cmd := exec.Command("go", "test", "-count=1", "-v",
				"-run", "^"+ct.testName+"$", ct.pkg)
			cmd.Dir = goDir
			out, err := cmd.CombinedOutput()
			text := string(out)
			if err != nil {
				t.Fatalf("%s (%s) did not pass: %v\n%s", ct.testName, ct.pkg, err, text)
			}
			if !strings.Contains(text, "--- PASS: "+ct.testName) {
				t.Fatalf("%s never ran (no `--- PASS: %s` in output) — the CHANGELOG cites a test that does not execute in %s:\n%s",
					ct.testName, ct.testName, ct.pkg, text)
			}
		})
	}
}

// acs-predicate: doc-artifact — structural check on the emitted deliverable.
func TestC1272_004_ClosureEntryHasNoDuplicatedBullets(t *testing.T) {
	hits := closureBlocks(t)
	if len(hits) != 1 {
		t.Fatalf("want exactly 1 closure section naming both todo-ids, got %d", len(hits))
	}
	seen := map[string]int{}
	for _, line := range strings.Split(hits[0], "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "---" {
			continue
		}
		seen[trimmed]++
	}
	for line, n := range seen {
		if n > 1 {
			t.Errorf("closure entry repeats a line %d times (duplicated-bullet corruption): %q", n, line)
		}
	}
}
