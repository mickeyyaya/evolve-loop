//go:build acs

package cycle2660

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	baseCommit = "83e996a69ddbd334da16cb1c4278ba09d8a6556b"
	ciwatchDir = "internal/ciwatch"
	testedKey  = "m/p TestK"
)

type ghRule struct {
	all []string
	out string
}

func installFakeGH(t *testing.T, rules []ghRule) {
	t.Helper()
	t.Setenv(ipcenv.PhaseKey, "")
	t.Setenv(ipcenv.DispatchDepthKey, "")
	dir := t.TempDir()
	var script strings.Builder
	script.WriteString("#!/bin/bash\nargs=\"$*\"\n")
	for i, r := range rules {
		outFile := filepath.Join(dir, fmt.Sprintf("out%d", i))
		if err := os.WriteFile(outFile, []byte(r.out), 0o644); err != nil {
			t.Fatal(err)
		}
		var conds []string
		for _, tok := range r.all {
			conds = append(conds, fmt.Sprintf(`[[ "$args" == *'%s'* ]]`, tok))
		}
		fmt.Fprintf(&script, "if %s; then cat '%s'; exit 0; fi\n", strings.Join(conds, " && "), outFile)
	}
	script.WriteString("echo \"fake gh: unexpected call: $args\" >&2\nexit 1\n")
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func runDoc(id int64, head string, attempt int, conclusion string) string {
	return fmt.Sprintf(`{"attempt":%d,"conclusion":%q,"createdAt":"2026-10-01T11:00:00Z","databaseId":%d,"headSha":%q,"status":"completed","url":"https://ci/%d"}`,
		attempt, conclusion, id, head, id)
}

const (
	macosStamp  = "unit (macos)\tRun make test\t2026-10-01T12:01:00.0000000Z "
	ubuntuStamp = "unit (ubuntu)\tRun make test\t2026-10-01T12:01:00.0000000Z "
)

var (
	testKLog     = ubuntuStamp + "--- FAIL: TestK (0.01s)\n" + ubuntuStamp + "FAIL\n" + ubuntuStamp + "FAIL\tm/p\t0.1s\n"
	testOtherLog = ubuntuStamp + "--- FAIL: TestOther (0.01s)\n" + ubuntuStamp + "FAIL\n" + ubuntuStamp + "FAIL\tm/p\t0.1s\n"

	timeoutOneRunningLog = macosStamp + "panic: test timed out after 30m0s\n" +
		macosStamp + "\trunning tests:\n" +
		macosStamp + "\t\tTestSlowAlpha (30m0s)\n" +
		macosStamp + "\n" +
		macosStamp + "goroutine 33 [running]:\n" +
		macosStamp + "FAIL\tm/cmd/evolve\t1800.123s\n"
	timeoutTwoRunningLog = macosStamp + "panic: test timed out after 30m0s\n" +
		macosStamp + "\trunning tests:\n" +
		macosStamp + "\t\tTestSlowAlpha (30m0s)\n" +
		macosStamp + "\t\tTestSlowBeta (30m0s)\n" +
		macosStamp + "\n" +
		macosStamp + "goroutine 33 [running]:\n" +
		macosStamp + "FAIL\tm/cmd/evolve\t1800.123s\n"
)

func prRunRules(log string) []ghRule {
	return []ghRule{
		{[]string{"run view 501", "--log-failed"}, log},
		{[]string{"run view 501", "--json jobs"}, `{"jobs":[]}`},
		{[]string{"run view 501"}, strings.Replace(runDoc(501, "h", 1, "failure"), "11:00:00Z", "12:00:00Z", 1)},
		{[]string{"actions/runs/501/artifacts"}, `{"total_count":0,"artifacts":[]}`},
		{[]string{"compare/main...h"}, `{"status":"identical","merge_base_commit":{"sha":"h"},"files":[]}`},
		{[]string{"commits/h"}, `{"sha":"h","parents":[{"sha":"b"}]}`},
		{[]string{"compare/b...h"}, `{"status":"ahead","merge_base_commit":{"sha":"b"},"files":[{"filename":"go/other/x.go"}]}`},
		{[]string{"run list", "--commit"}, `[]`},
	}
}

func classifyRun501(t *testing.T, rules []ghRule, mainWindow int) ciwatch.ClassifyReport {
	t.Helper()
	installFakeGH(t, rules)
	opts := ciwatch.ClassifyOptions{MainWindow: mainWindow, ModulePath: "m", ModuleDir: "go"}
	rep, err := ciwatch.ClassifyTarget(context.Background(), ciwatch.NewGHClassifySource(t.TempDir(), "m"),
		ciwatch.Target{Kind: ciwatch.TargetRun, Value: "501"}, opts)
	if err != nil {
		t.Fatalf("ClassifyTarget: %v", err)
	}
	return rep
}

func labelsOf(rep ciwatch.ClassifyReport) []string {
	var out []string
	for _, f := range rep.Failures {
		out = append(out, fmt.Sprintf("%s/%s=%s", f.Package, f.Test, f.Label))
	}
	return out
}

func TestC2660_001_TimeoutPanicClassifiesAsTimeoutClass(t *testing.T) {
	rep := classifyRun501(t, prRunRules(timeoutTwoRunningLog), 0)
	var timeoutRows []string
	for _, f := range rep.Failures {
		if string(f.Label) == "timeout" {
			raw, _ := json.Marshal(f)
			timeoutRows = append(timeoutRows, string(raw))
		}
		if f.Label == ciwatch.LabelUnknown {
			t.Errorf("a go test timeout panic stays class unknown: %v", labelsOf(rep))
		}
	}
	if len(timeoutRows) == 0 {
		t.Fatalf("no failure carries label \"timeout\": %v", labelsOf(rep))
	}
	joined := strings.Join(timeoutRows, "\n")
	for _, running := range []string{"TestSlowAlpha", "TestSlowBeta"} {
		if !strings.Contains(joined, running) {
			t.Errorf("running test %s is not part of the timeout failure key: %s", running, joined)
		}
	}
}

func TestC2660_002_NonTimeoutPanicAndPlainFailStayUnknown(t *testing.T) {
	cases := map[string]string{
		"plain package fail": macosStamp + "FAIL\tm/cmd/evolve\t1.5s\n",
		"other panic": macosStamp + "panic: runtime error: index out of range [3] with length 2\n" +
			macosStamp + "goroutine 9 [running]:\n" +
			macosStamp + "FAIL\tm/cmd/evolve\t0.5s\n",
	}
	for name, log := range cases {
		t.Run(name, func(t *testing.T) {
			rep := classifyRun501(t, prRunRules(log), 0)
			if len(rep.Failures) == 0 {
				t.Fatal("no failure parsed from the log")
			}
			for _, f := range rep.Failures {
				if string(f.Label) == "timeout" || f.Label != ciwatch.LabelUnknown {
					t.Errorf("%s must stay unknown, got %v", name, labelsOf(rep))
				}
			}
		})
	}
}

func mainRunRules(attempt1Log string) []ghRule {
	return []ghRule{
		{[]string{"run view 401", "--log-failed", "--attempt 1"}, attempt1Log},
		{[]string{"run view 401", "--log-failed"}, ""},
		{[]string{"run view 401", "--json jobs"}, `{"jobs":[]}`},
		{[]string{"run view 401", "--attempt 1"}, runDoc(401, "m1", 1, "failure")},
		{[]string{"run view 401"}, runDoc(401, "m1", 2, "success")},
		{[]string{"actions/runs/401/artifacts"}, `{"total_count":0,"artifacts":[]}`},
		{[]string{"run list", "--branch main"}, "[" + runDoc(401, "m1", 2, "success") + "]"},
	}
}

func recurredFor(rep ciwatch.ClassifyReport, test string) (ciwatch.Fact, bool) {
	for _, f := range rep.Failures {
		if f.Test == test {
			return f.Evidence.RecurredOnMain, true
		}
	}
	return "", false
}

func TestC2660_003_RedAttemptThatWentGreenOnRerunCountsAsRecurrence(t *testing.T) {
	rules := append(prRunRules(testKLog), mainRunRules(testKLog)...)
	got, found := recurredFor(classifyRun501(t, rules, 10), "TestK")
	if !found {
		t.Fatalf("%s is not among the classified failures", testedKey)
	}
	if got != ciwatch.FactYes {
		t.Errorf("recurred_on_main for %s = %q, want yes: attempt 1 of main run 401 failed it before the re-run went green", testedKey, got)
	}
}

func TestC2660_004_AttemptOneRedWithDifferentKeyDoesNotRecur(t *testing.T) {
	rules := append(prRunRules(testKLog), mainRunRules(testOtherLog)...)
	got, found := recurredFor(classifyRun501(t, rules, 10), "TestK")
	if !found {
		t.Fatalf("%s is not among the classified failures", testedKey)
	}
	if got != ciwatch.FactNo {
		t.Errorf("recurred_on_main for %s = %q, want no: only TestOther failed on main", testedKey, got)
	}
}

func TestC2660_005_EmptyMainRunListKeepsRecurrenceUnknown(t *testing.T) {
	rules := append([]ghRule{{[]string{"run list", "--branch main"}, `[]`}}, prRunRules(testKLog)...)
	got, found := recurredFor(classifyRun501(t, rules, 10), "TestK")
	if !found {
		t.Fatalf("%s is not among the classified failures", testedKey)
	}
	if got != ciwatch.FactUnknown {
		t.Errorf("recurred_on_main with no main runs = %q, want unknown", got)
	}
}

func TestC2660_006_TimeoutOnMainThatWentGreenOnRerunRecursOnTheSameKey(t *testing.T) {
	rules := append(prRunRules(timeoutOneRunningLog), mainRunRules(timeoutOneRunningLog)...)
	rep := classifyRun501(t, rules, 10)
	var timeoutRows int
	for _, f := range rep.Failures {
		if string(f.Label) != "timeout" {
			continue
		}
		timeoutRows++
		if f.Evidence.RecurredOnMain != ciwatch.FactYes {
			t.Errorf("timeout failure %s/%s recurred_on_main = %q, want yes (main attempt 1 timed out the same way)", f.Package, f.Test, f.Evidence.RecurredOnMain)
		}
	}
	if timeoutRows == 0 {
		t.Fatalf("no failure carries label \"timeout\": %v", labelsOf(rep))
	}
}

type funcSpan struct {
	file       string
	name       string
	start, end int
}

func ciwatchFuncSpans(t *testing.T, moduleDir string) []funcSpan {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(moduleDir, ciwatchDir))
	if err != nil {
		t.Fatal(err)
	}
	var spans []funcSpan
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(moduleDir, ciwatchDir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				spans = append(spans, funcSpan{name, fd.Name.Name, fset.Position(fd.Pos()).Line, fset.Position(fd.End()).Line})
			}
		}
	}
	return spans
}

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func changedLines(t *testing.T, moduleDir string) map[string][][2]int {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutputIn(moduleDir, "git", "diff", "--relative", "--unified=0", baseCommit, "--", ciwatchDir)
	if err != nil || code != 0 {
		t.Fatalf("git diff against %s: code=%d err=%v %s", baseCommit, code, err, stderr)
	}
	ranges := map[string][][2]int{}
	current := ""
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(line, "+++ b/"+ciwatchDir+"/"); ok {
			current = rest
			continue
		}
		m := hunkHeader.FindStringSubmatch(line)
		if m == nil || current == "" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		ranges[current] = append(ranges[current], [2]int{start, start + max(count, 1) - 1})
	}
	untracked, _, _, _ := acsassert.SubprocessOutputIn(moduleDir, "git", "ls-files", "--others", "--exclude-standard", "--", ciwatchDir)
	for _, p := range strings.Fields(untracked) {
		ranges[filepath.Base(p)] = [][2]int{{1, 1 << 30}}
	}
	return ranges
}

