//go:build acs

// Package cycle1272 materialises the cycle-1272 acceptance criteria for the one
// task triage committed to `## top_n`:
//
//	close-out-cycle1272-fleet-scope-verification  → CHANGELOG.md gains a dated
//	entry recording that both fleet-scope todo-ids
//	(infra-teardown-predicate-single-source, retro-fleet-worktree-dispatch) were
//	found already-implemented and verified-closed in cycle-1272, citing
//	TestInfraTeardownUnion_SpelledExactlyOnce and
//	TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate as the proof.
//
// The two dropped todo-ids get ZERO predicates (R9.3 floor-binding: predicates
// bind only to triage-committed work).
//
// Predicate-quality note (cycle-85 ban). The deliverable of this task IS a
// documentation artifact, so 001/004 necessarily read the emitted CHANGELOG.md —
// that is an assertion on a real emitted artifact, not a source-grep standing in
// for behaviour. The degenerate failure mode the ban targets (add a magic string
// to production source and the predicate greens regardless of the fix) is closed
// here by 002 and 003, which refuse to let the entry's CLAIM be decorative:
//
//   - 002 resolves every cited test name to a real `func Test…` definition in the
//     tree — a fabricated citation FAILS (negative axis).
//   - 003 is the crux: it RUNS both cited tests and requires `--- PASS: <name>`
//     in the verbose output, so a CHANGELOG entry claiming closure while the
//     proving tests are red — or while `-run` matched nothing at all — cannot
//     pass. Documenting a false closure is exactly the defect worth catching.
//   - 004 pins the entry against the duplicated-bullet corruption already
//     visible in the 22.13.1 section of this same file (edge axis).
//
// Roots: CHANGELOG.md and the Go tree are both read under acsassert.RepoRoot(t)
// (the cycle worktree), where Builder writes. Their absence is a FAILURE, not a
// skip. The 003 subprocess narrows each invocation to ONE named package with an
// anchored `-run` (≈2s each measured at RED) per the flaky-predicate-shape rules
// — no `./...` sweep, no wall-clock bound, no literal PID, and `cmd.Dir` is set
// explicitly rather than inherited from the lane's cwd.
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

// changelogRelPath is the single targetFile scout named for this task.
const changelogRelPath = "CHANGELOG.md"

// fleetScopeTodoIDs are the two todo-ids pinned to this lane whose closure the
// entry must record. Both must land in the SAME entry — a closure note split
// across two unrelated release sections does not document a joint verification.
var fleetScopeTodoIDs = []string{
	"infra-teardown-predicate-single-source",
	"retro-fleet-worktree-dispatch",
}

// citedTest binds each todo-id to the test scout named as its proof, plus the
// package that test lives in. 003 runs exactly these.
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

// closureMarkers are the accepted spellings of "these were found already done",
// case-folded. The entry must say the items were VERIFIED CLOSED, not merely
// mention them (a bare mention would let a re-listing of open backlog pass).
var closureMarkers = []string{
	"already-implemented",
	"already implemented",
	"verified-closed",
	"verified closed",
	"already-landed",
	"already landed",
}

// changelogBlocks splits CHANGELOG.md into top-level `## ` sections, each block
// being the heading line plus its body. Text before the first heading (the file
// preamble) is dropped: an entry must live under a dated section.
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

// closureBlocks returns the sections naming BOTH fleet-scope todo-ids. Exactly
// one is expected; the helper returns all so callers can report over/under-count.
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

// TestC1272_001_ChangelogRecordsBothFleetScopeIDsInOneEntry is the happy path:
// a single CHANGELOG section names both todo-ids, both proving tests, and the
// cycle it was verified in.
//
// acs-predicate: doc-artifact — the deliverable IS the CHANGELOG text; the
// claim it makes is separately exercised by 002 and 003.
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

// goTestFuncsInTree collects every `func TestX(` name defined under go/.
func goTestFuncsInTree(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join(acsassert.RepoRoot(t), "go")
	defs := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable subtree is not this predicate's concern
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

// TestC1272_002_CitedTestsAreNotFabricated is the negative axis: an entry may
// not cite evidence that does not exist. Every test name the closure entry names
// must resolve to a real `func Test…` definition in the tree.
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

// TestC1272_003_CitedTestsActuallyPass is the crux. The entry asserts both items
// are closed; this predicate makes that claim load-bearing by RUNNING each cited
// test in its own package and requiring an explicit `--- PASS: <name>` line, so
// a `-run` pattern that matched nothing cannot green vacuously.
func TestC1272_003_CitedTestsActuallyPass(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	for _, ct := range citedTests {
		ct := ct
		t.Run(ct.testName, func(t *testing.T) {
			cmd := exec.Command("go", "test", "-count=1", "-v",
				"-run", "^"+ct.testName+"$", ct.pkg)
			cmd.Dir = goDir // never inherit the lane's cwd
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

// TestC1272_004_ClosureEntryHasNoDuplicatedBullets is the edge axis: the 22.13.1
// section of this same file already carries a verbatim duplicated bullet, so the
// corruption is real, not hypothetical. Every non-empty content line in the new
// entry must be unique.
//
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
