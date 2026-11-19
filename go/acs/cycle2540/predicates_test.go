//go:build acs

package cycle2540

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	logTest      = "TestNewLaneLeases_TheLogPrintsTheDeadlineOfBothBases"
	laneFloor    = 10 * time.Minute
	runsWindow   = 50
	shippedSpan  = 40 * time.Minute
	failedSpan   = 6 * time.Hour
	policyPrefix = `{"fleet":{"orchestrator":{"deadline_basis":`
)

type lane struct {
	span    time.Duration
	phase   string
	verdict string
}

func shippedLane(span time.Duration) lane { return lane{span, "ship", "PASS"} }
func failedLane(span time.Duration) lane  { return lane{span, "ship", "FAIL"} }

func writeRuns(t *testing.T, lanes ...lane) string {
	t.Helper()
	runs := t.TempDir()
	start := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	for i, l := range lanes {
		cycle := i + 1
		events := []signalcenter.Event{
			{Seq: 1, TS: start.Format(time.RFC3339Nano), Cycle: cycle, Module: signalcenter.ModuleLoop, Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo},
			{Seq: 2, TS: start.Add(l.span).Format(time.RFC3339Nano), Cycle: cycle, Phase: l.phase, Module: signalcenter.ModuleOrchestrator, Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityInfo, Fields: map[string]string{"verdict": l.verdict}},
		}
		var lines []string
		for _, e := range events {
			b, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, string(b))
		}
		dir := filepath.Join(runs, "cycle-"+strconv.Itoa(cycle))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, signalcenter.StreamFileName), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return runs
}

func deadlineOf(t *testing.T, runs, basis string, floor time.Duration) time.Duration {
	t.Helper()
	lanes, err := orchestrator.LaneRuns(runs, runsWindow)
	if err != nil {
		t.Fatalf("LaneRuns: %v", err)
	}
	return orchestrator.DeadlineByBasis(lanes, basis, floor)
}

func TestC2540_001_ShippedBasisDeadlineCountsOnlyShippedLanes(t *testing.T) {
	for _, failed := range []time.Duration{failedSpan, 9 * time.Hour} {
		runs := writeRuns(t,
			shippedLane(shippedSpan), shippedLane(shippedSpan), shippedLane(shippedSpan),
			failedLane(failed), lane{failed, "audit", "PASS"})
		if got := deadlineOf(t, runs, "shipped", laneFloor); got != shippedSpan {
			t.Errorf("failed lanes of %v: shipped deadline = %v, want %v from the three shipped lanes", failed, got, shippedSpan)
		}
	}
}

func TestC2540_002_AllBasisDeadlineIsTheLegacyDeadlineOverEveryLane(t *testing.T) {
	runs := writeRuns(t,
		shippedLane(shippedSpan), shippedLane(shippedSpan), shippedLane(shippedSpan),
		failedLane(failedSpan), failedLane(failedSpan))
	times, err := orchestrator.LaneTimes(runs, runsWindow)
	if err != nil {
		t.Fatal(err)
	}
	legacy := orchestrator.DeadlineOf(times, laneFloor)
	if legacy != failedSpan {
		t.Fatalf("legacy deadline = %v, want %v: the fixture no longer separates the two bases", legacy, failedSpan)
	}
	if got := deadlineOf(t, runs, "all", laneFloor); got != legacy {
		t.Errorf("all-basis deadline = %v, want the legacy %v", got, legacy)
	}
}

func TestC2540_003_TheLoopLogLinePrintsTheDeadlineOfBothBases(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", "^"+logTest+"$", "./cmd/evolve")
	cmd.Dir = acsassert.GoModuleDir(t)
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: "+logTest) {
		t.Errorf("%s did not pass (err=%v):\n%s", logTest, err, acsassert.OutputTail(string(out), 25))
	}
}

func TestC2540_004_ShippedBasisWithNoShippedLaneTakesTheFloor(t *testing.T) {
	floor := time.Hour
	cases := map[string][]lane{
		"only failed lanes":              {failedLane(failedSpan), failedLane(failedSpan), lane{failedSpan, "audit", "PASS"}},
		"a shipped lane under the floor": {shippedLane(20 * time.Minute), failedLane(failedSpan)},
	}
	for name, lanes := range cases {
		if got := deadlineOf(t, writeRuns(t, lanes...), "shipped", floor); got != floor {
			t.Errorf("%s: shipped deadline = %v, want the %v floor", name, got, floor)
		}
	}
	if got := orchestrator.DeadlineByBasis(nil, "shipped", floor); got != floor {
		t.Errorf("no history: shipped deadline = %v, want the %v floor", got, floor)
	}
}

func TestC2540_005_TheDeadlineBasisDefaultsToAllAndAcceptsShipped(t *testing.T) {
	unset, err := policy.Parse([]byte(`{"fleet":{"orchestrator":{"stage":"enforce"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := unset.FleetConfig().Orchestrator.DeadlineBasis; got != "all" {
		t.Errorf("unset DeadlineBasis = %q, want all", got)
	}
	for _, basis := range []string{"all", "shipped"} {
		p, err := policy.Parse([]byte(policyPrefix + `"` + basis + `"}}}`))
		if err != nil {
			t.Fatalf("%s: %v", basis, err)
		}
		if got := p.FleetConfig().Orchestrator.DeadlineBasis; got != basis {
			t.Errorf("DeadlineBasis = %q, want %q", got, basis)
		}
	}
}

func TestC2540_006_TheDeadlineBasisRefusesAnyOtherValue(t *testing.T) {
	for _, raw := range []string{`"p99"`, `"Shipped"`, `" all"`, `"all,shipped"`, `"unshipped"`} {
		_, err := policy.Parse([]byte(policyPrefix + raw + `}}}`))
		if !errors.Is(err, policy.ErrDeadlineBasis) {
			t.Errorf("deadline_basis %s: err = %v, want policy.ErrDeadlineBasis", raw, err)
		}
	}
	for _, raw := range []string{`5`, `true`} {
		_, err := policy.Parse([]byte(policyPrefix + raw + `}}}`))
		var typeErr *json.UnmarshalTypeError
		if !errors.As(err, &typeErr) || typeErr.Field != "deadline_basis" {
			t.Errorf("deadline_basis %s: err = %v, want a json type error on the field deadline_basis", raw, err)
		}
	}
}