func coverageByFunc(t *testing.T, moduleDir string) map[string]float64 {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	if out, stderr, code, err := acsassert.SubprocessOutputIn(moduleDir, "go", "test", "-count=1", "-coverprofile="+profile, "./"+ciwatchDir); err != nil || code != 0 {
		t.Fatalf("go test ./%s: code=%d err=%v\n%s\n%s", ciwatchDir, code, err, out, stderr)
	}
	out, stderr, code, err := acsassert.SubprocessOutputIn(moduleDir, "go", "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover: code=%d err=%v %s", code, err, stderr)
	}
	byFunc := map[string]float64{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.HasSuffix(fields[len(fields)-1], "%") {
			continue
		}
		pct, err := strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "%"), 64)
		if err != nil {
			continue
		}
		loc := strings.Split(fields[0], ":")
		if len(loc) >= 2 {
			byFunc[filepath.Base(loc[0])+":"+loc[1]] = pct
		}
	}
	return byFunc
}

func TestC2660_007_EveryChangedFunctionIsFullyCovered(t *testing.T) {
	moduleDir := acsassert.GoModuleDir(t)
	ranges := changedLines(t, moduleDir)
	var changed []funcSpan
	for _, span := range ciwatchFuncSpans(t, moduleDir) {
		for _, r := range ranges[span.file] {
			if r[0] <= span.end && r[1] >= span.start {
				changed = append(changed, span)
				break
			}
		}
	}
	if len(changed) == 0 {
		t.Fatalf("no function of a non-test file in %s changed against %s", ciwatchDir, baseCommit)
	}
	coverage := coverageByFunc(t, moduleDir)
	for _, span := range changed {
		pct, ok := coverage[span.file+":"+strconv.Itoa(span.start)]
		if !ok {
			t.Errorf("cover output lists no %s (%s:%d)", span.name, span.file, span.start)
			continue
		}
		if pct < 100.0 {
			t.Errorf("changed function %s (%s:%d) is covered %.1f%%, want 100.0%%", span.name, span.file, span.start, pct)
		}
	}
}
