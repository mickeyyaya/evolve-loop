//go:build acs

// Package cycle1194 materialises the cycle-1194 acceptance criteria for the
// two fleet-scoped tasks pinned to this lane:
//
//   - bridgewatch-follow-macos-flake                       (predicates 001–002)
//   - loop-must-base-lanes-on-origin-main-not-stale-local   (predicate 003)
//
// CONTINUATION CONTEXT. This lane inherits a salvage snapshot (df84167e, ADR-0076
// continuation-on-fail) that already carries the fix for BOTH tasks, and a prior
// lane (cycle-1191, go/acs/cycle1191/predicates_test.go) already authored and
// GREENED the identical acceptance criteria against that same code:
//
//   - the follow suite (cmd/evolve/cmd_bridge_watch_test.go) replaced its fixed
//     10ms sleep / 200ms deadline race with an event-driven wait bounded by a
//     >=10s deadline in BOTH observing follow tests
//     (TestRunBridgeWatchFollow_SkipsMalformedAndEmptyLines and
//     TestRunBridgeWatchFollow_TailsNewLines);
//   - looppreflight.Run wires a `base-divergence` check (basedivergence.go) that
//     fetches origin, HALTs when local is behind, and names `evolve sync-main`.
//
// Every predicate below was authored and run FRESH against the LIVE artifacts in
// THIS worktree (not copied verbatim from cycle1191's cache) and is GREEN on
// first run — the disposition is "predicate / pre-existing GREEN", not RED. Per
// the TDD-engineer contract's "unexpected pass" rule, that status is logged
// explicitly in test-report.md rather than force-fitting an artificial failure.
// Each predicate still EXERCISES the system under test (the cycle-85
// degenerate-predicate ban): 001 runs the real follow-test suite under -race;
// 002 parses the Go AST of the real test file and asserts on the numeric
// deadline literal (a magic string cannot satisfy it); 003 runs the real
// looppreflight.Run against a real git repo whose local base is genuinely
// behind a real (file-remote) origin.
package cycle1194

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ---------------------------------------------------------------------------
// Task: bridgewatch-follow-macos-flake
// ---------------------------------------------------------------------------

// followTestFile is the file the flake lived in.
const followTestFile = "go/cmd/evolve/cmd_bridge_watch_test.go"

// observingFollowTests are the follow tests whose assertion depends on
// OBSERVING a line appended AFTER the follow loop asynchronously seeds its
// file offset — exactly the tests that could lose the seed race on a loaded
// macOS runner. The other two follow tests assert an ABSENCE (no output /
// non-fatal exit) and cannot flake on a slow runner, so they are correctly
// out of scope.
var observingFollowTests = []string{
	"TestRunBridgeWatchFollow_SkipsMalformedAndEmptyLines",
	"TestRunBridgeWatchFollow_TailsNewLines",
}

// minFollowDeadline is the inbox's floor: "the test's wait is event-driven
// with a deadline >= 10s".
const minFollowDeadline = 10

var (
	// secondsDeadlineRe matches a context deadline expressed in whole seconds.
	secondsDeadlineRe = regexp.MustCompile(`WithTimeout\([^,]+,\s*(\d+)\s*\*\s*time\.Second\s*\)`)
	// msSleepRe matches a fixed millisecond sleep — a bare wall-clock
	// assumption. A sleep of the poll interval (time.Sleep(watchFollowInterval))
	// is a retry cadence, not a deadline, and is deliberately NOT matched.
	msSleepRe = regexp.MustCompile(`time\.Sleep\([^)]*time\.Millisecond`)
)

// goFuncBody returns the source text of funcName in the Go file at path. A
// missing function is a LOUD error, never a silently-satisfied ==0 assertion.
func goFuncBody(path, funcName string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, raw, 0)
	if err != nil {
		return "", err
	}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != funcName {
			continue
		}
		return string(raw[fset.Position(fd.Pos()).Offset:fset.Position(fd.End()).Offset]), nil
	}
	return "", os.ErrNotExist
}

// TestC1194_001_follow_tests_race_clean_under_repetition executes the
// acceptance command (inbox acceptance criteria: 50/50 PASS under -race):
// the follow suite must be green under -race and repetition. This is the
// behavioural half of the flake criterion — a shape change that broke the
// tests' meaning cannot pass it.
func TestC1194_001_follow_tests_race_clean_under_repetition(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-race", "-count=50",
		"-run", "TestRunBridgeWatchFollow", "./cmd/evolve/")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test -race -count=50 -run TestRunBridgeWatchFollow ./cmd/evolve/ must be green: %v\n%s", err, out)
	}
}

