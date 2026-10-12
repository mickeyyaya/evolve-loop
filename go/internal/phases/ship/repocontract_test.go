package ship

import (
	"context"
	"errors"
	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func addedTestPackages(root string) (packages, excluded []string) {
	files, _ := changedpkgs.ChangedFilesChecked(root, "HEAD")
	groups, excluded, _ := addedtests.Groups(root, files)
	for _, group := range groups {
		packages = append(packages, group.Packages...)
	}
	sort.Strings(packages)
	return packages, excluded
}

const scanChatter = "=== RUN   TestGuardSuite\n"

func writeGreenGuardSuites(t *testing.T, goDir string) {
	t.Helper()
	for _, pattern := range repoContractPackages {
		dir := strings.TrimSuffix(strings.TrimPrefix(pattern, "./"), "/...")
		pkg := filepath.Base(dir)
		mustWrite(t, filepath.Join(goDir, filepath.FromSlash(dir), "pass_test.go"),
			"package "+pkg+"\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n")
	}
}

func swapRepoContractTest(t *testing.T, outcomes ...packOutcome) *[]string {
	t.Helper()
	var dirs []string
	prev := repoContractTestFn
	t.Cleanup(func() { repoContractTestFn = prev })
	repoContractTestFn = func(ctx context.Context, moduleDir string, out packLog) packOutcome {
		o := outcomes[len(outcomes)-1]
		if len(dirs) < len(outcomes) {
			o = outcomes[len(dirs)]
		}
		dirs = append(dirs, moduleDir)
		_, _ = io.WriteString(out, scanChatter)
		return o
	}
	return &dirs
}

func greenPack() packOutcome { return packOutcome{} }

func redPack(names ...string) packOutcome {
	failures := make([]packFailure, 0, len(names))
	for _, name := range names {
		failures = append(failures, packFailureOf(name))
	}
	return packOutcome{failures: failures, err: errors.New("exit status 1")}
}

func packFailureOf(name string) packFailure {
	if strings.Contains(name, "[build failed]") {
		return packFailure{Package: name}
	}
	dot := strings.LastIndex(name, ".")
	return packFailure{Package: name[:dot], Test: name[dot+1:]}
}

func ambiguousPack() packOutcome {
	return packOutcome{err: errors.New("signal: killed")}
}

func TestRepoContractGate_OffSkips(t *testing.T) {
	dirs := swapRepoContractTest(t, redPack("pkg.TestWouldFail"))
	for _, stage := range []string{"", "off"} {
		if err := runRepoContractGate(context.Background(), stage, "/lane", "", io.Discard); err != nil {
			t.Fatalf("stage %q must skip, got %v", stage, err)
		}
	}
	if len(*dirs) != 0 {
		t.Fatalf("off must not run the pack, ran in %v", *dirs)
	}
}

func TestRepoContractGate_EnforceGreenPasses(t *testing.T) {
	repo := repoDeclaring(t, evolveLoopGoMod)
	dirs := swapRepoContractTest(t, greenPack())
	if err := runRepoContractGate(context.Background(), "enforce", repo, "", io.Discard); err != nil {
		t.Fatalf("green pack must pass: %v", err)
	}
	if len(*dirs) != 1 || (*dirs)[0] != filepath.Join(repo, "go") {
		t.Fatalf("pack must run in the lane worktree module dir, got %v", *dirs)
	}
}

func TestRepoContractGate_EnforceRedFailsWithDedicatedCode(t *testing.T) {
	swapRepoContractTest(t, redPack("pkg.TestGuard"))
	err := runRepoContractGate(context.Background(), "enforce", evolveLoopLane(t), "", io.Discard)
	if err == nil {
		t.Fatal("RED pack must fail the ship")
	}
	var se *shiperr.ShipError
	if !errors.As(err, &se) {
		t.Fatalf("must be a structured ShipError, got %T: %v", err, err)
	}
	if se.Code != shiperr.CodeRepoContractGate {
		t.Fatalf("code = %q, want REPO_CONTRACT_GATE (never a git-failure alias)", se.Code)
	}
}

func TestRepoContractGate_UnknownStageFailsTowardEnforce(t *testing.T) {
	repo := repoDeclaring(t, evolveLoopGoMod)
	dirs := swapRepoContractTest(t, greenPack())
	var warn strings.Builder
	if err := runRepoContractGate(context.Background(), "shadwo", repo, "", &warn); err != nil {
		t.Fatalf("unknown stage with green pack: %v", err)
	}
	if len(*dirs) != 1 {
		t.Fatal("unknown stage must RUN the pack (typo must not disable a red-main guard)")
	}
	if !strings.Contains(warn.String(), "unknown stage") {
		t.Fatalf("unknown stage must WARN, got %q", warn.String())
	}
}

func TestNew_ThreadsRepoContractGate(t *testing.T) {
	t.Parallel()
	p := New(Config{RepoContractGate: "enforce"})
	if p.repoContractGate != "enforce" {
		t.Fatalf("Config.RepoContractGate must thread into the Phase, got %q", p.repoContractGate)
	}
	_ = os.Stderr
}

func TestRepoContractGate_RealTestFailureIsContractRedWithoutRetry(t *testing.T) {
	dirs := swapRepoContractTest(t, redPack("internal/profiles.TestTrackedProfilesBound"), greenPack())
	err := runRepoContractGate(context.Background(), "enforce", evolveLoopLane(t), "", io.Discard)
	if err == nil {
		t.Fatal("a genuine test failure must FAIL the ship, never be retried into a pass")
	}
	se, ok := shiperr.AsShipError(err)
	if !ok {
		t.Fatalf("must be a structured ShipError, got %T: %v", err, err)
	}
	if se.Code != shiperr.CodeRepoContractGate {
		t.Fatalf("code = %q, want REPO_CONTRACT_GATE — a real RED is a contract violation, not infra", se.Code)
	}
	if len(*dirs) != 1 {
		t.Fatalf("a real RED must run the pack EXACTLY once, ran %d times", len(*dirs))
	}
}

func TestRepoContractGate_TransientFailureRetriesOnceThenShips(t *testing.T) {
	repo := repoDeclaring(t, evolveLoopGoMod)
	dirs := swapRepoContractTest(t, ambiguousPack(), greenPack())
	if err := runRepoContractGate(context.Background(), "enforce", repo, "", io.Discard); err != nil {
		t.Fatalf("an unclassifiable failure that clears on retry must NOT block the ship, got %v", err)
	}
	if len(*dirs) != 2 {
		t.Fatalf("expected exactly 2 pack runs (attempt + one retry), got %d", len(*dirs))
	}
}

func TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns(t *testing.T) {
	dirs := swapRepoContractTest(t, ambiguousPack(), ambiguousPack())
	err := runRepoContractGate(context.Background(), "enforce", evolveLoopLane(t), "", io.Discard)
	if err == nil {
		t.Fatal("a pack that never ran green must not be allowed to ship")
	}
	se, ok := shiperr.AsShipError(err)
	if !ok {
		t.Fatalf("must be a structured ShipError, got %T: %v", err, err)
	}
	if se.Code != shiperr.CodeRepoContractInfra {
		t.Fatalf("code = %q, want REPO_CONTRACT_INFRA — no test failed, so this is not a contract violation", se.Code)
	}
	if se.Class != shiperr.ShipClassPrecondition {
		t.Fatalf("class = %q, want precondition (re-dispatchable)", se.Class)
	}
	if len(*dirs) != 2 {
		t.Fatalf("no retry storm: expected exactly 2 pack runs, got %d", len(*dirs))
	}
}

func TestRepoContractGate_ScanLogPersistedOnGreenAndRedRuns(t *testing.T) {
	repo := repoDeclaring(t, evolveLoopGoMod)
	for _, tc := range []struct {
		name    string
		outcome packOutcome
		wantErr bool
	}{
		{"green", greenPack(), false},
		{"red", redPack("internal/phasespec.TestSpecParity"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			swapRepoContractTest(t, tc.outcome)
			ws := t.TempDir()
			err := runRepoContractGate(context.Background(), "enforce", repo, ws, io.Discard)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			body, readErr := os.ReadFile(filepath.Join(ws, scanLogName))
			if readErr != nil {
				t.Fatalf("scan log must exist on the %s path: %v", tc.name, readErr)
			}
			if !strings.Contains(string(body), scanChatter) {
				t.Fatalf("scan log must carry the scanner output, got %q", string(body))
			}
			if !strings.Contains(string(body), "repo-contract scanner pack") {
				t.Fatalf("scan log must self-identify with a header, got %q", string(body))
			}
		})
	}
}

func TestRepoContractGate_RedErrorMessageNamesFailingTests(t *testing.T) {
	swapRepoContractTest(t, redPack("internal/profiles.TestTrackedProfilesBound", "internal/routingtest.TestRenderParity"))
	err := runRepoContractGate(context.Background(), "enforce", evolveLoopLane(t), "", io.Discard)
	if err == nil {
		t.Fatal("RED pack must fail the ship")
	}
	for _, want := range []string{"internal/profiles.TestTrackedProfilesBound", "internal/routingtest.TestRenderParity"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ship error must NAME the failing test %q, got %q", want, err.Error())
		}
	}
}

