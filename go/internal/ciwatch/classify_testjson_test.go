package ciwatch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	jobGo127  = "go / build + test (Go 1.27, ubuntu, every package, no gates)"
	jobGo123  = "go / build + test (Go 1.23, ubuntu, every package and every gate)"
	jobDarwin = "go / build + test (Go 1.23, macOS, the darwin package set)"
	jobCIReq  = "CI required"
	stepCIReq = "Require every selected suite to succeed"
	modPrefix = "github.com/mickeyyaya/evolve-loop/go/"
)

func fixtureRunGH(t *testing.T, runID string) *[]string {
	t.Helper()
	dir := filepath.Join("testdata", "cirun", runID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var docs []string
	for _, e := range entries {
		if e.IsDir() {
			docs = append(docs, fmt.Sprintf(`{"name":%q,"expired":false}`, e.Name()))
		}
	}
	docs = append(docs, `{"name":"landing-dist","expired":false}`, `{"name":"coverage-expired","expired":true}`)
	listing := `{"artifacts":[` + strings.Join(docs, ",") + `]}`
	var seen []string
	fakeGH(t, func(args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		seen = append(seen, joined)
		switch {
		case joined == "run view "+runID+" --log-failed":
			return os.ReadFile(filepath.Join(dir, "log-failed.txt"))
		case joined == "api repos/{owner}/{repo}/actions/runs/"+runID+"/artifacts?per_page=100":
			return []byte(listing), nil
		case strings.HasPrefix(joined, "run download "+runID+" --name "):
			return nil, copyFixtureArtifact(dir, args)
		}
		return nil, fmt.Errorf("unrouted gh call: %s", joined)
	})
	return &seen
}

func copyFixtureArtifact(dir string, args []string) error {
	name, dest := args[slices.Index(args, "--name")+1], args[slices.Index(args, "--dir")+1]
	raw, err := os.ReadFile(filepath.Join(dir, name, testJSONFile))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dest, testJSONFile), raw, 0o644)
}

func failedRun(id int64) RunStatus {
	return RunStatus{RunID: id, Status: StatusCompleted, Conclusion: "failure"}
}

func TestWithFailures_RealCIRunsYieldOnlyTheGoCommandsFailures(t *testing.T) {
	cases := []struct {
		run  int64
		want []FailingTest
	}{
		{39381369373, []FailingTest{
			{Jobs: []string{jobCIReq}},
			{Step: "make apicover-check", Jobs: []string{jobGo123}},
		}},
		{39385027012, []FailingTest{
			{Step: stepCIReq, Jobs: []string{jobCIReq}},
			{Package: modPrefix + "internal/repocontract", Test: "TestPackages_HoldEveryTestThatReadsTheWholeTree", Jobs: []string{jobGo123, jobGo127}},
		}},
		{39386265940, []FailingTest{
			{Step: stepCIReq, Jobs: []string{jobCIReq}},
			{Package: modPrefix + "cmd/evolve", Test: "TestRatchet_CheckPassesOnTheRealModuleAndRejectsOtherVerbs", Jobs: []string{jobDarwin, jobGo123, jobGo127}},
			{Package: modPrefix + "internal/sizeratchet", Test: "TestRatchet_ModuleFunctionsFitTheirAllowances", Jobs: []string{jobGo123, jobGo127}},
		}},
		{39389719309, []FailingTest{
			{Step: stepCIReq, Jobs: []string{jobCIReq}},
			{Package: modPrefix + "cmd/evolve", Test: "TestRatchet_CheckPassesOnTheRealModuleAndRejectsOtherVerbs", Jobs: []string{jobDarwin, jobGo123, jobGo127}},
			{Package: modPrefix + "internal/sizeratchet", Test: "TestRatchet_ModuleFunctionsFitTheirAllowances", Jobs: []string{jobGo123, jobGo127}},
		}},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.run), func(t *testing.T) {
			fixtureRunGH(t, fmt.Sprint(tc.run))
			got, err := ghClassifySource{root: "."}.withFailures(context.Background(), failedRun(tc.run))
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(got.FailingTests) != fmt.Sprint(tc.want) {
				t.Errorf("FailingTests:\n got %+v\nwant %+v", got.FailingTests, tc.want)
			}
		})
	}
}

func TestParseFailingTests_TheTextOfARealRunStillCarriesThePrintedFixtures(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "cirun", "39389719309", "log-failed.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var tests []string
	for _, f := range parseFailingTests(string(raw)) {
		tests = append(tests, f.Test)
	}
	for _, fixture := range []string{"TestC1946_001_CoreStatementCoverageIsFull", "TestInnerRunCycleErrorF4"} {
		if !slices.Contains(tests, fixture) {
			t.Errorf("the text alone attributes %s (the reason the test JSON wins); got %v", fixture, tests)
		}
	}
}

