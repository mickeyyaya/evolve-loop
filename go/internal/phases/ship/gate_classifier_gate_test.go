//go:build integration

package ship

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const guardTest = "package widget\n\nimport \"testing\"\n\nfunc TestWidget_RefusesGrowth(t *testing.T) {\n\tif 1+1 != 2 {\n\t\tt.Fatal(\"growth\")\n\t}\n}\n"

func repoWithAGuardTest(t *testing.T) (string, string) {
	t.Helper()
	repo := makeRepo(t)
	path := filepath.Join(repo, "go/internal/widget/widget_guard_test.go")
	mustWrite(t, path, guardTest)
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "add guard test")
	return repo, path
}

func skipTheGuard(t *testing.T, path string) {
	t.Helper()
	mustWrite(t, path, strings.Replace(guardTest, "\tif 1+1", "\tt.Skip(\"flaky\")\n\tif 1+1", 1))
}

func TestRefuseWeakenedGates_ACycleDiffThatSkipsAGuardTestIsRefused(t *testing.T) {
	t.Parallel()
	repo, path := repoWithAGuardTest(t)
	skipTheGuard(t, path)
	center := signalcenter.New()
	opts := &Options{ProjectRoot: repo, Class: ClassCycle, Runner: execRunner, CycleID: 4, RunID: "run-4", Signals: center}
	var res RunResult

	if err := verifyNoControlPlaneEdits(context.Background(), opts, &res); err != nil {
		t.Fatalf("the P2 check alone must pass this diff (it ships today): %v", err)
	}
	err := refuseWeakenedGates(context.Background(), opts, &res)

	var se *core.ShipError
	if !errors.As(err, &se) || se.Code != shiperr.CodeGateWeakened || !strings.Contains(err.Error(), "go/internal/widget/widget_guard_test.go") {
		t.Fatalf("err = %v, want GATE_WEAKENED that names the guard test", err)
	}
	var signals []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Code == gateclass.CodeI10Violation {
			signals = append(signals, e)
		}
	}
	if len(signals) != 1 || signals[0].Cycle != 4 || signals[0].Fields["patterns"] != string(gateclass.PatternAddedSkip) {
		t.Fatalf("I-10 signals = %+v, want one for cycle 4 with the added-skip pattern", signals)
	}
}

func TestVerifyClass_ACycleShipRunsTheGateClassifier(t *testing.T) {
	t.Parallel()
	repo, path := repoWithAGuardTest(t)
	skipTheGuard(t, path)
	opts := &Options{ProjectRoot: repo, Class: ClassCycle, Runner: execRunner, CycleID: 4}
	var res RunResult

	err := verifyClass(context.Background(), opts, &res)

	var se *core.ShipError
	if !errors.As(err, &se) || se.Code != shiperr.CodeGateWeakened {
		t.Fatalf("verifyClass err = %v, want GATE_WEAKENED before the audit binding", err)
	}
}

func TestRefuseWeakenedGates_AnOrdinaryDiffPassesWithALog(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "go/internal/widget/widget.go"), "package widget\n")
	opts := &Options{ProjectRoot: repo, Class: ClassCycle, Runner: execRunner, CycleID: 4}
	var res RunResult

	if err := refuseWeakenedGates(context.Background(), opts, &res); err != nil {
		t.Fatalf("an ordinary diff must pass: %v", err)
	}
	if !containsLog(res, "gate classifier") {
		t.Errorf("logs = %v, want the OK line of the gate classifier", res.Logs)
	}
}

func TestRefuseWeakenedGates_AGitFailureIsReturned(t *testing.T) {
	t.Parallel()
	opts := &Options{ProjectRoot: t.TempDir(), Class: ClassCycle, Runner: execRunner, CycleID: 4}
	var res RunResult

	if err := refuseWeakenedGates(context.Background(), opts, &res); err == nil {
		t.Fatal("a directory that is not a git checkout must fail the check, got nil")
	}
}

func TestRefuseWeakenedGates_AnUnreadableCycleStateIsAStateError(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), "not json")
	opts := &Options{ProjectRoot: repo, Class: ClassCycle, Runner: execRunner}
	var res RunResult

	err := refuseWeakenedGates(context.Background(), opts, &res)

	var se *core.ShipError
	if !errors.As(err, &se) || se.Code != core.CodeStateIO {
		t.Fatalf("err = %v, want a state error", err)
	}
}

func TestRefuseWeakenedGates_AFailedUntrackedListingIsReturned(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	opts := &Options{ProjectRoot: repo, Class: ClassCycle, CycleID: 4}
	opts.Runner = func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && argsContain(args, "ls-files") {
			return -1, errors.New("ls-files exploded")
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	var res RunResult

	if err := refuseWeakenedGates(context.Background(), opts, &res); err == nil || !strings.Contains(err.Error(), "ls-files exploded") {
		t.Fatalf("err = %v, want the ls-files error", err)
	}
}

func TestRefuseWeakenedGates_TheShippingCycleMayChangeItsOwnPredicates(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	predicate := filepath.Join(repo, "go/acs/cycle4/p_test.go")
	mustWrite(t, predicate, "package cycle4\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "add predicates")
	mustWrite(t, predicate, "package cycle4\n\nconst rows = 2\n")
	var res RunResult

	if err := refuseWeakenedGates(context.Background(), &Options{ProjectRoot: repo, Class: ClassCycle, Runner: execRunner, CycleID: 4}, &res); err != nil {
		t.Fatalf("cycle 4 changes its own predicate package: %v", err)
	}
	err := refuseWeakenedGates(context.Background(), &Options{ProjectRoot: repo, Class: ClassCycle, Runner: execRunner, CycleID: 9}, &res)
	if err == nil || !strings.Contains(err.Error(), "frozen-check go/acs/cycle4/p_test.go") {
		t.Fatalf("cycle 9 changes the predicates of cycle 4: err = %v, want a frozen-check refusal", err)
	}
}
