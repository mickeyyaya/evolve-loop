package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func writeHandedOffCycle(t *testing.T, evolveDir string, cycle int) {
	t.Helper()
	writeCycleTiming(t, evolveDir, cycle, zsNoWork)
	if err := committedset.WriteHandoff(filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycle)), committedset.Handoff{Cycle: cycle, Routed: []string{"item-" + strconv.Itoa(cycle)}}); err != nil {
		t.Fatal(err)
	}
}

func TestBlockerBreakerHalt_HandOffsFromEarlierBatchesCountInTheWindow(t *testing.T) {
	t.Parallel()
	c, got, _ := recordingLoopSignals()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	for cycle, handedOff := range map[int]bool{11: true, 12: false, 13: true, 14: false, 15: true, 16: true} {
		if handedOff {
			writeHandedOffCycle(t, evolveDir, cycle)
		} else {
			writeCycleTiming(t, evolveDir, cycle, shipped)
		}
	}

	rc, halted := blockerBreakerHalt(evolveDir, root, 16, io.Discard, c)

	if !halted || rc != systemFailureHaltExitCode {
		t.Fatalf("four hand-offs in the last twelve cycles must halt even when every one is from an earlier batch: halted=%v rc=%d", halted, rc)
	}
	halts := loopHalts(*got)
	if len(halts) != 1 {
		t.Fatalf("one halt is ONE INCIDENT: %+v", halts)
	}
	if e := halts[0]; e.Code != CodeLoopPipelineBlockerHalt || e.Cycle != 16 || e.Fields["rule"] != "no-work-handoffs" ||
		e.Fields["category"] != "no-work-handoff" || e.Fields["fingerprint"] != core.NoWorkHandoffFingerprint(16) {
		t.Errorf("the INCIDENT names the rule, the category, the last hand-off cycle and the ack fingerprint: %+v", e)
	}
	item := filepath.Join(evolveDir, "inbox", "pipeline-defect-no-work-handoff-cycle16.json")
	raw, err := os.ReadFile(item)
	if err != nil {
		t.Fatalf("the halt files its P0 pipeline-defect item: %v", err)
	}
	if !strings.Contains(string(raw), core.NoWorkHandoffFingerprint(16)) {
		t.Errorf("the P0 item carries the fingerprint its consumption acks:\n%s", raw)
	}
}

func TestBlockerBreakerHalt_AnAcknowledgedHandOffRunNeverHaltsAgain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	for _, cycle := range []int{11, 12, 13, 14} {
		writeHandedOffCycle(t, evolveDir, cycle)
	}
	if err := core.AppendResolvedFingerprint(evolveDir, core.NoWorkHandoffFingerprint(14), "operator", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, halted := blockerBreakerHalt(evolveDir, root, 14, io.Discard, nil); halted {
		t.Fatal("the operator acked hand-offs through cycle 14; they must not halt the loop again")
	}
	for _, cycle := range []int{15, 16, 17} {
		writeHandedOffCycle(t, evolveDir, cycle)
	}
	if _, halted := blockerBreakerHalt(evolveDir, root, 17, io.Discard, nil); halted {
		t.Fatal("three new hand-offs above the ack mark are under the ceiling of four")
	}
	writeHandedOffCycle(t, evolveDir, 18)
	if _, halted := blockerBreakerHalt(evolveDir, root, 18, io.Discard, nil); !halted {
		t.Fatal("the fourth new hand-off above the ack mark must halt")
	}
}

type handoffOrch struct {
	noSignals
	t         *testing.T
	evolveDir string
	cycle     int
	calls     int
}

func (o *handoffOrch) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	o.calls++
	writeHandedOffCycle(o.t, o.evolveDir, o.cycle)
	return core.CycleResult{Cycle: o.cycle, FinalVerdict: core.VerdictPASS}, nil
}

func (o *handoffOrch) RunCycleFromPhase(ctx context.Context, req core.CycleRequest, _ *core.ResumePoint) (core.CycleResult, error) {
	return o.RunCycle(ctx, req)
}

func TestRunLoop_AOneWaveBatchWhoseWaveTipsTheWindowHaltsAfterTheWave(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"dispatch":{"policy":"off"},"workflow":{"max_consecutive_fails":9}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, cycle := range []int{2, 3, 4} {
		writeHandedOffCycle(t, evolveDir, cycle)
	}
	orch := &handoffOrch{t: t, evolveDir: evolveDir, cycle: 6}
	defer installStubDeps(t, &fixtures.FakeStorage{}, newFakeLedger())()
	prev := loopOrchOverride
	loopOrchOverride = orch
	defer func() { loopOrchOverride = prev }()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--project-root", projectRoot, "--evolve-dir", evolveDir, "--goal-text", "hand-off goal", "--cycles", "1"}, nil, &stdout, &stderr)

	if orch.calls != 1 {
		t.Fatalf("the batch of one wave launched %d cycles\n%s", orch.calls, stderr.String())
	}
	if rc != systemFailureHaltExitCode || !strings.Contains(stderr.String(), "no-work-handoff") {
		t.Fatalf("the wave's hand-off is the fourth in the window: the batch must halt after the wave, rc=%d\n%s", rc, stderr.String())
	}
}

func TestBlockerBreakerHalt_TheNoWorkCloseoutsHandOffsReachTheAlarm(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for cycle := 21; cycle <= 24; cycle++ {
		id := "dropped-" + strconv.Itoa(cycle)
		if err := os.WriteFile(filepath.Join(inbox, "2026-10-20T00-00-00Z-"+id+".json"), []byte(`{"id":"`+id+`","title":"fixture","kind":"bug"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		ws := cycleWorkspace(root, cycle)
		writeCycleTiming(t, evolveDir, cycle, zsNoWork)
		if err := os.WriteFile(filepath.Join(ws, "lane-scope.json"), []byte(`{"todo_ids":["`+id+`"],"goal_hash":"g"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(`{"top_n":[],"dropped":[{"id":"`+id+`","reason":"stale"}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := applyCycleNoWorkOutcome(root, cycle, io.Discard, nil, nil); err != nil {
			t.Fatalf("cycle %d closeout: %v", cycle, err)
		}
	}

	var stderr bytes.Buffer
	if _, halted := blockerBreakerHalt(evolveDir, root, 24, &stderr, nil); !halted {
		t.Fatalf("four lanes whose no-work closeout routed their items to the console must halt the loop\n%s", stderr.String())
	}
}

func TestBlockerBreakerHalt_SaysWhichHandOffsAreAcknowledged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeHandedOffCycle(t, evolveDir, 11)
	if err := core.AppendResolvedFingerprint(evolveDir, core.NoWorkHandoffFingerprint(11), "operator", time.Now()); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	blockerBreakerHalt(evolveDir, root, 11, &stderr, nil)
	if !strings.Contains(stderr.String(), "hand-offs through cycle 11 are acknowledged") {
		t.Fatalf("a non-zero ack mark must be visible:\n%s", stderr.String())
	}
}
