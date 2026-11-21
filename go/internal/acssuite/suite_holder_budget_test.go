package acssuite

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

func holderSuiteRun(t *testing.T, holdLock bool, center *signalcenter.Center) (log string) {
	t.Helper()
	bin, logDir := t.TempDir(), t.TempDir()
	log = filepath.Join(logDir, "go-test.log")
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(strings.Replace(fakeGoScript, "LOGFILE", strconv.Quote(log), 1)), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	setLaneShare(t, "3")
	root := t.TempDir()
	writeFixtureModule(t, root, map[string]string{
		"go/go.mod":                       "module fixture\n\ngo 1.23\n",
		"go/acs/cycle7/acs_test.go":       "package cycle7\n\nimport \"testing\"\n\nfunc TestDeclared(t *testing.T) {}\n",
		"go/acs/regression/a/acs_test.go": "package a\n\nimport \"testing\"\n\nfunc TestDeclared(t *testing.T) {}\n",
		"go/acs/redteam/acs_test.go":      "package redteam\n\nimport \"testing\"\n\nfunc TestDeclared(t *testing.T) {}\n",
	})
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs", "cycle-7"), 0o755); err != nil {
		t.Fatal(err)
	}
	if holdLock {
		hold, err := verifylock.Acquire(context.Background(), root, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(hold)
		saved := suiteLockWait
		suiteLockWait = 50 * time.Millisecond
		t.Cleanup(func() { suiteLockWait = saved })
	}
	if _, err := Run(Options{Root: root, ProjectRoot: root, Cycle: 7, GoTimeout: generousBudget, Signals: center}); err != nil {
		t.Fatal(err)
	}
	return log
}

func goTestChildShares(t *testing.T, log string) []string {
	t.Helper()
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the fake go never ran a test: %v", err)
	}
	var shares []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		args, gomaxprocs, _ := strings.Cut(line, "|")
		fields := strings.Fields(args)
		if len(fields) < 3 || fields[1] != "-p" || fields[2] != gomaxprocs {
			t.Errorf("child ran %q with GOMAXPROCS=%q, want -p equal to GOMAXPROCS", args, gomaxprocs)
		}
		shares = append(shares, gomaxprocs)
	}
	return shares
}

func TestRun_TheSuiteThatHoldsTheHostLockSplitsTheHolderShare(t *testing.T) {
	requireParallelHost(t)
	log := holderSuiteRun(t, false, nil)

	_, procs := scopeShare(3, lanebudget.HolderProcs(runtime.NumCPU(), 3))
	for _, got := range goTestChildShares(t, log) {
		if got != strconv.Itoa(procs) {
			t.Errorf("child GOMAXPROCS = %s, want %d: the suite that holds the host lock splits the holder share", got, procs)
		}
	}
}

func TestRun_ASuiteThatLostTheHostLockKeepsTheLaneShare(t *testing.T) {
	requireParallelHost(t)
	log := holderSuiteRun(t, true, nil)

	_, procs := scopeShare(3, lanebudget.Procs(runtime.NumCPU(), 3))
	for _, got := range goTestChildShares(t, log) {
		if got != strconv.Itoa(procs) {
			t.Errorf("child GOMAXPROCS = %s, want %d: an unserialized suite keeps the lane share", got, procs)
		}
	}
}

func TestRun_EachSuiteAcquireEmitsOneWaitSignalOnTheGivenCenter(t *testing.T) {
	requireParallelHost(t)
	center := signalcenter.New()
	holderSuiteRun(t, false, center)

	var waits []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Code == signalcenter.CodeHostVerifyLockWait {
			waits = append(waits, e)
		}
	}
	if len(waits) != 1 || waits[0].Fields["source"] != "acs-suite" || waits[0].Cycle != 7 || waits[0].Fields["outcome"] != "acquired" {
		t.Errorf("wait signals = %+v, want exactly one from acs-suite, acquired, cycle 7", waits)
	}
}
