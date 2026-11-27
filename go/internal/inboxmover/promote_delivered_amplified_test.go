package inboxmover

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

func TestPromoterAmplified_ManyDeliveredItemsAllMoveUnderTheCycleDirectory(t *testing.T) {
	t.Parallel()
	items := map[string]string{"a": StateDelivered, "b": StateDelivered, "c": StateDelivered}
	fx := newPromoteFixture(t, items, "a", "b", "c")
	recLedger := &recordingAppender{}

	err := (Promoter{Options: Options{ProjectRoot: fx.root, Ledger: recLedger}}).Promote(context.Background(), integration.DeliverableRecord{Cycle: promoteCycle})

	if err != nil {
		t.Fatalf("Promote = %v", err)
	}
	files := fx.processedFiles(t)
	if len(files) != 3 {
		t.Fatalf("processed files = %v, want 3", files)
	}
	for _, f := range files {
		if !strings.Contains(f, "cycle-41") {
			t.Errorf("processed file %q is not under cycle-41", f)
		}
	}
	if len(recLedger.records) != 3 {
		t.Errorf("ledger records = %d, want 3", len(recLedger.records))
	}
}

func TestPromoterAmplified_AnIdWithNoFileOnDiskIsSkippedWithoutError(t *testing.T) {
	t.Parallel()
	fx := newPromoteFixture(t, map[string]string{"real": StateDelivered}, "ghost", "real")

	err := (Promoter{Options: Options{ProjectRoot: fx.root}}).Promote(context.Background(), integration.DeliverableRecord{Cycle: promoteCycle})

	if err != nil {
		t.Fatalf("Promote = %v", err)
	}
	files := fx.processedFiles(t)
	if len(files) != 1 || !strings.HasSuffix(files[0], "real.json") {
		t.Errorf("processed files = %v, want only real.json", files)
	}
}

func TestPromoterAmplified_ARecordWithNoIdsMovesNothing(t *testing.T) {
	t.Parallel()
	fx := newPromoteFixture(t, map[string]string{"stray": StateDelivered})

	err := (Promoter{Options: Options{ProjectRoot: fx.root}}).Promote(context.Background(), integration.DeliverableRecord{Cycle: promoteCycle})

	if err != nil {
		t.Fatalf("Promote = %v", err)
	}
	if got := len(fx.processedFiles(t)); got != 0 {
		t.Errorf("processed files = %d, want 0", got)
	}
	if !fx.exists("delivered/2026-11-26T00-00-00Z-stray.json") {
		t.Errorf("an unlisted delivered item was moved")
	}
}

func TestPromoterAmplified_APromotedItemKeepsItsContent(t *testing.T) {
	t.Parallel()
	fx := newPromoteFixture(t, map[string]string{"keep": StateDelivered}, "keep")

	if err := (Promoter{Options: Options{ProjectRoot: fx.root}}).Promote(context.Background(), integration.DeliverableRecord{Cycle: promoteCycle}); err != nil {
		t.Fatalf("Promote = %v", err)
	}

	files := fx.processedFiles(t)
	if len(files) != 1 {
		t.Fatalf("processed files = %v", files)
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"id":"keep"}` {
		t.Errorf("content = %q, want the original bytes", string(b))
	}
}
