package guardcmd

import (
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/preflight"
)

// runPreflight is the `evolve preflight-environment [--write|--summary|--json]` subcommand.
// Ports legacy/scripts/dispatch/preflight-environment.sh.
func RunPreflight(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	mode := "json"
	write := false
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve preflight-environment [--json|--summary|--write]")
			fmt.Fprintln(stdout, "  --json      Emit JSON to stdout (default)")
			fmt.Fprintln(stdout, "  --summary   Emit human-readable summary")
			fmt.Fprintln(stdout, "  --write     Also persist to .evolve/environment.json")
			return 0
		case "--json":
			mode = "json"
		case "--summary":
			mode = "summary"
		case "--write":
			write = true
		default:
			fmt.Fprintf(stderr, "preflight-environment: unexpected arg: %s\n", a)
			return 10
		}
	}
	// envOrCwd absolutizes a relative $EVOLVE_PROJECT_ROOT (cycle-119 class).
	root := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	pluginRoot := os.Getenv("EVOLVE_PLUGIN_ROOT")
	if pluginRoot == "" {
		pluginRoot = root
	}
	profile := preflight.Probe(preflight.Options{
		ProjectRoot:    root,
		PluginRoot:     pluginRoot,
		WorktreeBase:   policy.WorktreeBaseFor(root),
		SandboxCapable: preflight.MeasuredSandboxCapability,
	})
	if write {
		if err := profile.WriteToFile(root); err != nil {
			fmt.Fprintf(stderr, "[preflight-environment] WARN: could not persist profile: %v\n", err)
		} else {
			fmt.Fprintf(stderr, "[preflight-environment] wrote profile: %s/.evolve/environment.json\n", root)
		}
	}
	switch mode {
	case "json":
		fmt.Fprintln(stdout, profile.PrettyJSON())
	case "summary":
		fmt.Fprint(stdout, profile.Summary())
	}
	return 0
}
