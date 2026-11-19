package inboxcurator

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/landedclaim"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeProposal signalcenter.Code = "INBOX_CURATOR_PROPOSAL"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeProposal, "the inbox curator proposes consuming, rerouting, merging, or quarantining one open item. Deterministic proposals apply only at enforce on wave boundaries; judgment proposals wait for operator or orchestrator. Fields: kind, id, target, trigger, deterministic, stage, applied, evidence")
}

type Inputs struct {
	Open                  []inboxbatch.Item
	Changes               []Change
	History               []Change
	HasTest               func(string) bool
	PathExists            func(string) bool
	IsProtected           func(string) bool
	RerouteBuildableKinds []string
	HasOpenPR             func(string) bool
	Thresholds            Thresholds
	MinShare              float64
	FixSurface            func(inboxbatch.Item) []string
	Stale                 func(inboxbatch.Item) string
	Backstop              bool
	LandedCheck           landedclaim.Check
}

func Curate(in Inputs) []Proposal {
	var out []Proposal
	routing := in.Backstop
	out = append(out, CheckedClaims(in.Open, in.LandedCheck)...)
	for _, c := range in.Changes {
		out = append(out, MatchLanding(c, in.Open, in.HasTest, in.MinShare)...)
		routing = routing || TouchesRouting(c.Paths)
	}
	if in.Backstop {
		out = append(out, ClosedOnMain(in.History, in.Open)...)
	}
	if routing {
		out = append(out, StaleClaims(in.Open, in.Stale)...)
		out = append(out, Reroutes(in.Open, RerouteOptions{
			IsProtected:    in.IsProtected,
			BuildableKinds: in.RerouteBuildableKinds,
			HasOpenPR:      in.HasOpenPR,
			FixSurface:     in.FixSurface,
			Stale:          in.Stale,
			Done:           Tree{HasTest: in.HasTest, PathExists: in.PathExists}.ProbesPass,
		})...)
	}
	if in.Backstop {
		out = append(out, Sweep(in.Open, Tree{HasTest: in.HasTest, PathExists: in.PathExists}, in.MinShare)...)
		out = append(out, Duplicates(in.Open, in.Thresholds)...)
	}
	return firstPerItem(out)
}

func firstPerItem(proposals []Proposal) []Proposal {
	at := map[string]int{}
	var out []Proposal
	for _, p := range proposals {
		key := string(p.Kind) + "\x00" + p.ID
		i, seen := at[key]
		switch {
		case !seen:
			at[key] = len(out)
			out = append(out, p)
		case p.Deterministic && !out[i].Deterministic:
			out[i] = p
		}
	}
	return out
}

func (p Proposal) Event(stage string, applied bool) signalcenter.Event {
	return signalcenter.Event{
		Module: signalcenter.ModuleInbox, Origin: "inboxcurator.Curate", Kind: signalcenter.KindInboxWarning,
		Code: CodeProposal, Severity: signalcenter.SeverityInfo,
		Reason: string(p.Kind) + " " + p.ID + ": " + p.Reason,
		Fields: map[string]string{
			"kind": string(p.Kind), "id": p.ID, "target": p.Target, "trigger": string(p.Trigger),
			"deterministic": strconv.FormatBool(p.Deterministic), "stage": stage, "applied": strconv.FormatBool(applied),
			"evidence": strings.Join(p.Evidence, "; "),
		},
	}
}
