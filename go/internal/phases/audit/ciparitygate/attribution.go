package ciparitygate

import (
	"context"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeTierBaseRedAttributed signalcenter.Code = "AUDIT_CIPARITY_TIER_BASE_RED_ATTRIBUTED"
	CodeTierBaseRedUnjudged   signalcenter.Code = "AUDIT_CIPARITY_TIER_BASE_RED_UNJUDGED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeTierBaseRedAttributed, "an integration tier red was rerun on the lane base and every red test is red there too, so the audit warns and does not blame the lane; fields.gate, keys (package.Test of each base-red test), base, main_red (the keys that are red on main too), took, log")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeTierBaseRedUnjudged, "an integration tier red could not be rerun on the lane base, so the audit blames the lane as before and a GATE_FAILED follows; fields.gate, keys (package.Test of each red test), err, log")
}

type Attribution func(ctx context.Context, r baseattr.Reds) (baseattr.Verdict, error)

type BaseRed string

func (b BaseRed) Error() string { return string(b) }

func WithAttribution(a Attribution) Option {
	return func(g *Gates) { g.attribute = a }
}

var tierTestFlags = []string{"-race", "-tags", "integration"}

func (g *Gates) attributeToBase(req Request, red attempt, logPath string) error {
	if g.attribute == nil {
		return nil
	}
	reds := baseattr.ParseGoTest(red.out + "\n" + red.errOut)
	if !reds.Attributable() {
		return nil
	}
	reds.Flags = slices.Clone(tierTestFlags)
	keys := baseattr.Names(reds.Keys)
	v, err := g.attribute(context.Background(), reds)
	if err != nil {
		g.warn(gateTier, req, CodeTierBaseRedUnjudged,
			"the red tests of the integration tier could not be run on the lane base, so the audit blames the lane as before: "+err.Error(), "keys", keys, "err", err.Error(), "log", logPath)
		return nil
	}
	if v.Blamed() {
		return nil
	}
	notice := BaseRed("each red test of the integration tier is red on the lane base " + v.Base + " too, so the audit does not blame the lane: " + keys)
	g.warn(gateTier, req, CodeTierBaseRedAttributed, string(notice), "keys", keys, "base", v.Base, "main_red", baseattr.Names(v.MainRed), "took", v.Took.String(), "log", logPath)
	return notice
}
