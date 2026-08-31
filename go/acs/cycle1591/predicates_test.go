//go:build acs

// Package cycle1591 materializes the cycle-1591 acceptance criteria for the two
// committed tasks of this fleet lane (scout-report.md ## Selected Tasks;
// triage-report.md ## top_n): `retire-stale-retro-prompt-delivery-stall` and
// `retro-delivery-format-binding`. Per R9.3 no predicate binds to a deferred or
// dropped item (`todo-retire-vacuous-retro-prompt-delivery-stall-eval` is
// dropped and gets ZERO predicates here).
//
// The defect: the live inbox record
// `.evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json` keeps
// being "retired" by a filesystem-only removal (a delete, or a move into the
// .gitignore'd `.evolve/inbox/processed/`) with no matching `git rm`, so it stays
// in the Git INDEX and the next fresh checkout brings it back live. The
// build-floor gate that exists to catch a false removal claim,
// core.RemovalClaimFailures, asks only the worktree filesystem
// (build_removal_check.go:60-62), so the false retirement passes the floor. The
// second task pins the producer→consumer format binding the record's own
// incident depends on: the bridge's classified `submit_wedged` cause must
// survive into core.DeliveryFailureCause, which is what retro.go:203 keys its
// single relaunch on.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 tracked-but-absent claim → exactly one failure          → C1591_001
//	AC2 NEGATIVE: untracked-absent honest, non-repo fails open   → C1591_002
//	AC3 the live inbox record is retired as a TRACKED deletion   → C1591_003
//	AC4 eval `retire-stale-retro-prompt-delivery-stall` is rigorous → C1591_004
//	AC5 real bridge submit_wedged error is classified by the
//	    consumer's own classifier (format binding)               → C1591_005
//	AC6 NEGATIVE: a real generic silence timeout is NOT classified → C1591_006
//	AC7 retro's relaunch consumer stays green on that classifier → C1591_007
//	AC8 eval `retro-delivery-format-binding` is rigorous         → C1591_008
//	AC9 dispositions/provenance preserved in build evidence      → manual+checklist
//	AC10 repo-wide `go test -count=1 ./...` green                → manual+checklist
//	     (a /... sweep is a banned flaky-predicate shape; CI + the ship gate own it)
//
// Adversarial axes: negative (C1591_002 — an index-aware check must not start
// failing every absent path, and must fail open outside a repo; C1591_006 — the
// classifier must not over-fire on ordinary silence), edge (C1591_003 asserts
// BOTH index absence and disk absence — either alone is the exact half-retirement
// that caused the recurrence), semantic (index truth, record lifecycle, producer
// format, consumer relaunch and eval rigor are distinct behaviors).
//
// No source-grep predicates (cycle-85 rule): C1591_001/002/005/006/007 execute
// the real production code as subprocess `go test` runs and require a NAMED
// `--- PASS:` marker (a bare exit 0 would hide a renamed, skipped, or non-existent
// test); C1591_003 interrogates the real Git index and filesystem; C1591_004/008
// run the SSOT eval-quality checker. Every `go test` invocation names ONE package
// and is -run-narrowed (flaky-predicate-shape rule: no `/...` sweeps).
package cycle1591

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	retroPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"

	staleRecord = ".evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json"
)

// runNamedTest runs exactly one named test in ONE package and requires its
// `--- PASS:` marker. Never a /... sweep, always -run-narrowed.
func runNamedTest(t *testing.T, pkg, name string) string {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Errorf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			name, pkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("%s did not report PASS (renamed, skipped, or not authored)\nstdout:\n%s", name, stdout)
	}
	return stdout
}

// checkEval runs the SSOT eval-quality checker over one eval file and rejects a
// vacuous one (an eval with no graded commands PASSes trivially).
func checkEval(t *testing.T, slug string, minCommands int) {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", slug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: path})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", path, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", path, res.Overall)
	}
	if len(res.Commands) < minCommands {
		t.Fatalf("eval %s classified only %d command(s), want >= %d — a vacuous eval is not a PASS",
			path, len(res.Commands), minCommands)
	}
}

