package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/coversplit"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRetakeFloorRun_TheRerunHoldsTheTierLock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var heldDuring bool

	ReviewInput{ProjectRoot: root, Stderr: &strings.Builder{}}.retakeFloorRun("./q", retakeRedOutput, []string{"./q"}, func() (string, bool) {
		release, held, err := flock.TryLock(ciparity.TierLockPath(root))
		if err == nil && !held {
			release()
		}
		heldDuring = held
		return "ok", true
	})

	if !heldDuring {
		t.Fatal("the rerun ran without the integration-tier lock, so the retake serializes nothing")
	}
	release, held, err := flock.TryLock(ciparity.TierLockPath(root))
	if err != nil || held {
		t.Fatalf("TryLock after the retake = (%v, %v), want the lock released", held, err)
	}
	release()
}

func TestRetakeFloorRun_TheLockLivesUnderTheProjectRootNotTheLane(t *testing.T) {
	t.Parallel()
	root, worktree := t.TempDir(), t.TempDir()

	ReviewInput{ProjectRoot: root, Worktree: worktree, Stderr: &strings.Builder{}}.retakeFloorRun("./q", retakeRedOutput, []string{"./q"}, func() (string, bool) { return "ok", true })

	if _, err := os.Stat(ciparity.TierLockPath(root)); err != nil {
		t.Errorf("the lock file is not under the project root: %v", err)
	}
	if _, err := os.Stat(ciparity.TierLockPath(worktree)); err == nil {
		t.Error("a lane made a private lock file, so lanes do not serialize")
	}
}

func TestRetakeSelfCheckFailures_ReportsTheRetakeOutputNotTheFirst(t *testing.T) {
	t.Parallel()
	first := []selfCheckFailure{{Pkg: "./q", Output: "--- FAIL: TestFirst (0.00s)\n"}}

	got := ReviewInput{Stderr: &strings.Builder{}}.retakeSelfCheckFailures(first, func(string) (string, bool) { return "--- FAIL: TestSecond (0.00s)\n", false })

	if len(got) != 1 || !strings.Contains(got[0].Output, "TestSecond") || strings.Contains(got[0].Output, "TestFirst") {
		t.Fatalf("failures = %+v, want the output of the retake", got)
	}
}

func TestFloorRetakeSignals_NameTheirKindVerdictAndFirstOrRetakeTests(t *testing.T) {
	t.Parallel()
	signals := signalcenter.New()
	in := ReviewInput{Signals: signals, Stderr: &strings.Builder{}}

	in.emitFloorRetakeGreen("./a", []string{"TestA"})
	in.emitFloorRetakeRed("./b", []string{"TestB"})

	events := retakeEvents(signals)
	if len(events) != 2 {
		t.Fatalf("events = %+v, want two", events)
	}
	green, red := events[0], events[1]
	if green.Code != CodeBuildFloorRetakeGreen || green.Kind != signalcenter.KindGatePassed || green.Fields["retake"] != "green" || green.Fields["tests"] != "TestA" {
		t.Errorf("green = %+v", green)
	}
	if red.Code != CodeBuildFloorRetakeRed || red.Kind != signalcenter.KindGateRejected || red.Fields["retake"] != "red" || red.Fields["tests"] != "TestB" {
		t.Errorf("red = %+v", red)
	}
}

func TestCoverPlanRetake_APlumbingErrorOnTheRetakeKeepsTheFirstRedAndIsNotRED(t *testing.T) {
	t.Parallel()
	module := tinyModule(t, map[string]string{
		"p/p.go":      "package p\n\nfunc A() int { return 1 }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { _ = A() }\n",
	})
	profile := filepath.Join(t.TempDir(), "profile-is-a-directory")
	if err := os.Mkdir(profile, 0o755); err != nil {
		t.Fatal(err)
	}
	plan := coverPlan{moduleDir: module, profile: profile, label: "./p", pkgs: []string{"./p"}, runs: []coversplit.Run{{Packages: []string{"./p"}}, {Packages: []string{"./p"}}}}
	signals := signalcenter.New()
	var stderr strings.Builder
	in := ReviewInput{ProjectRoot: t.TempDir(), Signals: signals, Stderr: &stderr}

	out, status := plan.retake(context.Background(), in, retakeRedOutput)

	if status != coverStatusTestsFailed || !strings.Contains(out, "--- FAIL: TestFlaky") {
		t.Fatalf("(%q, %d), want the first red kept", out, status)
	}
	if events := retakeEvents(signals); len(events) != 0 {
		t.Errorf("signals = %+v, want no RED signal for a plumbing error", events)
	}
	if !strings.Contains(stderr.String(), "could not run") {
		t.Errorf("stderr = %q, want the plumbing error reported as such", stderr.String())
	}
}

func TestScopedCoverProfile_AFirstPassPlumbingErrorIsNotRetaken(t *testing.T) {
	t.Parallel()
	module := tinyModule(t, map[string]string{
		"p/p.go":       "package p\n\nfunc A() int { return 1 }\n",
		".cover-split": "not a valid split line\n",
	})
	signals := signalcenter.New()
	in := ReviewInput{ProjectRoot: t.TempDir(), Signals: signals, Stderr: &strings.Builder{}}

	_, _, status := scopedCoverProfile(context.Background(), in, module, []string{"./p"}, nil)

	if status != coverStatusPlumbingError || len(retakeEvents(signals)) != 0 {
		t.Fatalf("status = %d, signals = %+v, want a plumbing error and no retake", status, retakeEvents(signals))
	}
}
