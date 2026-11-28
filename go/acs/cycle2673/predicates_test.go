//go:build acs

package cycle2673

import (
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	orchestratorPkg = "./internal/orchestrator"
	stopCase        = "TestClockContract/system/stop_before_the_delay_prevents_the_run"
	stallReproducer = "TestClockContract_StopBeforeTheDelayHoldsWhenTheCallerIsDescheduled"
	mutantGuard     = "TestClockContract_StopBeforeTheDelayCatchesAClockWhoseStopDoesNotCancelTheRun"
	loadedRuns      = 200
)

func busyLoopsUntilDone(t *testing.T) {
	t.Helper()
	var stop atomic.Bool
	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
			}
		}()
	}
	t.Cleanup(func() {
		stop.Store(true)
		wg.Wait()
	})
}

func TestC2673_001_stop_before_the_delay_case_holds_across_200_raced_runs_on_a_loaded_host(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	busyLoopsUntilDone(t)
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(goDir, "go", "test", "-race", "-cpu", "2", "-count=200", "-v",
		"-run", "^"+strings.ReplaceAll(stopCase, "/", "$/^")+"$", orchestratorPkg)
	if err != nil || code != 0 {
		t.Fatalf("loaded 200-run exited %d (err=%v)\n%s\n%s", code, err, acsassert.OutputTail(stdout, 25), acsassert.OutputTail(stderr, 10))
	}
	if passes := strings.Count(stdout, "--- PASS: "+stopCase+" ("); passes != loadedRuns {
		t.Errorf("the system stop-before-delay case passed %d times, want %d", passes, loadedRuns)
	}
	if strings.Contains(stdout, "--- FAIL") {
		t.Errorf("a run failed:\n%s", acsassert.OutputTail(stdout, 25))
	}
	stalled, stalledErr, stalledCode, err := acsassert.SubprocessOutputIn(goDir, "go", "test", "-race", "-cpu", "2", "-count=3", "-v",
		"-run", "^"+stallReproducer+"$", orchestratorPkg)
	if err != nil || stalledCode != 0 {
		t.Errorf("a caller descheduled past the delay exited %d (err=%v)\n%s\n%s", stalledCode, err, acsassert.OutputTail(stalled, 15), acsassert.OutputTail(stalledErr, 10))
	}
}

func TestC2673_002_stop_stays_true_through_a_descheduled_caller_and_still_proves_the_contract(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir,
		Package: orchestratorPkg,
		Pattern: "^(" + stallReproducer + "|" + mutantGuard + ")$",
		Names:   []string{stallReproducer, mutantGuard},
	})
}

func TestC2673_003_reproducer_and_the_whole_clock_contract_pass_under_the_race_detector(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir,
		Package: orchestratorPkg,
		Pattern: "^(TestClockContract|" + stallReproducer + "|TestClockContract_CatchesAClockThatRunsCallbacksOnTheCallersGoroutine)$",
		Names:   []string{"TestClockContract", stallReproducer, "TestClockContract_CatchesAClockThatRunsCallbacksOnTheCallersGoroutine"},
		Race:    true,
	})
}
