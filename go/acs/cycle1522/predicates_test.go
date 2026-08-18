//go:build acs

// Package cycle1522 materializes the cycle-1522 acceptance criteria for the
// sole committed task of this fleet lane, `cap-audit-report-length`
// (scout-report.md ## Selected Tasks Task 1; triage-report.md ## top_n).
// fleet_scope pins this lane to that one todo-id, so per R9.3 no predicate here
// binds to any other lane's item, nor to the deferred
// `build-report-length-cap` sibling.
//
// Task: audit-report.md has no upper bound on total size. The ## Issues table
// grows one row per finding, ship re-reads and SHA-binds the whole file
// (go/internal/phases/ship/audit.go:83), and the next cycle's handoff carries
// the prior audit — so an unbounded report compounds token cost on every read.
// The fix follows defect_ledger.go:56-61's established idiom: bound it, RECORD
// the overflow, never silently drop or truncate.
//
// AC map (1:1 with .evolve/evals/cap-audit-report-length.md):
//
//	AC1 "auditReportMaxBytes const exists with a sane budget"
//	    → C1522_001 (the frozen unit contract's cap_value_sane sub-test; the
//	      const is also a compile dependency of that file, so an absent const
//	      is a build failure, not a silent skip).
//	AC2 "over-cap emits exactly one warning-severity diagnostic naming size
//	     and cap"
//	    → C1522_001 (over_cap_warns_once).
//	AC3 "under-cap is silent; the exact boundary is explicit (== cap silent,
//	     > cap warns)"
//	    → C1522_001 (under_cap_silent, exact_boundary_silent) — the negative /
//	      edge axis: an unconditional warner fails these.
//	AC4 "diagnostic-only: the verdict never flips on size, the on-disk
//	     artifact is never mutated (ship SHA-binds it)"
//	    → C1522_001 (over_cap_does_not_flip_verdict,
//	      over_cap_does_not_mutate_artifact) + C1522_003 (regression axis: the
//	      existing verdict-conflict semantics stay green alongside the cap).
//	AC5 "the documented cap matches the code cap — no prompt/gate drift"
//	    → C1522_002 (parses the value out of the Go const and requires the
//	      auditor reference doc to carry the SAME number).
//
// Predicate-quality posture (cycle-85 rule): C1522_001/003 EXECUTE the system
// under test — each shells the audit package's own tests, which call the real
// production seam hooks.Classify (the one runner.BaseRunner.Run invokes at
// runner.go:1117) — and they count NAMED "--- PASS:" markers rather than
// trusting exit 0, so a renamed, deleted, or skipped sub-test cannot green
// them. C1522_002 is a declared config/doc-sync check (waiver below): it is
// not a magic-string grep, because the expected string is DERIVED from the
// code's own constant, so it fails on drift in either direction.
//
// Reliability posture: every subprocess names ONE package and narrows with
// -run (the whole audit package is a 22s suite under no load — excluded
// deliberately); no wall-clock bounds, no literal PIDs, no bare-git, no
// unreaped load generators.
package cycle1522

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const auditPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"

// lengthSubTests is the full committed sub-test surface of the frozen RED
// contract (go/internal/phases/audit/audit_report_length_test.go). Each is
// required to report PASS by name.
var lengthSubTests = []string{
	"TestAuditReportLength/cap_value_sane",
	"TestAuditReportLength/under_cap_silent",
	"TestAuditReportLength/exact_boundary_silent",
	"TestAuditReportLength/over_cap_warns_once",
	"TestAuditReportLength/over_cap_does_not_flip_verdict",
	"TestAuditReportLength/over_cap_does_not_mutate_artifact",
}

// TestC1522_001_audit_report_length_contract_green — AC1-AC4. Runs the size
// contract against the real Classify path and requires an explicit PASS marker
// for every sub-test. RED until Builder declares auditReportMaxBytes (build
// failure) and emits the warning-only overflow diagnostic.
func TestC1522_001_audit_report_length_contract_green(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", "TestAuditReportLength", "-v", auditPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run TestAuditReportLength %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			auditPkg, code, err, stdout, stderr)
	}
	for _, name := range lengthSubTests {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("sub-test %s did not report PASS (renamed, skipped, or not run) — "+
				"exit 0 alone cannot prove the size contract ran\nstdout:\n%s", name, stdout)
		}
	}
}

