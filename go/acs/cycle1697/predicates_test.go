//go:build acs

// Package cycle1697 materialises the acceptance criteria of the one
// fleet-scoped task pinned to this lane: `dead-red-acs-corpus-cleanup`
// (.evolve/inbox/processing/cycle-1697/2026-08-04T05-03-00Z-dead-red-acs-corpus-cleanup.json;
// triage top_n: delete go/acs/cycle1257 + go/acs/cycle1259 predicate files and
// extend F5 of docs/operations/batch-integrity-review-2026-08-04.md in place).
//
// THE DEFECT. fcdd466e shipped go/acs/cycle1257/predicates_test.go and
// go/acs/cycle1259/predicates_test.go from cycles whose audits FAILED. They
// grade an abandoned acssuite-internal selection design (the phantom
// GoLaneSelection and Run-stage unit tests) that never existed at any commit —
// the token is never spelled literally in this file, so 001's corpus scan
// covers this package too — so they are red by
// construction: at this cycle's base f341bc89, `go test -tags acs` reports 5
// FAIL in cycle1257 and 2 FAIL in cycle1259 — FAIL, not SKIP.
//
// MEASURED PREMISE CORRECTION. The inbox asks to "verify the EGPS
// skipped_count drops". It cannot: EGPS (acssuite.goLanePatterns,
// go/internal/acssuite/acssuite.go:379-395) runs only the current cycle, the
// regression set and redteam — never a historical cycle dir — and the two
// packages FAIL rather than SKIP. skipped_count is invariant under this change
// by construction, so a predicate demanding a drop would be permanently RED
// (the cycle-644 unsatisfiable-AC shape). The honest measurable is that the
// dead packages no longer resolve at all (001); the F5 closure must record the
// skipped_count outcome (002), and the Auditor checks it is recorded truthfully.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - POSITIVE : 001 — the Go toolchain no longer resolves either dead package
//     (directory gone / no Go files). A tombstone, a skip-guarded shell or a
//     renamed copy still lists test functions and FAILS; a package that fails
//     to compile is never read as "removed".
//   - DOCS     : 002 — F5 extended in place: exactly one F5 heading, F6 still
//     next, Issue/Gap/Solution intact, and a Closure block citing both deleted
//     paths, the closing cycle and the skipped_count outcome (config-check).
//   - NEGATIVE : 003 — the change set deletes both dead files, touches no other
//     go/acs file (over-deletion / collateral edits FAIL), touches no protected
//     control-plane surface (guards.IsProtectedSurface, the production SSOT),
//     and is non-vacuous.
package cycle1697

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	reviewDocRel = "docs/operations/batch-integrity-review-2026-08-04.md"
	// ownDirRel is this cycle's own predicate package — the one go/acs path
	// the lane may add besides deleting the two dead files.
	ownDirRel = "go/acs/cycle1697/"
)

// deadPackages are the two red-by-construction predicate packages the task
// removes: the module-relative pattern `go test` resolves, and the tracked file.
var deadPackages = []struct{ pkg, file string }{
	{"./acs/cycle1257", "go/acs/cycle1257/predicates_test.go"},
	{"./acs/cycle1259", "go/acs/cycle1259/predicates_test.go"},
}

// goTestList runs `go test -tags acs -count=1 -list . <pkg>` for ONE named
// package from the module dir — compile and enumerate only; no predicate in
// the listed package executes. Returns the combined output and exit code.
func goTestList(t *testing.T, root, pkg string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", "test", "-tags", "acs", "-count=1", "-list", ".", pkg)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("go test -list %s could not start: %v", pkg, err)
	return "", -1
}

// listedTests returns the test function names a `go test -list` run printed.
func listedTests(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Test") {
			names = append(names, strings.TrimSpace(line))
		}
	}
	return names
}

// -----------------------------------------------------------------------------
// AC1 — the dead-red packages are gone from the predicate corpus.
// -----------------------------------------------------------------------------