func TestRepoContractGate_NewlyAddedFailingTestBlocksShip(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	goDir := filepath.Join(repo, "go")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
	writeGreenGuardSuites(t, goDir)
	runGit(t, repo, "add", "go")
	runGit(t, repo, "commit", "-qm", "add green scanner suites")

	const failingTest = "TestNewlyAddedRed"
	mustWrite(t, filepath.Join(goDir, "internal", "reproduction", "red_test.go"), "package reproduction\n\nimport \"testing\"\n\nfunc "+failingTest+"(t *testing.T) { t.Fatal(\"deliberate red\") }\n")
	runGit(t, repo, "add", "go/internal/reproduction/red_test.go")

	err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), io.Discard)
	if err == nil {
		t.Fatal("a newly added failing test must block ship before it can red main")
	}
	se, ok := shiperr.AsShipError(err)
	if !ok || se.Code != shiperr.CodeRepoContractGate {
		t.Fatalf("error = %v, want structured REPO_CONTRACT_GATE for %s", err, failingTest)
	}
	if !strings.Contains(err.Error(), failingTest) {
		t.Fatalf("gate error must name %s, got %q", failingTest, err)
	}
}

func TestRepoContractGate_AddedTestSelectionIgnoresModifiedAndNonTestFiles(t *testing.T) {
	t.Parallel()
	t.Run("modified test and non-test additions are not added-test candidates", func(t *testing.T) {
		repo := makeRepo(t)
		goDir := filepath.Join(repo, "go")
		mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
		writeGreenGuardSuites(t, goDir)
		trackedTest := filepath.Join(goDir, "internal", "tracked", "tracked_test.go")
		mustWrite(t, trackedTest, "package tracked\n\nimport \"testing\"\n\nfunc TestTracked(t *testing.T) {}\n")
		runGit(t, repo, "add", "go")
		runGit(t, repo, "commit", "-qm", "baseline: green suites + tracked test")

		mustWrite(t, trackedTest, "package tracked\n\nimport \"testing\"\n\nfunc TestTracked(t *testing.T) { t.Log(\"modified, not added\") }\n")
		runGit(t, repo, "add", "go/internal/tracked/tracked_test.go")

		mustWrite(t, filepath.Join(goDir, "internal", "reproduction", "helper.go"), "package reproduction\n\nfunc Helper() int { return 1 }\n")
		runGit(t, repo, "add", "go/internal/reproduction/helper.go")

		mustWrite(t, filepath.Join(goDir, "internal", "reproduction", "green_test.go"), "package reproduction\n\nimport \"testing\"\n\nfunc TestNewlyAddedGreen(t *testing.T) {}\n")
		runGit(t, repo, "add", "go/internal/reproduction/green_test.go")

		packages, excluded := addedTestPackages(repo)
		if want := []string{"./internal/reproduction"}; !reflect.DeepEqual(packages, want) || len(excluded) != 0 {
			t.Fatalf("added-test candidates = %v (excluded %v), want %v — the modified tracked test and the non-test file are not candidates", packages, excluded, want)
		}
		if err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), io.Discard); err != nil {
			t.Fatalf("a green modified test, a non-test addition and a green added test must not gate the ship, got %v", err)
		}
	})

	t.Run("a modified tracked test that goes red is caught by the importer backstop", func(t *testing.T) {
		repo := makeRepo(t)
		goDir := filepath.Join(repo, "go")
		mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
		writeGreenGuardSuites(t, goDir)
		trackedTest := filepath.Join(goDir, "internal", "tracked", "tracked_test.go")
		mustWrite(t, trackedTest, "package tracked\n\nimport \"testing\"\n\nfunc TestTracked(t *testing.T) {}\n")
		runGit(t, repo, "add", "go")
		runGit(t, repo, "commit", "-qm", "baseline: green suites + tracked test")
		mustWrite(t, trackedTest, "package tracked\n\nimport \"testing\"\n\nfunc TestTracked(t *testing.T) { t.Fatal(\"now red via modification, not addition\") }\n")
		runGit(t, repo, "add", "go/internal/tracked/tracked_test.go")

		if packages, _ := addedTestPackages(repo); len(packages) != 0 {
			t.Fatalf("a modified test is not an added-test candidate, got %v", packages)
		}
		err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), io.Discard)
		if err == nil || !strings.Contains(err.Error(), "importer backstop") || !strings.Contains(err.Error(), "TestTracked") {
			t.Fatalf("shipping a red modified test would red main; the importer backstop must block it naming TestTracked, got %v", err)
		}
	})

	t.Run("empty candidate set produces no false gate execution", func(t *testing.T) {
		repo := makeRepo(t)
		goDir := filepath.Join(repo, "go")
		mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
		writeGreenGuardSuites(t, goDir)
		runGit(t, repo, "add", "go")
		runGit(t, repo, "commit", "-qm", "baseline only, nothing staged")

		if err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), io.Discard); err != nil {
			t.Fatalf("a shipping diff that adds no test files must not produce a false RED, got %v", err)
		}
	})
}

