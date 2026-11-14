package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const retakeRedOutput = "=== RUN   TestFlaky\n--- FAIL: TestFlaky (0.01s)\n    flaky_test.go:9: one-off red\n--- FAIL: TestFlaky/sub (0.00s)\nFAIL\texample.com/m/q\t0.2s\nFAIL\n"

func retakeRepo(t *testing.T) (ReviewInput, *signalcenter.Center) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q.go"), "package q\n\nfunc F() int { return 1 }\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q_test.go"), "package q\n\nimport \"testing\"\n\nfunc TestFlaky(t *testing.T) { _ = F() }\n")
	signals := signalcenter.New()
	return ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, ProjectRoot: t.TempDir(), WorktreeBaseSHA: base, Signals: signals, Stderr: &strings.Builder{}}, signals
}

func stubUnitRunner(t *testing.T, results ...bool) *[]string {
	t.Helper()
	var calls []string
	prev := buildSelfCheckRunner
	buildSelfCheckRunner = func(_ context.Context, _, pkg string) (string, bool) {
		calls = append(calls, pkg)
		if results[len(calls)-1] {
			return "ok\t" + pkg + "\n", true
		}
		return retakeRedOutput, false
	}
	t.Cleanup(func() { buildSelfCheckRunner = prev })
	return &calls
}

func retakeEvents(c *signalcenter.Center) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range c.Recent() {
		if e.Code == CodeBuildFloorRetakeGreen || e.Code == CodeBuildFloorRetakeRed {
			out = append(out, e)
		}
	}
	return out
}

func TestChangedPackageFloorChecks_AOneOffRedIsRetakenAndTheBuildGoesOn(t *testing.T) {
	in, signals := retakeRepo(t)
	calls := stubUnitRunner(t, false, true)

	out := changedPackageFloorChecks(context.Background(), in, []string{"go/q/q.go", "go/q/q_test.go"})

	if len(out) != 0 {
		t.Fatalf("floor = %v, want no finding: the serialized retake of ./q is GREEN", out)
	}
	if strings.Join(*calls, " ") != "./q ./q" {
		t.Fatalf("runs = %v, want the first run and one retake of ./q", *calls)
	}
	events := retakeEvents(signals)
	if len(events) != 1 || events[0].Code != CodeBuildFloorRetakeGreen || events[0].Fields["pkg"] != "./q" || events[0].Fields["tests"] != "TestFlaky" {
		t.Fatalf("signals = %+v, want one %s for ./q that names TestFlaky", events, CodeBuildFloorRetakeGreen)
	}
	if !strings.Contains(in.Stderr.(*strings.Builder).String(), "BUILD_FLOOR_RETAKE_GREEN") {
		t.Errorf("stderr = %q, want the GREEN retake line for the operator", in.Stderr)
	}
}

func TestChangedPackageFloorChecks_ASecondRedFailsTheFloorAndNamesTheTest(t *testing.T) {
	in, signals := retakeRepo(t)
	calls := stubUnitRunner(t, false, false)

	out := strings.Join(changedPackageFloorChecks(context.Background(), in, []string{"go/q/q.go"}), "\n")

	if !strings.Contains(out, "./q: unit tests FAIL") || !strings.Contains(out, "--- FAIL: TestFlaky") {
		t.Fatalf("floor = %q, want the ./q failure that names TestFlaky", out)
	}
	if len(*calls) != 2 {
		t.Fatalf("runs = %v, want exactly one retake", *calls)
	}
	events := retakeEvents(signals)
	if len(events) != 1 || events[0].Code != CodeBuildFloorRetakeRed || events[0].Fields["tests"] != "TestFlaky" || events[0].Severity != signalcenter.SeverityWarn {
		t.Fatalf("signals = %+v, want one WARN %s that names TestFlaky", events, CodeBuildFloorRetakeRed)
	}
}

func TestChangedPackageFloorChecks_TheFloorKeepsTheAuditTierScope(t *testing.T) {
	in, _ := retakeRepo(t)
	writeFile(t, filepath.Join(in.Worktree, "go", "internal", "bridge", "b.go"), "package bridge\n")
	calls := stubUnitRunner(t, true)

	changedPackageFloorChecks(context.Background(), in, []string{"go/internal/bridge/b.go", "go/q/q.go"})

	if strings.Join(*calls, " ") != "./q" {
		t.Fatalf("runs = %v, want only ./q: internal/bridge is env-exclusive under a live loop, as at the audit tier", *calls)
	}
	if stderr := in.Stderr.(*strings.Builder).String(); !strings.Contains(stderr, "./internal/bridge") || !strings.Contains(stderr, "runs in CI") {
		t.Errorf("stderr = %q, want a NOTE that names the skipped package and its backstop", stderr)
	}
}