// TestC1697_001_DeadRedPackagesNoLongerResolve drives the Go toolchain against
// each dead package. The only passing outcome is the package being gone
// (`directory not found` / `no Go files`): a package that still lists test
// functions is the dead-red corpus still shipped, and any other nonzero exit
// (a compile error, an infra failure) fails loudly rather than reading as a
// removal.
func TestC1697_001_DeadRedPackagesNoLongerResolve(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, d := range deadPackages {
		out, code := goTestList(t, root, d.pkg)
		if code == 0 {
			t.Errorf("RED: %s still resolves as a predicate package listing %d test function(s) %v — the dead-red corpus was not removed (a tombstone or skip-guarded shell is not a removal)",
				d.pkg, len(listedTests(out)), listedTests(out))
			continue
		}
		if !strings.Contains(out, "directory not found") && !strings.Contains(out, "no Go files") {
			t.Errorf("%s: go test -list exited %d for a reason other than the package being gone (compile error / infra failure — never read as removed):\n%s",
				d.pkg, code, out)
		}
	}

	// Auxiliary (not load-bearing): no predicate source left anywhere in the
	// corpus grades the phantom machinery — the dead files were not merely moved.
	// The token is assembled so this file never matches itself.
	phantom := "TestGoLane" + "Selection_"
	corpus := filepath.Join(root, "go", "acs")
	err := filepath.WalkDir(corpus, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), phantom) {
			t.Errorf("RED: %s still references the phantom %s* machinery", p, phantom)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk predicate corpus %s: %v", corpus, err)
	}
}

// -----------------------------------------------------------------------------
// AC3 — F5 of the batch-integrity review is extended in place with closure.
// -----------------------------------------------------------------------------

// TestC1697_002_F5ExtendedInPlaceWithClosure pins the docs half of the task:
// the existing F5 issue/gap/solution section gains closure evidence without
// being duplicated, rewritten or moved.
//
// acs-predicate: config-check — the F5 record is operator-facing documentation
// no executable system consumes; its structure IS the contract (the inbox's
// operator DOCS REQUIRED directive). The Auditor reviews the closure's content.
func TestC1697_002_F5ExtendedInPlaceWithClosure(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, reviewDocRel))
	if err != nil {
		t.Fatalf("read %s: %v", reviewDocRel, err)
	}
	lines := strings.Split(string(data), "\n")

	start, count := -1, 0
	for i, l := range lines {
		if strings.HasPrefix(l, "### F5 ") {
			count++
			if start < 0 {
				start = i
			}
		}
	}
	if count != 1 {
		t.Fatalf("want exactly one `### F5 ` heading in %s (extend in place, never duplicate), got %d", reviewDocRel, count)
	}

	end, next := len(lines), "<end of file>"
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "### ") || strings.HasPrefix(lines[i], "## ") {
			end, next = i, lines[i]
			break
		}
	}
	if !strings.HasPrefix(next, "### F6 ") {
		t.Errorf("the heading after F5 must still be `### F6 ` (F5 extended in place, nothing inserted between); got %q", next)
	}

	body := lines[start+1 : end]
	for _, label := range []string{"**Issue.**", "**Gap.**", "**Solution**"} {
		if !hasLinePrefix(body, label) {
			t.Errorf("F5 lost its %s paragraph — extend the section, do not rewrite it", label)
		}
	}

	closureAt := -1
	for i, l := range body {
		if strings.HasPrefix(l, "**Closure") {
			closureAt = i
			break
		}
	}
	if closureAt < 0 {
		t.Fatalf("RED: F5 has no `**Closure…` paragraph — closure evidence for dead-red-acs-corpus-cleanup is not recorded")
	}
	closure := strings.Join(body[closureAt:], "\n")
	for _, want := range []string{
		"go/acs/cycle1257", // the first deleted package, cited by path
		"go/acs/cycle1259", // the second deleted package, cited by path
		"1697",             // the cycle that closed F5
		"skipped_count",    // the Solution's verification step, answered
	} {
		if !strings.Contains(closure, want) {
			t.Errorf("F5 closure does not cite %q:\n%s", want, closure)
		}
	}
}

