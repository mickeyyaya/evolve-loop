package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/capabilities"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/recipe"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

const bridgeUsage = `evolve bridge — native-Go multi-CLI agent bridge

Subcommands:
  launch    Run a subagent invocation through the chosen CLI
            ( bridge launch --cli=NAME --profile=PATH --model=M
              --prompt-file=PATH --workspace=DIR --stdout-log=PATH
              --stderr-log=PATH --artifact=PATH [--cycle=N] [--agent=NAME]
              [--worktree=DIR] [--permission-mode=M] [--allow-bypass]
              [--stream-output] [--session-name=NAME] [-- <inner-cli flags>] )
  probe     Detect available CLIs + capability tiers (JSON)
  send      Queue a live command for an already-running agent
            ( bridge send --workspace=DIR --agent=NAME
              [--kind=command|interrupt|nudge|system_rule] [--source=cli]
              <body...> )
  recipe    Drive a scripted slash-command sequence (e.g. plugin-install)
            ( bridge recipe <run|list|show> ... — see 'recipe --help' )
  capabilities  Print a CLI's capability catalog ( --cli=NAME [--json] )
  introspect    Diff a CLI's live /help against its catalog
                ( --cli=NAME [--pane-file=PATH | --workspace=DIR] )
  version   Print the bridge/evolve version
  help      Print this help

Exit codes: 0 ok | 2 safety-gate | 3 cost-leak | 10 bad-flags |
  80 repl-boot-timeout | 81 artifact-timeout | 85 unknown-prompt |
  86 respond-loop-guard | 99 require-full-unmet | 127 missing-binary
`

