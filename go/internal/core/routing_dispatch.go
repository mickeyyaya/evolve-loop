package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (o *Orchestrator) enforceNext(current, staticNext Phase, sig router.RoutingSignals, dec router.RouterDecision, shipPlanned bool) (Phase, bool) {
	isSkipped := func(p Phase) bool {
		for _, skip := range dec.SkipPhases {
			if Phase(skip) == p {
				return true
			}
		}
		return false
	}
	// Skip-advance: when the static successor is a phase the router has
	// declined (dec.SkipPhases — e.g. an EnableOff optional like build-planner,
	// or a plan veto), advance to the next NON-skipped phase so the spine-decline
	// fallback below never lands on a vetoed phase (cycle-238 D1).
	//
	// GUARD (cycle-240 e2e regression): nextInOrder reads o.cfg.Order, which is
	// EMPTY when no phase-registry.json is present (the e2e fixtures, and any
	// repo without the registry). An empty/exhausted order makes nextInOrder
	// return PhaseEnd, which would silently rewrite staticNext to "end" — turning
	// "skip this optional phase" into "terminate the cycle before build/audit/
	// ship". Only advance while the order can name a real successor; if it yields
	// PhaseEnd we cannot trust it, so keep the original staticNext (the cand
	// override + downstream gates still drive forward exactly as pre-regression).
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
	// Early-exit (guarded scout/triage→end): the advisor proposes ending a
	// no-ship convergence cycle. CanTerminateEarly is the SOLE authority — it
	// rejects any ship-intended cycle, so this can never bypass build/audit on a
	// path to ship. It deliberately precedes (and skips) the
	// SpineSatisfiedUpTo(end) gate — which would require build+audit — because a
	// no-ship early-exit legitimately happens before those anchors run.
	if cand == PhaseEnd {
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

// planRunsShip reports whether the advisor's clamped plan schedules the ship
// phase. A nil plan (static spine, no advisor) is treated as ship-intended so
// early-exit is never taken without an explicit advisor decision.
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

// candidatePhase resolves a router-proposed phase string to a runnable Phase:
// a built-in (via phaseFromRouter) OR a user phase present in the catalog. An
// unknown string yields "" so enforceNext declines it.
func (o *Orchestrator) candidatePhase(s string) Phase {
	if p := phaseFromRouter(s); p != "" {
		return p
	}
	if _, ok := o.catalog.Get(s); ok {
		return Phase(s)
	}
	return ""
}

// transitionLegal is the kernel legality gate for a proposed edge. Built-in
// phases use the hardcoded state-machine graph. A user phase (optional,
// catalog-defined) is legal iff it makes forward progress in the configured
// order (cfg.Order) — the router only proposes the next runnable optional, and
// SpineSatisfiedUpTo independently guards the mandatory anchors, so an optional
// insertion between anchors cannot skip the spine or reach ship illegitimately.
func (o *Orchestrator) transitionLegal(from, cand Phase) bool {
	if from.IsValid() && cand.IsValid() {
		return o.sm.CanTransition(from, cand) // both built-in: hardcoded graph
	}
	// At least one endpoint is NOT a built-in phase — validate via order
	// forward-progress (both-built-in edges took the sm.CanTransition branch above).
	// A user-phase candidate must be optional (the floor). Leapfrogging a
	// mandatory anchor is independently blocked by SpineSatisfiedUpTo in the caller.
	if !cand.IsValid() {
		spec, ok := o.catalog.Get(string(cand))
		if !ok || !spec.Optional {
			return false
		}
	}
	ci, fi := orderIndex(o.cfg.Order, string(cand)), orderIndex(o.cfg.Order, string(from))
	return ci >= 0 && fi >= 0 && ci > fi
}

// registerMintedPhases registers each advisor-minted phase from the clamped
// plan, making it BOTH dispatchable (runners map) and routable (catalog +
// cfg.Order, the same live-read path build-time user phases take). The
// trust-kernel clamp (envelope/allowed-CLIs) is enforced inside the injected
// registrar — a rejected config is a loud skip, never a registered dead phase,
// and a name that collides with a built-in is never clobbered. Best-effort: a
// nil plan, nil registrar, or empty MintPhases is a no-op (byte-identical
// legacy behavior).
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
		// The runner-collision check intentionally gates ALL THREE splices
		// below: a name already in runners (built-in or earlier mint) is left
		// wholly untouched — never half-registered into catalog/routing only.
		p := Phase(spec.Name)
		if _, exists := o.runners[p]; exists {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN minted phase %q clashes with an existing runner — keeping existing\n", spec.Name)
			continue
		}
		// Splice into the three live-read structures: runners (dispatch lookup),
		// catalog (candidatePhase/transitionLegal recognition), and cfg routing
		// (Order forward-progress + triggers/enable). Catalog.Merge keeps the
		// built-in on a name clash, so the splice cannot displace a built-in.
		// ApplyUserRouting extends o.cfg.Order in place; the mid-cycle RouteInput
		// copies built below inherit that extended order intentionally.
		merged, warns := o.catalog.Merge([]phasespec.PhaseSpec{spec})
		for _, w := range warns {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN minted phase %q: %s\n", spec.Name, w)
		}
		o.catalog = merged
		for _, w := range phasespec.ApplyUserRouting(&o.cfg, []phasespec.PhaseSpec{spec}) {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN minted phase %q routing: %s\n", spec.Name, w)
		}
		o.runners[p] = runner
	}
}