// constValueRE captures the right-hand side of the cap declaration, allowing
// either a plain literal (65536) or a product (64 * 1024), the two shapes the
// neighbouring bounding consts in this package use.
var constValueRE = regexp.MustCompile(`auditReportMaxBytes\s*=\s*([0-9]+(?:\s*\*\s*[0-9]+)*)`)

// capBytesFromSource parses auditReportMaxBytes out of audit.go. It is the
// SOURCE of the expected doc string, never an assertion on its own — the
// predicate below fails if the doc disagrees with whatever value it finds.
func capBytesFromSource(t *testing.T, root string) int {
	t.Helper()
	src := filepath.Join(root, "go", "internal", "phases", "audit", "audit.go")
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	m := constValueRE.FindSubmatch(body)
	if m == nil {
		t.Fatalf("auditReportMaxBytes is not declared with a numeric value in %s — "+
			"the cap has no single source for the doc to track", src)
	}
	value := 1
	for _, factor := range strings.Split(string(m[1]), "*") {
		n, convErr := strconv.Atoi(strings.TrimSpace(factor))
		if convErr != nil {
			t.Fatalf("cannot parse cap expression %q: %v", string(m[1]), convErr)
		}
		value *= n
	}
	return value
}

// TestC1522_002_doc_cap_matches_code_cap — AC5. The auditor persona is what
// actually shrinks reports at the source, so a doc that names a DIFFERENT
// budget than the gate enforces is the drift this predicate exists to catch.
// acs-predicate: config-check — this is an inherent cross-artifact consistency
// check (code constant vs. documented budget); the expected string is derived
// from the code, so it cannot be satisfied by pasting a magic string.
func TestC1522_002_doc_cap_matches_code_cap(t *testing.T) {
	root := acsassert.RepoRoot(t)
	capBytes := capBytesFromSource(t, root)
	if capBytes < 8*1024 || capBytes > 1<<20 {
		t.Errorf("auditReportMaxBytes=%d out of the sane range [8KiB, 1MiB]", capBytes)
	}
	doc := filepath.Join(root, "agents", "evolve-auditor-reference.md")
	// Accepted renderings of the same number: the raw byte count, or the KiB
	// form when the cap is a whole multiple of 1024 (how a prose budget reads).
	forms := []string{strconv.Itoa(capBytes)}
	if capBytes%1024 == 0 {
		kib := capBytes / 1024
		forms = append(forms,
			fmt.Sprintf("%dKB", kib), fmt.Sprintf("%d KB", kib),
			fmt.Sprintf("%dKiB", kib), fmt.Sprintf("%d KiB", kib))
	}
	if !acsassert.FileContainsAny(doc, forms...) {
		t.Errorf("agents/evolve-auditor-reference.md documents no budget matching the code cap "+
			"auditReportMaxBytes=%d (accepted forms: %v) — prompt and gate have drifted, which is "+
			"the failure mode the doc-sync AC exists to prevent", capBytes, forms)
	}
}

// TestC1522_003_verdict_semantics_unregressed — AC4 regression axis. The size
// check lands inside Classify, the same function the EGPS override lives in; a
// cap that perturbed diagnostic emission or the override would show up here as
// a verdict-conflict regression. Narrowed to the conflict suite (the whole
// audit package is a 22s run) and counts named PASS markers.
func TestC1522_003_verdict_semantics_unregressed(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", "TestVerdictConflict", "-v", auditPkg)
	if code != 0 || err != nil {
		t.Fatalf("verdict-conflict suite regressed: go test -run TestVerdictConflict %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			auditPkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: TestVerdictConflict") {
		t.Errorf("no TestVerdictConflict PASS marker — the regression axis did not actually run "+
			"(renamed or filtered out)\nstdout:\n%s", stdout)
	}
}