func hasLinePrefix(lines []string, prefix string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// AC4 + scope — the change set is exactly the cleanup, and spares the control
// plane.
// -----------------------------------------------------------------------------

// TestC1697_003_ChangeSetDeletesOnlyDeadCorpusAndSparesProtectedSurface is the
// negative half. Non-vacuous: both dead files must be DELETED in the lane's
// change set. Over-deletion: no other go/acs path may be touched besides this
// cycle's own predicate package. Protected surface: no changed path may be
// classified protected by guards.IsProtectedSurface — the same classifier the
// role guard enforces, so the predicate cannot drift from the manifest.
func TestC1697_003_ChangeSetDeletesOnlyDeadCorpusAndSparesProtectedSurface(t *testing.T) {
	root := acsassert.RepoRoot(t)
	changes := changeSet(t, root)

	isDead := map[string]bool{}
	for _, d := range deadPackages {
		isDead[d.file] = true
		if st := changes[d.file]; st != "D" {
			t.Errorf("RED: the change set does not delete %s (status %q) — the dead-red predicate file still ships", d.file, st)
		}
	}

	paths := make([]string, 0, len(changes))
	for p := range changes {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if guards.IsProtectedSurface(p) {
			t.Errorf("change set touches protected control-plane surface %s (status %s)", p, changes[p])
		}
		if strings.HasPrefix(p, "go/acs/") && !isDead[p] && !strings.HasPrefix(p, ownDirRel) {
			t.Errorf("change set touches go/acs corpus file %s (status %s) outside the two dead-red files and %s — over-deletion or a collateral edit",
				p, changes[p], ownDirRel)
		}
	}
}

// changeSet maps each path the lane changed to a git status letter (D, M, A):
// committed since the nearest fork point with main / origin/main, overlaid with
// the working tree — so it holds whether or not the Builder has committed.
func changeSet(t *testing.T, root string) map[string]string {
	t.Helper()
	base := forkPoint(t, root)
	changes := map[string]string{}

	out, errOut, code, _ := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-status", "--no-renames", base, "HEAD")
	if code != 0 {
		t.Fatalf("git diff --name-status %s HEAD: exit %d: %s", base, code, errOut)
	}
	for _, line := range strings.Split(out, "\n") {
		if st, p, ok := strings.Cut(line, "\t"); ok && st != "" && p != "" {
			changes[p] = st[:1]
		}
	}

	status, errOut, code, _ := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain", "--untracked-files=all", "--no-renames")
	if code != 0 {
		t.Fatalf("git status: exit %d: %s", code, errOut)
	}
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		xy, p := line[:2], line[3:]
		switch {
		case strings.Contains(xy, "D"):
			changes[p] = "D"
		case xy == "??":
			changes[p] = "A"
		default:
			if _, seen := changes[p]; !seen {
				changes[p] = "M"
			}
		}
	}
	return changes
}

// forkPoint is the nearest merge-base of HEAD with main / origin/main, so a
// lagging remote ref never drags sibling lanes' landed work into the diff.
func forkPoint(t *testing.T, root string) string {
	t.Helper()
	var bases []string
	for _, ref := range []string{"main", "origin/main"} {
		if mb, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", ref); code == 0 {
			bases = append(bases, strings.TrimSpace(mb))
		}
	}
	if len(bases) == 0 {
		t.Fatalf("no merge-base with main or origin/main in %s", root)
	}
	base := bases[0]
	for _, cand := range bases[1:] {
		// Prefer the nearer fork point: cand is nearer when base is its ancestor.
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "--is-ancestor", base, cand); code == 0 {
			base = cand
		}
	}
	return base
}
