package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/detectcli"
)

// runDetectCLI is the `evolve detect-cli` subcommand: ports
// legacy/scripts/dispatch/detect-cli.sh. Prints the matched CLI name
// to stdout (one of: claude, gemini, codex, antigravity, unknown, or
// the EVOLVE_PLATFORM override). `--json` switches to envelope form.
func runDetectCLI(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	jsonMode := false
	for _, a := range args {
		switch a {
		case "--json":
			jsonMode = true
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve detect-cli [--json]")
			return 0
		default:
			fmt.Fprintf(stderr, "evolve detect-cli: unknown flag %q\n", a)
			return 10
		}
	}
	r := detectcli.Detect(detectcli.Options{})
	if jsonMode {
		b, _ := json.Marshal(r)
		fmt.Fprintln(stdout, string(b))
	} else {
		fmt.Fprintln(stdout, r.CLI)
	}
	return 0
}