// runBridge is the `evolve bridge <subcommand>` shim — a thin CLI over
// the in-process bridge.Engine, preserving the historical
// `bridge <subcommand>` surface (launch / probe / version / help).
func runBridge(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, bridgeUsage)
		return 10
	}
	sub, rest := args[0], args[1:]
	eng := bridge.NewEngine(bridge.Deps{})

	switch sub {
	case "launch":
		return eng.LaunchArgs(context.Background(), rest, envMap(), stdout, stderr)

	case "probe":
		filter := ""
		for _, a := range rest {
			switch {
			case strings.HasPrefix(a, "--cli="):
				filter = strings.TrimPrefix(a, "--cli=")
			case a == "--help" || a == "-h":
				fmt.Fprintln(stdout, "Usage: evolve bridge probe [--cli=NAME]")
				return 0
			}
		}
		p, err := eng.Probe(context.Background())
		if err != nil {
			fmt.Fprintf(stderr, "evolve bridge probe: %v\n", err)
			return 1
		}
		emitProbeJSON(stdout, p, filter)
		return 0

	case "report":
		ws, artName := "", "artifact.md"
		for _, a := range rest {
			switch {
			case strings.HasPrefix(a, "--workspace="):
				ws = strings.TrimPrefix(a, "--workspace=")
			case strings.HasPrefix(a, "--artifact-name="):
				artName = strings.TrimPrefix(a, "--artifact-name=")
			case a == "--help" || a == "-h":
				fmt.Fprintln(stdout, "Usage: evolve bridge report --workspace=PATH [--artifact-name=NAME]")
				return 0
			}
		}
		if ws == "" {
			fmt.Fprintln(stderr, "evolve bridge report: --workspace required")
			return 10
		}
		rep, err := bridge.BuildReport(ws, artName, time.Now())
		if err != nil {
			fmt.Fprintf(stderr, "evolve bridge report: %v\n", err)
			return 10
		}
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return 0

	case "validate":
		fmt.Fprintln(stderr, "evolve bridge validate: pass --validate-only to `bridge launch` to dry-validate config")
		return 0

	case "add-rule":
		var cli, regex, keys, policy, name, note string
		for _, a := range rest {
			switch {
			case strings.HasPrefix(a, "--cli="):
				cli = strings.TrimPrefix(a, "--cli=")
			case strings.HasPrefix(a, "--regex="):
				regex = strings.TrimPrefix(a, "--regex=")
			case strings.HasPrefix(a, "--response="):
				keys = strings.TrimPrefix(a, "--response=")
			case strings.HasPrefix(a, "--policy="):
				policy = strings.TrimPrefix(a, "--policy=")
			case strings.HasPrefix(a, "--name="):
				name = strings.TrimPrefix(a, "--name=")
			case strings.HasPrefix(a, "--note="):
				note = strings.TrimPrefix(a, "--note=")
			case a == "--help" || a == "-h":
				fmt.Fprintln(stdout, "Usage: evolve bridge add-rule --cli=NAME --regex=R [--response=KEYS] [--policy=auto_respond|escalate] [--name=N] [--note=...]")
				return 0
			}
		}
		if cli == "" || regex == "" {
			fmt.Fprintln(stderr, "evolve bridge add-rule: --cli and --regex are required")
			return 10
		}
		if policy == "" {
			if keys != "" {
				policy = "auto_respond"
			} else {
				policy = "escalate"
			}
		}
		if name == "" {
			name = fmt.Sprintf("%s_rule_%d", strings.ReplaceAll(cli, "-", "_"), time.Now().Unix())
		}
		if note == "" {
			note = "Added by evolve bridge add-rule"
		}
		path, err := bridge.AddRule(cli, bridge.ManifestPrompt{Name: name, Regex: regex, ResponseKeys: keys, Policy: policy, Note: note})
		if err != nil {
			fmt.Fprintf(stderr, "evolve bridge add-rule: %v\n", err)
			return 10
		}
		fmt.Fprintf(stdout, "appended rule %q to %s\n", name, path)
		return 0

	case "send":
		ws, agent, kind, source := "", "", "command", "cli"
		var body []string
		for _, a := range rest {
			switch {
			case strings.HasPrefix(a, "--workspace="):
				ws = strings.TrimPrefix(a, "--workspace=")
			case strings.HasPrefix(a, "--agent="):
				agent = strings.TrimPrefix(a, "--agent=")
			case strings.HasPrefix(a, "--kind="):
				kind = strings.TrimPrefix(a, "--kind=")
			case strings.HasPrefix(a, "--source="):
				source = strings.TrimPrefix(a, "--source=")
			case a == "--help" || a == "-h":
				fmt.Fprintln(stdout, "Usage: evolve bridge send --workspace=DIR --agent=NAME [--kind=command|interrupt|nudge|system_rule] [--source=cli] <body...>")
				return 0
			case strings.HasPrefix(a, "--"):
				fmt.Fprintf(stderr, "evolve bridge send: unknown flag %q\n", a)
				return 10
			default:
				body = append(body, a)
			}
		}
		if ws == "" || agent == "" || len(body) == 0 {
			fmt.Fprintln(stderr, "evolve bridge send: --workspace, --agent, and a body are required")
			return 10
		}
		if !inbox.Kind(kind).Valid() {
			fmt.Fprintf(stderr, "evolve bridge send: invalid --kind %q (command|interrupt|nudge|system_rule|keystroke)\n", kind)
			return 10
		}
		env := inbox.Envelope{Kind: inbox.Kind(kind), Body: strings.Join(body, " "), Source: source}
		if err := inbox.Append(ws, agent, env, time.Now); err != nil {
			fmt.Fprintf(stderr, "evolve bridge send: %v\n", err)
			return 10
		}
		fmt.Fprintf(stdout, "queued %s envelope to %s\n", kind, inbox.Path(ws, agent))
		return 0

	case "recipe":
		return runBridgeRecipe(rest, stdout, stderr)

	case "capabilities":
		return runBridgeCapabilities(rest, stdout, stderr)

	case "introspect":
		return runBridgeIntrospect(rest, stdout, stderr)

	case "doctor":
		filter, deep, jsonMode := "", false, false
		for _, a := range rest {
			switch {
			case strings.HasPrefix(a, "--cli="):
				filter = strings.TrimPrefix(a, "--cli=")
			case a == "--deep":
				deep = true
			case a == "--json":
				jsonMode = true
			case a == "--help" || a == "-h":
				fmt.Fprintln(stdout, "Usage: evolve bridge doctor [--cli=NAME] [--deep] [--json]")
				return 0
			}
		}
		rep, code := eng.Doctor(context.Background(), filter, deep)
		if jsonMode {
			b, _ := json.MarshalIndent(rep, "", "  ")
			fmt.Fprintln(stdout, string(b))
		} else {
			for _, r := range rep.Results {
				fmt.Fprintf(stdout, "%-14s %-8s %s\n", r.CLI, r.Verdict, r.Auth.Source)
			}
			fmt.Fprintf(stdout, "summary: ready=%d warning=%d blocked=%d\n", rep.Summary.Ready, rep.Summary.Warning, rep.Summary.Blocked)
		}
		return code

	case "billing":
		if len(rest) == 0 {
			fmt.Fprintln(stderr, "Usage: evolve bridge billing snapshot DIR LABEL | compare BEFORE AFTER")
			return 10
		}
		switch rest[0] {
		case "snapshot":
			if len(rest) < 3 {
				fmt.Fprintln(stderr, "Usage: evolve bridge billing snapshot DIR LABEL")
				return 10
			}
			p, err := eng.BillingSnapshot(rest[1], rest[2])
			if err != nil {
				fmt.Fprintf(stderr, "evolve bridge billing: %v\n", err)
				return 10
			}
			fmt.Fprintln(stdout, p)
			return 0
		case "compare":
			if len(rest) < 3 {
				fmt.Fprintln(stderr, "Usage: evolve bridge billing compare BEFORE AFTER")
				return 10
			}
			verdict, code := bridge.BillingCompare(rest[1], rest[2])
			fmt.Fprintln(stdout, verdict)
			return code
		default:
			fmt.Fprintf(stderr, "evolve bridge billing: unknown subcommand %q\n", rest[0])
			return 10
		}

	case "version", "-v", "--version":
		fmt.Fprintln(stdout, version.Get())
		return 0

	case "help", "-h", "--help":
		fmt.Fprint(stdout, bridgeUsage)
		return 0

	default:
		fmt.Fprintf(stderr, "evolve bridge: unknown subcommand %q\n\n%s", sub, bridgeUsage)
		return 10
	}
}

