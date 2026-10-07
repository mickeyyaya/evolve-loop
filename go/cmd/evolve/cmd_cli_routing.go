package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

const (
	cliRoutingUsage    = "usage: evolve cli-routing show [--static] [--json] | check | explain <agent> [--phase P] [--static] | init --clis a,b | set <key> <a,b> [--model T] | unset <key> | migrate [--dry-run]  (all take --project-root DIR)"
	exitRoutingUsage   = 2
	exitRoutingFinding = 1
)

type cliRoutingFlags struct {
	root   string
	phase  string
	static bool
	json   bool
}

type tierRoute struct {
	Tier     string   `json:"tier"`
	CLI      string   `json:"cli,omitempty"`
	Model    string   `json:"model,omitempty"`
	Filtered []string `json:"filtered,omitempty"`
}

type agentRoute struct {
	Agent   string      `json:"agent"`
	Phase   string      `json:"phase,omitempty"`
	Rule    string      `json:"rule,omitempty"`
	Chain   []string    `json:"chain,omitempty"`
	Tiers   []tierRoute `json:"tiers,omitempty"`
	Allowed []string    `json:"allowed,omitempty"`
	Notes   []string    `json:"notes,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type routingReport struct {
	Mode     string             `json:"mode"`
	Static   bool               `json:"static"`
	Benched  []string           `json:"benched,omitempty"`
	Agents   []agentRoute       `json:"agents"`
	Findings []cliroute.Finding `json:"findings,omitempty"`
}

type routingView struct {
	root     string
	router   *cliroute.Router
	catalog  phasespec.Catalog
	findings []cliroute.Finding
	benched  map[string]bool
	static   bool
}

func runCLIRouting(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve cli-routing: "+cliRoutingUsage)
		return exitRoutingUsage
	}
	switch args[0] {
	case "show":
		return runCLIRoutingShow(args[1:], stdout, stderr)
	case "check":
		return runCLIRoutingCheck(args[1:], stdout, stderr)
	case "explain":
		return runCLIRoutingExplain(args[1:], stdout, stderr)
	case "init", "set", "unset", "migrate":
		return runCLIRoutingWrite(args[0], args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprintln(stdout, "evolve cli-routing: "+cliRoutingUsage)
		return 0
	}
	fmt.Fprintf(stderr, "evolve cli-routing: unknown subcommand %q (%s)\n", args[0], cliRoutingUsage)
	return exitRoutingUsage
}

func parseCLIRoutingFlags(name string, args []string, stderr io.Writer) (cliRoutingFlags, []string, bool) {
	var f cliRoutingFlags
	fs := flag.NewFlagSet("cli-routing "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.root, "project-root", "", "project root (default: EVOLVE_PROJECT_ROOT or the cwd)")
	fs.StringVar(&f.phase, "phase", "", "explain: the phase the agent dispatches for (default: its own phase)")
	fs.BoolVar(&f.static, "static", false, "resolve without the host: no probe, no bench, no discovered tail")
	fs.BoolVar(&f.json, "json", false, "show: print the report as JSON")
	if err := fs.Parse(args); err != nil {
		return f, nil, false
	}
	root, err := routingProjectRoot(f.root, os.Getwd)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cli-routing %s: %v\n", name, err)
		return f, nil, false
	}
	f.root = root
	return f, fs.Args(), true
}

func routingProjectRoot(flagged string, getwd func() (string, error)) (string, error) {
	if flagged != "" {
		return flagged, nil
	}
	if env := os.Getenv("EVOLVE_PROJECT_ROOT"); env != "" {
		return env, nil
	}
	wd, err := getwd()
	if err != nil {
		return "", fmt.Errorf("cannot read the working directory (%v): pass --project-root", err)
	}
	return wd, nil
}

func loadRoutingView(f cliRoutingFlags) (routingView, error) {
	evolveDir := filepath.Join(f.root, ".evolve")
	bridge.SetModelCatalogDirFn(func() string { return evolveDir })
	host := routingHost(io.Discard, time.Now)
	view := routingView{root: f.root, catalog: routingCatalog(f.root), static: f.static, benched: benchedFamilies(f.root)}
	if f.static {
		host, view.benched = cliroute.Host{LookPath: everyBinaryPresent}, nil
	}
	router, findings, err := buildCLIRouter(f.root, view.catalog, host)
	view.router, view.findings = router, findings
	return view, err
}

func everyBinaryPresent(bin string) (string, error) { return bin, nil }

func runCLIRoutingCheck(args []string, stdout, stderr io.Writer) int {
	f, rest, ok := parseCLIRoutingFlags("check", args, stderr)
	if !ok || len(rest) > 0 {
		fmt.Fprintln(stderr, "evolve cli-routing check: "+cliRoutingUsage)
		return exitRoutingUsage
	}
	f.static = true
	view, err := loadRoutingView(f)
	reportRoutingFindings(stdout, view.findings)
	if err != nil {
		fmt.Fprintf(stdout, "cli-routing check: REFUSED: %v\n", err)
		return exitRoutingFinding
	}
	fmt.Fprintf(stdout, "cli-routing check: OK — the table compiles (%s, %d warning(s))\n", routingMode(view.router), len(view.findings))
	return 0
}

func routingMode(r *cliroute.Router) string {
	if r.Policy().CLIRouting != nil {
		return "declared cli_routing table"
	}
	return "legacy projection, no cli_routing block"
}

func runCLIRoutingShow(args []string, stdout, stderr io.Writer) int {
	f, rest, ok := parseCLIRoutingFlags("show", args, stderr)
	if !ok || len(rest) > 0 {
		fmt.Fprintln(stderr, "evolve cli-routing show: "+cliRoutingUsage)
		return exitRoutingUsage
	}
	view, err := loadRoutingView(f)
	if err != nil {
		reportRoutingFindings(stderr, view.findings)
		fmt.Fprintf(stderr, "evolve cli-routing show: %v\n", err)
		return exitRoutingFinding
	}
	report, err := view.report()
	if err != nil {
		fmt.Fprintf(stderr, "evolve cli-routing show: %v\n", err)
		return exitRoutingFinding
	}
	if f.json {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "evolve cli-routing show: %v\n", err)
			return exitRoutingFinding
		}
		fmt.Fprintln(stdout, string(raw))
		return 0
	}
	printRoutingReport(stdout, report)
	return 0
}

func (v routingView) report() (routingReport, error) {
	agents, err := profiles.NewFromDir(routingProfilesDir(v.root)).List()
	if err != nil {
		return routingReport{}, err
	}
	sort.Strings(agents)
	report := routingReport{Mode: routingMode(v.router), Static: v.static, Findings: v.findings, Benched: sortedKeysOf(v.benched)}
	for _, agent := range agents {
		report.Agents = append(report.Agents, v.route(agent, agentPhase(agent, v.catalog)))
	}
	return report, nil
}

func (v routingView) route(agent, phase string) agentRoute {
	route := agentRoute{Agent: agent, Phase: phase}
	if profiles.IsClaudeFamilyFloor(agent) {
		route.Notes = append(route.Notes, "FLOOR")
	}
	d, err := v.router.Resolve(cliroute.Request{Agent: agent, Phase: phase, ProjectRoot: v.root, DefaultModel: unsetDispatchTier})
	if err != nil {
		route.Error = err.Error()
		return route
	}
	route.Rule, route.Chain, route.Allowed = d.Rule, d.Plan.Candidates, d.Allowed
	route.Tiers = tierRoutes(d.Plan)
	route.Notes = append(route.Notes, d.Trace...)
	route.Notes = append(route.Notes, v.agentNotes(agent, route)...)
	return route
}

const unsetDispatchTier = "balanced"

func tierRoutes(plan llmroute.Plan) []tierRoute {
	tiers := plan.Tiers
	if len(tiers) == 0 {
		tiers = []string{plan.Model}
	}
	out := make([]tierRoute, 0, len(tiers))
	for _, tier := range tiers {
		tr := tierRoute{Tier: tier}
		for _, cli := range plan.Candidates {
			switch {
			case !plan.Permits(cli, tier):
				tr.Filtered = append(tr.Filtered, cli)
			case tr.CLI == "":
				tr.CLI = cli
				tr.Model, _ = resolveModelTier(cli, policy.TierName(policy.TierRank(tier)))
			}
		}
		out = append(out, tr)
	}
	return out
}

func (v routingView) agentNotes(agent string, route agentRoute) []string {
	var notes []string
	for _, tr := range route.Tiers {
		if len(tr.Filtered) > 0 {
			notes = append(notes, fmt.Sprintf("ceiling: %s drops %v", tr.Tier, tr.Filtered))
		}
	}
	for _, f := range v.findings {
		if findingNamesAgent(f.Key, agent) {
			notes = append(notes, fmt.Sprintf("%s %s: %s", f.Severity, f.Key, f.Message))
		}
	}
	for _, cli := range route.Chain {
		if v.benched[llmroute.Family(cli)] {
			notes = append(notes, "host: "+llmroute.Family(cli)+" benched (demoted)")
		}
	}
	return notes
}

func findingNamesAgent(key, agent string) bool {
	if key == "agent."+agent || strings.HasPrefix(key, "agent."+agent+".") || key == "profiles."+agent {
		return true
	}
	pair, isPair := strings.CutPrefix(key, "cross_family_with.")
	return isPair && slices.Contains(strings.Split(pair, "+"), agent)
}

func agentPhase(agent string, cat phasespec.Catalog) string {
	var phases []string
	for _, c := range phasecontract.Contracts() {
		if c.AgentName == agent {
			phases = append(phases, c.Phase)
		}
	}
	sort.Strings(phases)
	switch {
	case slices.Contains(phases, agent):
		return agent
	case len(phases) > 0:
		return phases[0]
	}
	if _, ok := cat.Get(agent); ok {
		return agent
	}
	return ""
}

func sortedKeysOf(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
