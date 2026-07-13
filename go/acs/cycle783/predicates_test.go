//go:build acs

// Package cycle783 materializes the cycle-783 acceptance criteria for the sole
// committed task of this fleet lane, verify-and-close-token-cache-fidelity
// (scout-report.md ## Selected Tasks; fleet_scope pins this lane to the todo-id
// token-telemetry-input-cache-fidelity, so per R9.3 no predicates bind to any
// other lane's items).
//
// Task nature: CONVERGENCE. Scout found the underlying feature task
// (token-telemetry-input-cache-fidelity, inbox weight 0.96, cycle-779 TDD
// contract) already fully implemented on main — all 8 cycle-779 ACS predicates
// green under -race. This cycle's job is to (a) re-verify that claim
// behaviorally and (b) durably record closure so the id stops being
// re-proposed, without re-implementing anything.
//
// SEAM CORRECTION (surfaced per Core Rule 3): scout-report.md proposes marking
// `decision: "completed"` in `.evolve/state.json:evaluatedTasks`, but that key
// exists nowhere — not in state.json and not in any Go source (grep
// evaluatedTasks/EvaluatedTasks over go/internal: zero hits). Predicates bind
// to seams that actually exist instead: the live inbox (the real re-proposal
// source; the item already sits in inbox/processed/cycle-779/) and this
// cycle's build-report closure record.
//
// AC map (1:1, from scout-report.md Selected Task 1 verifiableBy + Acceptance
// Criteria Summary + Deferred):
//
//	AC1 "go test -race -tags acs ./acs/cycle779/... reports 8/8 PASS"
//	    → C783_001 re-runs the cycle-779 suite as a subprocess and counts the
//	      eight individual "--- PASS: TestC779_" markers (a bare exit-0 could
//	      hide a skipped/renamed predicate). PRE-EXISTING GREEN by design —
//	      this IS the verification half of a verify-and-close task; bound so
//	      audit re-proves the claim instead of trusting the scout.
//	AC2 "record completion so the task stops being re-proposed"
//	    → C783_002 (closure record in this cycle's build-report naming the id,
//	      the 8/8 evidence, and the completed decision — RED until Builder
//	      writes it) + C783_003 (negative: no LIVE inbox item for the id —
//	      the actual re-proposal channel; processed/ items don't re-surface).
//	AC3 "evolve eval quality-check confirms the eval asserts real command
//	    output, not existence checks"
//	    → C783_004 runs the SSOT checker (internal/evalqualitycheck, the exact
//	      code behind `evolve eval quality-check`) against the task's eval
//	      file and requires Overall==PASS over a NON-EMPTY command set (an
//	      eval with no ```bash block passes vacuously — that hole is closed
//	      here). RED until the eval file exists with classified-PASS commands;
//	      authored during the TDD phase per Step 6b.
//	AC4 "live-soak: evolve tokens report shows non-zero input / cache-hit
//	    ratio against a real soaked batch"
//	    → manual+checklist in test-report.md (needs a live batch; carried
//	      over verbatim from the cycle-779 contract's own deferral).
//
// Adversarial axes: negative (C783_003 live-inbox absence; C783_004 rejects
// the vacuous zero-command PASS), edge (C783_001 rejects exit-0-with-fewer-
// than-8-PASS — rename/skip gaming), semantic (re-verification vs closure
// bookkeeping vs eval rigor are distinct behaviors). No source-grep
// predicates (cycle-85 rule): C783_001/004 execute the system under test;
// C783_002/003 assert on real emitted runtime artifacts (build-report,
// inbox), not source files.
package cycle783

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	taskID      = "token-telemetry-input-cache-fidelity"
	evalSlug    = "verify-and-close-token-cache-fidelity"
	cycle779Pkg = "github.com/mickeyyaya/evolve-loop/go/acs/cycle779"
)

// cycle779PredicateNames is the full committed AC surface of the underlying
// task (go/acs/cycle779/predicates_test.go). C783_001 requires an explicit
// verbose PASS for every one — "8/8" is counted, never assumed from exit 0.
var cycle779PredicateNames = []string{
	"TestC779_001_scanner_extracts_input_and_cache",
	"TestC779_002_scanner_absent_cache_fields_not_fabricated",
	"TestC779_003_per_driver_coverage_warns_not_zeros",
	"TestC779_004_unknown_driver_fails_open_no_error",
	"TestC779_005_engine_passes_driver_to_resolver",
	"TestC779_006_tokens_report_coverage_line_present",
	"TestC779_007_tokens_report_zero_window_not_covered",
	"TestC779_008_tokenusage_package_race_clean",
}

// stateRoot resolves the MAIN project root (the STATE root): the suite exports
// EVOLVE_PROJECT_ROOT (issue #12), else the repo root (redteam idiom).
func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

// AC1: the cycle-779 acceptance suite is 8/8 green under -race in THIS tree.
// Pre-existing GREEN by design (verification task); rejects exit-0 gaming by
// counting each named PASS marker.
func TestC783_001_cycle779_suite_reverified_8of8(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-tags", "acs", "-v", cycle779Pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race -tags acs %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			cycle779Pkg, code, err, stdout, stderr)
	}
	for _, name := range cycle779PredicateNames {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("cycle-779 predicate %s did not report PASS (renamed, skipped, or not run)", name)
		}
	}
}

// AC2: this cycle's build-report durably records the closure — the task id,
// the verification evidence (8/8), and the completed decision must appear on
// one line each so downstream scouts/triage can grep the record. RED until
// Builder writes the record.
func TestC783_002_closure_recorded_in_build_report(t *testing.T) {
	report := filepath.Join(stateRoot(t), ".evolve", "runs", "cycle-783", "build-report.md")
	if !acsassert.FileExists(t, report) {
		return // FileExists already failed the test with the path
	}
	if !acsassert.LineContainsAll(report, taskID, "completed") {
		t.Errorf("build-report has no line marking %s as completed", taskID)
	}
	if !acsassert.LineContainsAll(report, taskID, "8/8") {
		t.Errorf("build-report closure for %s does not cite the 8/8 predicate evidence", taskID)
	}
}

// AC2 negative: no LIVE inbox item re-proposes the task id. Only top-level
// .evolve/inbox/*.json are live proposals (processed/ items never re-surface);
// a re-filed item would reopen the loop this task is closing.
func TestC783_003_no_live_inbox_reproposal(t *testing.T) {
	live, err := filepath.Glob(filepath.Join(stateRoot(t), ".evolve", "inbox", "*.json"))
	if err != nil {
		t.Fatalf("glob inbox: %v", err)
	}
	for _, f := range live {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("read live inbox item %s: %v", f, err)
			continue
		}
		if strings.Contains(string(data), `"id": "`+taskID+`"`) {
			t.Errorf("live inbox item %s still proposes %s — closure did not stick", f, taskID)
		}
	}
}

// AC3: the task's eval file passes the SSOT quality checker with a NON-EMPTY
// command set. Overall must be PASS (no WARN/HALT: no tautologies, no
// grep-own-argument checks) and at least two commands must be classified —
// an eval whose ```bash block is missing or empty PASSes vacuously, which is
// exactly the existence-check gaming the AC forbids.
func TestC783_004_eval_file_passes_quality_check(t *testing.T) {
	evalPath := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", evalSlug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", evalPath, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", evalPath, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Fatalf("eval %s classified only %d command(s) — a vacuous/empty eval is not a PASS", evalPath, len(res.Commands))
	}
}
