package inboxmover

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var (
	errHoldNoMove     = errors.New("the move did not happen")
	errDeliveryFailed = errors.New("deliver")
	errDeliveryKept   = errors.New("residual drain skipped")
	errResidualDrain  = errors.New("residual drain")
)

func holdFailedDelivery(opts Options, cycle int, id string, cause error) (bool, error) {
	pr, err := Promote(opts, id, StateQuarantine, PromoteOpts{Cycle: strconv.Itoa(cycle)})
	if err == nil && pr.NoOp {
		err = errHoldNoMove
	}
	if err != nil {
		opts.emitDeliveryFailed(cycle, id, StateProcessing, cause)
		return false, errors.Join(cause, fmt.Errorf("hold %q in quarantine/: %w", id, err))
	}
	opts.emitDeliveryFailed(cycle, id, StateQuarantine, cause)
	return true, cause
}

func (o Options) emitDeliveryFailed(cycle int, id, endState string, cause error) {
	if o.Signals == nil {
		return
	}
	o.Signals.Emit(signalcenter.Event{
		Module: signalcenter.ModuleInbox, Origin: "inboxmover.ApplyCycleOutcome", Kind: signalcenter.KindInboxWarning,
		Severity: signalcenter.SeverityIncident, Code: lifecycle.CodeDeliveryFailed, Cycle: cycle,
		Reason: "the move of " + id + " to delivered/ failed; the item ends in " + endState + "/ so no lane draws it: " + cause.Error(),
		Fields: map[string]string{"id": id, "step": "deliver", "end_state": endState},
	})
}
