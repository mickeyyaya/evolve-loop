package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func handoffProject(t *testing.T) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)
	if err := os.MkdirAll(filepath.Join(root, "go", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "bin", "evolve"), []byte("REBUILT-BINARY-BYTES"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, evolveDir
}

func writeHandoff(t *testing.T, evolveDir string, pid int, age time.Duration) string {
	t.Helper()
	marker := filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile)
	u13WriteJSON(t, marker, map[string]any{"running_commit": "cafebabe1234deadbeef", "batch": 7, "timestamp": time.Now().Add(-age).UTC().Format(time.RFC3339), "pid": pid, "waves_done": 1})
	return marker
}

func writeChainPolicy(t *testing.T, evolveDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"dispatch":{"policy":"off"},"chain":{"max_batches":2}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	u13WriteJSON(t, filepath.Join(evolveDir, "inbox", "pending.json"), map[string]any{"id": "pending"})
}

func stubLaneActive(t *testing.T, active *bool) {
	t.Helper()
	prev := chainBoundaryFleetLaneFn
	t.Cleanup(func() { chainBoundaryFleetLaneFn = prev })
	chainBoundaryFleetLaneFn = func(loopConfig) (bool, error) { return *active, nil }
}

func refreshSkipBoundaries(t *testing.T, evolveDir string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(evolveDir, "signals.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1<<16), 1<<22)
	for scanner.Scan() {
		var ev struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		}
		if json.Unmarshal(scanner.Bytes(), &ev) == nil && ev.Code == "LOOP_BOUNDARY_REFRESH_SKIPPED" {
			out = append(out, ev.Fields["batch"])
		}
	}
	return out
}

func TestRunLoop_ABoundaryReExecKeepsTheWaveIndexAndTheRemainingBudget(t *testing.T) {
	root, evolveDir := handoffProject(t)
	boundary := 0
	u13StubRefresh(t, func() bool { boundary++; return boundary == 2 })
	laneActive := false
	stubLaneActive(t, &laneActive)
	replaced := &brakeOrch{evolveDir: evolveDir}

	_, stdout, stderr := runBrakeLoop(t, root, replaced)
	if replaced.calls != 1 || !strings.Contains(stdout, `"stop_reason": "loop_boundary_refresh_reexec"`) {
		t.Fatalf("the first image runs one iteration and re-execs at boundary 2: calls=%d\n%s\n%s", replaced.calls, stdout, stderr)
	}

	chainRunningCommitFn = func() string { return "0ddba11beef0" }
	chainBoundaryAheadFn = func(string, string) (bool, error) { return true, nil }
	laneActive = true
	replacement := &brakeOrch{evolveDir: evolveDir}
	_, _, stderr = runBrakeLoop(t, root, replacement)

	if replacement.calls != 2 {
		t.Errorf("the replacement image runs the remaining budget max-1 = 2, ran %d\n%s", replacement.calls, stderr)
	}
	if got := strings.Join(refreshSkipBoundaries(t, evolveDir), ","); got != "2,3" {
		t.Errorf("the replacement's boundaries continue the wave index (first wave index 1, boundary 2): got boundaries %q\n%s", got, stderr)
	}
	if !strings.Contains(stderr, "[loop] boundary re-exec: continuing at wave 1") {
		t.Errorf("the replacement names where it continues: %s", stderr)
	}
}

func TestRunLoop_AHandoffArmedByAnotherProcessIsNotInherited(t *testing.T) {
	root, evolveDir := handoffProject(t)
	writeHandoff(t, evolveDir, os.Getpid()+1, 0)
	u13StubRefresh(t, func() bool { return false })
	chainRunningCommitFn = func() string { return "0ddba11beef0" }
	orch := &brakeOrch{evolveDir: evolveDir}

	_, _, stderr := runBrakeLoop(t, root, orch)

	if orch.calls != 3 {
		t.Errorf("a later launch with another pid starts at wave 0 with the full budget: ran %d\n%s", orch.calls, stderr)
	}
}

