package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/bridge"
	"github.com/mickeyyaya/evolveloop/go/internal/bridge/clicontrol"
)

// runBridgeControl implements `evolve bridge control <family> <event>` — the
// live exerciser for the CLI-control mapping table. It resolves the abstract
// event to the family's concrete command (via the manifest controls table),
// drives it through the tmux bridge, and prints the captured pane. Operators use
// it to verify/refine the table (e.g. `bridge control claude usage`) and to
// confirm an unsupported pairing fails cleanly (`bridge control ollama usage`).
func runBridgeControl(args []string, stdout, stderr io.Writer) int {
	var positional []string
	ws := ""
	allowBypass := false
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--workspace="):
			ws = strings.TrimPrefix(a, "--workspace=")
		case a == "--allow-bypass":
			allowBypass = true
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve bridge control <family> <event> --workspace=DIR [--allow-bypass]")
			fmt.Fprintln(stdout, "  family: claude | codex | agy | ollama   event: usage | status | clean_ctx | …")
			return 0
		case strings.HasPrefix(a, "--"):
			fmt.Fprintf(stderr, "evolve bridge control: unknown flag %q\n", a)
			return 10
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 2 {
		fmt.Fprintln(stderr, "evolve bridge control: need <family> <event> (e.g. 'control claude usage')")
		return 10
	}
	family, event := positional[0], positional[1]
	if ws == "" {
		fmt.Fprintln(stderr, "evolve bridge control: --workspace is required (the REPL scratch dir)")
		return 10
	}
	intent := bridge.LaunchIntent{}
	if allowBypass {
		intent.Permission = "bypass"
	}
	cli := family + "-tmux"
	cfg := &bridge.Config{
		CLI: cli, Workspace: ws, Agent: "control",
		AllowBypass: allowBypass, Realization: bridge.RealizeFor(cli, intent),
	}
	ctrl := bridge.NewController(cfg, bridge.Deps{})
	return emitControl(ctrl.Do, family, event, stdout, stderr)
}

// emitControl runs one control event through do and maps the outcome to output
// + exit code. Split from runBridgeControl so the outcome mapping is unit-
// testable without a tmux session. Exit codes: 0 ok, 3 unsupported pairing,
// 1 any other failure.
func emitControl(
	do func(context.Context, string, clicontrol.Event) (clicontrol.Response, error),
	family, event string, stdout, stderr io.Writer,
) int {
	resp, err := do(context.Background(), family, clicontrol.Event(event))
	switch {
	case errors.Is(err, clicontrol.ErrUnsupported):
		fmt.Fprintf(stderr, "evolve bridge control: %s does not support event %q\n", family, event)
		return 3
	case err != nil:
		fmt.Fprintf(stderr, "evolve bridge control: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, resp.Pane)
	if resp.Pane != "" && !strings.HasSuffix(resp.Pane, "\n") {
		fmt.Fprintln(stdout)
	}
	return 0
}
