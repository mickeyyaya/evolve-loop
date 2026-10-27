//go:build acs

package cycle2056

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runGo(t *testing.T, args ...string) string {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutput("go", append([]string{"-C", goDir(t)}, args...)...)
	if err != nil || code != 0 {
		t.Fatalf("go %s exit=%d: %v\n%s", strings.Join(args, " "), code, err, acsassert.OutputTail(out+errOut, 40))
	}
	return out + errOut
}

func requireRan(t *testing.T, output string, tests ...string) {
	t.Helper()
	for _, name := range tests {
		if !strings.Contains(output, "--- PASS: "+name) {
			t.Errorf("%s did not run and pass:\n%s", name, acsassert.OutputTail(output, 30))
		}
	}
}

func TestC2056_001_RowFromRequestCarriesCycleRunProfileAndRetryPurpose(t *testing.T) {
	out := runGo(t, "test", "-count=1", "-v", "./internal/bridge", "-run",
		`^(TestRecordModelAttempt_RowCarriesCycleRunProfileAndRetryPurpose|TestEngineLaunch_RowCarriesTheRequestAttribution)$`)
	requireRan(t, out,
		"TestRecordModelAttempt_RowCarriesCycleRunProfileAndRetryPurpose",
		"TestEngineLaunch_RowCarriesTheRequestAttribution")
}

func TestC2056_002_CorrectionDirectiveGivesContractCorrectionPurpose(t *testing.T) {
	out := runGo(t, "test", "-count=1", "-v", "./internal/bridge", "-run", `^TestRecordModelAttempt_PurposeDerivation$`)
	requireRan(t, out,
		"TestRecordModelAttempt_PurposeDerivation/correction_directive_on_a_first_attempt",
		"TestRecordModelAttempt_PurposeDerivation/correction_directive_outranks_retry",
		"TestRecordModelAttempt_PurposeDerivation/first_attempt",
		"TestRecordModelAttempt_PurposeDerivation/later_attempt_is_a_retry")
}

func TestC2056_003_OldRowWithoutNewFieldsDecodesUnchangedThroughReadAndImport(t *testing.T) {
	out := runGo(t, "test", "-count=1", "-v", "./internal/llmcalls", "-run",
		`^(TestRead_OldRowWithoutAttributionDecodesUnchanged|TestImport_OldRowCopiesByteForByteAndAttributedRowKeepsItsFields|TestRecord_UnattributedRowWritesNoAttributionKeys|TestRecord_AttributionFieldsRoundTripUnderTheirJSONKeys)$`)
	requireRan(t, out,
		"TestRead_OldRowWithoutAttributionDecodesUnchanged",
		"TestImport_OldRowCopiesByteForByteAndAttributedRowKeepsItsFields",
		"TestRecord_UnattributedRowWritesNoAttributionKeys",
		"TestRecord_AttributionFieldsRoundTripUnderTheirJSONKeys")
}

func TestC2056_004_ShadowFiresNothingEnforceFiresTokensRowUnattributedOnce(t *testing.T) {
	out := runGo(t, "test", "-count=1", "-v", "./internal/bridge", "-run",
		`^(TestRecordModelAttempt_ShadowStageFiresNoSignalForACycleRowWithoutCycle|TestRecordModelAttempt_EnforceFiresOnceForACycleRowWithoutCycle|TestRecordModelAttempt_EnforceStaysQuietWhenTheRowIsAttributedOrNotACycleRow|TestTokensRowUnattributed_CodeIsRegisteredOnTheTokensModule)$`)
	requireRan(t, out,
		"TestRecordModelAttempt_ShadowStageFiresNoSignalForACycleRowWithoutCycle",
		"TestRecordModelAttempt_EnforceFiresOnceForACycleRowWithoutCycle",
		"TestRecordModelAttempt_EnforceStaysQuietWhenTheRowIsAttributedOrNotACycleRow",
		"TestTokensRowUnattributed_CodeIsRegisteredOnTheTokensModule")
}

func TestC2056_005_TouchedPackagesAreFormattedVettedPassingAndTheNewLinesAreCovered(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gofmtOut, _, code, err := acsassert.SubprocessOutput("gofmt", "-l",
		filepath.Join(root, "go", "internal", "llmcalls"), filepath.Join(root, "go", "internal", "bridge"),
		filepath.Join(root, "go", "internal", "signalcenter"))
	if err != nil || code != 0 || strings.TrimSpace(gofmtOut) != "" {
		t.Errorf("gofmt -l lists unformatted files (exit=%d err=%v):\n%s", code, err, gofmtOut)
	}
	runGo(t, "vet", "./internal/llmcalls", "./internal/bridge", "./internal/signalcenter")
	runGo(t, "test", "-count=1", "./internal/llmcalls")
	runGo(t, "test", "-count=1", "./internal/signalcenter")

	profile := filepath.Join(t.TempDir(), "bridge.cover")
	runGo(t, "test", "-count=1", "-coverprofile", profile, "./internal/bridge", "-run",
		`^(TestRecordModelAttempt_|TestEngineLaunch_RowCarriesTheRequestAttribution|TestTokensRowUnattributed_)`)
	blocks := uncoveredBlocks(t, profile)

	baseOut, _, baseCode, baseErr := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "refs/remotes/origin/main")
	if baseErr != nil || baseCode != 0 {
		t.Fatalf("merge-base with origin/main failed (exit=%d): %v", baseCode, baseErr)
	}
	base := strings.TrimSpace(baseOut)
	for _, rel := range []string{"go/internal/bridge/attempt_telemetry.go", "go/internal/bridge/signal.go"} {
		diff, _, diffCode, diffErr := acsassert.SubprocessOutput("git", "-C", root, "diff", "-U0", base, "--", rel)
		if diffErr != nil || diffCode != 0 {
			t.Fatalf("git diff %s failed (exit=%d): %v", rel, diffCode, diffErr)
		}
		for _, line := range addedLines(diff) {
			for _, block := range blocks[filepath.Base(rel)] {
				if line >= block.start && line <= block.end {
					t.Errorf("%s:%d is added by this change and not executed by the attribution tests (block %d-%d)", rel, line, block.start, block.end)
				}
			}
		}
	}
}

type lineSpan struct{ start, end int }

var (
	coverLine  = regexp.MustCompile(`^.*/([^/]+\.go):(\d+)\.\d+,(\d+)\.\d+ (\d+) (\d+)$`)
	hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
)

func uncoveredBlocks(t *testing.T, profile string) map[string][]lineSpan {
	t.Helper()
	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("coverage profile: %v", err)
	}
	out := map[string][]lineSpan{}
	for _, line := range strings.Split(string(raw), "\n") {
		m := coverLine.FindStringSubmatch(line)
		if m == nil || m[5] != "0" {
			continue
		}
		start, _ := strconv.Atoi(m[2])
		end, _ := strconv.Atoi(m[3])
		out[m[1]] = append(out[m[1]], lineSpan{start, end})
	}
	return out
}

func addedLines(diff string) []int {
	var lines []int
	for _, line := range strings.Split(diff, "\n") {
		m := hunkHeader.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		first, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		for i := 0; i < count; i++ {
			lines = append(lines, first+i)
		}
	}
	return lines
}
