package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func runSyncMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve sync-main", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot string
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if projectRoot == "" {
		projectRoot = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if projectRoot == "" {
		var err error
		if projectRoot, err = os.Getwd(); err != nil {
			fmt.Fprintf(stderr, "evolve sync-main: cwd: %v\n", err)
			return 1
		}
	}
	absRoot := paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve sync-main: WARN: %s\n", m)
	})

	git := func(gitArgs ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", absRoot}, gitArgs...)...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	if ws := liveLeaseWorkspace(absRoot); ws != "" {
		if lease, ok, _ := runlease.Read(ws); ok && runlease.OwnerLive(lease, time.Now(), 0, pidAlive) {
			fmt.Fprintf(stderr, "evolve sync-main: refused — a run lease is live (pid %d, heartbeat fresh); another evolve loop owns this tree.\n", lease.OwnerPID)
			fmt.Fprintln(stderr, "evolve sync-main:   • let it finish, or `evolve loop --resume` to attach, then retry.")
			return 1
		}
	}

	porcelain, err := git("status", "--porcelain")
	if err != nil {
		fmt.Fprintf(stderr, "evolve sync-main: git status failed: %v\n%s", err, porcelain)
		return 1
	}
	if strings.TrimSpace(porcelain) != "" {
		fmt.Fprintf(stderr, "evolve sync-main: refused — working tree is dirty; commit or stash first:\n%s", porcelain)
		return 1
	}

	branch, err := git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		fmt.Fprintf(stderr, "evolve sync-main: cannot resolve current branch: %v\n%s", err, branch)
		return 1
	}
	branch = strings.TrimSpace(branch)

	if out, err := git("fetch", "origin"); err != nil {
		fmt.Fprintf(stderr, "evolve sync-main: git fetch origin failed: %v\n%s", err, out)
		return 1
	}

	if out, err := git("merge", "--no-edit", "origin/"+branch); err != nil {
		if _, aerr := git("merge", "--abort"); aerr != nil {
			fmt.Fprintf(stderr, "evolve sync-main: merge conflicted AND abort failed: %v\n%s", aerr, out)
			return 1
		}
		fmt.Fprintf(stderr, "evolve sync-main: refused — merging origin/%s conflicts with local history (aborted cleanly, tree unchanged).\n", branch)
		fmt.Fprintln(stderr, "evolve sync-main:   • resolve manually, or re-audit the local commit on the new base.")
		return 1
	}

	fmt.Fprintf(stdout, "sync-main: reconciled local %s with origin/%s (merge only; nothing pushed).\n", branch, branch)
	return 0
}

// Matches gc.discover's minimal cycle_id/workspace schema rather than
// depending on the heavier core loader.
func liveLeaseWorkspace(root string) string {
	b, err := os.ReadFile(filepath.Join(root, ".evolve", "cycle-state.json"))
	if err != nil {
		return ""
	}
	var cs struct {
		WorkspacePath string `json:"workspace_path"`
	}
	if json.Unmarshal(b, &cs) != nil {
		return ""
	}
	return cs.WorkspacePath
}
