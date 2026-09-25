package inboxbatch

// consoleroute.go — ADR-0074 I1: routing authority is typed plumbing, not
// prose. An inbox item is either lane-dispatchable or console-routed
// (operator-owned); this file is the ONE classifier every consumer shares —
// the plan-time gate (fleet.TodosFromTriage via RoutedResolver), triage
// prompt composition (advisory visibility), and inboxmover.Claim (handoff).
// Born from cycles 1034/1035/1036 (2026-07-22): one wave burned three
// pipelines on tasks whose fix surface is the ProtectedSurfaceManifest, which
// a cycle structurally cannot write; the routing existed only as annotations
// nothing consumed.

import "strings"

// routeLane is the explicit operator override: forces dispatchability when the
// protected-files derivation would route the item out (e.g. the item only
// READS a protected surface). Clamped for agent-autofiled items — see
// ConsoleRouted.
const routeLane = "lane"

// consoleRoutePrefix marks operator-owned routing values (console-manual,
// console-salvage, and future console-* refinements).
const consoleRoutePrefix = "console"

// pipelineKindPrefix marks pipeline-integrity work (pipeline-repair,
// pipeline-integrity, …): fixed hands-on in the console by operator policy
// (ADR-0072 halt autofiles this kind), so the kind alone routes the item out
// of lane reach — no files[] entry required. Provenance: wave 6, cycle 1688
// (2026-09-15) surfaced 15 lane-eligible pipeline-* items (8 with no files[]
// at all); one was claimed and failed triage before the breaker caught it.
const pipelineKindPrefix = "pipeline-" // every pipeline-* kind, present and future, is the pipeline's own work

// KindPipelineRepair is the kind the ADR-0072 halt autofiles for the operator
// (cmd_loop_escalation.go) — the one symbol the writer and this classifier
// share, so the two can never drift apart.
const KindPipelineRepair = pipelineKindPrefix + "repair"

// ConsoleRouted reports whether the item is operator-owned (not lane-
// dispatchable) and why. isProtected is the control-plane SCOPE predicate the
// routing roots inject (guards.IsProtectedScope — a path that is, or a
// directory that contains, protected surface; nil disables only the derived
// rules — the explicit route field and the kind always bind).
//
// Precedence: route "console-*" always routes; a pipeline-* kind or a
// protected surface routes unless route:"lane" AND the item is
// operator-authored (InjectedBy empty) — agent-autofiled items cannot
// widen agent authority by self-declaring lane dispatch of control-plane
// work (ADR-0073 clamp-parity: the field is unauthenticated, so the
// achievable floor is that an agent-authored override never *widens* what an
// agent may do; a hand-forged override at most forces a doomed pipeline,
// since the ship-time protectedsurface tripwire still blocks the merge).
//
// The surface: every whitespace token of each files[] entry (real items write
// "path (why)" and "(see path)" shapes) is judged in scope — a declared
// directory holding protected files routes (F29); with no DECLARED surface
// (Item.DeclaredSurface: no path-shaped token), the files the record's own
// text names are judged instead. Only surfaces ALREADY on the manifest match —
// a task that will CREATE a new gate-shaped file is caught later by the ship
// tripwire + disposition handoff, not here.
func ConsoleRouted(it Item, isProtected func(string) bool) (bool, string) {
	route := strings.ToLower(strings.TrimSpace(it.Route))
	if strings.HasPrefix(route, consoleRoutePrefix) {
		return true, "route:" + route
	}
	derived, reason := pipelineKindDerivation(it)
	if !derived {
		derived, reason = protectedDerivation(it, isProtected)
	}
	if !derived {
		return false, ""
	}
	if route == routeLane {
		if strings.TrimSpace(it.InjectedBy) == "" {
			return false, "" // operator-authored override honored
		}
		return true, reason + " (route:lane ignored: agent-autofiled item cannot override a console derivation)"
	}
	return true, reason
}

// pipelineKindDerivation reports whether the item's kind marks it as
// pipeline-integrity work (console-owned by policy).
func pipelineKindDerivation(it Item) (bool, string) {
	kind := strings.ToLower(strings.TrimSpace(it.Kind))
	if !strings.HasPrefix(kind, pipelineKindPrefix) {
		return false, ""
	}
	return true, "kind:" + kind + " (pipeline-integrity work is console-owned)"
}

// protectedDerivation reports whether any declared fix-surface token is on the
// protected manifest.
func protectedDerivation(it Item, isProtected func(string) bool) (bool, string) {
	if isProtected == nil {
		return false, ""
	}
	for _, tok := range declaredTokens(it.Files) {
		if isProtected(tok) {
			return true, "protected fix surface: " + tok
		}
	}
	if it.DeclaredSurface() {
		return false, "" // a declared surface wins: prose mentions are context
	}
	// F29: with no declared surface, the FILES the record's own text names ARE
	// its surface — the one triage's breaker would otherwise derive only after
	// a lane paid for scout and triage (18 of ~60 cycles died that way). A
	// mention can only move an item TO the console, never widen lane authority,
	// and it is a file spelling, so the injected scope predicate reduces to
	// membership for it.
	for _, p := range it.mentions {
		if isProtected(p) {
			return true, "mentions protected path: " + p + " (no declared files[]: the surface is derived from the item's own text)"
		}
	}
	return false, ""
}

// PartitionConsole splits items into lane-dispatchable and console-routed,
// preserving input order, with one human-readable reason per routed item so
// the exclusion is always loud (a silently narrowed backlog reads as full
// coverage).
func PartitionConsole(items []Item, isProtected func(string) bool) (dispatchable, console []Item, reasons []string) {
	for _, it := range items {
		routed, reason := ConsoleRouted(it, isProtected)
		if routed {
			console = append(console, it)
			reasons = append(reasons, it.ID+": "+reason)
			continue
		}
		dispatchable = append(dispatchable, it)
	}
	return dispatchable, console, reasons
}

// RoutedResolver loads dir once and returns the id→(routed, reason) closure
// the plan-time gate (fleet.TodosFromTriage) consumes. Unknown ids are
// dispatchable — scout-originated work has no inbox item and must never be
// blocked. Construct per wave so mid-batch inbox changes are seen fresh; a
// load failure resolves everything dispatchable (fail-open like LoadDir: a
// broken backlog must not stop the queue — ADR-0072 never-stop).
func RoutedResolver(dir string, isProtected func(string) bool) func(id string) (bool, string) {
	items, _, _ := LoadDir(dir)
	idx := make(map[string]Item, len(items))
	for _, it := range items {
		if _, dup := idx[it.ID]; !dup {
			idx[it.ID] = it
		}
	}
	return func(id string) (bool, string) {
		it, ok := idx[id]
		if !ok {
			return false, ""
		}
		return ConsoleRouted(it, isProtected)
	}
}