func TestWithFailures_ARerunAttemptReadsTheLogOnly(t *testing.T) {
	calls := routedGHF1(t, ghRouteF1{prefix: "run view 7 --log-failed --attempt 2", out: ghLog("j", "make: *** [lint] Error 2")})
	st := failedRun(7)
	st.Attempt = 2
	got, err := ghClassifySource{root: "."}.withFailures(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	want := []FailingTest{{Step: "make lint", Jobs: []string{"j"}}}
	if fmt.Sprint(got.FailingTests) != fmt.Sprint(want) || len(*calls) != 1 {
		t.Errorf("FailingTests = %+v after calls %v, want %+v from the log alone", got.FailingTests, *calls, want)
	}
}

func TestWithFailures_ARunWithoutATestJSONArtifactFallsBackToTheLog(t *testing.T) {
	routedGHF1(t,
		ghRouteF1{prefix: "run view 8 --log-failed", out: ghLog("j", "--- FAIL: TestA (0s)", "FAIL\tm/a\t0.1s")},
		ghRouteF1{prefix: "api repos/{owner}/{repo}/actions/runs/8/artifacts", out: `{"artifacts":[{"name":"coverage-x","expired":false}]}`},
		ghRouteF1{prefix: "run download 8 --name coverage-x", out: ""},
	)
	got, err := ghClassifySource{root: "."}.withFailures(context.Background(), failedRun(8))
	if err != nil {
		t.Fatal(err)
	}
	want := []FailingTest{{Package: "m/a", Test: "TestA", Jobs: []string{"j"}}}
	if fmt.Sprint(got.FailingTests) != fmt.Sprint(want) {
		t.Errorf("FailingTests = %+v, want the text attribution %+v", got.FailingTests, want)
	}
}

func TestWithFailures_ArtifactFailuresPropagate(t *testing.T) {
	logRoute := ghRouteF1{prefix: "run view 9 --log-failed", out: ghLog("j", "FAIL\tm/a\t0.1s")}
	listing := ghRouteF1{prefix: "api repos/{owner}/{repo}/actions/runs/9/artifacts", out: `{"artifacts":[{"name":"coverage-x","expired":false}]}`}
	cases := []struct {
		name   string
		routes []ghRouteF1
		want   string
	}{
		{"listing", []ghRouteF1{logRoute, {prefix: "api repos/{owner}/{repo}/actions/runs/9/artifacts", err: errors.New("HTTP 502")}}, "HTTP 502"},
		{"download", []ghRouteF1{logRoute, listing, {prefix: "run download 9", err: errors.New("download 410")}}, "download 410"},
		{"a listing over one page", []ghRouteF1{logRoute, {prefix: "api repos/{owner}/{repo}/actions/runs/9/artifacts", out: `{"total_count":101,"artifacts":[{"name":"coverage-x","expired":false}]}`}}, "lists 101 artifacts, over the 1 read"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			routedGHF1(t, tc.routes...)
			if _, err := (ghClassifySource{root: "."}).withFailures(context.Background(), failedRun(9)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestReadTestJSONFailures_KeepsFailedTestsOfFailedPackagesOnly(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"run","Package":"m/a","Test":"TestA"}`,
		`{"Action":"output","Package":"m/b","Test":"TestPrinter","Output":"--- FAIL: TestFake (0.1s)\n"}`,
		`{"Action":"fail","Package":"m/a","Test":"TestA/sub"}`,
		`{"Action":"fail","Package":"m/a","Test":"TestA"}`,
		`{"Action":"fail","Package":"m/a"}`,
		`{"Action":"fail","Package":"m/b","Test":"TestPrintedByAPassingPackage"}`,
		`{"Action":"pass","Package":"m/b"}`,
		`{"Action":"fail","Package":"m/c"}`,
		``,
	}, "\n")
	got, err := readTestJSONFailures(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	want := []FailingTest{{Package: "m/a", Test: "TestA"}, {Package: "m/c"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("readTestJSONFailures = %+v, want %+v", got, want)
	}
	if _, err := readTestJSONFailures(strings.NewReader(`{"Action":"fail",` + "\n")); err == nil {
		t.Error("a failure event that is not JSON must be reported")
	}
	if _, err := readTestJSONFailures(strings.NewReader(strings.Repeat("x", testJSONLineCap+1))); err == nil {
		t.Error("a line over the cap must be reported")
	}
}

func TestFailureKey_AStepIsItsOwnKey(t *testing.T) {
	a := failureKey(FailingTest{Step: "make lint", Jobs: []string{"j1"}})
	b := failureKey(FailingTest{Step: "make lint", Jobs: []string{"j2"}})
	if a != b || a == failureKey(FailingTest{Jobs: []string{"j1"}}) {
		t.Errorf("step keys %q %q must match each other and differ from the job key", a, b)
	}
}

func TestWithFailures_AnUnreadableArtifactIsReported(t *testing.T) {
	cases := []struct {
		name  string
		write func(dest string) error
		want  string
	}{
		{"the artifact is a file, not a directory", func(dest string) error { return os.WriteFile(dest, []byte("x"), 0o644) }, "artifact coverage-x: open"},
		{"a failure event that is not JSON", func(dest string) error {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dest, testJSONFile), []byte(`{"Action":"fail",`+"\n"), 0o644)
		}, "artifact coverage-x: test json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeGH(t, func(args []string) ([]byte, error) {
				switch joined := strings.Join(args, " "); {
				case strings.HasPrefix(joined, "run view 9 --log-failed"):
					return []byte(ghLog("j", "FAIL\tm/a\t0.1s")), nil
				case strings.HasPrefix(joined, "api repos/{owner}/{repo}/actions/runs/9/artifacts"):
					return []byte(`{"artifacts":[{"name":"coverage-x","expired":false}]}`), nil
				default:
					return nil, tc.write(args[slices.Index(args, "--dir")+1])
				}
			})
			if _, err := (ghClassifySource{root: "."}).withFailures(context.Background(), failedRun(9)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestWithFailures_ANoTempDirIsReported(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))
	routedGHF1(t,
		ghRouteF1{prefix: "run view 9 --log-failed", out: ghLog("j", "FAIL\tm/a\t0.1s")},
		ghRouteF1{prefix: "api repos/{owner}/{repo}/actions/runs/9/artifacts", out: `{"artifacts":[]}`},
	)
	if _, err := (ghClassifySource{root: "."}).withFailures(context.Background(), failedRun(9)); err == nil || !strings.Contains(err.Error(), "test json temp dir") {
		t.Fatalf("err = %v, want the temp dir failure", err)
	}
}
