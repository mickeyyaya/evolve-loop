//go:build acs

package cycle2511

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const orchestratorPkg = "github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"

func runContract(t *testing.T, pattern string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", pattern, orchestratorPkg)
	cmd.Dir = acsassert.GoModuleDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func requirePass(t *testing.T, out, name string) {
	t.Helper()
	if !strings.Contains(out, "--- PASS: "+name+" (") {
		t.Errorf("subtest %q did not report PASS; output tail:\n%s", name, acsassert.OutputTail(out, 25))
	}
}

func TestC2511_001_ContractTableRunsTheLockCaseOnTheFakeClock(t *testing.T) {
	out := runContract(t, "^TestClockContract$")
	requirePass(t, out, "TestClockContract/fake/callback_takes_a_lock_the_caller_holds")
}

func TestC2511_002_ContractTableRunsTheLockCaseOnSystemClock(t *testing.T) {
	out := runContract(t, "^TestClockContract$")
	requirePass(t, out, "TestClockContract/system/callback_takes_a_lock_the_caller_holds")
}

func TestC2511_003_ContractTableCoversTheSameCasesOnBothClocks(t *testing.T) {
	out := runContract(t, "^TestClockContract$")
	for _, clock := range []string{"fake", "system"} {
		for _, c := range []string{
			"runs_the_function_once_after_the_delay",
			"stop_before_the_delay_prevents_the_run",
			"stop_after_the_run_reports_false",
			"a_callback_may_arm_another_timer",
			"callback_takes_a_lock_the_caller_holds",
		} {
			requirePass(t, out, "TestClockContract/"+clock+"/"+c)
		}
	}
}

func TestC2511_004_ContractRejectsAClockThatRunsCallbacksInline(t *testing.T) {
	out := runContract(t, "^TestClockContract_CatchesAClockThatRunsCallbacksOnTheCallersGoroutine$")
	requirePass(t, out, "TestClockContract_CatchesAClockThatRunsCallbacksOnTheCallersGoroutine")
}
