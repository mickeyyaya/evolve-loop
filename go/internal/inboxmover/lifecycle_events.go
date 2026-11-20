package inboxmover

import (
	"fmt"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func (o Options) emitInbox(kind signalcenter.Kind, origin string, cycle int, reason string, fields map[string]string) {
	if o.Signals == nil {
		return
	}
	o.Signals.Emit(signalcenter.Event{
		Cycle: cycle, Module: signalcenter.ModuleInbox, Origin: origin, Kind: kind,
		Severity: signalcenter.SeverityInfo, Reason: reason, Fields: fields,
	})
}

func (o Options) emitClaimed(taskID, cycle string) {
	n, _ := strconv.Atoi(cycle)
	o.emitInbox(signalcenter.KindInboxClaimed, "inboxmover.ClaimDispatchable", n,
		fmt.Sprintf("claimed %s for cycle %s", taskID, cycle), map[string]string{"id": taskID})
}

func (o Options) emitReleased(rel ClaimRelease, reason string) {
	o.emitInbox(signalcenter.KindInboxReleased, "inboxmover.releaseHeld", rel.Cycle,
		fmt.Sprintf("released %s from cycle %d: %s", rel.ID, rel.Cycle, reason),
		map[string]string{"id": rel.ID, "outcome": string(rel.Outcome)})
}

func (o Options) warnSealedClaim(rel ClaimRelease, holder Holder) {
	if o.Signals == nil {
		return
	}
	o.Signals.Emit(signalcenter.Event{
		Cycle: rel.Cycle, Module: signalcenter.ModuleInbox, Origin: "inboxmover.ReleaseStaleClaims", Kind: signalcenter.KindInboxWarning,
		Severity: signalcenter.SeverityWarn, Code: lifecycle.CodeClaimStale,
		Reason: fmt.Sprintf("cycle %d sealed (%s) but still held the claim on %s", rel.Cycle, holder.Evidence.Closeout, rel.ID),
		Fields: map[string]string{"id": rel.ID, "closeout": holder.Evidence.Closeout, "step": "stale_sweep"},
	})
}
