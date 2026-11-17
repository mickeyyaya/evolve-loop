package loopwave

import (
	"context"
	"math"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank/rankinputs"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type preemptItem struct {
	id     string
	weight float64
	extra  map[string]any
}

type preemptScenario struct {
	plan    []string
	items   []preemptItem
	claimed []preemptItem
	count   int
	margin  *float64
	preview bool
}

type preemptOutcome struct {
	plan   []string
	lines  []string
	scores map[string]float64
	chain  error
}

var preemptLineRe = regexp.MustCompile(`^preempted (\S+) for (\S+) \(score ([0-9]+\.[0-9]+) to ([0-9]+\.[0-9]+)\)$`)

const scoreRounding = 5e-5

func writePreemptItem(t *testing.T, dir string, it preemptItem) {
	t.Helper()
	doc := map[string]any{"id": it.id, "weight": it.weight, "priority_class": "feature", "files": []string{"pkg/" + it.id + "/x.go"}}
	for k, v := range it.extra {
		doc[k] = v
	}
	writeJSON(t, filepath.Join(dir, it.id+".json"), doc)
}

func adoptedContinuation() map[string]any {
	return map[string]any{"continuation": map[string]any{
		"worktree": "/work/tree", "branch": "cycle-1", "snapshot_sha": "abc123", "base_sha": "def456", "findings_path": "/work/findings.json", "cycle": 5,
	}}
}

func setPreemptMargin(t *testing.T, h *harness, margin float64) {
	t.Helper()
	writeJSON(t, paths.PolicyPath(h.evolveDir), map[string]any{"inbox_priority": map[string]any{"preempt_margin": margin}})
	in, _ := rankinputs.Load(h.evolveDir, time.Now())
	if in.Config.PreemptMargin != margin {
		t.Fatalf("fixture: policy preempt_margin loads as %v, want %v", in.Config.PreemptMargin, margin)
	}
}

func seedLedger(t *testing.T, evolveDir string) {
	t.Helper()
	entry := core.LedgerEntry{TS: "2026-11-17T00:00:00Z", Role: "orchestrator", Kind: "seed"}
	if err := ledger.New(evolveDir).Append(context.Background(), entry); err != nil {
		t.Fatalf("fixture: seed the ledger: %v", err)
	}
}

func preemptLedgerLines(t *testing.T, evolveDir string) []string {
	t.Helper()
	it, err := ledger.New(evolveDir).Iter(context.Background())
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	defer func() { _ = it.Close() }()
	var lines []string
	for {
		e, ok, err := it.Next()
		if err != nil {
			t.Fatalf("read a ledger entry: %v", err)
		}
		if !ok {
			return lines
		}
		if strings.HasPrefix(e.Message, "preempted ") {
			lines = append(lines, e.Message)
		}
	}
}

func inboxScores(t *testing.T, evolveDir string) map[string]float64 {
	t.Helper()
	scan, err := inboxbatch.ScanDir(filepath.Join(evolveDir, "inbox"))
	if err != nil {
		t.Fatalf("scan the inbox: %v", err)
	}
	in, _ := rankinputs.Load(evolveDir, time.Now())
	scores := map[string]float64{}
	for _, r := range in.Order(scan.Items, scan.Items) {
		scores[r.Item.ID] = r.Breakdown.Score
	}
	return scores
}

func runPreempt(t *testing.T, sc preemptScenario) preemptOutcome {
	t.Helper()
	h := newHarness(t)
	for _, it := range sc.items {
		writePreemptItem(t, filepath.Join(h.evolveDir, "inbox"), it)
	}
	for _, it := range sc.claimed {
		writePreemptItem(t, inboxbatch.ProcessingCycleDir(filepath.Join(h.evolveDir, "inbox"), 9), it)
	}
	if sc.margin != nil {
		setPreemptMargin(t, h, *sc.margin)
	}
	withPriorDecision(t, h, sc.plan...)
	engine := selectorEngine(h, Selector{})
	seedLedger(t, h.evolveDir)
	scores := inboxScores(t, h.evolveDir)

	var data []byte
	var err error
	if sc.preview {
		data, _, err = engine.planFn(sc.count, false)(context.Background(), 1)
	} else {
		data, _, err = engine.PlanFn(sc.count)(context.Background(), 1)
	}
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	var planned []string
	for id := range topNIDs(t, data) {
		planned = append(planned, id)
	}
	sort.Strings(planned)
	return preemptOutcome{
		plan:   planned,
		lines:  preemptLedgerLines(t, h.evolveDir),
		scores: scores,
		chain:  ledger.New(h.evolveDir).Verify(context.Background()),
	}
}

func requireSwap(t *testing.T, out preemptOutcome, old, replacement string) {
	t.Helper()
	if len(out.lines) != 1 {
		t.Fatalf("ledger preempt lines = %q, want exactly one for %s -> %s", out.lines, old, replacement)
	}
	m := preemptLineRe.FindStringSubmatch(out.lines[0])
	if m == nil {
		t.Fatalf("ledger line %q does not read `preempted <old> for <new> (score <a> to <b>)`", out.lines[0])
	}
	if m[1] != old || m[2] != replacement {
		t.Errorf("ledger line %q names old=%s new=%s, want old=%s new=%s", out.lines[0], m[1], m[2], old, replacement)
	}
}

