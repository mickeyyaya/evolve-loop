package dashboard

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func writeInboxItem(t *testing.T, dir, name, body string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, name), body)
}

func TestReadQueue_PendingInRankOrderAndLifecycleCounts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeInboxItem(t, inbox, "a.json", `{"id":"low","title":"Low","weight":0.5,"kind":"feature"}`)
	writeInboxItem(t, inbox, "b.json", `{"id":"high","title":"High","weight":0.9,"kind":"pipeline-repair","route":"console-only","priority":"P1","class":"pipeline-architecture"}`)
	writeInboxItem(t, filepath.Join(inbox, "consumed"), "c1.json", `{"id":"c1"}`)
	writeInboxItem(t, filepath.Join(inbox, "consumed"), "c2.json", `{"id":"c2"}`)
	writeInboxItem(t, filepath.Join(inbox, "processing"), "p.json", `{"id":"p"}`)
	writeInboxItem(t, filepath.Join(inbox, "retry"), "r.json", `{"id":"r"}`)
	writeInboxItem(t, filepath.Join(inbox, "processed"), "d.json", `{"id":"d"}`)
	writeInboxItem(t, filepath.Join(inbox, "delivered"), "dl.json", `{"id":"dl"}`)
	writeInboxItem(t, filepath.Join(inbox, "rejected"), "rj.json", `{"id":"rj"}`)
	writeInboxItem(t, filepath.Join(inbox, "quarantine"), "q.json", `{"id":"q"}`)
	writeInboxItem(t, filepath.Join(inbox, "consumed"), "notes.md", `not an item`)

	q, warns := readQueue(root, time.Now())
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if len(q.Pending) != 2 || q.Pending[0].ID != "high" || q.Pending[1].ID != "low" {
		t.Fatalf("Pending = %+v, want high before low", q.Pending)
	}
	hi := q.Pending[0]
	if hi.Route != "console-only" || hi.Priority != "P1" || hi.Class != "pipeline-architecture" || hi.Kind != "pipeline-repair" {
		t.Fatalf("modelled fields lost: %+v", hi)
	}
	if q.Consumed != 2 || q.Processing != 1 || q.Retry != 1 || q.Processed != 1 || q.Delivered != 1 || q.Rejected != 1 || q.Quarantine != 1 {
		t.Fatalf("lifecycle counts = %+v", q)
	}
}

func TestReadQueue_MalformedItemIsWarnedNotFatal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeInboxItem(t, inbox, "ok.json", `{"id":"ok","title":"t","weight":0.7}`)
	writeInboxItem(t, inbox, "bad.json", `{`)
	q, warns := readQueue(root, time.Now())
	if len(q.Pending) != 1 || len(warns) != 1 {
		t.Fatalf("pending=%d warns=%v", len(q.Pending), warns)
	}
}

func TestReadQueue_MissingInboxIsEmpty(t *testing.T) {
	t.Parallel()
	q, warns := readQueue(t.TempDir(), time.Now())
	if len(q.Pending) != 0 || q.Consumed != 0 || q.Delivered != 0 || len(warns) != 0 {
		t.Fatalf("empty: %+v %v", q, warns)
	}
}

func TestReadQueue_EveryLifecycleStateHasACount(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	states := inboxmover.LifecycleStates()
	for i, state := range states {
		for j := 0; j < i+1; j++ {
			id := fmt.Sprintf("%s-%d", state, j)
			writeInboxItem(t, filepath.Join(inbox, state), id+".json", `{"id":"`+id+`"}`)
		}
	}
	q, warns := readQueue(root, time.Now())
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	raw, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	var counts map[string]json.RawMessage
	if err := json.Unmarshal(raw, &counts); err != nil {
		t.Fatal(err)
	}
	for i, state := range states {
		val, ok := counts[state]
		if !ok {
			t.Fatalf("QueueSummary JSON %s lacks field for lifecycle state %q", raw, state)
		}
		var n int
		if err := json.Unmarshal(val, &n); err != nil || n != i+1 {
			t.Errorf("state %q count = %s (err=%v), want %d", state, val, err, i+1)
		}
	}
}

func TestFingerprint_TracksDeliveredDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	before := fingerprint(root)
	writeInboxItem(t, filepath.Join(root, ".evolve", "inbox", inboxmover.StateDelivered), "item.json", `{"id":"item"}`)
	after := fingerprint(root)
	if after == before {
		t.Fatal("fingerprint did not change when a delivered item was added")
	}
}
