package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestLaneResume_TwoLanesOfOneScopeAdoptAPausedCycleExactlyOnce(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2107)
	signals := signalcenter.New()
	const lanes = 2
	adopted := make([]bool, lanes)
	stderrs := make([]bytes.Buffer, lanes)
	var found sync.WaitGroup
	found.Add(lanes)
	var wg sync.WaitGroup
	for i := range lanes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lane := p.lane(&stderrs[i], signals, "item-a")
			lane.record = func(path string, auto bool) (checkpoint.Checkpoint, bool, error) {
				found.Done()
				found.Wait()
				return checkpoint.RecordLaneResume(path, auto)
			}
			adopted[i] = lane.adopt() != nil
		}()
	}
	wg.Wait()

	winners := 0
	for _, a := range adopted {
		if a {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("lanes that adopted cycle 2107 = %d, want exactly 1\nlane 0: %s\nlane 1: %s", winners, stderrs[0].String(), stderrs[1].String())
	}
	if got := readCheckpoint(t, p.statePath).AutoResumeAttempts; got != 1 {
		t.Errorf("auto-resume attempts = %d, want 1: the refused lane spends no attempt", got)
	}
	loser := stderrs[0].String() + stderrs[1].String()
	if !strings.Contains(loser, "starts a fresh cycle") || !strings.Contains(loser, "held by run") {
		t.Errorf("the refused lane must name the holder and start a fresh cycle; stderr=%q", loser)
	}
}

func TestLaneResume_ALaneThatArrivesAfterTheWinnerLeasedTheCycleStartsFresh(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2107)
	signals := signalcenter.New()
	var winnerErr, loserErr bytes.Buffer
	winner := p.lane(&winnerErr, signals, "item-a")
	winner.record = checkpoint.RecordLaneResume
	loser := p.lane(&loserErr, signals, "item-a")
	loser.record = checkpoint.RecordLaneResume

	if winner.adopt() == nil {
		t.Fatalf("the first lane must adopt cycle 2107; stderr=%q", winnerErr.String())
	}
	if loser.adopt() != nil {
		t.Errorf("a lane that arrives after the lease must not adopt cycle 2107; stderr=%q", loserErr.String())
	}
	if got := readCheckpoint(t, p.statePath).AutoResumeAttempts; got != 1 {
		t.Errorf("auto-resume attempts = %d, want 1: the later lane spends no attempt", got)
	}
}

func TestLoopResume_TheOperatorResumeKeepsTheLeaseFreeRecorder(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2107)
	workspace := p.statePath[:strings.LastIndex(p.statePath, string(os.PathSeparator))]
	if err := runlease.Write(workspace, runlease.Lease{RunID: "other-run", OwnerPID: 4242}, time.Now()); err != nil {
		t.Fatal(err)
	}

	if _, admitted, err := recordResume(p.statePath, true); err != nil || !admitted {
		t.Fatalf("recordResume under a fresh lease = (%v, %v), want admitted: evolve loop --resume takes no lease", admitted, err)
	}
	if got, _, _ := runlease.Read(workspace); got.OwnerPID != 4242 {
		t.Errorf("the operator resume rewrote the lease: %+v", got)
	}
}

func TestRunCycleRun_TheLaneRecordsItsResumeThroughRecordLaneResume(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cmd_cycle.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var recorder string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if id, ok := lit.Type.(*ast.Ident); !ok || id.Name != "laneResume" {
			return true
		}
		for _, el := range lit.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok || kv.Key.(*ast.Ident).Name != "record" {
				continue
			}
			switch v := kv.Value.(type) {
			case *ast.SelectorExpr:
				recorder = v.X.(*ast.Ident).Name + "." + v.Sel.Name
			case *ast.Ident:
				recorder = v.Name
			}
		}
		return true
	})
	if recorder != "checkpoint.RecordLaneResume" {
		t.Errorf("runCycleRun wires the lane to %q, want checkpoint.RecordLaneResume", recorder)
	}
}

func TestLaneResume_UnleaseWarnsWhenTheLeaseCannotBeReleased(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2107)
	workspace := filepath.Dir(p.statePath)
	if err := os.WriteFile(runlease.PathIn(workspace), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	lane := p.lane(&stderr, signalcenter.New(), "item-a")

	lane.unlease(&core.ResumePoint{CycleID: 2107, StatePath: p.statePath})

	if !strings.Contains(stderr.String(), "cannot release the admission lease") {
		t.Errorf("a failed release must warn on stderr; got %q", stderr.String())
	}
}
