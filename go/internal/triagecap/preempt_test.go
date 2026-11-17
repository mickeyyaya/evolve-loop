package triagecap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

var preemptNow = time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)

func preemptInbox(t *testing.T, items map[string]string) string {
	t.Helper()
	evolveDir := t.TempDir()
	for id, body := range items {
		fixtures.MustWrite(t, filepath.Join(evolveDir, "inbox", id+".json"), body)
	}
	return evolveDir
}

func allowAll(string) bool { return true }

func planOf(ids ...string) []FleetCandidate {
	var out []FleetCandidate
	for _, id := range ids {
		out = append(out, FleetCandidate{ID: id, Files: []string{"pkg/" + id + "/x.go"}})
	}
	return out
}

func idsOf(cs []FleetCandidate) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

func item(id string, weight, extra string) string {
	return `{"id":"` + id + `","weight":` + weight + `,"priority_class":"feature","files":["pkg/` + id + `/x.go"]` + extra + `}`
}

func TestPreemptLowestSlot_TheHigherScoredItemTakesTheLowestSlot(t *testing.T) {
	dir := preemptInbox(t, map[string]string{"low": item("low", "0.3", ""), "high": item("high", "0.8", ""), "top": item("top", "0.95", "")})

	got, p := PreemptLowestSlot(dir, planOf("low", "high"), nil, noProtectedSurface, allowAll, preemptNow)

	if p == nil || p.Old != "low" || p.New != "top" || p.NewScore <= p.OldScore {
		t.Fatalf("preemption = %+v, want low replaced by top with a higher score", p)
	}
	if want := "preempted low for top (score "; !strings.HasPrefix(p.Line(), want) {
		t.Errorf("Line = %q, want prefix %q", p.Line(), want)
	}
	if ids := idsOf(got); len(ids) != 2 || ids[0] != "top" || ids[1] != "high" {
		t.Errorf("plan = %v, want [top high]: the new item takes the slot of the old one", ids)
	}
}

func TestPreemptLowestSlot_NothingMovesWhenNothingQualifies(t *testing.T) {
	cases := map[string]struct {
		items map[string]string
		plan  []FleetCandidate
		allow func(string) bool
	}{
		"empty plan":      {items: map[string]string{"top": item("top", "0.9", "")}, plan: nil, allow: allowAll},
		"narrow lead":     {items: map[string]string{"low": item("low", "0.5", ""), "top": item("top", "0.52", "")}, plan: planOf("low"), allow: allowAll},
		"nothing outside": {items: map[string]string{"low": item("low", "0.3", ""), "high": item("high", "0.8", "")}, plan: planOf("low", "high"), allow: allowAll},
		"selector bars":   {items: map[string]string{"low": item("low", "0.3", ""), "top": item("top", "0.95", "")}, plan: planOf("low"), allow: func(id string) bool { return id != "top" }},
		"unqueued slot":   {items: map[string]string{"top": item("top", "0.95", "")}, plan: planOf("running"), allow: allowAll},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, p := PreemptLowestSlot(preemptInbox(t, tc.items), tc.plan, nil, noProtectedSurface, tc.allow, preemptNow)
			if p != nil || len(got) != len(tc.plan) {
				t.Errorf("preemption = %+v plan = %v, want the plan unchanged", p, idsOf(got))
			}
		})
	}
}

func TestPreemptLowestSlot_AnAdoptedContinuationKeepsItsSlot(t *testing.T) {
	adopted := `,"continuation":{"worktree":"/w","branch":"b","snapshot_sha":"a","base_sha":"b","findings_path":"/f","cycle":5}`
	dir := preemptInbox(t, map[string]string{"adopted": item("adopted", "0.1", adopted), "mid": item("mid", "0.5", ""), "top": item("top", "0.95", "")})

	got, p := PreemptLowestSlot(dir, planOf("adopted", "mid"), nil, noProtectedSurface, allowAll, preemptNow)

	if p == nil || p.Old != "mid" || idsOf(got)[0] != "adopted" {
		t.Errorf("preemption = %+v plan = %v, want mid displaced and adopted kept", p, idsOf(got))
	}
}

func TestPreemptLowestSlot_ACandidateOverlappingAKeptSlotIsSkipped(t *testing.T) {
	dir := preemptInbox(t, map[string]string{
		"low":   item("low", "0.3", ""),
		"high":  item("high", "0.8", ""),
		"clash": `{"id":"clash","weight":0.99,"priority_class":"feature","files":["pkg/high/x.go"]}`,
		"top":   item("top", "0.9", ""),
	})

	_, p := PreemptLowestSlot(dir, planOf("low", "high"), nil, noProtectedSurface, allowAll, preemptNow)

	if p == nil || p.New != "top" {
		t.Errorf("preemption = %+v, want top: clash shares a file with a kept slot", p)
	}
}

func TestRecordPreemption_AppendsOneChainedLedgerLine(t *testing.T) {
	dir := t.TempDir()
	p := Preemption{Old: "low", New: "top", OldScore: 0.31, NewScore: 0.95}

	if err := RecordPreemption(dir, p, preemptNow); err != nil {
		t.Fatalf("RecordPreemption: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var e map[string]any
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("ledger line: %v", err)
	}
	if e["message"] != p.Line() || e["action"] != "preempt" || e["task_id"] != "top" {
		t.Errorf("ledger entry = %v, want the preempt line for top", e)
	}
	if err := ledger.New(dir).Verify(context.Background()); err != nil {
		t.Errorf("chain: %v", err)
	}
}

func TestPreemptLowestSlot_AnItemReleasedFromAClaimKeepsItsSlot(t *testing.T) {
	dir := preemptInbox(t, map[string]string{"released": item("released", "0.1", ""), "mid": item("mid", "0.5", ""), "top": item("top", "0.95", "")})

	got, p := PreemptLowestSlot(dir, planOf("released", "mid"), map[string]bool{"released": true}, noProtectedSurface, allowAll, preemptNow)

	if p == nil || p.Old != "mid" || idsOf(got)[0] != "released" {
		t.Errorf("preemption = %+v plan = %v, want mid displaced and the released item kept", p, idsOf(got))
	}
}

func TestPreemptLowestSlot_AnUnreadableInboxLeavesThePlanUnchanged(t *testing.T) {
	dir := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(dir, "inbox"), "not a directory")

	got, p := PreemptLowestSlot(dir, planOf("low"), nil, noProtectedSurface, allowAll, preemptNow)

	if p != nil || len(got) != 1 || got[0].ID != "low" {
		t.Errorf("preemption = %+v plan = %v, want the plan unchanged", p, idsOf(got))
	}
}
