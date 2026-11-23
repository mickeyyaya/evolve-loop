package gateregistry

import (
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type stageSeed struct {
	key     string
	signal  signalcenter.Code
	site    gatecontract.EmitSite
	fields  []string
	closure []string
}

var stageSeeds = []stageSeed{
	{key: "inbox.curator.stage"},
	{key: "inbox.curator.trailer.stage"},
	{key: "gc.remote_sweep.stage"},
	{key: "pr.merge_window.stage"},
	{key: "workflow.findings_repair.code-review.stage"},
	{key: "workflow.convergence.stage"},
	{
		key:     "workflow.retry_ladder.stage",
		signal:  core.CodeLadderDecision,
		site:    gatecontract.EmitSite{Path: "internal/core/retry_ladder.go", Func: "emitLadderDecision", Ref: "CodeLadderDecision"},
		fields:  []string{"next", "action", "fingerprint", "tier"},
		closure: []string{"internal/retryladder"},
	},
	{key: "workflow.retry_ladder.audit_fix.stage"},
	{key: "workflow.repair.fixability.stage"},
	{key: "workflow.cycle_spend.stage", signal: "CYCLE_SPEND_SHADOW", closure: []string{"internal/cyclespend"}},
	{key: "workflow.builder_selfcheck.stage"},
	{key: "workflow.supersede.stage"},
	{key: "swarm.stage"},
	{key: "failure_disposition.stage"},
	{key: "merge_gate.stage"},
	{key: "parallel_evaluate.stage", signal: "PARALLEL_EVALUATE_SHADOW"},
	{key: "catalog.refresh_stage"},
	{key: "docs_floor.stage"},
	{key: "docs_floor.ste_stage"},
	{key: "comment_floor.stage"},
	{
		key:     "held_proposals.stage",
		signal:  ship.CodeHoldShadow,
		site:    gatecontract.EmitSite{Path: "internal/phases/ship/held_proposal_gate.go", Func: "emitShadow", Ref: "CodeHoldShadow"},
		fields:  []string{"cycle", "tree", "reasons"},
		closure: []string{"internal/heldproposal"},
	},
	{key: "cover_changed.stage"},
	{key: "retrospectives.stage"},
	{key: "integrity.stage"},
	{key: "fleet.budget.stage"},
	{key: "fleet.orchestrator.stage"},
	{key: "fleet.integration.stage"},
	{
		key:     "fleet.orchestrator.window.adaptive.stage",
		signal:  orchestrator.CodeHostPressure,
		site:    gatecontract.EmitSite{Path: "internal/orchestrator/host_pressure.go", Func: "HostPressure", Ref: "CodeHostPressure"},
		fields:  []string{"old", "new", "sized", "cause", "stage", "applied", "cpus", "load1", "load_per_core", "mem_available_pct", "swap_used_mb", "score"},
		closure: []string{"internal/hostpressure"},
	},
	{key: "audit_cascade.stage", signal: "AUDIT_CASCADE_SHADOW", closure: []string{"internal/auditcascade"}},
	{key: "isolation.i2.stage"},
	{key: "isolation.i3.stage"},
	{key: "isolation.i5.stage"},
	{key: "isolation.i7.stage"},
	{key: "isolation.i8.stage"},
	{key: "isolation.i15.stage"},
	{key: "isolation.context.stage"},
	{key: "isolation.dispatch_requests.stage"},
	{key: "isolation.inbox_requests.stage"},
	{key: "healing.detector.stage"},
	{key: "healing.orchestrator.stage"},
	{key: "healing.validator.stage"},
	{key: "healing.live_reaction.stage"},
}

func StageEntries() []gatecontract.Entry {
	seeds := slices.Concat(stageSeeds, outsideSchemaStageSeeds)
	entries := make([]gatecontract.Entry, 0, len(seeds))
	for _, seed := range seeds {
		entries = append(entries, seed.entry())
	}
	return entries
}

func (s stageSeed) entry() gatecontract.Entry {
	shadow := &gatecontract.Shadow{
		PolicyKey:     s.key,
		Signal:        s.signal,
		Fields:        append([]string(nil), s.fields...),
		Closure:       append([]string(nil), s.closure...),
		PolicySubtree: subtreeOf(s.key),
	}
	if s.site != (gatecontract.EmitSite{}) {
		shadow.EmitSites = []gatecontract.EmitSite{s.site}
	}
	return gatecontract.Entry{Name: s.key, Kind: gatecontract.KindStageFeature, Shadow: shadow}
}

func subtreeOf(key string) string {
	cut := strings.LastIndex(key, ".")
	if cut < 0 {
		return key
	}
	return key[:cut]
}