// TestC1194_002_follow_waits_are_event_driven_with_long_deadline encodes the
// inbox acceptance criterion: "the test's wait is event-driven with a
// deadline >= 10s, no bare sleeps shorter than the deadline."
//
// It parses the AST and reads the NUMERIC deadline, so it cannot be satisfied
// by planting a magic string — the assertion is on the timing shape itself,
// which IS this task's deliverable.
func TestC1194_002_follow_waits_are_event_driven_with_long_deadline(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), followTestFile)
	for _, fn := range observingFollowTests {
		body, err := goFuncBody(path, fn)
		if err != nil {
			t.Errorf("%s: %v", fn, err)
			continue
		}
		m := secondsDeadlineRe.FindStringSubmatch(body)
		if m == nil {
			t.Errorf("%s: no seconds-scale context deadline found — a follow test that must OBSERVE an appended "+
				"line needs an event wait bounded by a deadline >= %ds, not a millisecond window that a loaded "+
				"macOS runner can miss", fn, minFollowDeadline)
			continue
		}
		secs, convErr := strconv.Atoi(m[1])
		if convErr != nil {
			t.Errorf("%s: unparsable deadline literal %q: %v", fn, m[1], convErr)
			continue
		}
		if secs < minFollowDeadline {
			t.Errorf("%s: context deadline is %ds, want >= %ds", fn, secs, minFollowDeadline)
		}
		if n := len(msSleepRe.FindAllString(body, -1)); n != 0 {
			t.Errorf("%s: %d fixed millisecond sleep(s) remain — the wait must synchronise on the observable "+
				"event (the rendered line), retrying at the poll interval, never on a wall-clock guess", fn, n)
		}
	}
}

// ---------------------------------------------------------------------------
// Task: loop-must-base-lanes-on-origin-main-not-stale-local
// ---------------------------------------------------------------------------

// git runs a git command in dir, failing the test loudly on error — a
// fixture that half-built would make the predicate assert on the wrong
// topology.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-c", "user.email=acs@example.com",
		"-c", "user.name=acs",
		"-c", "commit.gpgsign=false",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// commitFile writes a file and commits it.
func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-m", "acs: "+name)
}

// behindBaseRepo builds a real work tree on branch main whose local base is
// one commit BEHIND a real (file-remote) origin/main — the exact topology
// that made every cycle-969 lane ship GIT_PUSH_REJECTED. No network is
// involved.
func behindBaseRepo(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	work := filepath.Join(base, "work")
	for _, d := range []string{origin, work} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	git(t, origin, "init", "--bare", "-b", "main")
	git(t, work, "init", "-b", "main")
	git(t, work, "remote", "add", "origin", origin)
	commitFile(t, work, "a.txt", "one")
	git(t, work, "push", "origin", "main")
	commitFile(t, work, "b.txt", "two")
	git(t, work, "push", "origin", "main")
	// Rewind the LOCAL base only: origin/main keeps b.txt ⇒ local is behind 1.
	git(t, work, "reset", "--hard", "HEAD~1")
	return work
}

// TestC1194_003_boot_halts_on_base_behind_origin is the WIRING proof for the
// preflight halt: it runs the real looppreflight.Run (not the check function
// in isolation) against the behind-base topology and requires the
// base-divergence check to be REGISTERED, to fire at Halt, and to name the
// reconcile command. A check that exists but is not in Run's list fails here.
func TestC1194_003_boot_halts_on_base_behind_origin(t *testing.T) {
	work := behindBaseRepo(t)
	evolveDir := filepath.Join(work, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	res, err := looppreflight.Run(looppreflight.Options{
		ProjectRoot: work,
		EvolveDir:   evolveDir,
		ProfileDir:  filepath.Join(work, "profiles"),
		Stderr:      io.Discard,
		SkipBoot:    true,
	})
	if err != nil {
		t.Fatalf("looppreflight.Run harness fault: %v", err)
	}
	var found *looppreflight.CheckResult
	for i := range res.Checks {
		if res.Checks[i].Name == "base-divergence" {
			found = &res.Checks[i]
			break
		}
	}
	if found == nil {
		names := make([]string, 0, len(res.Checks))
		for _, c := range res.Checks {
			names = append(names, c.Name)
		}
		t.Fatalf("boot preflight runs no `base-divergence` check — lanes would still be cut from a stale base; checks=%v", names)
	}
	if found.Level != looppreflight.LevelHalt {
		t.Errorf("local main is 1 commit behind origin/main: base-divergence must HALT the batch before any lane spawns, got level=%v (%s / %s)",
			found.Level, found.Message, found.Detail)
	}
	if !strings.Contains(found.Detail, "evolve sync-main") {
		t.Errorf("the halt must name the reconcile command `evolve sync-main` so the stop comes with a next step; detail=%q", found.Detail)
	}
}