func TestAddedTaggedTestFailures_AOneOffRedIsRetaken(t *testing.T) {
	in, signals := retakeRepo(t)
	writeFile(t, filepath.Join(in.Worktree, "go", "acs", "cycle9", "predicates_test.go"), "//go:build acs\n\npackage cycle9\n\nimport \"testing\"\n\nfunc TestFlaky(t *testing.T) {}\n")
	runs := 0
	prev := buildSelfCheckTaggedRunner
	buildSelfCheckTaggedRunner = func(context.Context, string, string, []string) (string, bool) {
		runs++
		return retakeRedOutput, runs > 1
	}
	t.Cleanup(func() { buildSelfCheckTaggedRunner = prev })

	if got := addedTaggedTestFailures(context.Background(), in.withFloorAdded(), filepath.Join(in.Worktree, "go")); len(got) != 0 || runs != 2 {
		t.Fatalf("failures = %v after %d run(s), want none after one retake", got, runs)
	}
	if events := retakeEvents(signals); len(events) != 1 || events[0].Fields["pkg"] != "./acs/cycle9 (-tags acs)" {
		t.Fatalf("signals = %+v, want one GREEN retake for the tagged package", events)
	}
}

func flakyModule(t *testing.T, failures int) string {
	t.Helper()
	counter := filepath.Join(t.TempDir(), "runs")
	test := fmt.Sprintf("package p\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestFlaky(t *testing.T) {\n\tb, _ := os.ReadFile(%q)\n\t_ = os.WriteFile(%q, append(b, 'x'), 0o644)\n\tif len(b) < %d {\n\t\tt.Fatal(\"red\")\n\t}\n\t_ = A()\n}\n", counter, counter, failures)
	return tinyModule(t, map[string]string{
		"p/p.go":      "package p\n\nfunc A() int { return 1 }\n",
		"p/p_test.go": test,
	})
}

func TestScopedCoverProfile_AOneOffRedIsRetakenUnderTheTierLock(t *testing.T) {
	t.Parallel()
	signals := signalcenter.New()
	var stderr strings.Builder
	in := ReviewInput{ProjectRoot: t.TempDir(), Signals: signals, Stderr: &stderr}

	profile, out, status := scopedCoverProfile(context.Background(), in, flakyModule(t, 1), []string{"./p"}, nil)
	defer removeProfileDir(profile)

	if status != coverStatusOK {
		t.Fatalf("status = %d, output = %q, want OK after the serialized retake", status, out)
	}
	events := retakeEvents(signals)
	if len(events) != 1 || events[0].Code != CodeBuildFloorRetakeGreen || events[0].Fields["tests"] != "TestFlaky" || events[0].Fields["pkg"] != "./p" {
		t.Fatalf("signals = %+v, want one GREEN retake that names TestFlaky", events)
	}
}

func TestScopedCoverProfile_ARedRetakeStaysRed(t *testing.T) {
	t.Parallel()
	signals := signalcenter.New()
	in := ReviewInput{ProjectRoot: t.TempDir(), Signals: signals, Stderr: &strings.Builder{}}

	profile, out, status := scopedCoverProfile(context.Background(), in, flakyModule(t, 2), []string{"./p"}, nil)
	defer removeProfileDir(profile)

	if status != coverStatusTestsFailed || !strings.Contains(out, "--- FAIL: TestFlaky") {
		t.Fatalf("status = %d, output = %q, want the red retake", status, out)
	}
	if events := retakeEvents(signals); len(events) != 1 || events[0].Code != CodeBuildFloorRetakeRed {
		t.Fatalf("signals = %+v, want one RED retake", events)
	}
}

func TestRetakeFloorRun_AnUnavailableLockRunsTheRetakeUnserialized(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "file")
	writeFile(t, file, "x")
	for _, root := range []string{"", file} {
		var stderr strings.Builder
		in := ReviewInput{ProjectRoot: root, Stderr: &stderr}

		out, ok := in.retakeFloorRun("./q", retakeRedOutput, []string{"./q"}, func() (string, bool) { return "ok", true })

		if !ok || out != "ok" || !strings.Contains(stderr.String(), "the retake runs unserialized") {
			t.Errorf("root %q: (%q, %v), stderr %q, want the retake run with a WARN", root, out, ok, stderr.String())
		}
	}
}

func TestGoTestFailureLines_NamesEachFailedTestAndPackage(t *testing.T) {
	t.Parallel()
	lines, tests := goTestFailureLines(retakeRedOutput + "--- FAIL: \n")

	if strings.Join(lines, "|") != "--- FAIL: TestFlaky (0.01s)|--- FAIL: TestFlaky/sub (0.00s)|FAIL\texample.com/m/q\t0.2s" {
		t.Errorf("lines = %q", lines)
	}
	if strings.Join(tests, ",") != "TestFlaky" {
		t.Errorf("tests = %v, want the top-level name once, as evolve ci classify names a failure", tests)
	}
}

