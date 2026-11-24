package heldaudit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const brief = "Audit the attached diff of a held lane change. The hold reasons follow. " +
	"Find each way the change weakens a gate, a fence or a guarantee, and end with the verdict.\n"

type SeatRequest struct {
	Cycle int
	Input string
	Files []string
	Route string
	Admit func(cli, tier string) bool
}

type SeatOutcome struct {
	Verdict      string
	Report       string
	RunLabel     string
	LaunchRecord string
	CLI          string
	Tier         string
}

type SeatRunner func(ctx context.Context, req SeatRequest) (SeatOutcome, error)

type Deps struct {
	Seat        SeatRunner
	Resolve     func(key string) (cliroute.Decision, error)
	Floor       func(key string) string
	ModelFamily func(cli string) string
	Now         func() time.Time
}

type Dispatcher struct {
	Deps
	EvolveDir string
}

func Admitter(builders []string, modelFamily func(cli string) string) func(cli, tier string) bool {
	return func(cli, tier string) bool {
		return heldproposal.Independent(modelFamily(cli), tier, builders) == nil
	}
}

func (x Dispatcher) Dispatch(ctx context.Context, workspace string, rec heldproposal.Record) error {
	return flock.WithPathLock(heldproposal.AuditReportPath(x.EvolveDir, rec.Cycle), func() error {
		return x.dispatchLocked(ctx, workspace, rec)
	})
}

func (x Dispatcher) dispatchLocked(ctx context.Context, workspace string, rec heldproposal.Record) error {
	if kept, err := x.keptReport(rec); kept || err != nil {
		return err
	}
	rec, err := x.attributed(workspace, rec)
	if err != nil {
		return x.waitFor(rec, heldproposal.WaitBuilderUnattributed, err.Error())
	}
	if reason, detail := x.diffProblem(rec); reason != "" {
		return x.waitFor(rec, reason, detail)
	}
	admit := Admitter(rec.BuilderFamilies, x.ModelFamily)
	if _, reason, detail := routeChoice(x.Deps, admit); reason != "" {
		return x.waitFor(rec, reason, detail)
	}
	out, err := x.Seat(ctx, SeatRequest{
		Cycle: rec.Cycle, Input: brief + "Hold reasons: " + strings.Join(rec.Reasons, ", ") + "\n",
		Files: []string{heldproposal.AuditDiffPath(x.EvolveDir, rec.Cycle)}, Route: cliroute.HeldAuditKey, Admit: admit,
	})
	if err != nil {
		return x.waitFor(rec, heldproposal.WaitSeatFailed, err.Error())
	}
	return x.record(rec, out)
}

func (x Dispatcher) keptReport(rec heldproposal.Record) (bool, error) {
	report, _, found, err := heldproposal.ReadAuditReport(x.EvolveDir, rec.Cycle)
	if err != nil {
		return false, err
	}
	decided := report.Verdict == heldproposal.AuditPass || report.Verdict == heldproposal.AuditFail
	return found && decided && report.Tree == rec.Tree && report.DiffSHA256 == rec.DiffSHA256, nil
}

func (x Dispatcher) attributed(workspace string, rec heldproposal.Record) (heldproposal.Record, error) {
	if len(rec.BuilderFamilies) > 0 {
		return rec, nil
	}
	families, err := BuilderFamilies(workspace, rec.Cycle, x.ModelFamily)
	if err != nil {
		return rec, err
	}
	rec.BuilderFamilies = families
	return rec, heldproposal.WriteRecord(x.EvolveDir, rec)
}

func (x Dispatcher) diffProblem(rec heldproposal.Record) (string, string) {
	raw, err := os.ReadFile(heldproposal.AuditDiffPath(x.EvolveDir, rec.Cycle))
	if err != nil {
		return heldproposal.WaitDiffMissing, err.Error()
	}
	if heldproposal.DigestOf(raw) != rec.DiffSHA256 {
		return heldproposal.WaitDiffMismatch, "the diff file differs from the digest of the hold record"
	}
	return "", ""
}

func routeChoice(d Deps, admit func(cli, tier string) bool) (string, string, string) {
	decision, err := d.Resolve(cliroute.HeldAuditKey)
	if err != nil {
		return "", heldproposal.WaitNoIndependentRoute, err.Error()
	}
	for _, cli := range decision.Plan.Candidates {
		if admit(cli, policy.DeepTier) {
			return cli, "", ""
		}
	}
	if floor := d.Floor(cliroute.HeldAuditKey); floor != "" {
		return "", heldproposal.WaitFloorExcludesOtherFamilies, "the floor " + floor + " holds the audit seat"
	}
	return "", heldproposal.WaitNoIndependentRoute, fmt.Sprintf("no candidate of %v is another model family at the deep tier", decision.Plan.Candidates)
}

type Plan struct {
	CLI    string
	Family string
	Tier   string
	Wait   string
	Detail string
}

func PlanFor(d Deps, workspace string, rec heldproposal.Record) Plan {
	families := rec.BuilderFamilies
	if len(families) == 0 {
		var err error
		if families, err = BuilderFamilies(workspace, rec.Cycle, d.ModelFamily); err != nil {
			return Plan{Wait: heldproposal.WaitBuilderUnattributed, Detail: err.Error()}
		}
	}
	cli, reason, detail := routeChoice(d, Admitter(families, d.ModelFamily))
	if reason != "" {
		return Plan{Wait: reason, Detail: detail}
	}
	return Plan{CLI: cli, Family: d.ModelFamily(cli), Tier: policy.DeepTier}
}

func (x Dispatcher) waitFor(rec heldproposal.Record, reason, detail string) error {
	report := x.base(rec)
	report.Verdict, report.Reason = heldproposal.AuditWait, reason+": "+detail
	return heldproposal.WriteAuditReport(x.EvolveDir, report, nil)
}

func (x Dispatcher) base(rec heldproposal.Record) heldproposal.AuditReport {
	return heldproposal.AuditReport{Cycle: rec.Cycle, Tree: rec.Tree, DiffSHA256: rec.DiffSHA256, BuilderFamilies: rec.BuilderFamilies, At: x.Now().UTC()}
}

func (x Dispatcher) record(rec heldproposal.Record, out SeatOutcome) error {
	report := x.base(rec)
	report.CLI, report.Tier, report.Family = out.CLI, out.Tier, x.ModelFamily(out.CLI)
	report.RunLabel, report.LaunchRecord = out.RunLabel, out.LaunchRecord
	report.Verdict = heldproposal.AuditFail
	if out.Verdict == heldproposal.AuditPass {
		report.Verdict = heldproposal.AuditPass
	}
	refusal := heldproposal.Independent(report.Family, report.Tier, rec.BuilderFamilies)
	if refusal != nil {
		report.Verdict, report.Reason = heldproposal.AuditFail, refusal.Error()
	}
	return errors.Join(refusal, heldproposal.WriteAuditReport(x.EvolveDir, report, []byte(out.Report)))
}

func (x Dispatcher) RecordWait(rec heldproposal.Record, reason, detail string) error {
	return x.waitFor(rec, reason, detail)
}