// nextInOrder returns the phase immediately following p in the configured
// order, or PhaseEnd when p is last/absent. Used to resume the normal sequence
// after a user phase runs. Assumes cfg.Order is the complete registry order
// (applyRegistry appends every registry phase), so a built-in successor is
// always present when a registry is loaded.
func (o *Orchestrator) nextInOrder(p Phase) Phase {
	i := orderIndex(o.cfg.Order, string(p))
	if i < 0 || i+1 >= len(o.cfg.Order) {
		return PhaseEnd
	}
	return Phase(o.cfg.Order[i+1])
}

// orderIndex returns the position of phase in order, or -1 if absent.
func orderIndex(order []string, phase string) int {
	for i, p := range order {
		if p == phase {
			return i
		}
	}
	return -1
}

// worktreePhase reports whether next WRITES SOURCE — the write axis the
// role-gate, tree-diff guard, and build-commit normalize key off. Built-in
// tdd/build always do; a user phase does iff its spec sets writes_source.
// Method form (vs the free WorktreePhase) so it consults the injected catalog.
// Since CB.1 this no longer selects the subprocess cwd — every phase runs
// cwd=worktree (see the phaseWorktree assignment in the dispatch loop); this
// predicate is purely about write PERMISSION.
func (o *Orchestrator) worktreePhase(p Phase) bool {
	if WorktreePhase(p) {
		return true
	}
	if spec, ok := o.catalog.Get(string(p)); ok {
		return spec.WritesSource
	}
	return false
}

// phaseFromRouter denormalizes a router phase string back to a core.Phase.
// The router speaks canonical "retrospective"/"end"; core uses "retro"/
// PhaseEnd. An unknown string yields "" so enforceNext declines it.
//
// The core↔registry vocabulary skew this bridges is a DECIDED permanent boundary
// (ADR-0060 §57), not debt: do NOT "unify" PhaseRetro's wire string — "retro" is
// the trust-kernel serialized identity in state.json/ledger (pinned by
// cyclestate.TestPhaseConstants), so the converter stays; the rename does not.
func phaseFromRouter(s string) Phase {
	switch s {
	case "retrospective":
		return PhaseRetro
	case router.PhaseEnd: // "end" — same string as core.PhaseEnd
		return PhaseEnd
	}
	p := Phase(s)
	if !p.IsValid() {
		return ""
	}
	return p
}

