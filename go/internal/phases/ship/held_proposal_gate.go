package ship

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeHoldShadow signalcenter.Code = "SHIP_HOLD_SHADOW"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeHoldShadow, "at held_proposals.stage shadow, a lane landing that touches the fence or works a pipeline-kind item lands, and the ship records that enforce would hold it for the operator (ADR-0157); fields.cycle, tree, reasons")
}

type heldCheck struct {
	ctx       context.Context
	opts      *Options
	logf      func(format string, args ...any)
	cycle     int
	evolveDir string
	base      string
	commit    string
}

func refuseUnapprovedHeldProposal(ctx context.Context, opts *Options, logf func(string, ...any), base, commit string) error {
	if opts.Class != ClassCycle {
		return nil
	}
	stage, err := policy.HeldProposalsStageFor(opts.ProjectRoot)
	if err != nil {
		logf("[ship] HELD-PROPOSAL: %v; the check holds as at enforce", err)
	}
	if stage == config.StageOff {
		return nil
	}
	cycle, ok, err := cycleIDForShip(opts)
	if err != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageAtomicShip, "ship: read the shipping cycle for the held-proposal check: "+err.Error())
	}
	if !ok {
		return refuseUnknownCycle(stage)
	}
	h := heldCheck{ctx: ctx, opts: opts, logf: logf, cycle: cycle, evolveDir: filepath.Join(opts.ProjectRoot, ".evolve"), base: base, commit: commit}
	return h.run(stage)
}

func refuseUnknownCycle(stage config.Stage) error {
	if stage != config.StageEnforce {
		return nil
	}
	return shipErr(core.CodeStateIO, core.ShipClassPrecondition, core.StageAtomicShip, "ship: the held-proposal check at enforce needs the shipping cycle, and none is known")
}

func (h heldCheck) run(stage config.Stage) error {
	decision, err := h.decide()
	if err != nil || !decision.Hold {
		return err
	}
	tree, err := captureGitOutput(h.ctx, h.opts, "rev-parse", h.commit+"^{tree}")
	if err != nil {
		return err
	}
	rec := heldproposal.Record{
		Cycle: h.cycle, Commit: h.commit, Tree: strings.TrimSpace(tree), Reasons: decision.Reasons, At: time.Now().UTC(),
		Requires: []string{heldproposal.RequirementOperator, heldproposal.RequirementAdversarialAudit},
	}
	if stage == config.StageShadow {
		return h.recordShadow(rec)
	}
	return h.enforce(rec)
}

func (h heldCheck) decide() (heldproposal.Decision, error) {
	scan, err := inboxbatch.ScanDir(inboxbatch.ProcessingCycleDir(filepath.Join(h.evolveDir, "inbox"), h.cycle))
	if err != nil {
		return heldproposal.Decision{}, shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageAtomicShip, "ship: read the claimed inbox items for the held-proposal check: "+err.Error())
	}
	kinds := make([]string, 0, len(scan.Items))
	for _, it := range scan.Items {
		kinds = append(kinds, it.Kind)
	}
	out, err := captureGitOutput(h.ctx, h.opts, "diff", "--name-only", "-z", "--no-renames", h.base, h.commit)
	if err != nil {
		return heldproposal.Decision{}, err
	}
	changed := strings.FieldsFunc(out, func(r rune) bool { return r == 0 })
	return heldproposal.Decide(heldproposal.Inputs{Kinds: kinds, Changed: changed, Protected: guards.IsProtectedSurface}), nil
}

func (h heldCheck) recordShadow(rec heldproposal.Record) error {
	rec.Status = heldproposal.StatusWouldHold
	recordErr := ""
	if err := heldproposal.WriteRecord(h.evolveDir, rec); err != nil {
		h.logf("[ship] HELD-PROPOSAL: %v", err)
		recordErr = err.Error()
	}
	h.emitShadow(rec, recordErr)
	return nil
}

func (h heldCheck) enforce(rec heldproposal.Record) error {
	approval, found, err := heldproposal.ReadApproval(h.evolveDir, h.cycle)
	if err != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageAtomicShip, "ship: read the operator approval of the held proposal: "+err.Error())
	}
	verdict, err := h.judge(approval, found, rec.Tree)
	if err != nil {
		return err
	}
	switch verdict {
	case heldproposal.Approved:
		h.logf("[ship] HELD-PROPOSAL: the operator approved tree %s of cycle %d; the landing goes on", rec.Tree, h.cycle)
		return nil
	case heldproposal.Rejected:
		return shipErr(shiperr.CodeHeldProposalRejected, core.ShipClassPrecondition, core.StageAtomicShip,
			"ship: the operator rejected tree "+rec.Tree+" of cycle "+strconv.Itoa(h.cycle), "cycle", strconv.Itoa(h.cycle), "tree", rec.Tree)
	}
	rec.Status = heldproposal.StatusHeld
	if err := heldproposal.WriteRecord(h.evolveDir, rec); err != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageAtomicShip, "ship: write the hold record: "+err.Error())
	}
	return shipErr(shiperr.CodeHeldForOperator, core.ShipClassPrecondition, core.StageAtomicShip,
		"ship: cycle "+strconv.Itoa(h.cycle)+" is held for the operator ("+strings.Join(rec.Reasons, ", ")+"); approve it with evolve landing approve --cycle "+strconv.Itoa(h.cycle)+" --tree "+rec.Tree,
		"cycle", strconv.Itoa(h.cycle), "tree", rec.Tree, "reasons", strings.Join(rec.Reasons, ","))
}

func (h heldCheck) judge(approval heldproposal.Approval, found bool, tree string) (heldproposal.Verdict, error) {
	verdict := heldproposal.Judge(approval, found, tree)
	if verdict == heldproposal.Pending {
		return verdict, nil
	}
	prior, held, err := heldproposal.ReadRecord(h.evolveDir, h.cycle)
	if err != nil {
		return verdict, shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageAtomicShip, "ship: read the hold record: "+err.Error())
	}
	decided := prior.Status == heldproposal.StatusHeld || prior.Status == heldproposal.StatusAfter(approval.Decision)
	if !held || !decided || prior.Tree != tree {
		return heldproposal.Pending, nil
	}
	return verdict, nil
}

func (h heldCheck) emitShadow(rec heldproposal.Record, recordErr string) {
	h.opts.Signals.Emit(signalcenter.Event{
		Cycle: h.cycle, RunID: h.opts.RunID, Phase: "ship", Module: signalcenter.ModuleShip, Origin: "refuseUnapprovedHeldProposal",
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityInfo, Code: CodeHoldShadow,
		Reason: "shadow: enforce would hold this landing for the operator",
		Fields: map[string]string{"cycle": strconv.Itoa(h.cycle), "tree": rec.Tree, "reasons": strings.Join(rec.Reasons, ","), "record_error": recordErr},
	})
}
