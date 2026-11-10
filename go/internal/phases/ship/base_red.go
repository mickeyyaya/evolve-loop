package ship

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeBaseRedRebase  signalcenter.Code = "SHIP_BASE_RED_REBASE"
	CodeBaseRedMainRed signalcenter.Code = "SHIP_BASE_RED_MAIN_RED"

	scannerPackName = "scanner pack"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeBaseRedRebase, "the scanner pack was red, every red test is red on the lane base too and green on main, so ship asks for the fleet rebase instead of blaming the lane; fields.keys (package.Test of each base-red test), base, main, took")
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeBaseRedMainRed, "the scanner pack was red, every red test is red on the lane base and on main (or main could not be judged), so ship parks the lane instead of blaming it; fields.keys (package.Test of each base-red test), base, main (empty when unjudged), took")
}

type baseRedCtxKey struct{}

type baseRedGate struct {
	opts *Options
	base string
}

func withBaseRed(ctx context.Context, opts *Options, base string) context.Context {
	return context.WithValue(ctx, baseRedCtxKey{}, baseRedGate{opts: opts, base: base})
}

func baseRedGateOf(ctx context.Context) (baseRedGate, bool) {
	g, ok := ctx.Value(baseRedCtxKey{}).(baseRedGate)
	return g, ok
}

func contractVerdict(ctx context.Context, out io.Writer, name string, o packOutcome) error {
	if g, ok := baseRedGateOf(ctx); ok && name == scannerPackName {
		if err := attributeContractRed(ctx, g, out, o); err != nil {
			return err
		}
	}
	return contractRed(name, o)
}

func attributeContractRed(ctx context.Context, g baseRedGate, out io.Writer, o packOutcome) error {
	v, err := baseRedAttributor(g.opts, out).Attribute(ctx, g.base, redsOf(o))
	if err == nil {
		return baseRedError(g.opts, v)
	}
	fmt.Fprintf(out, "[ship] WARN repo-contract scanner pack: the red could not be rerun on the lane base (%v), so the lane is blamed\n", err)
	return nil
}

func redsOf(o packOutcome) baseattr.Reds {
	var named []string
	unnamed := false
	for _, f := range o.failures {
		if f.Test == "" {
			unnamed = true
			continue
		}
		named = append(named, f.String())
	}
	reds := baseattr.KeysOf(named)
	reds.Unnamed = reds.Unnamed || unnamed
	return reds
}

func baseRedAttributor(opts *Options, warn io.Writer) baseattr.Attributor {
	gates := policy.LoadOrDefaults(filepath.Join(opts.ProjectRoot, ".evolve", "policy.json"), warn).GatesConfig()
	return baseattr.New(gitexec.Git{Dir: opts.ProjectRoot, Exec: opts.Runner}, baseRedRunner(opts), gates.BaseRedRerun, nil)
}

func baseRedRunner(opts *Options) baseattr.Runner {
	if opts.baseRun != nil {
		return opts.baseRun
	}
	return baseattr.GoTestRunner(ipcenv.GoTestEnv())
}

func baseRedError(opts *Options, v baseattr.Verdict) error {
	if v.Blamed() {
		return nil
	}
	if v.MainGreen() {
		emitBaseRed(opts, CodeBaseRedRebase, signalcenter.KindGateCorrected, signalcenter.SeverityInfo, v)
		return baseRedShipError(shiperr.CodeGitFleetRebaseNeeded, shiperr.ShipClassTransient, v,
			"the red scanner pack tests are red on the lane base too and green on main; rebase the lane onto main and verify again")
	}
	emitBaseRed(opts, CodeBaseRedMainRed, signalcenter.KindShipWarning, signalcenter.SeverityWarn, v)
	return baseRedShipError(shiperr.CodeMainRedPark, shiperr.ShipClassPrecondition, v,
		"the red scanner pack tests are red on the lane base and on main; the lane is not blamed and waits for a green main")
}

func baseRedShipError(code shiperr.ShipErrorCode, class shiperr.ShipErrorClass, v baseattr.Verdict, message string) error {
	return shiperr.NewShipError(code, class, shiperr.StageAtomicShip, message+": "+baseattr.Names(v.BaseRed), v.Debug()...)
}

func emitBaseRed(opts *Options, code signalcenter.Code, kind signalcenter.Kind, sev signalcenter.Severity, v baseattr.Verdict) {
	keys := baseattr.Names(v.BaseRed)
	opts.Signals.Emit(signalcenter.Event{
		Cycle: opts.CycleID, RunID: opts.RunID, Phase: phaseName,
		Module: signalcenter.ModuleShip, Origin: "baseRedError", Kind: kind, Severity: sev, Code: code,
		Reason: "repo-contract scanner pack: " + keys + " red on the lane base " + v.Base + " too, so the lane is not blamed",
		Fields: map[string]string{"keys": keys, "base": v.Base, "main": v.Main, "took": v.Took.String()},
	})
}