// canonicalCatalogName maps a core.Phase to its catalog/registry key, bridging
// the core↔router vocabulary skew (PhaseRetro stringifies to "retro" but the
// registry names the phase "retrospective"). It is the inverse of
// phaseFromRouter's alias cases — used by specFor so a descriptor lookup cannot
// silently miss on the skew and fall through to a wrong edge (ADR-0058). The skew
// is a decided permanent boundary (ADR-0060 §57); this converter is the accepted
// solution, not a deferred unification.
func canonicalCatalogName(p Phase) string {
	if p == PhaseRetro {
		return "retrospective"
	}
	return string(p)
}

// recallForPlan builds the WS2 recall-memory context for the advisor's plan: the
// short reason of the most recent failure and the prior lessons that match it.
// It is the orchestrator's I/O (a KB lookup), kept out of the pure advisor so the
// advisor only renders. Returns ("", nil) when no KB is wired or there is no
// failure history — the legacy no-recall behavior. An empty lesson result is the
// novel-failure signal (nothing in the corpus matches yet), surfaced as the
// reason-without-lessons case.
func (o *Orchestrator) recallForPlan(ctx context.Context, history []FailedRecord) (lastReason string, lessons []string) {
	if o.kb == nil || len(history) == 0 {
		return "", nil
	}
	latest := history[len(history)-1]
	if latest.Summary == "" && latest.Classification == "" {
		return "", nil
	}
	found, err := o.kb.Lookup(ctx, research.Query{
		Consequence: latest.Classification,
		Keywords:    strings.Fields(latest.Summary),
	})
	if err != nil {
		// Recall is best-effort: a KB read error must never block planning.
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN KB recall lookup failed (continuing without lessons): %v\n", err)
		return latest.Summary, nil
	}
	digests := make([]string, 0, len(found))
	for _, l := range found {
		digests = append(digests, l.Digest())
	}
	return latest.Summary, digests
}

// resolvedShipFloor returns the integrity floor to clamp the advisor's plan to:
// the user-configured floor (WS4) when set, else the router's safe structural
// default ({tdd,build,audit}). The router self-seals the non-removable evaluator
// either way, so this never returns a floor that could reach ship without audit.
func (o *Orchestrator) resolvedShipFloor() []string {
	if len(o.shipFloor) > 0 {
		return o.shipFloor
	}
	return router.DefaultShipFloor()
}

// phaseCardsFromCatalog projects the composable phases (Plan/Build/Evaluate
// archetypes) of the catalog into advisor-facing PhaseCards, so the advisor can
// SELECT a pre-defined phase instead of minting a new one (WS3: prefer reuse —
// DRY at the agent level). Control-archetype phases (ship/retro/memo/debugger)
// are kernel-managed, not advisor-composed, so they are omitted. Order follows
// the catalog's registry order for a deterministic, prompt-cache-friendly result.
func phaseCardsFromCatalog(cat phasespec.Catalog) []router.PhaseCard {
	var cards []router.PhaseCard
	for _, spec := range cat.All() {
		role := spec.RoleOrDefault()
		if role == phasespec.RoleControl {
			continue
		}
		cards = append(cards, router.PhaseCard{
			Name:         spec.Name,
			Role:         string(role),
			Tier:         spec.ModelOrDefault(),
			WritesSource: spec.WritesSource,
			Optional:     spec.Optional,
			Description:  spec.Description,
			WhenToUse:    spec.WhenToUse,
			Categories:   spec.Categories,
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

// entriesFromRecords converts FailedRecord values into failureadapter.Entry.
// Inlined here (rather than exposed from failureadapter) to avoid a
// circular import between core and failureadapter.
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

// backfillArtifactPath returns the absolute path to the backfilled artifact file.
func backfillArtifactPath(workspacePath, phase string) string {
	var filename string
	switch phase {
	case "retro":
		filename = "retrospective-report.md"
	case "build-planner":
		filename = "build-plan.md"
	case "tdd":
		filename = "test-report.md"
	case "intent":
		filename = "intent.md"
	default:
		filename = phase + "-report.md"
	}
	return filepath.Join(workspacePath, filename)
}
