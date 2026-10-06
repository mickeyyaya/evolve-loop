package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func installPreflightSpy(t *testing.T, order *[]string) {
	t.Helper()
	prev := runLoopPreflightFn
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result {
		*order = append(*order, "preflight")
		return looppreflight.Result{}
	}
	t.Cleanup(func() { runLoopPreflightFn = prev })
}

func smokeFailingHost() *fakeCLIHost {
	h := claudeRaisedHost()
	h.smokeErr = map[string]error{"claude": errors.New("doctor live claude-tmux rc=80 pattern=\"\"")}
	return h
}

func boundaryCoordinator(t *testing.T, resumeWaves int) (*loopBatchCoordinator, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	u13StubRefresh(t, func() bool { return false })
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{
		ctx:      context.Background(),
		cfg:      loopConfig{ProjectRoot: root, EvolveDir: evolveDir, ResumeWaves: resumeWaves},
		cycleEnv: map[string]string{"EVOLVE_CLI_HEALTH": "0"},
		result:   &loopResult{},
		stdout:   &stdout,
		stderr:   &stderr,
	}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, &stderr)
	b.deps.Storage = &fixtures.FakeStorage{}
	return b, &stdout, &stderr
}

func TestRunLoop_TheBootCLIUpdateRunsBeforeThePreflightGate(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)
	var order []string
	installPreflightSpy(t, &order)
	h := claudeRaisedHost()
	h.onRun = func() { order = append(order, "cli update") }
	h.install(t)
	orch := &brakeOrch{evolveDir: evolveDir}

	rc, _, stderr := runBrakeLoop(t, projectRoot, orch, "--cycles", "1")

	if rc != 0 || len(order) < 2 || order[0] != "cli update" || order[1] != "preflight" {
		t.Fatalf("the boot updater runs before the readiness gate: rc=%d order=%q\n%s", rc, order, stderr)
	}
	if !strings.Contains(stderr, "[loop] cli-update: claude updated 2.1.285 → 2.1.286") {
		t.Errorf("the loop log names each family's outcome:\n%s", stderr)
	}
}

