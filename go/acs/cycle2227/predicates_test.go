//go:build acs

package cycle2227

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	productionWindow = 500 * time.Millisecond
	shortTestWindow  = 5 * time.Millisecond
	missingReadBound = 50 * time.Millisecond
	runnerTestBound  = 1.0
	probeAttempts    = 5
)

type goTestEvent struct {
	Action  string  `json:"Action"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
}

func moduleDir(t *testing.T) string {
	t.Helper()
	dir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatalf("module dir: %v", err)
	}
	return dir
}

func goTestEvents(t *testing.T, pkg, runPattern string) (map[string]goTestEvent, string) {
	t.Helper()
	cmd := exec.Command("go", "test", "-count=1", "-json", "-run", runPattern, pkg)
	cmd.Dir = moduleDir(t)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	_ = cmd.Run()
	terminal := map[string]goTestEvent{}
	sc := bufio.NewScanner(&out)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var ev goTestEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Test == "" {
			continue
		}
		if ev.Action == "pass" || ev.Action == "fail" || ev.Action == "skip" {
			terminal[ev.Test] = ev
		}
	}
	return terminal, acsassert.OutputTail(out.String(), 15)
}

func missingBuildReport(t *testing.T) phasecontract.Roots {
	t.Helper()
	return phasecontract.Roots{Workspace: t.TempDir()}
}

func bestVerifyElapsed(t *testing.T, roots func() phasecontract.Roots) (time.Duration, deliverable.Result) {
	t.Helper()
	best := time.Duration(1<<62 - 1)
	var last deliverable.Result
	for i := 0; i < probeAttempts; i++ {
		start := time.Now()
		res, err := deliverable.Verify("build", roots())
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		last = res
		if elapsed < best {
			best = elapsed
		}
	}
	return best, last
}

func hasViolation(res deliverable.Result, code string) bool {
	for _, v := range res.Violations {
		if v.Code == code {
			return true
		}
	}
	return false
}

func TestC2227_001_RunnerArtifactTimeoutTestsFinishUnderOneSecond(t *testing.T) {
	wanted := []string{
		"TestRun_OptionalPhase_ArtifactTimeout_DegradesToWarn",
		"TestRun_MandatoryPhase_ArtifactTimeout_StillFails",
	}
	events, tail := goTestEvents(t, "./internal/phases/runner", `^TestRun_(OptionalPhase|MandatoryPhase)_ArtifactTimeout_`)
	for _, name := range wanted {
		ev, ok := events[name]
		if !ok {
			t.Errorf("RED: %s did not run — go test tail:\n%s", name, tail)
			continue
		}
		if ev.Action != "pass" {
			t.Errorf("RED: %s action=%s, want pass — go test tail:\n%s", name, ev.Action, tail)
			continue
		}
		if ev.Elapsed >= runnerTestBound {
			t.Errorf("RED: %s took %.2fs, want under %.0fs (16 settle probes each wait the full read grace)", name, ev.Elapsed, runnerTestBound)
		}
	}
}

func TestC2227_002_MissingDeliverableReadUnderTestWindowReturnsFast(t *testing.T) {
	t.Cleanup(deliverable.SetReadGraceWindowForTest(shortTestWindow))
	best, res := bestVerifyElapsed(t, func() phasecontract.Roots { return missingBuildReport(t) })
	if best >= missingReadBound {
		t.Errorf("RED: missing-file Verify took %v at best of %d under a %v test window, want under %v", best, probeAttempts, shortTestWindow, missingReadBound)
	}
	if res.OK || !hasViolation(res, deliverable.CodeMissingArtifact) {
		t.Errorf("RED: a shortened window must still confirm the artifact missing, got OK=%v violations=%+v", res.OK, res.Violations)
	}
}

func TestC2227_003_ShortWindowStillClassifiesPresentAndBlankDeliverables(t *testing.T) {
	t.Cleanup(deliverable.SetReadGraceWindowForTest(shortTestWindow))

	presentRoots := missingBuildReport(t)
	if err := os.WriteFile(filepath.Join(presentRoots.Workspace, "build-report.md"), []byte("present but not a well-formed report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := deliverable.Verify("build", presentRoots)
	if err != nil {
		t.Fatalf("Verify present: %v", err)
	}
	if hasViolation(res, deliverable.CodeMissingArtifact) || hasViolation(res, deliverable.CodeEmptyArtifact) {
		t.Errorf("RED: a present non-blank file must be read at once under any window, got %+v", res.Violations)
	}

	blankRoots := missingBuildReport(t)
	if err := os.WriteFile(filepath.Join(blankRoots.Workspace, "build-report.md"), []byte("  \n\t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	res, err = deliverable.Verify("build", blankRoots)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Verify blank: %v", err)
	}
	if !hasViolation(res, deliverable.CodeEmptyArtifact) {
		t.Errorf("RED: a blank file must be confirmed empty under the short window, got %+v", res.Violations)
	}
	if elapsed >= productionWindow {
		t.Errorf("RED: blank-file Verify took %v under a %v test window, want under the %v production window", elapsed, shortTestWindow, productionWindow)
	}
}

func TestC2227_004_ProductionKeepsFiveHundredMillisecondsAndRestoreReturnsToIt(t *testing.T) {
	readMissing := func() time.Duration {
		start := time.Now()
		if _, err := deliverable.Verify("build", missingBuildReport(t)); err != nil {
			t.Fatalf("Verify: %v", err)
		}
		return time.Since(start)
	}
	if got := readMissing(); got < productionWindow {
		t.Errorf("RED: default missing read took %v, want at least the %v production window", got, productionWindow)
	}
	restore := deliverable.SetReadGraceWindowForTest(shortTestWindow)
	restore()
	if got := readMissing(); got < productionWindow {
		t.Errorf("RED: after restore the missing read took %v, want at least the %v production window — the cleanup must put 500 ms back", got, productionWindow)
	}
}

func TestC2227_005_GraceTestFileKeepsOneRealWindowTest(t *testing.T) {
	events, tail := goTestEvents(t, "./internal/deliverable", `^TestReadDeliverableWithGrace`)
	if len(events) == 0 {
		t.Fatalf("RED: no TestReadDeliverableWithGrace test ran — go test tail:\n%s", tail)
	}
	realWindow := ""
	for name, ev := range events {
		if ev.Action == "fail" {
			t.Errorf("RED: %s failed — go test tail:\n%s", name, tail)
		}
		if ev.Action == "pass" && ev.Elapsed >= 0.45 {
			realWindow = name
		}
	}
	if realWindow == "" {
		t.Errorf("RED: no passing grace test waited the real %v window — grace_test.go must keep one", productionWindow)
	}
}

func TestC2227_006_ChangedGraceFunctionsHaveFullCoverage(t *testing.T) {
	dir := moduleDir(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	test := exec.Command("go", "test", "-count=1", "-run", "Grace", "-coverprofile="+profile, "./internal/deliverable")
	test.Dir = dir
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("RED: deliverable tests failed: %v\n%s", err, acsassert.OutputTail(string(out), 15))
	}
	cover := exec.Command("go", "tool", "cover", "-func="+profile)
	cover.Dir = dir
	out, err := cover.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, acsassert.OutputTail(string(out), 15))
	}
	for _, fn := range []string{"SetReadGraceWindowForTest", "readDeliverableWithGrace"} {
		pct, err := acsassert.FuncCoverage(string(out), fn)
		if err != nil {
			t.Errorf("RED: %v", err)
			continue
		}
		if pct < 100.0 {
			t.Errorf("RED: %s coverage %.1f%%, want 100.0%%", fn, pct)
		}
	}
}