func TestClassifyPackEvents_SeparatesRealFailuresFromNoise(t *testing.T) {
	t.Parallel()
	feed := strings.Join([]string{
		`{"Action":"run","Package":"p/profiles","Test":"TestBound"}`,
		`{"Action":"output","Package":"p/profiles","Test":"TestBound","Output":"    profiles_test.go:12: mismatch\n"}`,
		`{"Action":"fail","Package":"p/profiles","Test":"TestBound"}`,
		`{"Action":"pass","Package":"p/phasespec","Test":"TestOK"}`,
		`{"Action":"fail","Package":"p/profiles"}`,
		`go: downloading something (not a JSON event)`,
		`{"Action":"output","Package":"p/broken","Output":"FAIL\tp/broken [build failed]\n"}`,
	}, "\n")

	var tee strings.Builder
	got, _ := classifyPackEvents(strings.NewReader(feed), &tee)

	if len(got) != 2 {
		t.Fatalf("expected exactly 2 classified failures (one test, one build), got %v", got)
	}
	if got[0] != (packFailure{Package: "p/profiles", Test: "TestBound"}) {
		t.Errorf("failing test = %+v, want p/profiles.TestBound", got[0])
	}
	if !strings.Contains(got[1].String(), "[build failed]") || got[1].Test != "" {
		t.Errorf("compile break must classify as a real RED with no test name, got %+v", got[1])
	}
	if !strings.Contains(tee.String(), "profiles_test.go:12: mismatch") {
		t.Errorf("Output text must be teed to the scan log, got %q", tee.String())
	}
	if !strings.Contains(tee.String(), "go: downloading something") {
		t.Errorf("non-JSON lines must be teed verbatim, not dropped, got %q", tee.String())
	}
}

