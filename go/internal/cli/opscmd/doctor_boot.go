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

func doctorBootConfig(sandbox bool, ws string, stderr io.Writer) (cfg *bridge.Config, cleanup func(), code int, ok bool) {
	cwd, _ := os.Getwd()
	cfg = &bridge.Config{Workspace: ws, ProjectRoot: cwd}
	cleanup = func() {}
	if !sandbox {
		return cfg, cleanup, 0, true
	}
	wt, err := os.MkdirTemp("", "evolve-doctorboot-wt-*")
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
		return 0
	case bridge.ExitBadFlags:
		fmt.Fprintf(stderr, "[doctor] boot: %q is not a known *-tmux driver\n", driver)
		return 10
	case bridge.ExitModelMismatch:
		fmt.Fprintf(stderr, "[doctor] BOOT WRONG MODEL: %s rc=%d (sandbox=%v) — the REPL booted a model outside the target's model family, or showed no readable model label\n", driver, rc, sandbox)
		if tail := bridge.ScrollbackTail(scrollback, 6); tail != "" {
			fmt.Fprintf(stderr, "[doctor] final pane:\n%s\n", tail)
		}
		return 1
	default:
		fmt.Fprintf(stderr, "[doctor] BOOT FAILED: %s rc=%d (sandbox=%v)\n", driver, rc, sandbox)
		if tail := bridge.ScrollbackTail(scrollback, 12); tail != "" {
			fmt.Fprintf(stderr, "[doctor] final pane:\n%s\n", tail)
		}
		return 1
	}
}

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
