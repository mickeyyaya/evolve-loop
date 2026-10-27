package inboxcurator

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeProposal signalcenter.Code = "INBOX_CURATOR_PROPOSAL"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeProposal, "the inbox curator proposes a change to one open item: consume it, reroute it or merge it (fields.kind, id, target, trigger, deterministic, stage, applied, evidence); the curator applies a deterministic proposal only in the enforce stage at the wave boundary, and a judgment proposal waits for the operator or the orchestrator")
}

type Inputs struct {
	Open        []inboxbatch.Item
	Changes     []Change
	HasTest     func(string) bool
	IsProtected func(string) bool
	Thresholds  Thresholds
	MinShare    float64
	Backstop    bool
}

func Curate(in Inputs) []Proposal {
	var out []Proposal
	routing := in.Backstop
	for _, c := range in.Changes {
		out = append(out, MatchLanding(c, in.Open, in.HasTest, in.MinShare)...)
		routing = routing || TouchesRouting(c.Paths)
	}
	if routing {
		out = append(out, Reroutes(in.Open, in.IsProtected)...)
	}
	if in.Backstop {
		out = append(out, Sweep(in.Open, in.HasTest, in.MinShare)...)
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
