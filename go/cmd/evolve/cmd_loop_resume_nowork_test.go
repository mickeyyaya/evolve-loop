//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type resumedNoWorkRunner struct{ cycle int }

func (r resumedNoWorkRunner) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	return plannedNoWork(r.cycle), nil
}

func (r resumedNoWorkRunner) RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error) {
	return plannedNoWork(r.cycle), nil
}

func (resumedNoWorkRunner) SignalSummary() signalcenter.Summary { return signalcenter.Summary{} }

func TestRunResumeBatch_APlannedNoWorkLaneIsHandedOver(t *testing.T) {
	root := t.TempDir()
	evolveDir := seedNoWorkLane(t, root, 7)
	st := storage.New(evolveDir)
	ctx := context.Background()
	cs := core.CycleState{CycleID: 7, RunID: "lane-run", Phase: "triage", WorkspacePath: cycleWorkspace(root, 7)}
	if err := st.WriteState(ctx, core.State{LastCycleNumber: 7}); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteCycleState(ctx, cs); err != nil {
		t.Fatal(err)
	}
	if err := checkpoint.ApplyToStateFile(filepath.Join(evolveDir, core.CycleStateFile), checkpoint.Compose(cs, checkpoint.ReasonQuotaLikely, 0, "", time.Now())); err != nil {
		t.Fatal(err)
	}
	fake := newFakeLedger()
	var stdout, stderr bytes.Buffer
	lr := loopResult{}
	cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir, Resume: true}
	if rc := runResumeBatch(ctx, cfg, resumedNoWorkRunner{cycle: 7}, fake, nil, map[string]string{}, map[string]string{}, &lr, &stdout, &stderr); rc != 0 {
		t.Fatalf("a planned-no-work resume completes: rc=%d stderr=%s", rc, stderr.String())
	}
	if got := routeOf(t, evolveDir, "answered"); got != "console-manual" {
		t.Fatalf("the resumed lane's answered item is handed to the console: route=%q stderr=%s", got, stderr.String())
	}
	if len(fake.lifecycle) == 0 {
		t.Error("through the root's ledger")
	}
}

type resumedFailRunner struct {
	result core.CycleResult
	err    error
}

func (r resumedFailRunner) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	return r.result, r.err
}

func (r resumedFailRunner) RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error) {
	return r.result, r.err
}

func (resumedFailRunner) SignalSummary() signalcenter.Summary { return signalcenter.Summary{} }

func writeResumeCheckpoint(t *testing.T, evolveDir, ws string) {
	t.Helper()
	st := storage.New(evolveDir)
	ctx := context.Background()
	cs := core.CycleState{CycleID: 7, RunID: "lane-run", Phase: "build", WorkspacePath: ws}
	if err := st.WriteState(ctx, core.State{LastCycleNumber: 7}); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteCycleState(ctx, cs); err != nil {
		t.Fatal(err)
	}
	if err := checkpoint.ApplyToStateFile(filepath.Join(evolveDir, core.CycleStateFile), checkpoint.Compose(cs, checkpoint.ReasonQuotaLikely, 0, "", time.Now())); err != nil {
		t.Fatal(err)
	}
}

func TestRunResumeBatch_AResumedFailWalksTheFailureLifecycle(t *testing.T) {
	for name, runner := range map[string]resumedFailRunner{
		"a FAIL verdict":        {result: core.CycleResult{Cycle: 7, FinalVerdict: core.VerdictFAIL}},
		"a cycle-level failure": {result: core.CycleResult{Cycle: 7}, err: &core.ErrCycleLevelFailure{Phase: "build", Cause: errors.New("builder crashed")}},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			evolveDir, ws := seedFailedCycleInbox(t, root, "worked", 7)
			if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "worked", "7"); err != nil {
				t.Fatalf("claim: %v", err)
			}
			writeResumeCheckpoint(t, evolveDir, ws)
			fake := newFakeLedger()
			var stdout, stderr bytes.Buffer
			lr := loopResult{}
			runResumeBatch(context.Background(), loopConfig{ProjectRoot: root, EvolveDir: evolveDir, Resume: true}, runner, fake, nil, map[string]string{}, map[string]string{}, &lr, &stdout, &stderr)
			if entries, _ := os.ReadDir(filepath.Join(evolveDir, "inbox", "processing", "cycle-7")); len(entries) != 0 {
				t.Errorf("the resumed FAIL's claim is released to the root: %d file(s) left; stderr=%s", len(entries), stderr.String())
			}
			if len(fake.lifecycle) == 0 {
				t.Errorf("the walk's lifecycle lines go through the root's ledger; stderr=%s", stderr.String())
			}
		})
	}
}
