//go:build acs

package cycle2112

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	panelossFile   = "driver_tmux_wait_paneloss_test.go"
	dwellFile      = "transient_dwell_shortcircuit_test.go"
	perTestCeiling = 2 * time.Second
	dwellCeiling   = 5 * time.Second
	runBudget      = 9 * time.Minute
)

var assertionFloor = map[string]map[string]int{
	panelossFile: {
		"TestRunTmuxREPL_ATmuxServerKilledMidPhaseEndsTheDispatchAsPaneLostWithinOneLivenessInterval": 4,
		"TestRunTmuxREPL_ACaptureFailureOnALiveSessionDoesNotEndTheDispatch":                          2,
		"TestRunTmuxREPL_OneTickWithoutTheSessionIsNotYetAPaneLoss":                                   2,
		"TestRunTmuxREPL_ACancelDuringTheLossConfirmationEndsAsContextCancelledNeverPaneLost":         1,
		"TestRunTmuxREPL_AHealthyCaptureNeverQueriesTheSession":                                       2,
	},
	dwellFile: {
		"TestRunTmuxREPL_TransientDwell_EnforceStopsBeforeTheArtifactReviewer": 2,
		"TestRunTmuxREPL_TransientDwell_ReusesExistingExitAndArtifacts":        9,
		"TestRunTmuxREPL_TransientDwell_DoesNotFireBeforeSixtySeconds":         2,
		"TestRunTmuxREPL_TransientDwell_ResetsOnNonMatchingFrame":              3,
		"TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted":              3,
		"TestRunTmuxREPL_TransientDwell_ShadowObservesWithoutActing":           4,
		"TestRunTmuxREPL_TransientDwell_EnforceRecordsFastFailed":              2,
		"TestRunTmuxREPL_TransientDwell_OffStageIsLegacy":                      2,
		"TestRunTmuxREPL_TransientDwell_EnforceDelaysRedispatch":               2,
		"TestRunTmuxREPL_TransientDwell_CancelBeforeCooldownSkipsDelay":        3,
		"TestRunTmuxREPL_TransientDwell_NoDelayOnOrdinaryTimeout":              3,
		"TestRunTmuxREPL_TransientDwell_CaptureErrorsDoNotResetTheDwell":       2,
	},
}

var tickLoopTests = map[string][]string{
	panelossFile: {
		"TestRunTmuxREPL_ACaptureFailureOnALiveSessionDoesNotEndTheDispatch",
		"TestRunTmuxREPL_OneTickWithoutTheSessionIsNotYetAPaneLoss",
		"TestRunTmuxREPL_AHealthyCaptureNeverQueriesTheSession",
	},
	dwellFile: {
		"TestRunTmuxREPL_TransientDwell_ResetsOnNonMatchingFrame",
		"TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted",
		"TestRunTmuxREPL_TransientDwell_ShadowObservesWithoutActing",
		"TestRunTmuxREPL_TransientDwell_OffStageIsLegacy",
	},
}

var ceilingOf = map[string]time.Duration{
	"TestRunTmuxREPL_ACaptureFailureOnALiveSessionDoesNotEndTheDispatch":   perTestCeiling,
	"TestRunTmuxREPL_OneTickWithoutTheSessionIsNotYetAPaneLoss":            perTestCeiling,
	"TestRunTmuxREPL_AHealthyCaptureNeverQueriesTheSession":                perTestCeiling,
	"TestRunTmuxREPL_TransientDwell_ResetsOnNonMatchingFrame":              perTestCeiling,
	"TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted":              perTestCeiling,
	"TestRunTmuxREPL_TransientDwell_OffStageIsLegacy":                      perTestCeiling,
	"TestRunTmuxREPL_TransientDwell_ShadowObservesWithoutActing":           dwellCeiling,
	"TestRunTmuxREPL_TransientDwell_EnforceStopsBeforeTheArtifactReviewer": dwellCeiling,
	"TestRunTmuxREPL_TransientDwell_ReusesExistingExitAndArtifacts":        dwellCeiling,
	"TestRunTmuxREPL_TransientDwell_EnforceRecordsFastFailed":              dwellCeiling,
	"TestRunTmuxREPL_TransientDwell_EnforceDelaysRedispatch":               dwellCeiling,
	"TestRunTmuxREPL_TransientDwell_CancelBeforeCooldownSkipsDelay":        dwellCeiling,
	"TestRunTmuxREPL_TransientDwell_CaptureErrorsDoNotResetTheDwell":       dwellCeiling,
}

