package ship

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func refuseWeakenedGates(ctx context.Context, opts *Options, res *RunResult) error {
	input, err := gateclass.Read(func(args ...string) (string, error) { return captureGitOutput(ctx, opts, args...) }, "HEAD")
	if err != nil {
		return err
	}
	cycle, _, err := cycleIDForShip(opts)
	if err != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: read the shipping cycle for the gate classifier: "+err.Error())
	}
	input.Cycle, input.Protected, input.Constitution = cycle, guards.IsProtectedSurface, guards.IsHealingConstitution
	flags := gateclass.Classify(input)
	if len(flags) == 0 {
		res.Logs = append(res.Logs, "[ship] OK: the gate classifier flags no kernel surface and no weaker gate in the cycle diff")
		return nil
	}
	event := gateclass.ViolationEvent(cycle, opts.RunID, string(core.PhaseShip), flags)
	opts.Signals.Emit(event)
	return shipErr(shiperr.CodeGateWeakened, core.ShipClassPrecondition, core.StageVerifyClass,
		"INTEGRITY VIOLATION ("+string(gateclass.CodeI10Violation)+"): "+event.Reason+
			". A loop cycle may not change a kernel surface or weaken a gate. Ship an intentional change with `evolve ship --class manual` instead.",
		"patterns", event.Fields["patterns"], "paths", event.Fields["paths"])
}