func TestRunLoop_AHandoffThatCannotBeConsumedIsNotHonouredAndSaysSo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes through a read-only file")
	}
	root, evolveDir := handoffProject(t)
	marker := writeHandoff(t, evolveDir, os.Getpid(), 0)
	if err := os.Chmod(marker, 0o444); err != nil {
		t.Fatal(err)
	}
	u13StubRefresh(t, func() bool { return false })
	chainRunningCommitFn = func() string { return "0ddba11beef0" }
	orch := &brakeOrch{evolveDir: evolveDir}

	_, _, stderr := runBrakeLoop(t, root, orch)

	if orch.calls != 3 || !strings.Contains(stderr, "[loop] WARN: boundary re-exec handoff not honoured") {
		t.Errorf("an unconsumable handoff starts at wave 0 with the full budget and says so: ran %d\n%s", orch.calls, stderr)
	}
}

func TestRunLoop_AStaleHandoffIsNotHonoured(t *testing.T) {
	root, evolveDir := handoffProject(t)
	writeHandoff(t, evolveDir, os.Getpid(), time.Hour)
	u13StubRefresh(t, func() bool { return false })
	chainRunningCommitFn = func() string { return "0ddba11beef0" }
	orch := &brakeOrch{evolveDir: evolveDir}

	_, _, stderr := runBrakeLoop(t, root, orch)

	if orch.calls != 3 || strings.Contains(stderr, "boundary re-exec: continuing") {
		t.Errorf("a pid-matching handoff older than the bound starts at wave 0 with the full budget: ran %d\n%s", orch.calls, stderr)
	}
}

func TestRunLoopChain_TheHandoffIsTakenOnceAtBootAndResumesOnlyTheFirstBatch(t *testing.T) {
	root, evolveDir := handoffProject(t)
	writeChainPolicy(t, evolveDir)
	writeHandoff(t, evolveDir, os.Getpid(), 0)
	u13StubRefresh(t, func() bool { return false })
	chainRunningCommitFn = func() string { return "0ddba11beef0" }
	orch := &brakeOrch{evolveDir: evolveDir}

	_, _, stderr := runBrakeLoop(t, root, orch, "--until-inbox-empty")

	if orch.calls != 2+3 || strings.Count(stderr, "boundary re-exec: continuing at wave 1") != 1 {
		t.Errorf("batch 1 resumes after its one completed wave (2 of 3 run), batch 2 runs its full 3: ran %d\n%s", orch.calls, stderr)
	}
}

func TestRunLoopChain_AnUnconsumableHandoffIsRefusedOnceAtBoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes through a read-only file")
	}
	root, evolveDir := handoffProject(t)
	writeChainPolicy(t, evolveDir)
	if err := os.Chmod(writeHandoff(t, evolveDir, os.Getpid(), 0), 0o444); err != nil {
		t.Fatal(err)
	}
	u13StubRefresh(t, func() bool { return false })
	chainRunningCommitFn = func() string { return "0ddba11beef0" }
	orch := &brakeOrch{evolveDir: evolveDir}

	_, _, stderr := runBrakeLoop(t, root, orch, "--until-inbox-empty")

	if orch.calls != 3+3 || strings.Count(stderr, "boundary re-exec handoff not honoured") != 1 {
		t.Errorf("the handoff is read once, at boot, and neither batch resumes: ran %d\n%s", orch.calls, stderr)
	}
}

func TestRunLoop_AChainBoundaryReExecArmsNoHandoff(t *testing.T) {
	root, evolveDir := handoffProject(t)
	writeChainPolicy(t, evolveDir)
	u13StubRefresh(t, func() bool { return true })
	laneActive := false
	stubLaneActive(t, &laneActive)
	orch := &brakeOrch{evolveDir: evolveDir}

	_, stdout, stderr := runBrakeLoop(t, root, orch, "--until-inbox-empty")

	raw, err := os.ReadFile(filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile))
	if err != nil || !strings.Contains(stdout, `"chain_stop_reason": "chain_boundary_refresh_reexec"`) {
		t.Fatalf("the chain boundary refreshed and armed the breaker: %v\n%s\n%s", err, stdout, stderr)
	}
	if strings.Contains(string(raw), `"pid"`) || strings.Contains(string(raw), `"waves_done"`) {
		t.Errorf("a chain-boundary re-exec arms no wave handoff: %s", raw)
	}
}