var tickWord = regexp.MustCompile(`(?i)ticks?`)

type testResult struct {
	action  string
	elapsed time.Duration
}

type bridgeRun struct {
	results map[string]testResult
	err     error
	output  string
}

var (
	runOnce   sync.Once
	cachedRun bridgeRun
)

func allTestNames() []string {
	var names []string
	for _, per := range assertionFloor {
		for name := range per {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func bridgeDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "bridge")
}

func serialRaceRun(t *testing.T) bridgeRun {
	t.Helper()
	dir := bridgeDir(t)
	runOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), runBudget)
		defer cancel()
		pattern := "^(" + strings.Join(allTestNames(), "|") + ")$"
		cmd := exec.CommandContext(ctx, "go", "test", "-race", "-json", "-count=1", "-p", "1", "-parallel", "1", "-timeout", "480s", "-run", pattern, ".")
		cmd.Dir = dir
		cmd.WaitDelay = 10 * time.Second
		var out, errb bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errb
		runErr := cmd.Run()
		cachedRun = bridgeRun{results: map[string]testResult{}, err: runErr, output: errb.String()}
		sc := bufio.NewScanner(&out)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for sc.Scan() {
			var ev struct {
				Action  string
				Test    string
				Elapsed float64
			}
			if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Test == "" {
				continue
			}
			switch ev.Action {
			case "pass", "fail", "skip":
				cachedRun.results[ev.Test] = testResult{action: ev.Action, elapsed: time.Duration(ev.Elapsed * float64(time.Second))}
			}
		}
	})
	return cachedRun
}

func parseTestFile(t *testing.T, name string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(bridgeDir(t), name), nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return f
}

func funcDecl(f *ast.File, name string) *ast.FuncDecl {
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name {
			return fd
		}
	}
	return nil
}

func assertionCalls(fd *ast.FuncDecl) int {
	n := 0
	ast.Inspect(fd.Body, func(x ast.Node) bool {
		if c, ok := x.(*ast.CallExpr); ok {
			if s, ok := c.Fun.(*ast.SelectorExpr); ok {
				switch s.Sel.Name {
				case "Errorf", "Fatalf", "Fatal", "Error":
					n++
				}
			}
		}
		return true
	})
	return n
}

func intLiterals(root ast.Node) []int {
	var vals []int
	ast.Inspect(root, func(x ast.Node) bool {
		if bl, ok := x.(*ast.BasicLit); ok && bl.Kind == token.INT {
			if v, err := strconv.Atoi(bl.Value); err == nil {
				vals = append(vals, v)
			}
		}
		return true
	})
	return vals
}

func conditionsMentionTicks(fd *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fd.Body, func(x ast.Node) bool {
		ifs, ok := x.(*ast.IfStmt)
		if !ok {
			return true
		}
		ast.Inspect(ifs.Cond, func(c ast.Node) bool {
			switch v := c.(type) {
			case *ast.Ident:
				found = found || tickWord.MatchString(v.Name)
			case *ast.SelectorExpr:
				found = found || tickWord.MatchString(v.Sel.Name)
			}
			return true
		})
		return true
	})
	return found
}

func standaloneLiteralCount(f *ast.File, want int) int {
	n := 0
	var walk func(node ast.Node, underBinary bool)
	walk = func(node ast.Node, underBinary bool) {
		ast.Inspect(node, func(x ast.Node) bool {
			switch v := x.(type) {
			case *ast.BinaryExpr:
				walk(v.X, true)
				walk(v.Y, true)
				return false
			case *ast.BasicLit:
				if !underBinary && v.Kind == token.INT {
					if val, err := strconv.Atoi(v.Value); err == nil && val == want {
						n++
					}
				}
			}
			return true
		})
	}
	walk(f, false)
	return n
}

func TestC2112_001_EveryBridgeTickLoopTestPassesWithinItsTickCeilingUnderRaceWhenRunAlone(t *testing.T) {
	run := serialRaceRun(t)
	if len(run.results) == 0 {
		t.Fatalf("go test produced no test results (err=%v)\nstderr:\n%s", run.err, run.output)
	}
	var slow []string
	for _, name := range allTestNames() {
		res, ok := run.results[name]
		switch {
		case !ok:
			t.Errorf("%s: no result — the test was renamed, removed or never ran", name)
		case res.action != "pass":
			t.Errorf("%s: action %q, want pass", name, res.action)
		case ceilingOf[name] > 0 && res.elapsed >= ceilingOf[name]:
			slow = append(slow, name+" "+res.elapsed.String()+" (ceiling "+ceilingOf[name].String()+")")
		}
	}
	if len(slow) > 0 {
		t.Errorf("RED: %d test(s) reach their state in too many ticks (-race, serial): %s", len(slow), strings.Join(slow, "; "))
	}
}