// emitProbeJSON renders a BridgeProbe in the {os, results:[{cli,tier}]}
// shape the historical `bridge probe` emitted (consumed by the adapter).
func emitProbeJSON(stdout io.Writer, p core.BridgeProbe, filter string) {
	type result struct {
		CLI  string `json:"cli"`
		Tier string `json:"tier"`
	}
	clis := make([]string, 0, len(p.CLIs))
	for cli := range p.CLIs {
		clis = append(clis, cli)
	}
	sort.Strings(clis)
	results := make([]result, 0, len(clis))
	for _, cli := range clis {
		if filter != "" && cli != filter {
			continue
		}
		results = append(results, result{CLI: cli, Tier: p.CLIs[cli]})
	}
	out := struct {
		OS      string   `json:"os"`
		Results []result `json:"results"`
	}{OS: p.Version, Results: results}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(stdout, string(b))
}

const recipeUsage = `Usage: evolve bridge recipe <run|list|show> [args]

  recipe list                 List available recipes
  recipe show <name>          Print a recipe definition (JSON)
  recipe run <name> --cli=CLI --workspace=DIR [--param=k=v ...]
            [--agent=NAME] [--session-name=NAME] [--worktree=DIR]
            [--permission-mode=M] [--allow-bypass]

Drives a scripted, multi-step interactive slash-command sequence (e.g.
plugin-install) through the CLI's tmux REPL. Independent of the cycle loop.
`

// runBridgeRecipe implements `evolve bridge recipe <run|list|show>`. It keeps
// the bridge independently drivable — an operator or the orchestrator can
// install a plugin or run any scripted sequence with just a CLI + workspace.
func runBridgeRecipe(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, recipeUsage)
		return 10
	}
	action, rest := args[0], args[1:]
	switch action {
	case "list":
		for _, n := range recipe.RecipeNames() {
			fmt.Fprintln(stdout, n)
		}
		return 0
	case "show":
		if len(rest) == 0 {
			fmt.Fprintln(stderr, "evolve bridge recipe show: recipe name required")
			return 10
		}
		r, err := recipe.LoadRecipe(rest[0])
		if err != nil {
			fmt.Fprintf(stderr, "evolve bridge recipe show: %v\n", err)
			return 10
		}
		b, _ := json.MarshalIndent(r, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return 0
	case "run":
		return runBridgeRecipeRun(rest, stdout, stderr)
	case "--help", "-h":
		fmt.Fprint(stdout, recipeUsage)
		return 0
	default:
		fmt.Fprintf(stderr, "evolve bridge recipe: unknown action %q\n", action)
		return 10
	}
}

