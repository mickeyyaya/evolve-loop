//go:build acs

package cycle1961

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const protectedRatchet = "go/internal/sizeratchet/offenders.json"

func runNamedTests(t *testing.T, pkg, pattern string, mustRun ...string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-v", "-run", pattern, pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: go test %s -run %s exited %d: %v\n%s\n%s", pkg, pattern, code, err, out, errOut)
	}
	for _, name := range mustRun {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s never ran green under -run %s (a pattern that matches nothing passes vacuously)\n%s", name, pattern, out)
		}
	}
}

func TestC1961_001_TwoConsecutiveZeroShipWavesHaltBeforeTheThirdLaunch(t *testing.T) {
	runNamedTests(t, "./cmd/evolve",
		`^(TestRunLoop_ZeroShip_TwoConsecutiveZeroShipWavesHaltBeforeTheThird|TestPrepareIteration_ZeroShip_AWaveIsTheCyclesBetweenTwoIterationBoundaries)$`,
		"TestRunLoop_ZeroShip_TwoConsecutiveZeroShipWavesHaltBeforeTheThird",
		"TestPrepareIteration_ZeroShip_AWaveIsTheCyclesBetweenTwoIterationBoundaries")
}

func TestC1961_002_AShipResetsTheCountAndNoWorkLanesCountAsZeroShip(t *testing.T) {
	runNamedTests(t, "./cmd/evolve",
		`^(TestRunLoop_ZeroShip_FailedAndSkippedNoWorkWavesCountAsZeroShip|TestRunLoop_ZeroShip_AShipResetsTheCount|TestPrepareIteration_ZeroShip_AnIterationThatLaunchedNothingIsNotAWave)$`,
		"TestRunLoop_ZeroShip_FailedAndSkippedNoWorkWavesCountAsZeroShip",
		"TestRunLoop_ZeroShip_AShipResetsTheCount",
		"TestPrepareIteration_ZeroShip_AnIterationThatLaunchedNothingIsNotAWave")
}

func TestC1961_003_PureRuleCountsOnlyTheTrailingZeroShipRun(t *testing.T) {
	runNamedTests(t, "./internal/core",
		`^(TestEvaluateZeroShip|TestEvaluateZeroShip_DoesNotMutateItsInput)$`,
		"TestEvaluateZeroShip", "TestEvaluateZeroShip_DoesNotMutateItsInput")
}

func TestC1961_004_ThresholdIsReadFromPolicyJSONWithACompiledDefaultOfTwo(t *testing.T) {
	runNamedTests(t, "./internal/policy", `^TestZeroShipHalt_`,
		"TestZeroShipHalt_CompiledDefaultIsTwo",
		"TestZeroShipHalt_AbsentPolicyBlockResolvesToTheDefault",
		"TestZeroShipHalt_ReadsFromPolicyJSON",
		"TestZeroShipHalt_OverrideLeavesSiblingThresholdsAtTheirDefaults")
	runNamedTests(t, "./cmd/evolve", `^TestRunLoop_ZeroShip_ThresholdComesFromPolicyJSON$`,
		"TestRunLoop_ZeroShip_ThresholdComesFromPolicyJSON")
}

func TestC1961_005_TheSizeRatchetBaselineIsUntouched(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain", "--", protectedRatchet)
	if err != nil || code != 0 {
		t.Fatalf("RED: git status exited %d: %v", code, err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("RED: %s was modified:\n%s", protectedRatchet, out)
	}
}

// acs-predicate: config-check
func TestC1961_006_OperatingPolicyNoLongerClaimsNoCompiledZeroShipCounter(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "operating-policy.md")
	if !acsassert.FileNotContains(t, doc, "No Go code counts zero-ship cycles") {
		t.Errorf("RED: operating-policy.md section 4.1 still says no Go code counts zero-ship cycles")
	}
	if !acsassert.FileContains(t, doc, "zero_ship_halt") {
		t.Errorf("RED: operating-policy.md section 4.1 does not name the zero_ship_halt dial")
	}
}

// acs-predicate: config-check
func TestC1961_007_RuntimeReferenceDocumentsTheZeroShipHaltDialAndItsDefault(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
	if !acsassert.LineContainsAll(doc, "failure_policy.thresholds.zero_ship_halt", "`2`", "LOOP_ZERO_SHIP_HALT") {
		t.Errorf("RED: runtime-reference.md has no env/dial row naming failure_policy.thresholds.zero_ship_halt, its compiled default `2` and LOOP_ZERO_SHIP_HALT")
	}
}

var countWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
	"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
}

var statedCount = regexp.MustCompile(`(?i)\b(\d+|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve)\b`)

func TestC1961_008_ExplanationSerialTestCountMatchesTheSerialRegistry(t *testing.T) {
	root := acsassert.RepoRoot(t)
	registry, err := os.ReadFile(filepath.Join(root, "go", "cmd", "evolve", "parallel_serial_tests_test.go"))
	if err != nil {
		t.Fatalf("read serial registry: %v", err)
	}
	registered := 0
	for _, line := range strings.Split(string(registry), "\n") {
		if strings.Contains(line, `"main.`) && strings.Contains(line, "ZeroShip") {
			registered++
		}
	}
	if registered == 0 {
		t.Fatalf("RED: no ZeroShip test is registered serial in parallel_serial_tests_test.go")
	}
	docs, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1961-*.md"))
	if err != nil || len(docs) != 1 {
		t.Fatalf("want exactly one cycle-1961 explanation document, got %v (err %v)", docs, err)
	}
	raw, err := os.ReadFile(docs[0])
	if err != nil {
		t.Fatal(err)
	}
	checked := false
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, "parallel_serial_tests_test.go") {
			continue
		}
		after := line[strings.Index(line, "parallel_serial_tests_test.go")+len("parallel_serial_tests_test.go"):]
		m := statedCount.FindString(after)
		if m == "" {
			continue
		}
		checked = true
		got, ok := countWords[strings.ToLower(m)]
		if !ok {
			got = 0
			for _, r := range m {
				got = got*10 + int(r-'0')
			}
		}
		if got != registered {
			t.Errorf("RED: explanation document says %q serial tests, parallel_serial_tests_test.go registers %d ZeroShip tests", m, registered)
		}
	}
	if !checked {
		t.Logf("explanation document states no count for the serial registry; nothing to contradict")
	}
}
