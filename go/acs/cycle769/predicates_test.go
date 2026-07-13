//go:build acs

// Package cycle769 materializes the cycle-769 acceptance criteria for the sole
// committed top_n task boot-orphan-sweep-bounded-tombstone (triage-report.md
// ## top_n; fleet_scope pins this lane to exactly that id — the scout's own
// proposals stay with their owning cycles, so per R9.3 no predicates bind to
// them and nothing binds to deferred/dropped work).
//
// Task source: inbox id boot-orphan-sweep-bounded-tombstone (weight 0.90):
// every loop boot re-reaps the ENTIRE run history serially with
// context.Background() — 4,388 recorded sessions across 304 registries,
// unbounded growth, and a wedged tmux hangs boot forever. Fix contract:
// (1) tombstone fully-reaped registries so sweeps stop redoing history,
// (2) partial failure leaves the registry unmarked for retry,
// (3) the preflight boot sweep is deadline-bounded (orphanGCTimeout
//
//	discipline) via an injectable killer seam.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 second sweep skips fully-reaped runs      → C769_001
//	AC2 partial failure NOT tombstoned (retried)  → C769_002
//	AC3 preflight reap deadline-bounded           → C769_003
//	AC4 go vet / -race / apicover green           → C769_004 (vet); -race is
//	    embedded in every runGoTest invocation; apicover -enforce runs in the
//	    repo-wide audit gate (ADR-0069), not re-implemented here.
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract, which EXERCISES ReapOrphans / looppreflight.Run through
// injected counting/failing/deadline-capturing killers — behavioral via
// subprocess, no source-grep predicates (cycle-85 rule). The `-v` +
// "--- PASS:" guard rejects rename/no-tests-matched silent greens. The unit
// contract embeds the adversarial axes: negative (partial failure must NOT
// be tombstoned; a sibling's failure must not re-open a successful run's
// tombstone), edge/anti-overfit (a run appearing between sweeps is still
// reaped — the skip must key on the per-run marker, not global state),
// semantic (tombstone-skip, retry-on-failure, and deadline-bounding are
// three separate behaviors in two packages).
package cycle769

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	reaperPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/sessionreaper"
	preflightPkg = "github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
)

// runGoTest executes the named unit test under -race and requires an explicit
// verbose PASS marker so the predicate fails on: compile failure, test
// failure, a race report, a missing package, OR the test not existing
// (rename gaming).
func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

// AC1: a fully-successfully-reaped run is skipped by the next sweep, while a
// run that appeared between sweeps is still reaped (per-run marker, not
// process/dir-global "already ran" state).
func TestC769_001_SecondSweepSkipsReapedRuns(t *testing.T) {
	runGoTest(t, reaperPkg, "TestReapOrphans_SecondSweepSkipsReapedRuns")
}

// AC2 (negative axis): a killer error during a run's reap leaves that run
// unmarked — retried next sweep — and does not disturb a sibling run's
// tombstone. Over-eager tombstoning (mark-on-attempt) fails here.
func TestC769_002_PartialFailureNotTombstoned(t *testing.T) {
	runGoTest(t, reaperPkg, "TestReapOrphans_PartialFailureNotTombstoned")
}

// AC3: the loop-boot preflight orphan sweep hands its killer a context with a
// boot-scale deadline (≤30s), through the injectable Options.OrphanKill seam,
// instead of context.Background() — a wedged tmux no longer hangs boot.
func TestC769_003_PreflightOrphanReapDeadlineBounded(t *testing.T) {
	runGoTest(t, preflightPkg, "TestPreflight_OrphanReapIsDeadlineBounded")
}

// AC4: go vet clean on every package the fix touches (the inbox item's five
// named files). -race rides in each runGoTest; apicover -enforce is the
// repo-wide audit gate's job.
func TestC769_004_VetTouchedPackages(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "vet",
		"github.com/mickeyyaya/evolve-loop/go/internal/sessionreaper",
		"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight",
		"github.com/mickeyyaya/evolve-loop/go/internal/swarm",
		"github.com/mickeyyaya/evolve-loop/go/cmd/evolve",
	)
	if code != 0 || err != nil {
		t.Fatalf("go vet exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
