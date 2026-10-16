package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (o *Orchestrator) enforceNext(current, staticNext Phase, verdict string, sig router.RoutingSignals, dec router.RouterDecision, shipPlanned bool) (Phase, bool) {
	isSkipped := func(p Phase) bool {
		for _, skip := range dec.SkipPhases {
			if Phase(skip) == p {
				return true
			}
		}
		return false
	}
	advanced := false
	for isSkipped(staticNext) {
		nxt := o.nextInOrder(staticNext)
		if nxt == PhaseEnd {
			break
		}
		staticNext = nxt
		advanced = true
	}

	cand := o.candidatePhase(dec.NextPhase)
	if cand == "" || cand == staticNext {
		return staticNext, advanced
	}
	if cand == PhaseEnd {
		if current == PhaseTriage {
			if terminal := decideTriageTermination(verdict, sig); terminal.stop {
				return PhaseEnd, true
			}
		}
		if o.sm.CanTerminateEarly(current, shipPlanned) {
			return PhaseEnd, true
		}
		return staticNext, advanced
	}
	if !o.transitionLegal(current, cand) {
		return staticNext, advanced
	}
	if !o.sm.SpineSatisfiedUpTo(cand, sig, o.cfg) {
		return staticNext, advanced
	}
	return cand, true
}

func planRunsShip(plan *router.PhasePlan) bool {
	if plan == nil {
		return true
	}
	for _, e := range plan.Entries {
		if Phase(e.Phase) == PhaseShip {
			return e.Run
		}
	}
	return false
}

func (o *Orchestrator) candidatePhase(s string) Phase {
	if p := phaseFromRouter(s); p != "" {
		return p
	}
	if _, ok := o.catalog.Get(s); ok {
		return Phase(s)
	}
	return ""
}

func (o *Orchestrator) transitionLegal(from, cand Phase) bool {
	if decisionOnlyEdge(from, cand) {
		return false
	}
	if from.IsValid() && cand.IsValid() {
		return o.sm.CanTransition(from, cand)
	}
	if !cand.IsValid() {
		spec, ok := o.catalog.Get(string(cand))
		if !ok || !spec.Optional {
			return false
		}
	}
	ci, fi := orderIndex(o.cfg.Order, string(cand)), orderIndex(o.cfg.Order, string(from))
	return ci >= 0 && fi >= 0 && ci > fi
}

func (o *Orchestrator) registerMintedPhases(plan *router.PhasePlan) {
	if plan == nil || o.registrar == nil || len(plan.MintPhases) == 0 {
		return
	}
	for _, cfg := range plan.MintPhases {
		spec, runner, err := o.registrar.Register(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN minted phase %q rejected (skipping): %v\n", cfg.Name, err)
			continue
		}
		p := Phase(spec.Name)
		if _, exists := o.runners[p]; exists {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN minted phase %q clashes with an existing runner — keeping existing\n", spec.Name)
			continue
		}
		merged, warns := o.catalog.Merge([]phasespec.PhaseSpec{spec})
		for _, w := range warns {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN minted phase %q: %s\n", spec.Name, w)
		}
		o.catalog = merged
		for _, w := range phasespec.ApplyUserRouting(&o.cfg, []phasespec.PhaseSpec{spec}, o.catalog) {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN minted phase %q routing: %s\n", spec.Name, w)
		}
		o.runners[p] = runner
		if o.catalogPublisher != nil {
			o.catalogPublisher(o.catalog)
		}
	}
}

func (o *Orchestrator) nextInOrder(p Phase) Phase {
	i := orderIndex(o.cfg.Order, string(p))
	if i < 0 || i+1 >= len(o.cfg.Order) {
		return PhaseEnd
	}
	return Phase(o.cfg.Order[i+1])
}

func orderIndex(order []string, phase string) int {
	for i, p := range order {
		if p == phase {
			return i
		}
	}
	return -1
}

func (o *Orchestrator) worktreePhase(p Phase) bool {
	if WorktreePhase(p) {
		return true
	}
	if spec, ok := o.catalog.Get(string(p)); ok {
		return spec.WritesSource
	}
	return false
}

func (o *Orchestrator) worktreeReadOnly(p Phase) bool { return !o.worktreePhase(p) }

