package inboxmover

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

func TestPromoter_Promote_AFailedRenameKeepsTheItemDeliveredAndReturnsErrMvFailed(t *testing.T) {
	t.Parallel()
	fx := newPromoteFixture(t, map[string]string{"item-a": StateDelivered}, "item-a")
	blocking := filepath.Join(fx.inbox, StateProcessed, "cycle-41", "2026-11-26T00-00-00Z-item-a.json", "keep")
	if err := os.MkdirAll(blocking, 0o755); err != nil {
		t.Fatal(err)
	}

	err := (Promoter{Options: Options{ProjectRoot: fx.root}}).Promote(context.Background(), integration.DeliverableRecord{Cycle: promoteCycle})

	if !errors.Is(err, ErrMvFailed) {
		t.Fatalf("Promote = %v, want ErrMvFailed", err)
	}
	if !fx.exists("delivered/2026-11-26T00-00-00Z-item-a.json") {
		t.Errorf("the item left delivered/ although the rename failed")
	}
}
