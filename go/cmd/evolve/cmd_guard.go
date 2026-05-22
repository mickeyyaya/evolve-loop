package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

// runGuard implements `evolve guard <name>` — reads tool_input JSON from
// stdin, dispatches to the named guard, and exits 0 on Allow or 2 on Deny.
// Mirrors the bash hook contract (scripts/guards/*.sh).
func runGuard(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve guard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var evolveDir string
	fs.StringVar(&evolveDir, "evolve-dir", ".evolve", "path to .evolve/ state directory")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "evolve guard: usage: evolve guard <name> [--evolve-dir DIR] < stdin.json")
		return 10
	}
	name := fs.Arg(0)
	in, err := readGuardInput(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "evolve guard %s: stdin parse: %v\n", name, err)
		return 10
	}
	g, err := buildGuard(name, evolveDir)
	if err != nil {
		fmt.Fprintf(stderr, "evolve guard: %v\n", err)
		return 10
	}
	dec := g.Decide(context.Background(), in)
	payload := map[string]any{"guard": name, "allow": dec.Allow, "reason": dec.Reason}
	if buf, mErr := json.Marshal(payload); mErr == nil {
		fmt.Fprintf(stdout, "%s\n", buf)
	}
	if !dec.Allow {
		if dec.Reason != "" {
			fmt.Fprintf(stderr, "[guard:%s] DENY: %s\n", name, dec.Reason)
		}
		return 2
	}
	return 0
}

func readGuardInput(r io.Reader) (core.GuardInput, error) {
	var in core.GuardInput
	if r == nil {
		return in, nil
	}
	buf, err := io.ReadAll(r)
	if err != nil {
		return in, fmt.Errorf("read stdin: %w", err)
	}
	if len(buf) == 0 {
		return in, nil
	}
	var raw struct {
		ToolName  string         `json:"tool_name"`
		ToolInput map[string]any `json:"tool_input"`
		CWD       string         `json:"cwd"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return in, fmt.Errorf("unmarshal: %w", err)
	}
	in.ToolName = raw.ToolName
	in.ToolInput = raw.ToolInput
	in.CWD = raw.CWD
	return in, nil
}

func buildGuard(name, evolveDir string) (core.Guard, error) {
	switch name {
	case "ship":
		return guards.NewShip(), nil
	case "phase":
		return guards.NewPhase(storage.New(evolveDir)), nil
	case "role":
		return guards.NewRole(storage.New(evolveDir)), nil
	case "docdelete":
		return guards.NewDocDelete(nil), nil
	case "quota":
		return guards.NewQuota(guards.QuotaConfig{}), nil
	case "chain":
		return guards.NewChain(ledger.New(evolveDir)), nil
	default:
		return nil, fmt.Errorf("unknown guard %q (known: ship phase role docdelete quota chain)", name)
	}
}

