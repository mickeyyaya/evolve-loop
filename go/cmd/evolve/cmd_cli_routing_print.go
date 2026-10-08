package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type routeGroup struct {
	chain  string
	tiers  string
	agents []agentRoute
}

func printRoutingReport(w io.Writer, report routingReport) {
	view := "live: probe, cli-health bench and the discovered tail applied"
	if report.Static {
		view = "static: no host probe, bench or discovery"
	}
	fmt.Fprintf(w, "cli-routing: %s (%s)\n", report.Mode, view)
	if len(report.Benched) > 0 {
		fmt.Fprintf(w, "benched now: %s\n", strings.Join(report.Benched, ", "))
	}
	for _, g := range groupRoutes(report.Agents) {
		fmt.Fprintf(w, "\nchain %s\n  %s\n", g.chain, g.tiers)
		for _, a := range g.agents {
			printAgentRoute(w, a)
		}
	}
	if len(report.Findings) > 0 {
		fmt.Fprintln(w, "\nfindings:")
		reportRoutingFindings(w, report.Findings)
	}
}

func printAgentRoute(w io.Writer, a agentRoute) {
	label := a.Agent
	if a.Phase != "" && a.Phase != a.Agent {
		label += " (phase " + a.Phase + ")"
	}
	if a.Error != "" {
		fmt.Fprintf(w, "    %-34s REFUSED: %s\n", label, a.Error)
		return
	}
	notes := ""
	if len(a.Notes) > 0 {
		notes = "  [" + strings.Join(a.Notes, "; ") + "]"
	}
	fmt.Fprintf(w, "    %-34s rule %s%s\n", label, a.Rule, notes)
}

func groupRoutes(agents []agentRoute) []routeGroup {
	index := map[string]int{}
	var groups []routeGroup
	for _, a := range agents {
		chain, tiers := strings.Join(a.Chain, " → "), tierSummary(a.Tiers)
		if a.Error != "" {
			chain, tiers = "(refused)", "no route"
		}
		key := chain + "|" + tiers
		if i, seen := index[key]; seen {
			groups[i].agents = append(groups[i].agents, a)
			continue
		}
		index[key] = len(groups)
		groups = append(groups, routeGroup{chain: chain, tiers: tiers, agents: []agentRoute{a}})
	}
	sort.SliceStable(groups, func(i, j int) bool { return len(groups[i].agents) > len(groups[j].agents) })
	return groups
}

func tierSummary(tiers []tierRoute) string {
	parts := make([]string, 0, len(tiers))
	for _, tr := range tiers {
		switch {
		case tr.CLI == "":
			parts = append(parts, tr.Tier+": no CLI the ceiling permits")
		case tr.Model != "":
			parts = append(parts, fmt.Sprintf("%s: %s %s%s", tr.Tier, tr.CLI, tr.Model, effortLabel(tr)))
		default:
			parts = append(parts, tr.Tier+": "+tr.CLI+effortLabel(tr))
		}
	}
	return strings.Join(parts, " · ")
}

func effortLabel(tr tierRoute) string {
	if tr.Effort == "" {
		return ""
	}
	return fmt.Sprintf(" effort %s (%s)", tr.Effort, tr.EffortSource)
}

func runCLIRoutingExplain(args []string, stdout, stderr io.Writer) int {
	agent, rest := "", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		agent, rest = args[0], args[1:]
	}
	f, extra, ok := parseCLIRoutingFlags("explain", rest, stderr)
	if !ok || agent == "" || len(extra) > 0 {
		fmt.Fprintln(stderr, "evolve cli-routing explain: "+cliRoutingUsage)
		return exitRoutingUsage
	}
	view, err := loadRoutingView(f)
	if err != nil {
		reportRoutingFindings(stderr, view.findings)
		fmt.Fprintf(stderr, "evolve cli-routing explain: %v\n", err)
		return exitRoutingFinding
	}
	phase := f.phase
	if phase == "" {
		phase = agentPhase(agent, view.catalog)
	}
	route := view.route(agent, phase)
	printExplanation(stdout, routingMode(view.router), route)
	if route.Error != "" {
		return exitRoutingFinding
	}
	return 0
}

func printExplanation(w io.Writer, mode string, a agentRoute) {
	fmt.Fprintf(w, "agent %s, phase %q — %s\n", a.Agent, a.Phase, mode)
	if a.Error != "" {
		fmt.Fprintf(w, "refused: %s\n", a.Error)
		return
	}
	allowed := "every family (the profile does not restrict allowed_clis)"
	if a.Allowed != nil {
		allowed = strings.Join(a.Allowed, ", ")
	}
	fmt.Fprintf(w, "rule:    %s\nallowed: %s\nchain:   %s\ntiers:   %s\n", a.Rule, allowed, strings.Join(a.Chain, " → "), tierSummary(a.Tiers))
	for _, note := range a.Notes {
		fmt.Fprintf(w, "note:    %s\n", note)
	}
}
