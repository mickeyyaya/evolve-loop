package ship

// repocontract_test.go — the ship-time repo-contract scanner pack. The gate
// exists because four lane landings redded main in one week; these tests pin:
// off skips, enforce-green passes, enforce-RED fails with the DEDICATED code
// (never a git-failure alias), unknown stage fails toward enforce, and the
// module dir is the lane worktree's go/.
//
// cycle-1409 adds the classification + persistence contract: a genuine RED is
// never retried and names its failing tests, an unclassifiable failure is
// retried EXACTLY once, a twice-unclassifiable failure is the distinct infra
// code, and every run tees its output to the run-dir scan log.

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// scanChatter is written by every fake pack run so the scan-log assertions
// prove real scanner output reached the artifact, not just that a file was
// touched open.
const scanChatter = "=== RUN   TestGuardSuite\n"

// swapRepoContractTest installs a fake pack that returns the given outcomes in
// order (the last one repeats, so an over-eager retry loop is caught by the
// invocation count rather than a panic). The returned slice records one entry
// per invocation: its LENGTH is the "exactly N runs" assertion.
func swapRepoContractTest(t *testing.T, outcomes ...packOutcome) *[]string {
	t.Helper()
	var dirs []string
	prev := repoContractTestFn
	t.Cleanup(func() { repoContractTestFn = prev })
	repoContractTestFn = func(ctx context.Context, moduleDir string, out io.Writer) packOutcome {
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

// redPack is a GENUINE contract violation: nonzero exit WITH named test
// failures — the pack itself said "your code is broken".
func redPack(names ...string) packOutcome {
	return packOutcome{failedTests: names, err: errors.New("exit status 1")}
}

// ambiguousPack is the cycle-1402/1403/1405 shape: nonzero exit, but not one
// guard suite reported a failure. The toolchain died, the contract did not.
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
	dirs := swapRepoContractTest(t, greenPack())
	if err := runRepoContractGate(context.Background(), "enforce", "/lane/worktree", "", io.Discard); err != nil {
		t.Fatalf("green pack must pass: %v", err)
	}
	if len(*dirs) != 1 || (*dirs)[0] != "/lane/worktree/go" {
		t.Fatalf("pack must run in the lane worktree module dir, got %v", *dirs)
	}
}

func TestRepoContractGate_EnforceRedFailsWithDedicatedCode(t *testing.T) {
	swapRepoContractTest(t, redPack("pkg.TestGuard"))
	err := runRepoContractGate(context.Background(), "enforce", "/lane", "", io.Discard)
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
	dirs := swapRepoContractTest(t, greenPack())
	var warn strings.Builder
	if err := runRepoContractGate(context.Background(), "shadwo", "/lane", "", &warn); err != nil {
		t.Fatalf("unknown stage with green pack: %v", err)
	}
	if len(*dirs) != 1 {
		t.Fatal("unknown stage must RUN the pack (typo must not disable a red-main guard)")
	}
	if !strings.Contains(warn.String(), "unknown stage") {
		t.Fatalf("unknown stage must WARN, got %q", warn.String())
	}
}

// TestNew_ThreadsRepoContractGate is the cycle-1064 anti-trap: the production
// construction site must thread the dial or the gate is permanently off no
// matter what policy says.
func TestNew_ThreadsRepoContractGate(t *testing.T) {
	p := New(Config{RepoContractGate: "enforce"})
	if p.repoContractGate != "enforce" {
		t.Fatalf("Config.RepoContractGate must thread into the Phase, got %q", p.repoContractGate)
	}
	_ = os.Stderr // keep os import parallel with production file expectations
}

// TestRepoContractGate_RealTestFailureIsContractRedWithoutRetry — AC2, the
// crux anti-regression of the cycle-1409 rework. A pack that NAMES a failing
// test is a genuine violation: it must still fail the ship closed with
// CodeRepoContractGate and must run EXACTLY ONCE. Retrying a real RED both
// doubles every red ship's wall-time and gives a flaky-but-real suite a second
// chance to pass by luck, laundering the violation onto main.
func TestRepoContractGate_RealTestFailureIsContractRedWithoutRetry(t *testing.T) {
	// Second outcome is GREEN on purpose: if the gate wrongly retried a real
	// RED, it would return nil and this test would catch the laundering.
	dirs := swapRepoContractTest(t, redPack("internal/profiles.TestTrackedProfilesBound"), greenPack())
	err := runRepoContractGate(context.Background(), "enforce", "/lane", "", io.Discard)
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

// TestRepoContractGate_TransientFailureRetriesOnceThenShips — AC3, the defect
// this cycle exists to kill. Attempt 1 exits nonzero with no test-level
// failure (build-cache contention / OOM kill), attempt 2 is clean: the ship
// MUST proceed. This is exactly what would have unblocked the audit-green
// cycles 1402/1403/1405.
func TestRepoContractGate_TransientFailureRetriesOnceThenShips(t *testing.T) {
	dirs := swapRepoContractTest(t, ambiguousPack(), greenPack())
	if err := runRepoContractGate(context.Background(), "enforce", "/lane", "", io.Discard); err != nil {
		t.Fatalf("an unclassifiable failure that clears on retry must NOT block the ship, got %v", err)
	}
	if len(*dirs) != 2 {
		t.Fatalf("expected exactly 2 pack runs (attempt + one retry), got %d", len(*dirs))
	}
}

// TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns — AC4,
// the NEGATIVE case. Ambiguity on BOTH attempts must fail the ship with the
// DISTINCT infra code (not silently proceed — the pack never proved the repo
// green) and must not spawn a third run. No constant-return implementation can
// satisfy this alongside AC2 and AC3.
func TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns(t *testing.T) {
	dirs := swapRepoContractTest(t, ambiguousPack(), ambiguousPack())
	err := runRepoContractGate(context.Background(), "enforce", "/lane", "", io.Discard)
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

// TestRepoContractGate_ScanLogPersistedOnGreenAndRedRuns — AC6. Red-only
// persistence is the exact gap that made cycle-1403 undiagnosable: proving a
// RED false needs the green baseline from the same artifact path.
func TestRepoContractGate_ScanLogPersistedOnGreenAndRedRuns(t *testing.T) {
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
			err := runRepoContractGate(context.Background(), "enforce", "/lane", ws, io.Discard)
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

// TestRepoContractGate_RedErrorMessageNamesFailingTests — AC7. The parsed
// failing test names must ride in the ship error so ship-error.json carries
// them directly, instead of the generic "(exit status 1)" that forced a manual
// worktree re-run to diagnose cycle-1402.
func TestRepoContractGate_RedErrorMessageNamesFailingTests(t *testing.T) {
	swapRepoContractTest(t, redPack("internal/profiles.TestTrackedProfilesBound", "internal/routingtest.TestRenderParity"))
	err := runRepoContractGate(context.Background(), "enforce", "/lane", "", io.Discard)
	if err == nil {
		t.Fatal("RED pack must fail the ship")
	}
	for _, want := range []string{"internal/profiles.TestTrackedProfilesBound", "internal/routingtest.TestRenderParity"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ship error must NAME the failing test %q, got %q", want, err.Error())
		}
	}
}

// TestClassifyPackEvents_SeparatesRealFailuresFromNoise pins the parser that
// makes the whole classification real. The seam-swapping tests above cannot
// reach it, so without this the `go test -json` decoding would be untested
// production logic sitting under every ship.
func TestClassifyPackEvents_SeparatesRealFailuresFromNoise(t *testing.T) {
	feed := strings.Join([]string{
		`{"Action":"run","Package":"p/profiles","Test":"TestBound"}`,
		`{"Action":"output","Package":"p/profiles","Test":"TestBound","Output":"    profiles_test.go:12: mismatch\n"}`,
		`{"Action":"fail","Package":"p/profiles","Test":"TestBound"}`,
		`{"Action":"pass","Package":"p/phasespec","Test":"TestOK"}`,
		`{"Action":"fail","Package":"p/profiles"}`, // package roll-up: already counted
		`go: downloading something (not a JSON event)`,
		`{"Action":"output","Package":"p/broken","Output":"FAIL\tp/broken [build failed]\n"}`,
	}, "\n")

	var tee strings.Builder
	got := classifyPackEvents(strings.NewReader(feed), &tee)

	if len(got) != 2 {
		t.Fatalf("expected exactly 2 classified failures (one test, one build), got %v", got)
	}
	if got[0] != "p/profiles.TestBound" {
		t.Errorf("failing test name = %q, want p/profiles.TestBound", got[0])
	}
	if !strings.Contains(got[1], "[build failed]") {
		t.Errorf("compile break must classify as a real RED, got %q", got[1])
	}
	if !strings.Contains(tee.String(), "profiles_test.go:12: mismatch") {
		t.Errorf("Output text must be teed to the scan log, got %q", tee.String())
	}
	if !strings.Contains(tee.String(), "go: downloading something") {
		t.Errorf("non-JSON lines must be teed verbatim, not dropped, got %q", tee.String())
	}
}

// TestClassifyPackEvents_AmbiguousFeedNamesNothing is the other half of the
// parser contract and the root of the false-RED fix: a feed with no failure
// event must yield ZERO names, so the gate routes to the retry/infra path
// instead of blocking the ship.
func TestClassifyPackEvents_AmbiguousFeedNamesNothing(t *testing.T) {
	feed := `{"Action":"output","Package":"p/profiles","Output":"signal: killed\n"}`
	if got := classifyPackEvents(strings.NewReader(feed), io.Discard); len(got) != 0 {
		t.Fatalf("an OOM-killed run names no failing test; got %v", got)
	}
}

// TestRunNative_RepoContractGateReceivesRunWorkspace — AC8, the WIRING PROOF.
// It drives the PRODUCTION caller (Phase.runNative, ship.go) rather than
// runRepoContractGate directly, and asserts the scan log landed in
// req.Workspace: a log seam reachable only from a test is dead code. The pack
// is faked RED so runNative returns at the gate without touching git.
func TestRunNative_RepoContractGateReceivesRunWorkspace(t *testing.T) {
	swapRepoContractTest(t, redPack("internal/phasecoherence.TestPairing"))
	ws := t.TempDir()
	p := New(Config{RepoContractGate: "enforce"})

	resp, err := p.runNative(context.Background(), core.PhaseRequest{
		Cycle:       1409,
		Workspace:   ws,
		ProjectRoot: "/lane",
	}, "msg", time.Now())

	if err == nil {
		t.Fatal("runNative must surface the gate block")
	}
	if !strings.Contains(err.Error(), "repo-contract gate") {
		t.Fatalf("error must come from the gate, got %v", err)
	}
	if resp.Verdict == core.VerdictPASS {
		t.Fatalf("gate block must not report PASS, got %q", resp.Verdict)
	}
	if _, statErr := os.Stat(filepath.Join(ws, scanLogName)); statErr != nil {
		t.Fatalf("runNative must thread req.Workspace into the gate so the scan log lands in the run dir: %v", statErr)
	}
}
