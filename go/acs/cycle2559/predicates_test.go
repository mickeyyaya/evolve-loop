//go:build acs

package cycle2559

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgePkg      = "./internal/bridge"
	packageNoteRel = "docs/architecture/packages/internal-bridge.md"
	behaviorTest   = "TestNewTestEngine_SandboxReadsNeverSpawnABinary"
	guardTest      = "TestBridgeTestEnginesInjectSandboxFakes"
	maxSeconds     = 5.0
)

var timedTest = regexp.MustCompile(`^(TestTryRecord_|TestLaunchArgs_)[^/]*$`)

type testEvent struct {
	Action  string
	Test    string
	Elapsed float64
}

func moduleDir(t *testing.T) string {
	t.Helper()
	return acsassert.GoModuleDir(t)
}

func runBridgeTests(t *testing.T, pattern string) []testEvent {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	args := []string{"test", "-json", "-count=1", "-run", pattern, bridgePkg}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir(t)
	cmd.Env = ipcenv.GoTestEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	_ = cmd.Run()
	var events []testEvent
	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 1<<20), 16<<20)
	for scanner.Scan() {
		var ev testEvent
		if json.Unmarshal(scanner.Bytes(), &ev) == nil {
			events = append(events, ev)
		}
	}
	if len(events) == 0 {
		t.Fatalf("go %v produced no test event\nstderr:\n%s", args, acsassert.OutputTail(stderr.String(), 30))
	}
	return events
}

func outcomes(events []testEvent) map[string]testEvent {
	out := map[string]testEvent{}
	for _, ev := range events {
		if ev.Test != "" && (ev.Action == "pass" || ev.Action == "fail" || ev.Action == "skip") {
			out[ev.Test] = ev
		}
	}
	return out
}

func TestC2559_001_NoTestEngineReachesTheRealLogShowPoll(t *testing.T) {
	dir := moduleDir(t)
	for _, name := range []string{guardTest, "TestRealSandboxEngines_FlagsADirectNewEngineWithoutBothFakes"} {
		acsassert.GoTests(t, acsassert.GoTestSpec{Dir: dir, Package: bridgePkg, Pattern: "^" + name + "$", Names: []string{name}})
	}
}

func TestC2559_002_ATestEngineAnswersTheFenceAndTheReportsWithoutASpawn(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir: moduleDir(t), Package: bridgePkg, Pattern: "^" + behaviorTest + "$", Names: []string{behaviorTest},
	})
}

func TestC2559_003_TheTryRecordAndLaunchArgsTestsEachRunUnderFiveSeconds(t *testing.T) {
	events := runBridgeTests(t, "^("+behaviorTest+"|TestTryRecord_.*|TestLaunchArgs_.*)$")
	results := outcomes(events)
	timed := 0
	for name, ev := range results {
		if !timedTest.MatchString(name) {
			continue
		}
		timed++
		if ev.Action != "pass" {
			t.Errorf("%s %sed in the scrubbed environment", name, ev.Action)
		}
		if ev.Elapsed >= maxSeconds {
			t.Errorf("%s ran %.1fs, want under %.0fs", name, ev.Elapsed, maxSeconds)
		}
	}
	if timed == 0 {
		t.Fatal("no TestTryRecord_ or TestLaunchArgs_ test ran, so the timing proves nothing")
	}
	if got := results[behaviorTest]; got.Action != "pass" {
		t.Errorf("%s did not pass (%q)", behaviorTest, got.Action)
	}
}

var (
	headingLine   = regexp.MustCompile(`^(#{2,6}) `)
	engineHeading = regexp.MustCompile(`(?i)^#{2,6} .*test engines?\b`)
)

func sectionUnderHeading(text string) (string, bool) {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !engineHeading.MatchString(line) {
			continue
		}
		level := len(headingLine.FindStringSubmatch(line)[1])
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if m := headingLine.FindStringSubmatch(lines[j]); m != nil && len(m[1]) <= level {
				end = j
				break
			}
		}
		return strings.Join(lines[i:end], "\n"), true
	}
	return "", false
}

// acs-predicate: config-check
func TestC2559_004_ThePackageNoteRecordsTheTestEngineRuleInSteWithZeroFindings(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, packageNoteRel))
	if err != nil {
		t.Fatal(err)
	}
	section, ok := sectionUnderHeading(string(raw))
	if !ok {
		t.Fatalf("RED: %s has no heading about test engines, so the rule that every test engine injects the sandbox fakes is not recorded", packageNoteRel)
	}
	for _, want := range []string{"newTestEngine", guardTest, "SandboxReports", "SandboxFence"} {
		if !strings.Contains(section, want) {
			t.Errorf("the test engine section does not name %s", want)
		}
	}
	standard, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for _, f := range stelint.Check([]byte(section), stelint.Options{Words: standard}) {
		if !f.IsSkipNotice() {
			t.Errorf("%s [%s] %s", packageNoteRel, f.Rule, f.Message)
		}
	}
}