func (o *Orchestrator) withWorktreeFence(req PhaseRequest, next Phase, cs CycleState) PhaseRequest {
	req.WorktreeReadOnly = o.worktreeReadOnly(next)
	req.WorktreeWritablePaths = worktreeWritablePaths(next, cs)
	return req
}

func (o *Orchestrator) leakRecoverablePhase(p Phase) bool {
	return LeakRecoverablePhase(p) || o.worktreePhase(p)
}

func phaseFromRouter(s string) Phase {
	switch s {
	case "retrospective":
		return PhaseRetro
	case router.PhaseEnd:
		return PhaseEnd
	}
	p := Phase(s)
	if !p.IsValid() {
		return ""
	}
	return p
}

func canonicalCatalogName(p Phase) string {
	return phasecontract.RegistryKey(string(p))
}

func (o *Orchestrator) recallForPlan(ctx context.Context, history []FailedRecord, goal ...string) (lastReason string, lessons []string) {
	if o.kb == nil {
		return "", nil
	}
	var latest FailedRecord
	if len(history) > 0 {
		latest = history[len(history)-1]
	}
	query := strings.TrimSpace(latest.Summary + " " + strings.Join(goal, " "))
	if query == "" && latest.Classification == "" {
		return "", nil
	}
	found, err := o.kb.Lookup(ctx, research.Query{
		Consequence: latest.Classification,
		Keywords:    strings.Fields(query),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN KB recall lookup failed (continuing without lessons): %v\n", err)
		return latest.Summary, nil
	}
	digests := make([]string, 0, len(found))
	for _, l := range found {
		digests = append(digests, l.Digest())
	}
	return latest.Summary, digests
}

func (o *Orchestrator) resolvedShipFloor() []string {
	if len(o.shipFloor) > 0 {
		return o.shipFloor
	}
	return router.DefaultShipFloor()
}

func phaseCardsFromCatalog(cat phasespec.Catalog) []router.PhaseCard {
	var cards []router.PhaseCard
	for _, spec := range cat.All() {
		role := spec.RoleOrDefault()
		if role == phasespec.RoleControl {
			continue
		}
		if spec.IsOnDemand() {
			continue
		}
		cards = append(cards, router.PhaseCard{
			Name:              spec.Name,
			Role:              string(role),
			Tier:              spec.ModelOrDefault(),
			WritesSource:      spec.WritesSource,
			Optional:          spec.Optional,
			Description:       spec.Description,
			WhenToUse:         spec.WhenToUse,
			Categories:        spec.Categories,
			AllowedCLIs:       spec.AllowedCLIs,
			ModelTierEnvelope: spec.ModelTierEnvelope,
		})
	}
	return cards
}

func carryoverTodosForAdvisor(todos []CarryoverTodo) []router.CarryoverTodo {
	if len(todos) == 0 {
		return nil
	}
	out := make([]router.CarryoverTodo, len(todos))
	for i, t := range todos {
		out[i] = router.CarryoverTodo{
			ID:             t.ID,
			Action:         t.Action,
			Priority:       t.Priority,
			FirstSeenCycle: t.FirstSeenCycle,
			CyclesUnpicked: t.CyclesUnpicked,
		}
	}
	return out
}

func entriesFromRecords(records []FailedRecord) []failureadapter.Entry {
	out := make([]failureadapter.Entry, len(records))
	for i, r := range records {
		out[i] = failureadapter.Entry{
			TS:                r.TS,
			Cycle:             r.Cycle,
			Verdict:           r.Verdict,
			Classification:    failureadapter.Classification(r.Classification),
			RecordedAt:        r.RecordedAt,
			ExpiresAt:         r.ExpiresAt,
			AuditReportPath:   r.AuditReportPath,
			AuditReportSHA256: r.AuditReportSHA256,
			GitHead:           r.GitHead,
			TreeStateSHA:      r.TreeStateSHA,
			Defects:           r.Defects,
			Retrospected:      r.Retrospected,
			Summary:           r.Summary,
		}
	}
	return out
}

func backfillArtifactPath(workspacePath, phase string) string {
	return filepath.Join(workspacePath, phasecontract.ArtifactFilename(phase))
}

func onDemandCatalogNames(cat phasespec.Catalog) []string {
	var out []string
	for _, spec := range cat.All() {
		if spec.RoleOrDefault() == phasespec.RoleControl {
			continue
		}
		if spec.IsOnDemand() {
			out = append(out, spec.Name)
		}
	}
	return out
}
