//go:build acs

package cycle1967

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var evolveBuild struct {
	once      sync.Once
	dir, path string
	failure   string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		if err := os.RemoveAll(evolveBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1967: remove %s: %v\n", evolveBuild.dir, err)
		}
	}
	os.Exit(code)
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func evolveBin(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	evolveBuild.once.Do(func() { evolveBuild.failure = buildEvolve(dir) })
	if evolveBuild.failure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.failure)
	}
	return evolveBuild.path
}

func buildEvolve(dir string) string {
	tmp, err := os.MkdirTemp("", "cycle1967-evolve-")
	if err != nil {
		return err.Error()
	}
	evolveBuild.dir, evolveBuild.path = tmp, filepath.Join(tmp, "evolve")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", evolveBuild.path, "./cmd/evolve")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("%v\n%s", err, out)
	}
	return ""
}

func isolatedEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

func runIn(t *testing.T, dir, name string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = dir, isolatedEnv()
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if exit, ok := err.(*exec.ExitError); ok {
		return out.String(), errb.String(), exit.ExitCode()
	}
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return out.String(), errb.String(), 0
}

var passLine = regexp.MustCompile(`(?m)^--- PASS: (\S+)`)

func requirePassed(t *testing.T, pkg, runPattern string, want ...string) {
	t.Helper()
	stdout, stderr, code := runIn(t, goDir(t), "go", "test", "-count=1", "-v", "-run", runPattern, pkg)
	if code != 0 {
		t.Fatalf("go test -run %s %s: exit %d\n%s%s", runPattern, pkg, code, stdout, stderr)
	}
	passed := map[string]bool{}
	for _, m := range passLine.FindAllStringSubmatch(stdout, -1) {
		passed[m[1]] = true
	}
	for _, name := range want {
		if !passed[name] {
			t.Errorf("%s did not run and pass in %s (a pattern that matches no test passes silently)\n%s", name, pkg, stdout)
		}
	}
}

func TestC1967_001_AnIdleTransitionBelowMinLanesEmitsOneUnderfilledWarn(t *testing.T) {
	requirePassed(t, "./internal/orchestrator", `^TestUnderfill_(TheCodeIsRegisteredToTheLoopModule|AnIdleTransitionBelowMinLanesWarnsOnceWithTheFacts|ARunningPoolWarnsOnEveryIdleTransitionBelowMinLanes)$`,
		"TestUnderfill_TheCodeIsRegisteredToTheLoopModule",
		"TestUnderfill_AnIdleTransitionBelowMinLanesWarnsOnceWithTheFacts",
		"TestUnderfill_ARunningPoolWarnsOnEveryIdleTransitionBelowMinLanes")
}

func TestC1967_002_NoUnderfilledWarnAtMinLanesOrUnderTheBrake(t *testing.T) {
	requirePassed(t, "./internal/orchestrator", `^TestUnderfill_(NoWarnWhileTheLiveCountHoldsMinLanes|NoWarnWhileTheOperatorBrakeIsEngaged)$`,
		"TestUnderfill_NoWarnWhileTheLiveCountHoldsMinLanes",
		"TestUnderfill_NoWarnWhileTheOperatorBrakeIsEngaged")
}

func TestC1967_003_TheLoopHandsFleetMinLanesToThePoolItBuilds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", "^TestOrchestratorPool_WiresTheEnforceKeysAndTheFileRuleWithoutAGraph$", "./cmd/evolve")
	cmd.Dir, cmd.Env = goDir(t), isolatedEnv()
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: TestOrchestratorPool_WiresTheEnforceKeysAndTheFileRuleWithoutAGraph") {
		t.Fatalf("err %v, want the named test to run and pass\n%s", err, out)
	}
}

func TestC1967_004_ThePoolTransitionCarriesThePendingCountTheWarnReports(t *testing.T) {
	requirePassed(t, "./internal/fleet", `^TestRunPool_EveryTransitionCarriesTheTodosStillPending$`,
		"TestRunPool_EveryTransitionCarriesTheTodosStillPending")
}

var waveStart = time.Date(2026, 10, 19, 6, 0, 0, 0, time.UTC)

func signalLine(seq int, at time.Time, severity, kind, code, reason string) string {
	return fmt.Sprintf(`{"schema_version":"1.0","seq":%d,"pid":1,"ts":%q,"module":"loop","origin":"Pool.Run","kind":%q,"code":%q,"severity":%q,"reason":%q,"fields":{"live":"1","target":"4"}}`+"\n",
		seq, at.Format(time.RFC3339Nano), kind, code, severity, reason)
}

