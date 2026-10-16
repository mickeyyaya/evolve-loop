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

var doctorBootWorktreeTemp = os.MkdirTemp

func doctorBootConfig(sandbox bool, ws string, stderr io.Writer) (cfg *bridge.Config, cleanup func(), code int, ok bool) {
	cwd, _ := os.Getwd()
	cfg = &bridge.Config{Workspace: ws, ProjectRoot: cwd}
	cleanup = func() {}
	if !sandbox {
		return cfg, cleanup, 0, true
	}
	wt, err := doctorBootWorktreeTemp("", "evolve-doctorboot-wt-*")
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor boot: temp worktree: %v\n", err)
		return nil, cleanup, 1, false
	}
	cfg.Worktree = wt
	cfg.Agent = "build"
	return cfg, func() { _ = os.RemoveAll(wt) }, 0, true
}

func reportDoctorBootResult(driver string, sandbox, asJSON bool, rc int, scrollback string, stdout, stderr io.Writer) int {
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
		return exitOK
	case bridge.ExitBadFlags:
		fmt.Fprintf(stderr, "[doctor] boot: %q is not a known *-tmux driver\n", driver)
		return exitUsage
	case bridge.ExitModelMismatch:
		fmt.Fprintf(stderr, "[doctor] BOOT WRONG MODEL: %s rc=%d (sandbox=%v) — the REPL booted a model outside the target's model family, or showed no readable model label\n", driver, rc, sandbox)
		if tail := bridge.ScrollbackTail(scrollback, 6); tail != "" {
			fmt.Fprintf(stderr, "[doctor] final pane:\n%s\n", tail)
		}
		return exitFail
	default:
		fmt.Fprintf(stderr, "[doctor] BOOT FAILED: %s rc=%d (sandbox=%v)\n", driver, rc, sandbox)
		if tail := bridge.ScrollbackTail(scrollback, 12); tail != "" {
			fmt.Fprintf(stderr, "[doctor] final pane:\n%s\n", tail)
		}
		return exitFail
	}
}

func runDoctorBoot(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve doctor boot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var sandbox, asJSON bool
	fs.BoolVar(&sandbox, "sandbox", false, "exercise the sandboxed write-phase boot path (worktree + build agent)")
	fs.BoolVar(&asJSON, "json", false, "emit JSON payload")
	if err := fs.Parse(cmdutil.ReorderArgs(args)); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "evolve doctor boot: usage: evolve doctor boot <driver> [--sandbox] [--json]")
		return exitUsage
	}
	driver := fs.Arg(0)

	ws, err := os.MkdirTemp("", "evolve-doctorboot-*")
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor boot: temp workspace: %v\n", err)
		return exitFail
	}
	defer func() { _ = os.RemoveAll(ws) }()

	cfg, cleanup, code, ok := doctorBootConfig(sandbox, ws, stderr)
	defer cleanup()
	if !ok {
		return code
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	rc, scrollback := bridge.BootSmokeTest(ctx, driver, cfg, bridge.Deps{Stderr: stderr})

	return reportDoctorBootResult(driver, sandbox, asJSON, rc, scrollback, stdout, stderr)
}