func requirePlan(t *testing.T, out preemptOutcome, want ...string) {
	t.Helper()
	sort.Strings(want)
	if !reflect.DeepEqual(out.plan, want) {
		t.Errorf("planned ids = %v, want %v", out.plan, want)
	}
}

func requireNoPreemption(t *testing.T, out preemptOutcome) {
	t.Helper()
	if len(out.lines) != 0 {
		t.Errorf("ledger holds preempt lines %q, want none", out.lines)
	}
}

func requireGap(t *testing.T, out preemptOutcome, high, low string, atLeast, below float64) {
	t.Helper()
	gap := out.scores[high] - out.scores[low]
	if gap < atLeast || gap >= below {
		t.Fatalf("fixture: score(%s) - score(%s) = %.4f, want [%.4f, %.4f)", high, low, gap, atLeast, below)
	}
}

func floatPtr(v float64) *float64 { return &v }

func plainBoundaryScenario() preemptScenario {
	return preemptScenario{
		plan:  []string{"slot-low", "keep-high"},
		items: []preemptItem{{id: "slot-low", weight: 0.3}, {id: "keep-high", weight: 0.8}, {id: "fresh-top", weight: 0.95}},
		count: 2,
	}
}

func narrowLeadScenario() preemptScenario {
	return preemptScenario{
		plan:  []string{"slot-low", "keep-high"},
		items: []preemptItem{{id: "slot-low", weight: 0.5}, {id: "keep-high", weight: 0.8}, {id: "fresh-top", weight: 0.55}},
		count: 2,
	}
}

func TestPreempt_AHigherScoredItemTakesTheLowestUnstartedSlot(t *testing.T) {
	out := runPreempt(t, plainBoundaryScenario())

	requireGap(t, out, "fresh-top", "slot-low", 0.05, 1)
	requirePlan(t, out, "keep-high", "fresh-top")
	requireSwap(t, out, "slot-low", "fresh-top")
}

func TestPreempt_OnlyTheLowestScoredSlotIsReplacedWhateverItsPlace(t *testing.T) {
	sc := plainBoundaryScenario()
	sc.plan = []string{"keep-high", "slot-low"}

	out := runPreempt(t, sc)

	requirePlan(t, out, "keep-high", "fresh-top")
	requireSwap(t, out, "slot-low", "fresh-top")
}

func TestPreempt_ARunningLaneIsNeverDisplaced(t *testing.T) {
	sc := preemptScenario{
		plan:    []string{"running-low", "slot-mid", "keep-high"},
		items:   []preemptItem{{id: "slot-mid", weight: 0.4}, {id: "keep-high", weight: 0.8}, {id: "fresh-top", weight: 0.95}},
		claimed: []preemptItem{{id: "running-low", weight: 0.1}},
		count:   2,
		preview: true,
	}

	out := runPreempt(t, sc)

	requireGap(t, out, "fresh-top", "slot-mid", 0.05, 1)
	requirePlan(t, out, "keep-high", "fresh-top")
}

func TestPreempt_AnAdoptedContinuationIsNeverDisplaced(t *testing.T) {
	sc := preemptScenario{
		plan: []string{"adopted-low", "plain-mid", "keep-high"},
		items: []preemptItem{
			{id: "adopted-low", weight: 0.2, extra: adoptedContinuation()},
			{id: "plain-mid", weight: 0.5},
			{id: "keep-high", weight: 0.8},
			{id: "fresh-top", weight: 0.95},
		},
		count: 3,
	}

	out := runPreempt(t, sc)

	requireGap(t, out, "fresh-top", "adopted-low", 0.05, 1)
	requirePlan(t, out, "adopted-low", "keep-high", "fresh-top")
	requireSwap(t, out, "plain-mid", "fresh-top")
}

func TestPreempt_APlanOfOnlyAdoptedContinuationsIsNeverPreempted(t *testing.T) {
	adopted := preemptScenario{
		plan: []string{"adopted-a", "adopted-b"},
		items: []preemptItem{
			{id: "adopted-a", weight: 0.2, extra: adoptedContinuation()},
			{id: "adopted-b", weight: 0.3, extra: adoptedContinuation()},
			{id: "fresh-top", weight: 0.95},
		},
		count: 2,
	}
	control := adopted
	control.items = []preemptItem{
		{id: "adopted-a", weight: 0.2},
		{id: "adopted-b", weight: 0.3},
		{id: "fresh-top", weight: 0.95},
	}

	controlOut := runPreempt(t, control)
	out := runPreempt(t, adopted)

	requireSwap(t, controlOut, "adopted-a", "fresh-top")
	requirePlan(t, out, "adopted-a", "adopted-b")
	requireNoPreemption(t, out)
}