func runBridgeRecipeRun(args []string, stdout, stderr io.Writer) int {
	var name, cli, ws, agent, session, worktree, permMode string
	allowBypass := false
	params := map[string]string{}
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--cli="):
			cli = strings.TrimPrefix(a, "--cli=")
		case strings.HasPrefix(a, "--workspace="):
			ws = strings.TrimPrefix(a, "--workspace=")
		case strings.HasPrefix(a, "--agent="):
			agent = strings.TrimPrefix(a, "--agent=")
		case strings.HasPrefix(a, "--session-name="):
			session = strings.TrimPrefix(a, "--session-name=")
		case strings.HasPrefix(a, "--worktree="):
			worktree = strings.TrimPrefix(a, "--worktree=")
		case strings.HasPrefix(a, "--permission-mode="):
			permMode = strings.TrimPrefix(a, "--permission-mode=")
		case a == "--allow-bypass":
			allowBypass = true
		case strings.HasPrefix(a, "--param="):
			k, v, ok := strings.Cut(strings.TrimPrefix(a, "--param="), "=")
			if !ok {
				fmt.Fprintf(stderr, "evolve bridge recipe run: --param must be k=v, got %q\n", a)
				return 10
			}
			params[k] = v
		case a == "--help" || a == "-h":
			fmt.Fprint(stdout, recipeUsage)
			return 0
		case strings.HasPrefix(a, "--"):
			fmt.Fprintf(stderr, "evolve bridge recipe run: unknown flag %q\n", a)
			return 10
		default:
			if name == "" {
				name = a
			} else {
				fmt.Fprintf(stderr, "evolve bridge recipe run: unexpected argument %q\n", a)
				return 10
			}
		}
	}
	if name == "" || cli == "" || ws == "" {
		fmt.Fprintln(stderr, "evolve bridge recipe run: <name>, --cli, and --workspace are required")
		return 10
	}
	if agent == "" {
		agent = "recipe"
	}

	intent := bridge.LaunchIntent{Permission: permMode}
	if allowBypass && permMode == "" {
		intent.Permission = "bypass"
	}
	cfg := &bridge.Config{
		CLI:         cli,
		Workspace:   ws,
		Agent:       agent,
		SessionName: session,
		Worktree:    worktree,
		AllowBypass: allowBypass,
		Realization: bridge.RealizeFor(cli, intent),
	}
	res, err := bridge.RunRecipe(context.Background(), cfg, bridge.Deps{}, cli, name, params)
	emitRecipeResult(stdout, res)
	if err != nil {
		fmt.Fprintf(stderr, "evolve bridge recipe run: %v\n", err)
		return 1
	}
	return 0
}

// emitRecipeResult prints a recipe run's per-step outcome as JSON.
func emitRecipeResult(stdout io.Writer, res recipe.Result) {
	b, _ := json.MarshalIndent(res, "", "  ")
	fmt.Fprintln(stdout, string(b))
}

// runBridgeCapabilities implements `evolve bridge capabilities --cli=X [--json]`
// — print the static, research-grounded capability catalog for a CLI.
func runBridgeCapabilities(args []string, stdout, stderr io.Writer) int {
	cli, jsonMode := "", false
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--cli="):
			cli = strings.TrimPrefix(a, "--cli=")
		case a == "--json":
			jsonMode = true
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve bridge capabilities --cli=NAME [--json]")
			return 0
		case strings.HasPrefix(a, "--"):
			fmt.Fprintf(stderr, "evolve bridge capabilities: unknown flag %q\n", a)
			return 10
		}
	}
	if cli == "" {
		fmt.Fprintln(stderr, "evolve bridge capabilities: --cli is required")
		return 10
	}
	cat, err := capabilities.LoadCatalog(cli)
	if err != nil {
		fmt.Fprintf(stderr, "evolve bridge capabilities: %v\n", err)
		return 10
	}
	if jsonMode {
		b, _ := json.MarshalIndent(cat, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return 0
	}
	emitCatalogText(stdout, cat)
	return 0
}