// AC1: the build floor must reject a removal claim for a path that is gone from
// the worktree but still in that worktree's Git index — the shape that let the
// stale record be "retired" three times and come back on every fresh checkout.
// The named test drives the production BuildFloorCheckFn against a REAL git
// index, not a stub.
func TestC1591_001_removal_claim_asks_the_git_index(t *testing.T) {
	runNamedTest(t, corePkg, "TestRemovalClaimFailures_TrackedButAbsentFromDisk")
}

// AC2 (NEGATIVE): the index check must not degenerate into "every absent path
// is a false claim". An untracked absent path stays an honest removal, and a
// worktree that is not a Git repository must still fail open — the floor may
// never false-block a build over its own plumbing. A fix that returns a failure
// unconditionally passes AC1 and dies here.
func TestC1591_002_honest_and_failopen_removals_preserved(t *testing.T) {
	runNamedTest(t, corePkg, "TestRemovalClaimFailures_UntrackedAbsent_StaysHonest")
}

// AC3: the live record must be retired as a TRACKED deletion — absent from the
// Git index AND absent from disk. Asserting only one half is exactly the
// half-retirement that recurred: gone from disk but still indexed (restored by
// the next checkout), or removed from the index but left on disk (re-added by a
// later `git add`). Both are interrogated against the real repository state.
func TestC1591_003_stale_inbox_record_retired_as_tracked_deletion(t *testing.T) {
	root := acsassert.RepoRoot(t)
	_, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", "--", staleRecord)
	if err == nil && code == 0 {
		t.Errorf("%s is STILL in the Git index — a filesystem-only retirement is undone by the next "+
			"fresh checkout (retire it with `git rm`, not a delete or a move into .evolve/inbox/processed/)", staleRecord)
	}
	if _, serr := os.Lstat(filepath.Join(root, staleRecord)); serr == nil {
		t.Errorf("%s is still present on disk — the record is still live for the queue scanner", staleRecord)
	}
}

// AC4: the task's eval is a permanent regression entry, so it must survive the
// SSOT quality checker over a non-empty command set.
func TestC1591_004_retire_eval_passes_quality_check(t *testing.T) {
	checkEval(t, "retire-stale-retro-prompt-delivery-stall", 3)
}

// AC5 (format binding): the bridge is the PRODUCER of the classified
// `submit_wedged` cause and core.DeliveryFailureCause is the CONSUMER retro
// keys its single relaunch on (retro.go:203). Today the bridge test asserts
// only that the marker substrings appear in the error text — a producer-side
// change to the reason= framing (quoting, key name, ordering) keeps that test
// green while the classifier silently returns "" and every wedged retro decays
// into a generic artifact-timeout. This binds the real Engine.Launch error to
// the consumer's own classifier.
func TestC1591_005_bridge_submit_wedged_error_is_classified_by_the_consumer(t *testing.T) {
	runNamedTest(t, bridgePkg, "TestEngineLaunch_PromptSubmitWedged_DeliveryCauseSurvivesClassifier")
}

// AC6 (NEGATIVE): the same classifier must return "" for a real, generically
// silent pane timeout. Over-firing would turn every ordinary slow phase into a
// bridge relaunch — the false-positive half of the contract.
func TestC1591_006_generic_silence_timeout_is_not_a_delivery_failure(t *testing.T) {
	runNamedTest(t, bridgePkg, "TestEngineLaunch_SilentPaneTimeout_NoDeliveryCause")
}

// AC7: the retro consumer itself must keep relaunching exactly once on a
// classified delivery failure and never on a generic timeout. Pinned so a fix
// that repairs the producer format while breaking the consumer branch cannot
// ship (pre-existing GREEN at RED time — bound deliberately).
func TestC1591_007_retro_relaunches_once_on_classified_delivery_failure(t *testing.T) {
	runNamedTest(t, retroPkg, "TestRun_SubmitWedgedDeliveryFailure_RelaunchesOnce")
	runNamedTest(t, retroPkg, "TestRun_GenericArtifactTimeout_DoesNotRelaunch")
}

// AC8: the format-binding task's permanent eval must also be rigorous.
func TestC1591_008_format_binding_eval_passes_quality_check(t *testing.T) {
	checkEval(t, "retro-delivery-format-binding", 3)
}