func TestC2112_002_SlowTickLoopTestsBoundTheirTicksByATestScopedThreshold(t *testing.T) {
	for file, names := range tickLoopTests {
		f := parseTestFile(t, file)
		for _, name := range names {
			fd := funcDecl(f, name)
			if fd == nil {
				t.Errorf("%s: test function missing", name)
				continue
			}
			for _, v := range intLiterals(fd.Body) {
				if v >= 100 {
					t.Errorf("RED: %s carries the literal %d — its tick bound still derives from a production-sized threshold instead of a test-scoped one", name, v)
					break
				}
			}
			if !conditionsMentionTicks(fd) {
				t.Errorf("RED: %s has no conditional on a tick count — the tick count is not asserted through the fake sleep", name)
			}
		}
	}
}

func TestC2112_003_NoTickLoopTestWeakensItsStateAssertions(t *testing.T) {
	for file, per := range assertionFloor {
		f := parseTestFile(t, file)
		for name, floor := range per {
			fd := funcDecl(f, name)
			if fd == nil {
				t.Errorf("%s: test function missing from %s", name, file)
				continue
			}
			if got := assertionCalls(fd); got < floor {
				t.Errorf("%s: %d assertion call(s), want >= %d — a state assertion was dropped to make the test faster", name, got, floor)
			}
		}
	}
}

func TestC2112_004_OneTestKeepsEachProductionThreshold(t *testing.T) {
	cases := []struct {
		file      string
		threshold int
	}{
		{panelossFile, 1200},
		{dwellFile, 300},
	}
	for _, c := range cases {
		f := parseTestFile(t, c.file)
		if standaloneLiteralCount(f, c.threshold) == 0 {
			t.Errorf("%s: no test passes the production threshold %d s — every test uses a reduced value", c.file, c.threshold)
		}
	}
}

type mutant struct {
	name     string
	source   string
	original string
	mutated  string
	test     string
}

var mutants = []mutant{
	{
		name:     "dwell streak never resets on a non-matching frame",
		source:   "exhaustion_persistence.go",
		original: "\t\tg.streak = 0\n",
		mutated:  "",
		test:     "TestRunTmuxREPL_TransientDwell_ResetsOnNonMatchingFrame",
	},
	{
		name:     "dwell counts a busy pane as a transient match",
		source:   "autorespond.go",
		original: "matched := !paneBusy && ar.transientPattern",
		mutated:  "matched := ar.transientPattern",
		test:     "TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted",
	},
}

func goTestInBridge(t *testing.T, overlay, run string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	args := []string{"test", "-count=1", "-run", "^" + run + "$"}
	if overlay != "" {
		args = append(args, "-overlay", overlay)
	}
	args = append(args, ".")
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = bridgeDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeOverlay(t *testing.T, m mutant) string {
	t.Helper()
	target := filepath.Join(bridgeDir(t), m.source)
	src, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	if n := strings.Count(string(src), m.original); n != 1 {
		t.Fatalf("mutant %q: %q occurs %d times in %s, want exactly 1 — the production seam moved", m.name, m.original, n, m.source)
	}
	dir := t.TempDir()
	mutatedPath := filepath.Join(dir, m.source)
	if err := os.WriteFile(mutatedPath, []byte(strings.Replace(string(src), m.original, m.mutated, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	overlayJSON, err := json.Marshal(map[string]map[string]string{"Replace": {target: mutatedPath}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlayJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	return overlayPath
}

func TestC2112_005_EachTickLoopTestStillFailsWhenItsProductionBehaviourIsBroken(t *testing.T) {
	for _, m := range mutants {
		out, err := goTestInBridge(t, "", m.test)
		if err != nil {
			t.Fatalf("%s: control run of %s on unmutated code failed: %v\n%s", m.name, m.test, err, out)
		}
		mutOut, mutErr := goTestInBridge(t, writeOverlay(t, m), m.test)
		if mutErr == nil {
			t.Errorf("RED: %s — %s still PASSES with the mutant applied, so it no longer detects the defect it guards", m.name, m.test)
			continue
		}
		if !strings.Contains(mutOut, "--- FAIL: "+m.test) {
			t.Errorf("%s: mutant run failed but not through an assertion of %s (build error?):\n%s", m.name, m.test, mutOut)
		}
	}
}
