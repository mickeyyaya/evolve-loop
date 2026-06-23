package opscmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mickeyyaya/evolveloop/go/cmd/evolve/cmdutil"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/bridge"
)

// runDoctorLive implements `evolve doctor live <driver> [--json]`: a REAL
// launch that submits one trivial prompt via bridge.LiveSmokeTest — the only
// probe shape that can see a provider quota wall (boot smoke passes against a
// rate-limited CLI; the wall appears only after work is submitted —
// cycle-283). Exit: 0 healthy, 1 walled (the escalation pattern is printed,
// e.g. rate_limit) or failed, 10 usage (unknown or non-tmux driver).
func runDoctorLive(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve doctor live", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	fs.BoolVar(&asJSON, "json", false, "emit JSON payload")
	if err := fs.Parse(cmdutil.ReorderArgs(args)); err != nil {
		return 10
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "evolve doctor live: usage: evolve doctor live <driver> [--json]")
		return 10
	}
	driver := fs.Arg(0)

	ws, err := os.MkdirTemp("", "evolve-doctorlive-*")
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor live: temp workspace: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(ws) }()
	cwd, _ := os.Getwd()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	rc, pattern, scrollback := bridge.LiveSmokeTest(ctx, driver,
		&bridge.Config{Workspace: ws, ProjectRoot: cwd}, bridge.Deps{Stderr: stderr})

	if asJSON {
		buf, _ := json.MarshalIndent(struct {
			Driver   string `json:"driver"`
			ExitCode int    `json:"exit_code"`
			Healthy  bool   `json:"healthy"`
			Pattern  string `json:"pattern,omitempty"`
		}{driver, rc, rc == bridge.ExitOK, pattern}, "", "  ")
		fmt.Fprintf(stdout, "%s\n", buf)
	}

	switch {
	case rc == bridge.ExitOK:
		fmt.Fprintf(stderr, "[doctor] LIVE OK: %s answered the probe\n", driver)
		return 0
	case rc == bridge.ExitBadFlags:
		fmt.Fprintf(stderr, "[doctor] live: %q is not a known *-tmux driver\n", driver)
		return 10
	case pattern != "":
		fmt.Fprintf(stderr, "[doctor] LIVE WALLED: %s rc=%d pattern=%s\n", driver, rc, pattern)
		if tail := bridge.ScrollbackTail(scrollback, 6); tail != "" {
			fmt.Fprintf(stderr, "[doctor] final pane:\n%s\n", tail)
		}
		return 1
	default:
		fmt.Fprintf(stderr, "[doctor] LIVE FAILED: %s rc=%d\n", driver, rc)
		if tail := bridge.ScrollbackTail(scrollback, 12); tail != "" {
			fmt.Fprintf(stderr, "[doctor] final pane:\n%s\n", tail)
		}
		return 1
	}
}