func TestRunLoop_ASmokeFailureAtBootHaltsBeforeAnyCycleNamingTheFamilyAndVersion(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)
	var order []string
	installPreflightSpy(t, &order)
	smokeFailingHost().install(t)
	orch := &brakeOrch{evolveDir: evolveDir}

	rc, stdout, stderr := runBrakeLoop(t, projectRoot, orch)

	if rc != 2 || orch.calls != 0 || len(order) != 0 {
		t.Fatalf("a smoke failure halts before the gate and before any cycle: rc=%d cycles=%d order=%q\n%s", rc, orch.calls, order, stderr)
	}
	var result struct {
		StopReason    string             `json:"stop_reason"`
		CLIUpdateHalt []cliupdate.Result `json:"cli_update_halt"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("the loop result is JSON: %v\n%s", err, stdout)
	}
	if result.StopReason != cliUpdateSmokeHalt || len(result.CLIUpdateHalt) != 1 || result.CLIUpdateHalt[0].Family != "claude" || result.CLIUpdateHalt[0].NewVersion != "2.1.286" {
		t.Errorf("the result names the stop, the family and the version: %+v", result)
	}
	if !strings.Contains(stderr, "LOOP_HALT") || !strings.Contains(stderr, "claude smoke-failed 2.1.285 → 2.1.286") || !strings.Contains(stderr, "rc=80") {
		t.Errorf("the halt is a Signal Center incident naming the family, versions and probe:\n%s", stderr)
	}
}

func TestBootCLIUpdate_AReexecHandoffLeavesTheUpdateToTheBoundaryThatRanIt(t *testing.T) {
	for _, wavesDone := range []int{0, 2} {
		h := smokeFailingHost()
		h.install(t)
		b, _, _ := boundaryCoordinator(t, wavesDone)
		b.cfg.HandedOff = true
		lr := &loopResult{}

		if bootCLIUpdateHalts(context.Background(), b.cfg, b.deps, lr, io.Discard, io.Discard) || len(h.calls) != 0 {
			t.Fatalf("a process re-exec'd at wave %d continues a batch whose boundary already updated; it must not update again at boot: calls=%q", wavesDone, h.calls)
		}
	}
}

func TestPrepareIteration_ASmokeFailureAtAWaveBoundaryHaltsTheNextWave(t *testing.T) {
	smokeFailingHost().install(t)
	b, stdout, stderr := boundaryCoordinator(t, 0)
	fc, bin := policy.FleetConfig{Count: 2, Concurrency: 2, Scheduling: "wave"}, ""

	d := b.prepareIteration(1, &fc, &bin, 0)
	b.deps.Signals.Flush()

	if d.flow != batchReturn || d.exitCode != 2 || b.result.StopReason != cliUpdateSmokeHalt || bin != "" {
		t.Fatalf("the next wave must not launch: decision=%+v stop=%q waveBinary=%q\n%s", d, b.result.StopReason, bin, stderr)
	}
	if len(b.result.CLIUpdateHalt) != 1 || !strings.Contains(stdout.String(), cliUpdateSmokeHalt) {
		t.Errorf("the emitted result carries the halting family: %+v\n%s", b.result.CLIUpdateHalt, stdout)
	}
	if !strings.Contains(stderr.String(), "LOOP_HALT") || !strings.Contains(stderr.String(), "claude") {
		t.Errorf("the halt is loud:\n%s", stderr)
	}
}

func TestPrepareIteration_TheFirstIterationOfAProcessLeavesTheUpdateToBoot(t *testing.T) {
	for _, tc := range []struct {
		resumeWaves, iteration int
		wantUpdate             bool
	}{
		{0, 0, false},
		{0, 1, true},
		{3, 3, false},
		{3, 4, true},
	} {
		h := claudeRaisedHost()
		h.install(t)
		b, _, stderr := boundaryCoordinator(t, tc.resumeWaves)
		fc, bin := policy.FleetConfig{Count: 1}, ""

		if d := b.prepareIteration(tc.iteration, &fc, &bin, 0); d.flow != batchProceed {
			t.Fatalf("resume=%d iteration=%d: want proceed, got %+v\n%s", tc.resumeWaves, tc.iteration, d, stderr)
		}
		if got := len(h.calls) > 0; got != tc.wantUpdate {
			t.Errorf("resume=%d iteration=%d: updated=%v, want %v (calls %q)", tc.resumeWaves, tc.iteration, got, tc.wantUpdate, h.calls)
		}
	}
}

func TestPrepareIteration_AnUpdateFailureWithAHealthySmokeWarnsAndTheWaveProceeds(t *testing.T) {
	h := claudeRaisedHost()
	h.versions = map[string][]string{"claude": {"2.1.285"}}
	h.runErr = map[string]error{"claude": errors.New("network unreachable")}
	h.install(t)
	b, _, stderr := boundaryCoordinator(t, 0)
	fc, bin := policy.FleetConfig{Count: 1}, ""

	d := b.prepareIteration(1, &fc, &bin, 0)

	if d.flow != batchProceed || b.result.StopReason != "" {
		t.Fatalf("an updater error on a CLI whose smoke passed keeps the wave: %+v stop=%q\n%s", d, b.result.StopReason, stderr)
	}
	if !strings.Contains(stderr.String(), "[loop] WARN: cli-update: claude update-failed") || !strings.Contains(stderr.String(), "network unreachable") {
		t.Errorf("the failed update is a loud WARN:\n%s", stderr)
	}
}

func TestPrepareIteration_AnInterruptDuringTheUpdateStopsTheBatchWithoutAHalt(t *testing.T) {
	h := smokeFailingHost()
	b, _, stderr := boundaryCoordinator(t, 0)
	ctx, cancel := context.WithCancel(context.Background())
	b.ctx = ctx
	h.onRun = cancel
	h.install(t)
	fc, bin := policy.FleetConfig{Count: 1}, ""

	d := b.prepareIteration(1, &fc, &bin, 0)

	if d.flow != batchReturn || d.exitCode != 130 || b.result.StopReason == cliUpdateSmokeHalt {
		t.Fatalf("an interrupt is an interrupt, not a smoke failure: %+v stop=%q\n%s", d, b.result.StopReason, stderr)
	}
}

func TestRunLoop_AnUnrecordedAgyVersionChangeIsSmokeBootedBeforeThePreflightGate(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)
	lastBoundary := cliupdate.Report{Results: []cliupdate.Result{{Family: "agy", Status: cliupdate.StatusUnchanged, OldVersion: "1.2.16", NewVersion: "1.2.16"}}}
	if err := cliupdate.Remember(evolveDir, lastBoundary, cliUpdateNow); err != nil {
		t.Fatal(err)
	}
	var order []string
	installPreflightSpy(t, &order)
	h := &fakeCLIHost{
		families: []cliupdate.Family{{Name: "agy", UpdateArgv: []string{"agy", "update"}}},
		versions: map[string][]string{"agy": {"1.2.17"}},
		onSmoke:  func(family string) { order = append(order, "smoke "+family) },
	}
	h.install(t)

	rc, _, stderr := runBrakeLoop(t, projectRoot, &brakeOrch{evolveDir: evolveDir}, "--cycles", "1")

	if rc != 0 || len(order) < 2 || order[0] != "smoke agy" || order[1] != "preflight" {
		t.Fatalf("agy moved 1.2.16 → 1.2.17 between boundaries; its first launch must be the boot smoke, before the readiness gate: rc=%d order=%q\n%s", rc, order, stderr)
	}
	if !strings.Contains(stderr, "[loop] cli-update: agy self-updated 1.2.16 → 1.2.17: outside a boundary; smoke OK") {
		t.Errorf("the loop log names the absorbed self-update:\n%s", stderr)
	}
	records, err := cliupdate.LoadRecords(evolveDir)
	if err != nil || len(records) != 2 || records[1].Kind != cliupdate.KindSelfUpdate || records[1].Old != "1.2.16" || records[1].New != "1.2.17" {
		t.Errorf("the smoke-booted self-update is recorded for cli-version-drift: %+v %v", records, err)
	}
}

func TestPrepareIteration_AProbeFailureSkipIsALoudWarn(t *testing.T) {
	h := claudeRaisedHost()
	h.versions = map[string][]string{"claude": {"2.1.285"}}
	h.probeErr = map[string]error{"claude": errors.New("doctor live claude-tmux rc=1 pattern=\"rate_limit\"")}
	h.install(t)
	b, _, stderr := boundaryCoordinator(t, 0)
	fc, bin := policy.FleetConfig{Count: 1}, ""

	if d := b.prepareIteration(1, &fc, &bin, 0); d.flow != batchProceed {
		t.Fatalf("an unsubscribed family does not stop the wave: %+v", d)
	}
	if !strings.Contains(stderr.String(), "[loop] WARN: cli-update: claude skipped 2.1.285: doctor live did not answer") {
		t.Errorf("a family skipped on a failed probe is a WARN, not a plain line:\n%s", stderr)
	}
}