func TestPreempt_TheLedgerRecordsTheOldAndTheNewItemWithBothScores(t *testing.T) {
	out := runPreempt(t, plainBoundaryScenario())

	requireSwap(t, out, "slot-low", "fresh-top")
	m := preemptLineRe.FindStringSubmatch(out.lines[0])
	oldScore, _ := strconv.ParseFloat(m[3], 64)
	newScore, _ := strconv.ParseFloat(m[4], 64)
	if math.Abs(oldScore-out.scores["slot-low"]) > scoreRounding {
		t.Errorf("ledger old score = %v, want the rank score %v of slot-low", oldScore, out.scores["slot-low"])
	}
	if math.Abs(newScore-out.scores["fresh-top"]) > scoreRounding {
		t.Errorf("ledger new score = %v, want the rank score %v of fresh-top", newScore, out.scores["fresh-top"])
	}
	if out.chain != nil {
		t.Errorf("the hash chain is broken by the preempt line: %v", out.chain)
	}
}

func TestPreempt_ALeadBelowThePreemptMarginDoesNotPreempt(t *testing.T) {
	control := plainBoundaryScenario()
	narrow := narrowLeadScenario()

	controlOut := runPreempt(t, control)
	out := runPreempt(t, narrow)

	requireSwap(t, controlOut, "slot-low", "fresh-top")
	requireGap(t, out, "fresh-top", "slot-low", 0.001, 0.05)
	requirePlan(t, out, "slot-low", "keep-high")
	requireNoPreemption(t, out)
}

func TestPreempt_TheMarginComesFromThePolicyNotAConstant(t *testing.T) {
	wide := plainBoundaryScenario()
	wide.margin = floatPtr(0.5)
	zero := narrowLeadScenario()
	zero.margin = floatPtr(0)

	zeroOut := runPreempt(t, zero)
	wideOut := runPreempt(t, wide)

	requireSwap(t, zeroOut, "slot-low", "fresh-top")
	requireGap(t, wideOut, "fresh-top", "slot-low", 0.05, 0.5)
	requirePlan(t, wideOut, "slot-low", "keep-high")
	requireNoPreemption(t, wideOut)
}

func TestPreempt_AnItemTheLanesCannotTakeNeverPreempts(t *testing.T) {
	sc := preemptScenario{
		plan: []string{"slot-low", "keep-high"},
		items: []preemptItem{
			{id: "slot-low", weight: 0.3},
			{id: "keep-high", weight: 0.8},
			{id: "waiting-top", weight: 0.99, extra: map[string]any{"deps": []string{"keep-high"}}},
			{id: "protected-top", weight: 0.99, extra: map[string]any{"files": []string{"protected/x.go"}}},
			{id: "eligible-top", weight: 0.9},
		},
		count: 2,
	}

	out := runPreempt(t, sc)

	requirePlan(t, out, "keep-high", "eligible-top")
	requireSwap(t, out, "slot-low", "eligible-top")
}

func TestPreempt_APreviewPlansTheSwapButWritesNoLedgerLine(t *testing.T) {
	sc := plainBoundaryScenario()
	sc.preview = true

	out := runPreempt(t, sc)

	requirePlan(t, out, "keep-high", "fresh-top")
	requireNoPreemption(t, out)
}

func TestPreempt_ABacklogWithNothingOutsideThePlanLeavesPlanAndLedgerUntouched(t *testing.T) {
	control := plainBoundaryScenario()
	sc := preemptScenario{
		plan:  []string{"slot-low", "keep-high"},
		items: []preemptItem{{id: "slot-low", weight: 0.3}, {id: "keep-high", weight: 0.8}},
		count: 2,
	}

	controlOut := runPreempt(t, control)
	out := runPreempt(t, sc)

	requireSwap(t, controlOut, "slot-low", "fresh-top")
	requirePlan(t, out, "slot-low", "keep-high")
	requireNoPreemption(t, out)
}

func TestPreempt_AnEmptyPriorPlanDoesNotPanicOrPreempt(t *testing.T) {
	control := plainBoundaryScenario()
	h := newHarness(t)
	for _, it := range []preemptItem{{id: "alpha-ready", weight: 0.9}, {id: "bravo-ready", weight: 0.8}} {
		writePreemptItem(t, filepath.Join(h.evolveDir, "inbox"), it)
	}
	withPriorDecision(t, h)
	engine := selectorEngine(h, Selector{})
	seedLedger(t, h.evolveDir)

	controlOut := runPreempt(t, control)
	_, _, err := engine.PlanFn(2)(context.Background(), 1)

	requireSwap(t, controlOut, "slot-low", "fresh-top")
	if err != nil {
		t.Fatalf("plan over an empty prior decision: %v", err)
	}
	if lines := preemptLedgerLines(t, h.evolveDir); len(lines) != 0 {
		t.Errorf("an empty prior plan holds no slot to take, ledger holds %q", lines)
	}
}
