package dashboard

import (
	"path/filepath"
	"testing"
)

func writeInboxItem(t *testing.T, dir, name, body string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, name), body)
}

func TestReadQueue_PendingSortedByWeightAndLifecycleCounts(t *testing.T) {
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
	writeInboxItem(t, filepath.Join(inbox, "consumed"), "notes.md", `not an item`)

	q, warns := readQueue(root)
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
	if q.Consumed != 2 || q.Processing != 1 || q.Retry != 1 || q.Processed != 1 {
		t.Fatalf("lifecycle counts = %+v", q)
	}
}

func TestReadQueue_MalformedItemIsWarnedNotFatal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeInboxItem(t, inbox, "ok.json", `{"id":"ok","title":"t","weight":0.7}`)
	writeInboxItem(t, inbox, "bad.json", `{`)
	q, warns := readQueue(root)
	if len(q.Pending) != 1 || len(warns) != 1 {
		t.Fatalf("pending=%d warns=%v", len(q.Pending), warns)
	}
}

func TestReadQueue_MissingInboxIsEmpty(t *testing.T) {
	t.Parallel()
	q, warns := readQueue(t.TempDir())
	if len(q.Pending) != 0 || q.Consumed != 0 || len(warns) != 0 {
		t.Fatalf("empty: %+v %v", q, warns)
	}
}
