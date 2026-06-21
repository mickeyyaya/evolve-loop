package opscmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// runDoctorBoot implements `evolve doctor boot <driver> [--sandbox] [--json]`:
// a standalone "is my bridge bootable right now?" probe. It really boots the
// driver's REPL (boot-only, no prompt/artifact) via bridge.BootSmokeTest and
// reports whether the marker appeared — the fast operator check that the loop
// readiness gate runs automatically. Exit: 0 booted, 1 boot failed (timeout /
// missing binary), 10 usage (unknown or non-tmux driver).
func runDoctorBoot(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve doctor boot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var sandbox, asJSON bool
	fs.BoolVar(&sandbox, "sandbox", false, "exercise the sandboxed write-phase boot path (worktree + build agent)")
	fs.BoolVar(&asJSON, "json", false, "emit JSON payload")
	if err := fs.Parse(cmdutil.ReorderArgs(args)); err != nil {
		return 10
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "evolve doctor boot: usage: evolve doctor boot <driver> [--sandbox] [--json]")
		return 10
	}
	driver := fs.Arg(0)

	ws, err := os.MkdirTemp("", "evolve-doctorboot-*")
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor boot: temp workspace: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(ws) }()
	cwd, _ := os.Getwd()
	cfg := &bridge.Config{Workspace: ws, ProjectRoot: cwd}
	if sandbox {
		wt, werr := os.MkdirTemp("", "evolve-doctorboot-wt-*")
		if werr != nil {
			fmt.Fprintf(stderr, "evolve doctor boot: temp worktree: %v\n", werr)
			return 1
		}
		defer func() { _ = os.RemoveAll(wt) }()
		cfg.Worktree = wt
		cfg.Agent = "build"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	rc, scrollback := bridge.BootSmokeTest(ctx, driver, cfg, bridge.Deps{Stderr: stderr})

	if asJSON {
		buf, _ := json.MarshalIndent(struct {
			Driver   string `json:"driver"`
			Sandbox  bool   `json:"sandbox"`
			ExitCode int    `json:"exit_code"`
			Booted   bool   `json:"booted"`
		}{driver, sandbox, rc, rc == bridge.ExitOK}, "", "  ")
		fmt.Fprintf(stdout, "%s\n", buf)
	}

	switch rc {
	case bridge.ExitOK:
		fmt.Fprintf(stderr, "[doctor] BOOT OK: %s REPL booted (sandbox=%v)\n", driver, sandbox)
		return 0
	case bridge.ExitBadFlags:
		fmt.Fprintf(stderr, "[doctor] boot: %q is not a known *-tmux driver\n", driver)
		return 10
	default:
		fmt.Fprintf(stderr, "[doctor] BOOT FAILED: %s rc=%d (sandbox=%v)\n", driver, rc, sandbox)
		if tail := bridge.ScrollbackTail(scrollback, 12); tail != "" {
			fmt.Fprintf(stderr, "[doctor] final pane:\n%s\n", tail)
		}
		return 1
	}
}
