package lifecycle

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeClaimStale signalcenter.Code = "INBOX_CLAIM_STALE"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeClaimStale, "the stale-claim sweep found a claim in processing/cycle-N/ whose cycle already sealed (a closeout exists) — the cycle exited without draining its claim, so the sweep releases it; fields.id, closeout, step=stale_sweep")
}

func (m *Mover) emitReleased(taskID string, cycle int, reason string) {
	if c := m.center(); c != nil {
		c.Emit(signalcenter.Event{
			Cycle: cycle, Module: signalcenter.ModuleInbox, Origin: "Mover.Release", Kind: signalcenter.KindInboxReleased,
			Severity: signalcenter.SeverityInfo, Reason: fmt.Sprintf("released %s from cycle %d: %s", taskID, cycle, reason),
			Fields: map[string]string{"id": taskID},
		})
	}
}
