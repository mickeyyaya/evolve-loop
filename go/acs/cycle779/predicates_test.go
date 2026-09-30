//go:build acs

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

func TestC779_001_scanner_extracts_input_and_cache(t *testing.T) {
	runGoTest(t, tokPkg, "TestScanner_ExtractsInputAndCacheFromClaudeUsageBlocks")
}

func TestC779_002_scanner_absent_cache_fields_not_fabricated(t *testing.T) {
	runGoTest(t, tokPkg, "TestScanner_UsageBlockMissingCacheFieldsStaysZeroNotFabricated")
}

func TestC779_003_per_driver_coverage_warns_not_zeros(t *testing.T) {
	runGoTest(t, tokPkg, "TestScanner_PerDriverCoverageWarnsNotZeros")
}

func TestC779_004_unknown_driver_fails_open_no_error(t *testing.T) {
	runGoTest(t, tokPkg, "TestScanner_UnknownDriverFailsOpenNoError")
}

func TestC779_005_engine_passes_driver_to_resolver(t *testing.T) {
	runGoTest(t, bridgePkg, "TestRecordTokenUsage_PassesDriverToResolver")
}

func TestC779_006_tokens_report_coverage_line_present(t *testing.T) {
	runGoTest(t, cmdPkg, "TestTokensReport_CoverageLinePresent")
}

func TestC779_007_tokens_report_zero_window_not_covered(t *testing.T) {
	runGoTest(t, cmdPkg, "TestTokensReport_CoverageCountsOnlyPhasesWithData")
}

func TestC779_008_tokenusage_package_race_clean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", tokPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			tokPkg, code, err, stdout, stderr)
	}
}
