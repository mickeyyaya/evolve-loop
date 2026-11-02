package phasecmd

import (
	"context"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

const codeBuildHandoffFloor = "build_handoff_floor"

type BuildSelfCheck struct {
	Floor BuildHandoffFloorFor
	Probe core.BuildHandoffProbe
}

type BuildSelfCheckResult struct {
	deliverable.Result
	Unbound error
}

func (s BuildSelfCheck) Verify(stderr io.Writer) (BuildSelfCheckResult, error) {
	in, unbound := s.Probe.Input()
	if unbound != nil {
		fmt.Fprintf(stderr, "build self-check: WARN no cycle binding (%v) — the contract and the build handoff floor ran without the cycle's base, contract version and workspace, so passing them does not predict the floor\n", unbound)
	}
	res, err := contractHalf(in, unbound)
	if err != nil {
		return BuildSelfCheckResult{Result: res, Unbound: unbound}, err
	}
	checked := s.withFloor(res, in, stderr)
	if unbound == nil {
		writeSelfCheckReceipt(in, checked.OK, stderr)
	}
	return BuildSelfCheckResult{Result: checked, Unbound: unbound}, nil
}

func writeSelfCheckReceipt(in core.ReviewInput, green bool, stderr io.Writer) {
	if err := core.WriteSelfCheckReceipt(context.Background(), in.Workspace, in.Worktree, green); err != nil {
		fmt.Fprintf(stderr, "build self-check: WARN no self-check receipt (%v), so the build session gets the floor findings again before it closes\n", err)
	}
}

func contractHalf(in core.ReviewInput, unbound error) (deliverable.Result, error) {
	roots := deliverable.RootsFor(in)
	if unbound != nil {
		return verifyDeliverable("build", roots, phaseVerifyResolver())
	}
	return deliverable.VerifyWithStage("build", roots, phaseVerifyResolver(), phaseVerifyPhaseIO())
}

func (s BuildSelfCheck) withFloor(res deliverable.Result, in core.ReviewInput, stderr io.Writer) deliverable.Result {
	if s.Floor == nil {
		fmt.Fprintln(stderr, "build self-check: WARN no build handoff floor is wired into this command, so the floor did not run")
		return res
	}
	for _, failure := range s.Floor(in.ProjectRoot).Failures(context.Background(), in) {
		res.Violations = append(res.Violations, deliverable.Violation{Code: codeBuildHandoffFloor, Message: failure})
		res.OK = false
	}
	return res
}
