//go:build acs

// Package cycle669 materialises the cycle-669 acceptance criteria for the single
// triage-committed (`## top_n`) task: observer-sink-close-race.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this lane:
//	  observer-sink-close-race (weight 0.92, priority H) — C669_001..003
//	Nothing is deferred or dropped in this lane (fleet scope pins it to this
//	item); the scout-report's `new-package-graduation-buildentry-gate`
//	narrative belongs to ANOTHER lane and gets ZERO predicates here.
//
// FEATURE CONTEXT
//
//	core_adapter.go's cancel closure waits ≤10s for the watcher goroutine,
//	then (pre-fix) unconditionally closed the events-sink *os.File. A watcher
//	wedged past the bound (e.g. a stuck liveness probe) later calls
//	emit() → sink.Write() — a use-after-close race on the fd. Fix of record:
//	close ONLY on the <-done arm (closeSinkAfterWait); the timeout arm accepts
//	the fd leak (OS reclaims at exit) and WARNs.
//
// PRE-EXISTING GREEN (disclosed, not gamed): the fix and its behavioural
// tests LANDED in commit 22a90595 (2026-07-08) — core_adapter.go's
// closeSinkAfterWait + core_adapter_sinkclose_test.go (+ the _amplify twin).
// The inbox item was stale when this cycle picked it up. These predicates are
// therefore GREEN at authoring time; their value is REGRESSION PINNING: any
// later edit that reverts to an unconditional Close, renames/deletes the
// behavioural tests, or introduces a race in the package flips them RED at
// audit time. The TDD report records the pre-existing-GREEN status per the
// RED-verification rules.
//
// PREDICATE QUALITY (cycle-85): every predicate EXERCISES the SUT — it shells
// `go test` / `go vet` against the real package and asserts the NAMED
// behavioural test emitted a `--- PASS:` marker in -v output. `go test -run X`
// on a missing test exits 0 with "no tests to run", so a bare exit-code check
// would vacuously green; the marker assertion defeats that hole.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Negative : C669_001 — the timeout arm must NOT Close (Close count 0
//     asserted by the bound test); run under -race so the exact reported
//     failure mode is exercised.
//   - Positive : C669_002 — the <-done arm still closes EXACTLY once (defeats
//     a degenerate "never close" fix) + nil-closer edge stays panic-free.
//   - Hygiene  : C669_003 — `go vet` clean AND the WHOLE package green under
//     -race (the inbox AC-3), not just the two named tests.
//
// TEST-NAME CONTRACT — the predicates target these exact committed test names
// in internal/adapters/observer (do NOT rename without updating this file):
//
//	TestCoreAdapter_NoSinkCloseRaceOnTimeout
//	TestCoreAdapter_SinkClosedOnNormalDone
//	TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe
package cycle669

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the worktree's go/ module directory for `go test -C <dir>` calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runNamedTest shells `go test -C <goDir> [extra...] -run <pattern> -v <pkg>`
// and returns combined stdout+stderr plus the exit code. It is the behavioural
// harness: the SUT is really executed, never grepped.
func runNamedTest(t *testing.T, extra []string, pattern, pkg string) (string, int) {
	t.Helper()
	args := append([]string{"test", "-C", goDir(t), "-count=1"}, extra...)
	args = append(args, "-run", pattern, "-v", pkg)
	out, errOut, code, _ := acsassert.SubprocessOutput("go", args...)
	return out + "\n" + errOut, code
}

// assertPassed fails the predicate unless every named test emitted a
// `--- PASS: <name>` marker (and the run exited 0). Defeats the
// "no tests to run" vacuous-green hole.
func assertPassed(t *testing.T, out string, code int, names ...string) {
	t.Helper()
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			t.Errorf("expected `--- PASS: %s` (exit=%d) — the behavioural test is missing, renamed, or failing.\nOutput:\n%s",
				n, code, tail(out))
			return
		}
	}
	if code != 0 {
		t.Errorf("named tests present but the package exited %d (a sibling test failed or the package does not build).\nOutput:\n%s",
			code, tail(out))
	}
}

// tail bounds captured output so a runaway log cannot bloat the verdict.
func tail(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return strings.Join(lines, "\n")
}

// TestC669_001_TimeoutArmNeverClosesSink binds inbox AC-1 (the negative/race
// criterion): with a wedged watcher (done never fires), the timeout arm must
// NOT close the events sink — Close count 0 is asserted by
// TestCoreAdapter_NoSinkCloseRaceOnTimeout, exercised here under -race so the
// reported use-after-close mode is the exact one probed. A revert to the
// pre-fix unconditional Close flips this RED.
func TestC669_001_TimeoutArmNeverClosesSink(t *testing.T) {
	out, code := runNamedTest(t, []string{"-race"},
		"TestCoreAdapter_NoSinkCloseRaceOnTimeout",
		"./internal/adapters/observer/")
	assertPassed(t, out, code, "TestCoreAdapter_NoSinkCloseRaceOnTimeout")
}

// TestC669_002_DoneArmClosesExactlyOnce binds inbox AC-2 (the positive twin):
// the normal <-done path must still close the sink EXACTLY once (no fd leak on
// the healthy path — a degenerate "never close" fix FAILs this), and the
// nil-closer edge (caller-supplied a.Sink with no Closer) stays panic-free.
func TestC669_002_DoneArmClosesExactlyOnce(t *testing.T) {
	out, code := runNamedTest(t, []string{"-race"},
		"TestCoreAdapter_SinkClosedOnNormalDone|TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe",
		"./internal/adapters/observer/")
	assertPassed(t, out, code,
		"TestCoreAdapter_SinkClosedOnNormalDone",
		"TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe")
}

// TestC669_003_ObserverPackageVetAndRaceClean binds inbox AC-3 (hygiene): the
// whole internal/adapters/observer package is `go vet` clean and green under
// the race detector — not merely the two named tests. Flips RED the moment a
// data race or vet regression lands anywhere in the package.
func TestC669_003_ObserverPackageVetAndRaceClean(t *testing.T) {
	vout, verr, vcode, _ := acsassert.SubprocessOutput("go", "vet", "-C", goDir(t),
		"./internal/adapters/observer/")
	if vcode != 0 {
		t.Errorf("AC-3: `go vet ./internal/adapters/observer/` exited %d.\nOutput:\n%s",
			vcode, tail(vout+"\n"+verr))
	}
	rout, rerr, rcode, _ := acsassert.SubprocessOutput("go", "test", "-C", goDir(t),
		"-race", "-count=1", "./internal/adapters/observer/")
	if rcode != 0 {
		t.Errorf("AC-3: package must be race-clean; `go test -race ./internal/adapters/observer/` exited %d.\nOutput:\n%s",
			rcode, tail(rout+"\n"+rerr))
	}
}
