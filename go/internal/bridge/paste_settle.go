package bridge

// paste_settle.go — the ONE home of "paste, let the TUI ingest it, press
// Enter": the prompt paste (human_input.go) and the mid-turn inject
// (tmux_inject.go) both deliver through settlePasteThenEnter, so the timing
// belief the 2026-09-14 wedge incident corrected cannot survive in a second
// copy (docs/incidents/2026-09-14-codex-prompt-submit-wedge.md).
//
// A multi-KB bracketed paste is ingested by the TUI for seconds; an Enter that
// lands mid-ingestion is swallowed into the paste and the submission is later
// declared wedged. So: settle by size, wait for the pane to stop changing,
// then Enter — each bounded, each said out loud when it gives up, and the
// evidence returned so the submit-verify ledger records it on the SUCCESS path
// too (a recovered stall is a rate only with a denominator).

import (
	"context"
	"fmt"
	"time"
)

const (
	// pasteSettleBase / pasteSettlePer10KB / pasteSettleMax size the pause
	// between the paste and the first Enter: the old fixed second plus 1.5 s
	// per 10 KB, capped. 1.5 s — not 1 s — so no size lands on exactly the
	// artifactWaitInterval the wedge short-circuit pins count by value
	// (TestPasteTiming_NeverEqualsTheArtifactWaitInterval).
	pasteSettleBase    = time.Second
	pasteSettlePer10KB = 1500 * time.Millisecond
	pasteSettleMax     = 6 * time.Second
	// pasteSettleMaxBytes is the size an unreadable prompt is assumed to have:
	// it maps to pasteSettleMax and sits over the stability floor.
	pasteSettleMaxBytes = 50_000
	// pasteStabilityPoll / pasteStabilityMaxPolls bound the wait for the pane to
	// stop changing before that Enter — the observable "ingestion finished".
	pasteStabilityPoll     = 500 * time.Millisecond
	pasteStabilityMaxPolls = 8
	// pasteStabilityMinBytes: below it the delivery is byte-for-byte the old
	// one (a 1 s settle, no capture) — the no-regression floor every existing
	// fixture sits under; the stability poll is for the multi-KB phase prompts.
	pasteStabilityMinBytes = 2_000
)

// The stability vocabulary the ledger records.
const (
	pasteStable       = "stable"        // two consecutive captures agreed
	pasteUnstable     = "unstable"      // still changing at the poll cap — Enter pressed anyway
	pasteSkipped      = "skipped"       // under the floor — no capture taken
	pasteCaptureFault = "capture_fault" // tmux could not be read — Enter pressed anyway
)

// pasteOutcome is what the delivery learned: the settle it slept and how the
// stability wait ended.
type pasteOutcome struct {
	Settle    time.Duration
	Stability string
}

// pasteSettleFor is the pause between a paste of size bytes and the first
// Enter.
func pasteSettleFor(size int) time.Duration {
	d := pasteSettleBase + time.Duration(size/10_000)*pasteSettlePer10KB
	if d > pasteSettleMax {
		d = pasteSettleMax
	}
	return d
}

// settlePasteThenEnter is the delivery tail after a paste: sleep settle, wait
// for a size-worthy paste to stop changing on screen, then press Enter. pfx
// is the per-driver console prefix.
func settlePasteThenEnter(ctx context.Context, deps Deps, pfx, session string, settle time.Duration, size int) (pasteOutcome, error) {
	out := pasteOutcome{Settle: settle, Stability: pasteSkipped}
	deps.Sleep(settle)
	if size >= pasteStabilityMinBytes {
		out.Stability = waitPaneStable(ctx, deps, pfx, session)
	}
	return out, deps.Tmux.SendKeys(ctx, session, "", true)
}

// waitPaneStable polls the visible pane (scrollback 0 on purpose: history
// never changes and would only mask motion at the input line) until two
// consecutive captures agree, bounded by pasteStabilityMaxPolls. A capture
// fault or an ever-changing pane is said out loud and the caller presses on —
// a bounded wait, never a hold.
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
