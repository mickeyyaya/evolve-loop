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
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
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