func TestClassifyPackEvents_AmbiguousFeedNamesNothing(t *testing.T) {
	t.Parallel()
	feed := `{"Action":"output","Package":"p/profiles","Output":"signal: killed\n"}`
	if got, _ := classifyPackEvents(strings.NewReader(feed), io.Discard); len(got) != 0 {
		t.Fatalf("an OOM-killed run names no failing test; got %v", got)
	}
}

func TestRunNative_RepoContractGateReceivesRunWorkspace(t *testing.T) {
	swapRepoContractTest(t, redPack("internal/phasecoherence.TestPairing"))
	ws := t.TempDir()
	p := New(Config{RepoContractGate: "enforce"})

	resp, err := p.runNative(context.Background(), core.PhaseRequest{
		Cycle:       1409,
		Workspace:   ws,
		ProjectRoot: evolveLoopLane(t),
	}, "msg", time.Now())

	if err == nil {
		t.Fatal("runNative must surface the gate block")
	}
	if !strings.Contains(err.Error(), "repo-contract gate") || !strings.Contains(err.Error(), "TestPairing") {
		t.Fatalf("error must come from the gate's faked red pack, got %v", err)
	}
	if resp.Verdict == core.VerdictPASS {
		t.Fatalf("gate block must not report PASS, got %q", resp.Verdict)
	}
	if _, statErr := os.Stat(filepath.Join(ws, scanLogName)); statErr != nil {
		t.Fatalf("runNative must thread req.Workspace into the gate so the scan log lands in the run dir: %v", statErr)
	}
}