func watchPlane(t *testing.T, signals string) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := wave.NewStore(evolveDir).Save(wave.Record{Number: 130, RunID: "r130", PID: 9, StartedAt: waveStart}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "signals.ndjson"), []byte(signals), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func watch(t *testing.T, root string, extra ...string) (stdout, stderr string, code int) {
	t.Helper()
	args := append([]string{"wave", "watch", "--project-root", root}, extra...)
	return runIn(t, t.TempDir(), evolveBin(t), args...)
}

func TestC1967_005_WaveWatchPrintsThePoolTransitionsAndEveryLoopWaveWarn(t *testing.T) {
	at := func(m int) time.Time { return waveStart.Add(time.Duration(m) * time.Minute) }
	root := watchPlane(t,
		signalLine(1, at(1), "INFO", "loop.wave", "LOOP_ORCH_LANE_LAUNCHED", "pool launched todo-a")+
			signalLine(2, at(2), "INFO", "loop.wave", "LOOP_ORCH_LANE_EXITED", "pool exited todo-a")+
			signalLine(3, at(3), "INFO", "loop.wave", "LOOP_ORCH_LANE_BACKFILLED", "pool backfilled todo-b")+
			signalLine(4, at(4), "INFO", "loop.wave", "LOOP_ORCH_POOL_IDLE", "pool idle")+
			signalLine(5, at(5), "WARN", "loop.wave", "LOOP_ORCH_POOL_UNDERFILLED", "pool underfilled: 1 of 4 lanes live")+
			signalLine(6, at(6), "WARN", "loop.wave", "LOOP_ORCH_LANE_DEADLINE", "lane todo-b passed its deadline")+
			signalLine(7, at(7), "INFO", "loop.wave", "LOOP_ORCH_LANE_STALLED", "lane todo-b is stalled")+
			signalLine(8, at(8), "WARN", "loop.wave", "LOOP_ORCH_LANE_RECOVERY_FAILED", "release of todo-b failed"))

	stdout, stderr, code := watch(t, root)

	if code != 0 {
		t.Fatalf("evolve wave watch: exit %d\n%s%s", code, stdout, stderr)
	}
	for _, want := range []string{"LOOP_ORCH_LANE_LAUNCHED", "LOOP_ORCH_LANE_EXITED", "LOOP_ORCH_LANE_BACKFILLED", "LOOP_ORCH_POOL_IDLE",
		"LOOP_ORCH_POOL_UNDERFILLED", "LOOP_ORCH_LANE_DEADLINE", "LOOP_ORCH_LANE_STALLED", "LOOP_ORCH_LANE_RECOVERY_FAILED"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %s\n%s", want, stdout)
		}
	}
}

func TestC1967_006_WaveWatchLeavesOutWhatIsNeitherAPoolTransitionNorAWarn(t *testing.T) {
	at := func(m int) time.Time { return waveStart.Add(time.Duration(m) * time.Minute) }
	root := watchPlane(t,
		signalLine(1, at(-5), "WARN", "loop.wave", "LOOP_ORCH_POOL_UNDERFILLED", "before the wave")+
			signalLine(2, at(1), "INFO", "loop.wave", "LOOP_ORCH_WINDOW", "window grew")+
			signalLine(3, at(2), "INFO", "loop.wave", "LOOP_ORCH_SUPPLY", "supply census")+
			signalLine(4, at(3), "WARN", "phase.outcome", "RUNNER_PHASE_WARNED", "a phase warning of another kind"))

	stdout, stderr, code := watch(t, root)

	if code != 0 {
		t.Fatalf("evolve wave watch: exit %d\n%s%s", code, stdout, stderr)
	}
	for _, absent := range []string{"before the wave", "LOOP_ORCH_WINDOW", "LOOP_ORCH_SUPPLY", "RUNNER_PHASE_WARNED"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("stdout names %s, want it left out\n%s", absent, stdout)
		}
	}
}

func TestC1967_007_WaveWatchJSONPrintsAPoolEventPerLine(t *testing.T) {
	root := watchPlane(t, signalLine(1, waveStart.Add(time.Minute), "WARN", "loop.wave", "LOOP_ORCH_POOL_UNDERFILLED", "pool underfilled"))

	stdout, stderr, code := watch(t, root, "--json")

	if code != 0 {
		t.Fatalf("evolve wave watch --json: exit %d\n%s%s", code, stdout, stderr)
	}
	first, _, _ := strings.Cut(stdout, "\n")
	if !strings.Contains(first, `"kind":"pool"`) || !strings.Contains(first, "LOOP_ORCH_POOL_UNDERFILLED") {
		t.Errorf("first JSON line = %q, want kind pool naming LOOP_ORCH_POOL_UNDERFILLED", first)
	}
}
