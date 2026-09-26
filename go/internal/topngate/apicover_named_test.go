package topngate

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestNewReviewer_Named(t *testing.T) {
	t.Parallel()

	var enforce core.DeliverableReviewer = NewReviewer(config.StageEnforce)
	if enforce == nil {
		t.Fatal("NewReviewer(StageEnforce) must return a non-nil core.DeliverableReviewer")
	}

	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
	in := core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws}

	if res := enforce.Review(context.Background(), in); !res.Approve {
		t.Errorf("enforce reviewer approves label drift (advisory); got reason=%q", res.Reason)
	}

	var shadow core.DeliverableReviewer = NewReviewer(config.StageShadow)
	if res := shadow.Review(context.Background(), in); !res.Approve {
		t.Errorf("shadow reviewer must approve (log-only) the same violation; got reason=%q", res.Reason)
	}
}
