package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolveloop/go/internal/detectcli"
)

// runDetectCLI is the `evolve detect-cli` subcommand: ports
// legacy/scripts/dispatch/detect-cli.sh. Prints the matched CLI name
// to stdout (one of: claude, gemini, codex, antigravity, unknown, or
// the --platform override). `--json` switches to envelope form.
func runDetectCLI(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	jsonMode := false
	platformOverride := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--json":
			jsonMode = true
		case "--platform":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "evolve detect-cli: --platform missing value")
				return 10
			}
			platformOverride = args[i]
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve detect-cli [--json] [--platform CLI]")
			return 0
		default:
			fmt.Fprintf(stderr, "evolve detect-cli: unknown flag %q\n", a)
			return 10
		}
	}
	r := detectcli.Detect(detectcli.Options{Platform: platformOverride})
	if jsonMode {
		b, _ := json.Marshal(r)
		fmt.Fprintln(stdout, string(b))
	} else {
		fmt.Fprintln(stdout, r.CLI)
	}
	return 0
}
