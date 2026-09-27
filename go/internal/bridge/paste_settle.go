package bridge

import (
	"context"
	"fmt"
	"time"
)

const (
	pasteSettleBase     = time.Second
	pasteSettlePer10KB  = 1500 * time.Millisecond
	pasteSettleMax      = 6 * time.Second
	pasteSettleMaxBytes = 50_000

	pasteStabilityPoll     = 500 * time.Millisecond
	pasteStabilityMaxPolls = 8
	pasteStabilityMinBytes = 2_000
)

const (
	pasteStable       = "stable"
	pasteUnstable     = "unstable"
	pasteSkipped      = "skipped"
	pasteCaptureFault = "capture_fault"
)

type pasteOutcome struct {
	Settle    time.Duration
	Stability string
}

func pasteSettleFor(size int) time.Duration {
	d := pasteSettleBase + time.Duration(size/10_000)*pasteSettlePer10KB
	if d > pasteSettleMax {
		d = pasteSettleMax
	}
	return d
}

// pfx is the per-driver console prefix used in the stderr messages below.
func settlePasteThenEnter(ctx context.Context, deps Deps, pfx, session string, settle time.Duration, size int) (pasteOutcome, error) {
	out := pasteOutcome{Settle: settle, Stability: pasteSkipped}
	deps.Sleep(settle)
	if size >= pasteStabilityMinBytes {
		out.Stability = waitPaneStable(ctx, deps, pfx, session)
	}
	return out, deps.Tmux.SendKeys(ctx, session, "", true)
}

// waitPaneStable captures scrollback 0 on purpose: history never changes and
// would only mask motion at the input line.
func waitPaneStable(ctx context.Context, deps Deps, pfx, session string) string {
	captureFault := func(err error) {
		fmt.Fprintf(deps.Stderr, "%s paste settle: capture failed, pane state unknown — pressing Enter anyway: %v\n", pfx, err)
	}
	prev, err := deps.Tmux.CapturePane(ctx, session, 0)
	if err != nil {
		captureFault(err)
		return pasteCaptureFault
	}
	for polls := 0; polls < pasteStabilityMaxPolls; polls++ {
		deps.Sleep(pasteStabilityPoll)
		next, err := deps.Tmux.CapturePane(ctx, session, 0)
		if err != nil {
			captureFault(err)
			return pasteCaptureFault
		}
		if next == prev {
			return pasteStable
		}
		prev = next
	}
	fmt.Fprintf(deps.Stderr, "%s paste settle: pane still changing after %d polls — pressing Enter anyway\n", pfx, pasteStabilityMaxPolls)
	return pasteUnstable
}
