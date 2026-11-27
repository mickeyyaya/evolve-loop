package inboxmover

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const promoteCycle = 41

type promoteFixture struct {
	root      string
	inbox     string
	delivered string
}

func newPromoteFixture(t *testing.T, items map[string]string, recordIDs ...string) promoteFixture {
	t.Helper()
	root := t.TempDir()
	fx := promoteFixture{root: root, inbox: filepath.Join(root, ".evolve", "inbox")}
	fx.delivered = filepath.Join(fx.inbox, StateDelivered)
	if err := os.MkdirAll(fx.delivered, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, dir := range items {
		target := filepath.Join(fx.inbox, dir)
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "2026-11-26T00-00-00Z-"+id+".json"), []byte(`{"id":"`+id+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: promoteCycle, ItemIDs: recordIDs}); err != nil {
		t.Fatal(err)
	}
	return fx
}

func (fx promoteFixture) processedFiles(t *testing.T) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(filepath.Join(fx.inbox, "processed"), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func (fx promoteFixture) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(fx.inbox, rel))
	return err == nil
}

func TestPromoter_Promote_MovesTheDeliveredItemOfAMergedMemberToProcessed(t *testing.T) {
	t.Parallel()
	fx := newPromoteFixture(t, map[string]string{"item-a": StateDelivered}, "item-a")
	recLedger := &recordingAppender{}
	var stderr bytes.Buffer
	var port integration.Promoter = Promoter{Options: Options{ProjectRoot: fx.root, Ledger: recLedger, Stderr: &stderr}}

	err := port.Promote(context.Background(), integration.DeliverableRecord{Cycle: promoteCycle})

	if err != nil {
		t.Fatalf("Promote = %v", err)
	}
	if fx.exists("delivered/2026-11-26T00-00-00Z-item-a.json") {
		t.Errorf("the item still sits in delivered/ after its member merged")
	}
	files := fx.processedFiles(t)
	if len(files) != 1 || !strings.Contains(files[0], "cycle-41") || !strings.HasSuffix(files[0], "item-a.json") {
		t.Fatalf("processed files = %v, want one item-a.json under processed/cycle-41/", files)
	}
	if len(recLedger.records) != 1 || recLedger.records[0].Action != "promote" || recLedger.records[0].TaskID != "item-a" || recLedger.records[0].Cycle != promoteCycle {
		t.Errorf("ledger = %+v, want one promote record of item-a for cycle 41", recLedger.records)
	}
}

func TestPromoter_Promote_ADeferredIdThatNeverLeftTheInboxStaysOpen(t *testing.T) {
	t.Parallel()
	fx := newPromoteFixture(t,
		map[string]string{"item-a": StateDelivered, "item-deferred": ".", "item-other-cycle": StateDelivered},
		"item-a", "item-deferred")

	err := (Promoter{Options: Options{ProjectRoot: fx.root}}).Promote(context.Background(), integration.DeliverableRecord{Cycle: promoteCycle})

	if err != nil {
		t.Fatalf("Promote = %v", err)
	}
	if !fx.exists("2026-11-26T00-00-00Z-item-deferred.json") {
		t.Errorf("the deferred id left the inbox root: it must stay open")
	}
	if !fx.exists("delivered/2026-11-26T00-00-00Z-item-other-cycle.json") {
		t.Errorf("a delivered item that the record of cycle 41 does not name was promoted")
	}
	files := fx.processedFiles(t)
	if len(files) != 1 || !strings.HasSuffix(files[0], "item-a.json") {
		t.Errorf("processed files = %v, want only item-a", files)
	}
}

func TestPromoter_Promote_AnIdListedTwiceMovesOnceAndASecondCallIsANoOp(t *testing.T) {
	t.Parallel()
	fx := newPromoteFixture(t, map[string]string{"item-a": StateDelivered}, "item-a", "item-a")
	recLedger := &recordingAppender{}
	promoter := Promoter{Options: Options{ProjectRoot: fx.root, Ledger: recLedger}}

	for call := 1; call <= 2; call++ {
		if err := promoter.Promote(context.Background(), integration.DeliverableRecord{Cycle: promoteCycle}); err != nil {
			t.Fatalf("Promote call %d = %v", call, err)
		}
	}

	if got := len(fx.processedFiles(t)); got != 1 {
		t.Errorf("processed files = %d, want 1", got)
	}
	if len(recLedger.records) != 1 {
		t.Errorf("ledger records = %d, want 1", len(recLedger.records))
	}
}

func TestPromoter_Promote_NoOpsAndErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	if err := (Promoter{}).Promote(ctx, integration.DeliverableRecord{Cycle: promoteCycle}); !errors.Is(err, errNoProjectRoot) {
		t.Errorf("empty Options = %v, want errNoProjectRoot", err)
	}

	fx := newPromoteFixture(t, map[string]string{"item-a": StateDelivered}, "item-a")
	promoter := Promoter{Options: Options{ProjectRoot: fx.root}}
	if err := promoter.Promote(ctx, integration.DeliverableRecord{Cycle: 0}); err != nil {
		t.Errorf("cycle 0 = %v, want nil", err)
	}
	if err := promoter.Promote(ctx, integration.DeliverableRecord{Cycle: 99}); err != nil {
		t.Errorf("a cycle with no lineage record = %v, want nil", err)
	}
	if !fx.exists("delivered/2026-11-26T00-00-00Z-item-a.json") {
		t.Errorf("a no-op call moved the item")
	}
}

func TestPromoter_Promote_AFailedMoveKeepsTheItemDeliveredAndReturnsErrMvFailed(t *testing.T) {
	t.Parallel()
	fx := newPromoteFixture(t, map[string]string{"item-a": StateDelivered}, "item-a")
	if err := os.MkdirAll(filepath.Join(fx.inbox, "processed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fx.inbox, "processed", "cycle-41"), []byte("a file where the cycle directory belongs"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := (Promoter{Options: Options{ProjectRoot: fx.root}}).Promote(context.Background(), integration.DeliverableRecord{Cycle: promoteCycle})

	if !errors.Is(err, ErrMvFailed) {
		t.Fatalf("Promote = %v, want ErrMvFailed", err)
	}
	if !fx.exists("delivered/2026-11-26T00-00-00Z-item-a.json") {
		t.Errorf("the item left delivered/ although the move failed")
	}
}

func TestPromoter_Promote_AFailedLedgerAppendWarnsAndStillPromotes(t *testing.T) {
	t.Parallel()
	fx := newPromoteFixture(t, map[string]string{"item-a": StateDelivered}, "item-a")
	var stderr bytes.Buffer

	err := (Promoter{Options: Options{ProjectRoot: fx.root, Ledger: failingAppender{}, Stderr: &stderr}}).Promote(context.Background(), integration.DeliverableRecord{Cycle: promoteCycle})

	if err != nil {
		t.Fatalf("Promote = %v, want the ledger failure to warn only", err)
	}
	if !strings.Contains(stderr.String(), "ledger append") {
		t.Errorf("stderr = %q, want a ledger append warning", stderr.String())
	}
	if got := len(fx.processedFiles(t)); got != 1 {
		t.Errorf("processed files = %d, want the item promoted", got)
	}
}
