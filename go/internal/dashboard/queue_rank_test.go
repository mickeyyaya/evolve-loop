package dashboard

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadQueue_PendingFollowsTheInboxRankAndCarriesEachScore(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeInboxItem(t, inbox, "a.json", `{"id":"heavier-security","weight":0.6,"priority_class":"security"}`)
	writeInboxItem(t, inbox, "b.json", `{"id":"lighter-correctness","weight":0.55,"priority_class":"correctness"}`)
	q, warns := readQueue(root, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if len(q.Pending) != 2 || q.Pending[0].ID != "lighter-correctness" || math.Abs(q.Pending[0].Score-0.4475) > 1e-9 || math.Abs(q.Pending[1].Score-0.295) > 1e-9 {
		t.Errorf("Pending = %+v, want the rank's order with each score", q.Pending)
	}
}

func TestReadQueue_AnUnreadableLedgerIsAWarning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeInboxItem(t, filepath.Join(root, ".evolve", "inbox"), "a.json", `{"id":"a","weight":0.5}`)
	writeFile(t, filepath.Join(root, ".evolve", "recurrence-ledger.json"), `{not json`)
	q, warns := readQueue(root, time.Now())
	if len(q.Pending) != 1 || len(warns) != 1 || !strings.HasPrefix(warns[0], "inbox rank: recurrence ledger unreadable") {
		t.Errorf("pending=%d warns=%v", len(q.Pending), warns)
	}
}
