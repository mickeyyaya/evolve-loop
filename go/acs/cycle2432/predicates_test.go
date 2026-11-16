//go:build acs

package cycle2432

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	packageNote     = "docs/architecture/packages/internal-orchestrator.md"
	cmdPackage      = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	wiringTest      = "TestOrchestratedRun_AWaveStopThatMeetsTheRunCapNamesTheOperatorStop"
	poolTest        = "TestPool_ABrakedPoolPastItsRunCapNamesTheOperatorStop"
	cappedLaunch    = 1
	publishedBuffer = 256
)

type recorder struct {
	events []signalcenter.Event
}

func (r *recorder) causes(code signalcenter.Code) []string {
	var out []string
	for _, e := range r.events {
		if e.Code == code {
			out = append(out, e.Fields["cause"])
		}
	}
	return out
}

func runCappedPool(brakeOnLaunch bool) *recorder {
	var braked atomic.Bool
	published := make(chan signalcenter.Event, publishedBuffer)
	launch := func(context.Context, fleet.CycleSpec) (int, error) {
		if brakeOnLaunch {
			braked.Store(true)
		}
		return 0, nil
	}
	pool := orchestrator.Pool{
		Target: 4, MinLanes: 4, MaxLanes: cappedLaunch,
		Launch: launch, Publish: func(e signalcenter.Event) { published <- e }, Halted: braked.Load,
	}
	pool.Run(context.Background(), []fleet.Todo{{ID: "first"}, {ID: "second"}})
	close(published)
	rec := &recorder{}
	for e := range published {
		rec.events = append(rec.events, e)
	}
	return rec
}

func TestC2432_001_ABrakedPoolPastItsRunCapReportsOperatorStop(t *testing.T) {
	rec := runCappedPool(true)
	idle := rec.causes(orchestrator.CodePoolIdle)
	if len(idle) == 0 {
		t.Fatalf("%s events = 0, want the idle transition of the drained pool", orchestrator.CodePoolIdle)
	}
	for _, cause := range idle {
		if cause != string(orchestrator.CauseOperatorStop) {
			t.Errorf("idle cause = %q, want %q: the brake drained a pool that also reached its run cap", cause, orchestrator.CauseOperatorStop)
		}
	}
	if n := len(rec.causes(orchestrator.CodePoolUnderfilled)); n != 0 {
		t.Errorf("%s events = %d, want 0: an operator stop is not an underfill", orchestrator.CodePoolUnderfilled, n)
	}
}

func TestC2432_002_AnUnbrakedPoolPastItsRunCapStillReportsRunCap(t *testing.T) {
	rec := runCappedPool(false)
	for _, code := range []signalcenter.Code{orchestrator.CodePoolIdle, orchestrator.CodePoolUnderfilled} {
		causes := rec.causes(code)
		if len(causes) == 0 {
			t.Fatalf("%s events = 0, want the idle transitions of a capped run", code)
		}
		for _, cause := range causes {
			if cause != string(orchestrator.CauseRunCap) {
				t.Errorf("%s cause = %q, want %q: nothing braked this pool", code, cause, orchestrator.CauseRunCap)
			}
		}
	}
}

func TestC2432_003_TheLoopPoolRunReportsOperatorStopWhenTheWaveStopMeetsTheRunCap(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", "^("+wiringTest+")$", cmdPackage)
	out := stdout + stderr
	if code != 0 || !strings.Contains(out, "--- PASS: "+wiringTest) {
		t.Fatalf("the production caller orchestratedRun does not report operator_stop (exit %d, %v):\n%s", code, err, acsassert.OutputTail(out, 20))
	}
}

func TestC2432_004_ThePoolLevelRegressionTestIsPinnedInThePackageSuite(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", "^("+poolTest+")$", "github.com/mickeyyaya/evolve-loop/go/internal/orchestrator")
	out := stdout + stderr
	if code != 0 || !strings.Contains(out, "--- PASS: "+poolTest) {
		t.Fatalf("the pool regression test did not pass (exit %d, %v):\n%s", code, err, acsassert.OutputTail(out, 20))
	}
}

func TestC2432_005_ThePackageNoteStatesTheOperatorStopPrecedenceInSTE(t *testing.T) {
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, packageNote)
	raw, err := os.ReadFile(note)
	if err != nil {
		t.Fatal(err)
	}
	statement := map[int]bool{}
	for i, line := range strings.Split(string(raw), "\n") {
		stop, capped := strings.Index(line, string(orchestrator.CauseOperatorStop)), strings.Index(line, string(orchestrator.CauseRunCap))
		if stop >= 0 && capped > stop {
			statement[i+1] = true
		}
	}
	if len(statement) == 0 {
		t.Fatalf("%s has no line that names %s before %s: the idle cause order is undocumented", packageNote, orchestrator.CauseOperatorStop, orchestrator.CauseRunCap)
	}
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := stelint.LintFile(note, stelint.Options{Words: words})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if statement[f.Line] && !f.IsSkipNotice() {
			t.Errorf("%s:%d breaks the ASD-STE100 rule %s: %s", packageNote, f.Line, f.Rule, f.Message)
		}
	}
}
