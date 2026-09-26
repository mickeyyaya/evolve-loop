package bridge

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

const fatalPanePersistObservations = exhaustionPersistObservations

type fatalPaneGate struct {
	threshold int
	streak    int
}

func newFatalPaneGate() *fatalPaneGate {
	return &fatalPaneGate{threshold: fatalPanePersistObservations}
}

func (g *fatalPaneGate) verdict(det *recovery.FatalPaneDetector, ev StopEvent, stage string, rec *interaction.Recorder, stderr io.Writer, pfx string) (ReviewVerdict, bool) {
	if g == nil || !fatalPaneClassifies(det, stage) {
		return ReviewVerdict{}, false
	}
	stripped := strippedForFatalPaneScan(ev.StdoutTail, ev.InjectedPrompt, det.Signatures())
	_, _, ok := det.Detect(stripped)
	if !g.observe(ok && !ev.Busy) {
		return ReviewVerdict{}, false
	}
	ev.StdoutTail = stripped
	ev.InjectedPrompt = ""
	return fatalPaneVerdict(det, ev, stage, rec, stderr, pfx)
}

func (g *fatalPaneGate) observe(matched bool) bool {
	if !matched {
		g.streak = 0
		return false
	}
	g.streak++
	return g.streak >= g.threshold
}
