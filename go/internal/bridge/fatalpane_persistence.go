package bridge

// fatalpane_persistence.go — persistence guard for the ADR-0044 C2 fatal-pane
// fast-fail, the exact sibling of exhaustion_persistence.go's quota-wall guard.
//
// The fatal-pane detector matches SUBSTRINGS against the RAW captured pane
// ("There's an issue with the selected model", "Please restart Codex" — see
// fatalpane.go's cycle-262 postmortem). That pane is not just CLI chrome: a
// WORKING agent can render fatal-shaped TEXT into it by cat/grep/diffing an
// incident report, a test fixture, or a log excerpt — and then be killed for
// quoting the thing it was asked to read. That is the cardinal false-FAIL
// (cycle-254/255/314/641) the quota-wall guard was built to close; the
// fatal-pane seam matched the same way but fired on ONE observation.
//
// Same regex-independent discriminator, same asymmetry argument: a genuinely
// fatal pane is the CLI's TERMINAL state — it parks there and the text is
// present on the next checkpoint too — while fatal-shaped text merely passing
// through a working agent's pane is gone by then. Cost of the guard is one
// extra checkpoint of latency on a real fatal pane; the trade is trivially
// worth it, because a missed fatal pane merely waits out the legacy reviewer
// (the pre-C2 behavior, safe), while a killed working agent is not.
//
// The gate is ADDITIVE state around fatalPaneVerdict, not a change to it: an
// observation that crosses the threshold delegates to the unchanged seam, so
// the stage discipline, the verdict shape, and the R8.3 durable evidence are
// all byte-identical for a gate-crossed call. Un-crossed observations record
// NOTHING at any stage — shadow must predict the GATED enforce action or the
// R8.5 would/did parity check compares two different semantics.
//
// One gate instance per checkpoint loop, exactly like checkpointExhaustGate:
// a gate reconstructed per checkpoint can never accumulate a streak, which
// would leave the unit tests green while production stayed un-gated.

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// Same persistence bar as the quota-wall guard: the two seams share one
// false-FAIL class, so they must share one threshold — a lower bar here would
// be the un-gated behavior wearing a gate's name.
const fatalPanePersistObservations = exhaustionPersistObservations

type fatalPaneGate struct {
	threshold int
	streak    int
}

func newFatalPaneGate() *fatalPaneGate {
	return &fatalPaneGate{threshold: fatalPanePersistObservations}
}

// verdict runs one fatal-pane observation through the persistence guard and
// returns what the checkpoint should do — the same (verdict, preempted) pair
// fatalPaneVerdict returns, so the call site is a drop-in swap.
//
// A nil gate never preempts and never records: an unwired call site must fail
// SAFE (fall through to the legacy reviewer), never kill.
func (g *fatalPaneGate) verdict(det *recovery.FatalPaneDetector, ev StopEvent, stage string, rec *interaction.Recorder, stderr io.Writer, pfx string) (ReviewVerdict, bool) {
	// A disabled classification path must not even OBSERVE: banking a streak
	// while off would let a later stage flip cash it in on its first enforce
	// checkpoint, killing on evidence gathered while classification was
	// disabled.
	if g == nil || !fatalPaneClassifies(det, stage) {
		return ReviewVerdict{}, false
	}
	// Busy outranks the detector at every stage (never kill a working agent),
	// so a Busy checkpoint is a NON-match that RESETS the streak — not a
	// skipped observation. Otherwise fatal-shaped text on either side of a
	// visibly-working checkpoint accumulates into a kill.
	//
	// The gate observes the SAME stripped pane the verdict detects on — one
	// meaning of the pane per checkpoint (salvage review HIGH: observing raw
	// while the verdict reads stripped let agent-diff noise saturate the
	// streak, so one transient stripped match crossed with zero real
	// persistence — the exact transient-frame kill this gate exists to block).
	stripped := strippedForFatalPaneScan(ev.StdoutTail, ev.InjectedPrompt, det.Signatures())
	_, _, ok := det.Detect(stripped)
	if !g.observe(ok && !ev.Busy) {
		return ReviewVerdict{}, false
	}
	ev.StdoutTail = stripped
	ev.InjectedPrompt = "" // already applied; the verdict must not strip twice
	return fatalPaneVerdict(det, ev, stage, rec, stderr, pfx)
}

// observe records one observation and reports whether the fatal state has now
// persisted for `threshold` CONSECUTIVE observations. matched=false resets the
// streak — the defining property that keeps a transient frame from ever
// crossing, and keeps two NON-consecutive lone matches from accumulating.
func (g *fatalPaneGate) observe(matched bool) bool {
	if !matched {
		g.streak = 0
		return false
	}
	g.streak++
	return g.streak >= g.threshold
}
