//go:build acs

// Package cycle779 materializes the cycle-779 acceptance criteria for the sole
// committed top_n task token-telemetry-input-cache-fidelity (triage-report.md
// ## top_n; lane-scope.json assigns exactly this one id, so per R9.3 no
// predicates bind to the scout report's other-lane items — ship-window-lease,
// mechanical-scans-to-native, disjoint-composition-fastpath belong to other
// concurrent lanes).
//
// Task source: inbox 2026-07-13T13-06-00Z-token-input-cache-fidelity.json
// (weight 0.96, operator-boosted 2026-07-13). Incident: the first live token
// baseline (cycles 767-774) reported input=0 / cache_read=0 / cache_write=0
// for EVERY phase — output-only telemetry hides the dominant cost dimension
// (input outweighs output 2:1-100:1 per
// knowledge-base/research/token-optimization-2026) and blocks re-ranking the
// gated tokenopt-* items.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 "RED: TestScanner_ExtractsInputAndCacheFromClaudeUsageBlocks"
//	    → C779_001 (+ C779_002 no-fabrication edge). Authored this cycle in
//	      internal/tokenusage/inputcache_fidelity_test.go; observed
//	      PRE-EXISTING GREEN — the claude transcript scanner already extracts
//	      input/cache. Bound as regression so the fix cannot regress the
//	      already-working half; the LIVE defect is AC2/AC3.
//	AC2 "RED: TestScanner_PerDriverCoverageWarnsNotZeros"
//	    → C779_003 (coverage surfaced per driver), C779_004 (negative:
//	      unknown/uncovered driver fails OPEN — no error, explicit uncovered
//	      signal, never a silent zero attributed as covered), C779_005
//	      (bridge engine forwards the launch's CLI/driver into the resolver
//	      Window so per-driver dispatch is possible at all). These name unit
//	      contracts the Builder authors with the new seam (Window carries no
//	      driver today); each predicate stays RED (no PASS marker) until the
//	      named test exists AND passes.
//	AC3 "evolve tokens report shows non-zero input and a real cache-hit ratio
//	    for a soaked batch; coverage line present"
//	    → C779_006 (Coverage: line with phases-with-data/phases-run ratio),
//	      C779_007 (negative: an all-zero window reports 0/N, never claims
//	      coverage). Authored RED this cycle in
//	      cmd/evolve/cmd_tokens_coverage_test.go. The soaked-batch non-zero
//	      half needs a live batch → manual+checklist in test-report.md.
//	AC4 "go test -race PASS; apicover clean"
//	    → every delegated predicate runs under -race; C779_008 runs the whole
//	      touched tokenusage package under -race. apicover runs in the
//	      repo-wide CI-parity gate the audit executes (ADR-0069).
//
// Adversarial axes: negative (C779_004 uncovered-driver must not silently
// zero; C779_007 all-zero window must not claim coverage), edge (C779_002
// absent cache fields stay zero, not fabricated), semantic (extraction vs
// coverage-accounting vs driver-plumbing vs report-rendering are distinct
// behaviors). No source-grep predicates (cycle-85 rule) — every predicate
// exercises the system under test via `go test -race -run '^<name>$'` with a
// verbose "--- PASS:" guard rejecting rename/no-tests-matched silent greens.
package cycle779

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	tokPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	cmdPkg    = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
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

// AC1: claude transcript usage blocks yield non-zero Input/CacheRead/CacheWrite
// (pre-existing GREEN — bound as regression so the AC2/AC3 work can't break it).
func TestC779_001_scanner_extracts_input_and_cache(t *testing.T) {
	runGoTest(t, tokPkg, "TestScanner_ExtractsInputAndCacheFromClaudeUsageBlocks")
}

// AC1 edge: a usage block without cache fields contributes zero cache counts —
// the scanner must not fabricate data it cannot observe.
func TestC779_002_scanner_absent_cache_fields_not_fabricated(t *testing.T) {
	runGoTest(t, tokPkg, "TestScanner_UsageBlockMissingCacheFieldsStaysZeroNotFabricated")
}

// AC2: per-driver extraction surfaces a coverage outcome — a driver (agy/codex)
// whose sources carry no usage data yields an explicit uncovered/WARN signal,
// NOT a silent zero-usage result recorded as if covered (the baseline defect).
func TestC779_003_per_driver_coverage_warns_not_zeros(t *testing.T) {
	runGoTest(t, tokPkg, "TestScanner_PerDriverCoverageWarnsNotZeros")
}

// AC2 negative: an unknown driver fails OPEN — nil error, launch unaffected,
// coverage marked uncovered (telemetry must never fail a launch).
func TestC779_004_unknown_driver_fails_open_no_error(t *testing.T) {
	runGoTest(t, tokPkg, "TestScanner_UnknownDriverFailsOpenNoError")
}

// AC2 plumbing: the bridge engine forwards the launch's CLI/driver identity
// into the resolver Window — without it per-driver dispatch cannot exist
// (recordTokenUsage currently drops req.CLI on the floor).
func TestC779_005_engine_passes_driver_to_resolver(t *testing.T) {
	runGoTest(t, bridgePkg, "TestRecordTokenUsage_PassesDriverToResolver")
}

// AC3: `evolve tokens report` renders a Coverage: line with the
// phases-with-token-data / phases-run ratio.
func TestC779_006_tokens_report_coverage_line_present(t *testing.T) {
	runGoTest(t, cmdPkg, "TestTokensReport_CoverageLinePresent")
}

// AC3 negative: an all-zero window (the 2026-07-13 baseline shape) reports
// coverage 0/N — zero-token phases are uncovered, never covered-and-free.
func TestC779_007_tokens_report_zero_window_not_covered(t *testing.T) {
	runGoTest(t, cmdPkg, "TestTokensReport_CoverageCountsOnlyPhasesWithData")
}

// AC4: the whole touched tokenusage package passes under -race (the repo-wide
// apicover/CI-parity gate runs in audit per ADR-0069).
func TestC779_008_tokenusage_package_race_clean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", tokPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			tokPkg, code, err, stdout, stderr)
	}
}