func emitCatalogText(stdout io.Writer, cat capabilities.Catalog) {
	fmt.Fprintf(stdout, "%s (%s)\n", cat.DisplayName, cat.CLI)
	fmt.Fprintf(stdout, "Extension: %s — %s\n", cat.Extension.Kind, cat.Extension.Summary)
	if len(cat.Extension.InstallFlow) > 0 {
		fmt.Fprintln(stdout, "Install flow:")
		for _, s := range cat.Extension.InstallFlow {
			fmt.Fprintf(stdout, "  - %s\n", s)
		}
	}
	fmt.Fprintf(stdout, "Slash commands (%d):\n", len(cat.SlashCommands))
	for _, c := range cat.SlashCommands {
		fmt.Fprintf(stdout, "  %-18s %s\n", c.Name, c.Purpose)
	}
	if cat.Headless.Entrypoint != "" {
		fmt.Fprintf(stdout, "Headless: %s\n", cat.Headless.Entrypoint)
	}
}

// runBridgeIntrospect implements `evolve bridge introspect --cli=X`. With
// --pane-file it diffs a captured /help pane against the catalog offline
// (no tmux); otherwise it drives the live REPL to capture /help itself.
func runBridgeIntrospect(args []string, stdout, stderr io.Writer) int {
	cli, paneFile, ws, session := "", "", "", ""
	allowBypass := false
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--cli="):
			cli = strings.TrimPrefix(a, "--cli=")
		case strings.HasPrefix(a, "--pane-file="):
			paneFile = strings.TrimPrefix(a, "--pane-file=")
		case strings.HasPrefix(a, "--workspace="):
			ws = strings.TrimPrefix(a, "--workspace=")
		case strings.HasPrefix(a, "--session-name="):
			session = strings.TrimPrefix(a, "--session-name=")
		case a == "--allow-bypass":
			allowBypass = true
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve bridge introspect --cli=NAME [--pane-file=PATH] [--workspace=DIR] [--session-name=NAME] [--allow-bypass]")
			return 0
		case strings.HasPrefix(a, "--"):
			fmt.Fprintf(stderr, "evolve bridge introspect: unknown flag %q\n", a)
			return 10
		}
	}
	if cli == "" {
		fmt.Fprintln(stderr, "evolve bridge introspect: --cli is required")
		return 10
	}
	cat, err := capabilities.LoadCatalog(cli)
	if err != nil {
		fmt.Fprintf(stderr, "evolve bridge introspect: %v\n", err)
		return 10
	}

	var pane string
	if paneFile != "" {
		b, rerr := os.ReadFile(paneFile)
		if rerr != nil {
			fmt.Fprintf(stderr, "evolve bridge introspect: %v\n", rerr)
			return 10
		}
		pane = string(b)
	} else {
		if ws == "" {
			fmt.Fprintln(stderr, "evolve bridge introspect: --workspace is required for live capture (or pass --pane-file)")
			return 10
		}
		intent := bridge.LaunchIntent{}
		if allowBypass {
			intent.Permission = "bypass"
		}
		cfg := &bridge.Config{
			CLI: cli, Workspace: ws, Agent: "introspect", SessionName: session,
			AllowBypass: allowBypass, Realization: bridge.RealizeFor(cli, intent),
		}
		pane, err = bridge.CaptureHelp(context.Background(), cfg, bridge.Deps{}, cli)
		if err != nil {
			fmt.Fprintf(stderr, "evolve bridge introspect: %v\n", err)
			return 1
		}
	}

	drift := capabilities.Diff(cat, capabilities.ParseHelp(pane))
	b, _ := json.MarshalIndent(drift, "", "  ")
	fmt.Fprintln(stdout, string(b))
	if !drift.Clean() {
		return 3 // drift detected — non-fatal, but a distinct exit code
	}
	return 0
}

// envMap snapshots the process environment as a map for LaunchArgs's
// BRIDGE_* fallbacks.
func envMap() map[string]string {
	env := os.Environ()
	m := make(map[string]string, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i > 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}