func TestFloorFailureDiagnostic_KeepsTheTestNameBefore10KBOfLaterOutput(t *testing.T) {
	t.Parallel()
	later := strings.Repeat("[engine] WARN: later output that is not the verdict\n", 200)
	output := retakeRedOutput + later

	got := floorFailureDiagnostic(output)

	if len(later) < 10*1024 || len(got) > floorFailureDiagnosticMax+len("…") {
		t.Fatalf("diagnostic is %d bytes for %d bytes of later output, want <= %d", len(got), len(later), floorFailureDiagnosticMax+len("…"))
	}
	if !strings.HasPrefix(got, "--- FAIL: TestFlaky (0.01s)\n") || !strings.Contains(got, "FAIL\texample.com/m/q") || !strings.Contains(got, "…") || !strings.HasSuffix(got, "verdict\n") {
		t.Fatalf("diagnostic = %q, want the failed names first, then the marked tail", got)
	}
}

func TestFloorFailureDiagnostic_ManyNamesStayInTheBound(t *testing.T) {
	t.Parallel()
	output := strings.Repeat("--- FAIL: TestAVeryLongNameThatRepeatsManyTimes (0.00s)\n", 40)

	got := floorFailureDiagnostic(output)

	if len(got) != floorFailureDiagnosticMax+len("…") || !strings.HasPrefix(got, "--- FAIL: TestAVeryLong") || !strings.HasSuffix(got, "…") {
		t.Fatalf("diagnostic = %d bytes %q, want the names cut at the bound", len(got), got)
	}
}

func removeProfileDir(profile string) {
	if profile != "" {
		_ = os.RemoveAll(filepath.Dir(profile))
	}
}

func TestCoverRetakeResult_APlumbingRetakeKeepsTheFirstRed(t *testing.T) {
	t.Parallel()
	if out, status := coverRetakeResult(retakeRedOutput, "merge failed", coverStatusPlumbingError); status != coverStatusTestsFailed || !strings.Contains(out, "--- FAIL: TestFlaky") || !strings.Contains(out, "merge failed") {
		t.Errorf("(%q, %d), want the first red kept with the plumbing text", out, status)
	}
	for _, status := range []int{coverStatusOK, coverStatusTestsFailed} {
		if out, got := coverRetakeResult(retakeRedOutput, "retake", status); got != status || out != "retake" {
			t.Errorf("status %d: (%q, %d), want the retake verdict", status, out, got)
		}
	}
}

func TestFloorFailureDiagnostic_ACutNeverSplitsARune(t *testing.T) {
	t.Parallel()
	for pad := 0; pad < 2; pad++ {
		for _, output := range []string{strings.Repeat("x", pad) + strings.Repeat("é", 300), strings.Repeat("é", 300) + strings.Repeat("x", pad+1), strings.Repeat("--- FAIL: Testé"+strings.Repeat("x", pad)+" (0.00s)\n", 30), strings.Repeat("--- FAIL: Testéé"+strings.Repeat("x", pad)+" (0.00s)\n", 30)} {
			if got := floorFailureDiagnostic(output); !utf8.ValidString(got) {
				t.Errorf("diagnostic %q is not valid UTF-8", got)
			}
		}
	}
}

func TestRetakeFloorRun_WaitsOnTheLockFileOfTheAuditRetake(t *testing.T) {
	prev := floorRetakeLockWait
	floorRetakeLockWait = 50 * time.Millisecond
	t.Cleanup(func() { floorRetakeLockWait = prev })
	root := t.TempDir()
	release, held, err := flock.TryLock(ciparity.TierLockPath(root))
	if err != nil || held {
		t.Fatalf("TryLock = (%v, %v)", held, err)
	}
	var stderr strings.Builder

	_, ok := ReviewInput{ProjectRoot: root, Stderr: &stderr}.retakeFloorRun("./q", retakeRedOutput, []string{"./q"}, func() (string, bool) { return "ok", true })
	release()

	if !ok || !strings.Contains(stderr.String(), "lock deadline passed") {
		t.Fatalf("stderr = %q, want the retake to wait on the audit's lock file and then run unserialized", stderr.String())
	}
	worktree := t.TempDir()
	ReviewInput{Worktree: worktree, Stderr: &stderr}.retakeFloorRun("./q", retakeRedOutput, []string{"./q"}, func() (string, bool) { return "ok", true })
	if _, err := os.Stat(ciparity.TierLockPath(worktree)); err != nil {
		t.Errorf("without a project root the lock must live under the worktree: %v", err)
	}
}
