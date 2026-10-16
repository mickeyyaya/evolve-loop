package inboxbatch

import "strings"

const RouteLaneValue = "lane"

const loadWarningCode = "INBOX_LOAD_WARNING"

const consoleRoutePrefix = "console"

const pipelineKindPrefix = "pipeline-"

const KindPipelineRepair = pipelineKindPrefix + "repair"

func IsConsoleRoute(route string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(route)), consoleRoutePrefix)
}

func ConsoleRouted(it Item, isProtected func(string) bool) (bool, string) {
	route := strings.ToLower(strings.TrimSpace(it.Route))
	if IsConsoleRoute(route) {
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
	if route == RouteLaneValue {
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

type surfaceDerivation struct {
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
		return scope
	}
	for _, p := range it.mentions {
		if isProtected(p) {
			return surfaceDerivation{routed: true, reason: "mentions protected path: " + p + " (no declared files[]: the surface is derived from the item's own text)"}
		}
	}
	return surfaceDerivation{}
}

func PartitionConsole(items []Item, isProtected func(string) bool) (laneRoutable, console []Item, reasons []string) {
	for _, it := range items {
		routed, reason := ConsoleRouted(it, isProtected)
		if routed {
			console = append(console, it)
			reasons = append(reasons, it.ID+": "+reason)
			continue
		}
		laneRoutable = append(laneRoutable, it)
	}
	return laneRoutable, console, reasons
}

func RoutedResolver(dir string, isProtected func(string) bool) func(id string) (bool, string) {
	scan, _ := ScanDir(dir)
	idx := indexByID(scan.Items)
	return func(id string) (bool, string) {
		at, ok := idx[id]
		if !ok {
			return false, ""
		}
		it := scan.Items[at]
		routed, reason := ConsoleRouted(it, isProtected)
		if !routed {
			return false, reason
		}
		for _, w := range scan.Warnings {
			if w.file == it.Path {
				reason += "; " + loadWarningCode + ": " + w.Text
			}
		}
		return true, reason
	}
}
