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