func TestPhaseRunNative_NewlyAddedFailingTestPreventsRun(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	goDir := filepath.Join(repo, "go")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
	writeGreenGuardSuites(t, goDir)
	runGit(t, repo, "add", "go")
	runGit(t, repo, "commit", "-qm", "add green scanner suites")

	const failingTest = "TestNewlyAddedRedViaRunNative"
	mustWrite(t, filepath.Join(goDir, "internal", "reproduction", "red_test.go"), "package reproduction\n\nimport \"testing\"\n\nfunc "+failingTest+"(t *testing.T) { t.Fatal(\"deliberate red\") }\n")
	runGit(t, repo, "add", "go/internal/reproduction/red_test.go")

	ws := t.TempDir()
	p := New(Config{RepoContractGate: "enforce"})
	resp, err := p.runNative(context.Background(), core.PhaseRequest{
		Cycle:       1555,
		Workspace:   ws,
		ProjectRoot: repo,
	}, "msg", time.Now())

	if err == nil {
		t.Fatal("runNative must stop before ship/git work when a newly added test is red")
	}
	if !strings.Contains(err.Error(), "repo-contract gate") {
		t.Fatalf("block must come from the repo-contract gate (proves it fired before any git/ship action), got %v", err)
	}
	if !strings.Contains(err.Error(), failingTest) {
		t.Fatalf("error must name %s, got %v", failingTest, err)
	}
	if resp.Verdict == core.VerdictPASS {
		t.Fatalf("gate block must not report PASS, got %q", resp.Verdict)
	}
	if _, statErr := os.Stat(filepath.Join(ws, scanLogName)); statErr != nil {
		t.Fatalf("scan log must land in the run workspace even on the real-repo path: %v", statErr)
	}
}

func TestRunNative_AddedSkippedTestDoesNotBlockShip(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	goDir := filepath.Join(repo, "go")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
	writeGreenGuardSuites(t, goDir)
	runGit(t, repo, "add", "go")
	runGit(t, repo, "commit", "-qm", "add green scanner suites")

	mustWrite(t, filepath.Join(goDir, "internal", "reproduction", "skip_test.go"),
		"package reproduction\n\nimport \"testing\"\n\nfunc TestNewlyAddedSkipped(t *testing.T) { t.Skip(\"tracked known gap\") }\n")
	runGit(t, repo, "add", "go/internal/reproduction/skip_test.go")

	ws := t.TempDir()
	p := New(Config{RepoContractGate: "enforce"})
	_, err := p.runNative(context.Background(), core.PhaseRequest{
		Cycle:       1555,
		Workspace:   ws,
		ProjectRoot: repo,
	}, "msg", time.Now())

	if err != nil && strings.Contains(err.Error(), "repo-contract gate") {
		t.Fatalf("a skipped reproducer must not block at the repo-contract gate, got %v", err)
	}
	body, readErr := os.ReadFile(filepath.Join(ws, scanLogName))
	if readErr != nil {
		t.Fatalf("scan log must still be written on the skip path: %v", readErr)
	}
	if strings.Contains(string(body), "TestNewlyAddedSkipped") && strings.Contains(string(body), "FAIL") {
		t.Fatalf("skipped test must not be classified as a failure in the scan log, got %q", body)
	}
}

func TestRepoContractGate_AddedEnvExclusiveTestBackstopRecorded(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	goDir := filepath.Join(repo, "go")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
	writeGreenGuardSuites(t, goDir)
	runGit(t, repo, "add", "go")
	runGit(t, repo, "commit", "-qm", "add green scanner suites")

	const excludedFile = "go/internal/reproduction/tmux_test.go"
	mustWrite(t, filepath.Join(repo, excludedFile),
		"//go:build requires_tmux\n\npackage reproduction\n\nimport \"testing\"\n\nfunc TestNewlyAddedRequiresTmux(t *testing.T) { t.Fatal(\"must never run without tmux\") }\n")
	runGit(t, repo, "add", excludedFile)

	ws := t.TempDir()
	err := runRepoContractGate(context.Background(), "enforce", repo, ws, io.Discard)
	if err != nil {
		t.Fatalf("an honestly env-exclusive added test must not block the ship, got %v", err)
	}
	body, readErr := os.ReadFile(filepath.Join(ws, scanLogName))
	if readErr != nil {
		t.Fatalf("scan log must be written on the env-exclusive path: %v", readErr)
	}
	text := string(body)
	if !strings.Contains(text, excludedFile) {
		t.Fatalf("scan log must name the excluded candidate %s, got %q", excludedFile, text)
	}
	if !strings.Contains(text, "requires_tmux") {
		t.Fatalf("scan log must record the exclusion reason (requires_tmux), got %q", text)
	}
	if !strings.Contains(strings.ToLower(text), "exclud") {
		t.Fatalf("scan log must explicitly mark the candidate as EXCLUDED, not silently omitted or claimed green, got %q", text)
	}
}
