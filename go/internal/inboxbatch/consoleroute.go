package inboxbatch

import "strings"

// routeLane is the operator override: it relaxes a heuristic derivation, never a declared protected file.
const routeLane = "lane"

// consoleRoutePrefix marks operator-owned route values (console-manual, console-salvage, ...).
const consoleRoutePrefix = "console"

// pipelineKindPrefix marks pipeline-integrity work, which the operator owns even with no files[].
const pipelineKindPrefix = "pipeline-"

// KindPipelineRepair is the kind the loop's halt escalation autofiles; the halt writer shares this symbol.
const KindPipelineRepair = pipelineKindPrefix + "repair"

// ConsoleRouted reports whether the item is operator-owned and why; a nil isProtected disables only surface rules.
// route:"lane" relaxes a heuristic derivation for operator-authored items only.
// See ADR-0074.
func ConsoleRouted(it Item, isProtected func(string) bool) (bool, string) {
	route := strings.ToLower(strings.TrimSpace(it.Route))
	if strings.HasPrefix(route, consoleRoutePrefix) {
		return true, "route:" + route
	}
	surface := protectedDerivation(it, isProtected)
	derived, reason := pipelineKindDerivation(it)
	if !derived {
		derived, reason = surface.routed, surface.reason
	}
	if !derived {
		return false, ""
	}
	if route == routeLane {
		switch {
		case surface.binding:
			return true, surface.reason + " (route:lane cannot relax a declared protected file: triage's breaker and the ship tripwire refuse it whatever the route)"
		case strings.TrimSpace(it.InjectedBy) == "":
			return false, ""
		default:
			return true, reason + " (route:lane ignored: agent-autofiled item cannot override a console derivation)"
		}
	}
	return true, reason
}

// surfaceDerivation is one item's protected-surface judgment.
type surfaceDerivation struct {
	// binding marks a declared protected file, which triage's breaker and the ship tripwire refuse whatever the route.
	routed, binding bool
	reason          string
}

func pipelineKindDerivation(it Item) (bool, string) {
	kind := strings.ToLower(strings.TrimSpace(it.Kind))
	if !strings.HasPrefix(kind, pipelineKindPrefix) {
		return false, ""
	}
	return true, "kind:" + kind + " (pipeline-integrity work is console-owned)"
}

// protectedDerivation binds on a declared protected file; a protected declared directory, or with no
// declared surface a protected file the text names, routes without binding.
func protectedDerivation(it Item, isProtected func(string) bool) surfaceDerivation {
	if isProtected == nil {
		return surfaceDerivation{}
	}
	var scope surfaceDerivation
	for _, tok := range declaredTokens(it.Files) {
		switch {
		case !isProtected(tok):
		case isFileSpelling(tok):
			return surfaceDerivation{routed: true, binding: true, reason: "protected fix surface: " + tok}
		case !scope.routed:
			scope = surfaceDerivation{routed: true, reason: "protected fix surface: " + tok}
		}
	}
	if scope.routed || it.DeclaredSurface() {
		return scope // a declared surface wins; mentions are context
	}
	// A mention never binds: the text may only cite the file.
	for _, p := range it.mentions {
		if isProtected(p) {
			return surfaceDerivation{routed: true, reason: "mentions protected path: " + p + " (no declared files[]: the surface is derived from the item's own text)"}
		}
	}
	return surfaceDerivation{}
}

// PartitionConsole splits items in input order, with one reason per routed item so no exclusion is silent.
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

// RoutedResolver loads dir once and classifies by id; unknown ids and a failed load resolve dispatchable.
// Build one per wave so inbox changes are seen; failing open keeps a broken backlog from stopping the queue.
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
