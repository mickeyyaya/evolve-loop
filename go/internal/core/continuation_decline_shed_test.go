package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func TestRunCycle_ARejectedAdoptionLeavesNoContinuationOnTheItem(t *testing.T) {
	t.Parallel()
	root, wt := initContinuationRepo(t, 86)
	m := stampedContinuation(t, root, wt, 86)
	m.SnapshotSHA = "2222222222222222222222222222222222222222"
	seedStampedItemDirect(t, root, m, "task-a")
	runners := buildRunners(nil)
	runners[PhaseTriage] = &claimingTriageRunner{fakeRunner: runners[PhaseTriage].(*fakeRunner), root: root, taskID: "task-a"}
	var stderr bytes.Buffer
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithStderr(&stderr),
		WithContinuationResolver(productionResolver(t)), WithContinuationShedder(RealInboxForTest.Shed))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	claimed := filepath.Join(root, ".evolve", "inbox", "processing", cycleDirName(res.Cycle), "task-a.json")
	body, rerr := os.ReadFile(claimed)
	if rerr != nil {
		t.Fatalf("the claimed item: %v", rerr)
	}
	var item map[string]json.RawMessage
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	if _, kept := item["continuation"]; kept {
		t.Errorf("a rejected adoption must leave no continuation block on the item: %s", body)
	}
	if !strings.Contains(stderr.String(), `SHED the declined continuation pointer of "task-a"`) {
		t.Errorf("the shed must be logged, got:\n%s", stderr.String())
	}
}

func TestShedDeclinedPointer_AShedderFaultIsAWarning(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(&stderr),
		WithContinuationShedder(func(string, int, continuation.Continuation) ([]string, error) {
			return nil, errors.New("disk full")
		}))

	o.shedDeclinedPointer("/root", 86, continuation.Continuation{SnapshotSHA: "2222222222222222", Cycle: 84})

	if !strings.Contains(stderr.String(), "WARN cycle 86 continuation: the declined pointer of cycle 84 was not shed: disk full") {
		t.Errorf("a shed fault must be a WARN naming the cycles, got:\n%s", stderr.String())
	}
	NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)).shedDeclinedPointer("/root", 86, continuation.Continuation{})
}

func TestContinuationShedProbe_ReportsWhetherAShedderIsWired(t *testing.T) {
	t.Parallel()
	if _, wired, _ := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)).ContinuationShedProbe("/r", 1, continuation.Continuation{}); wired {
		t.Error("no shedder option means not wired")
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithContinuationShedder(func(string, int, continuation.Continuation) ([]string, error) {
		return []string{"a"}, errors.New("partial")
	}))
	if shed, wired, err := o.ContinuationShedProbe("/r", 1, continuation.Continuation{}); !wired || len(shed) != 1 || err == nil {
		t.Errorf("shed = %v, wired = %v, err = %v; the probe reports the shedder fault", shed, wired, err)
	}
}
