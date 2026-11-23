package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const archiveUsage = "evolve archive: usage: archive restore <cycle> [--worktree DIR] [--project-root P]"

func runArchive(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, archiveUsage)
		return 10
	}
	if args[0] != "restore" {
		fmt.Fprintf(stderr, "evolve archive: unknown subcommand %q (want: restore)\n", args[0])
		return 10
	}
	return runArchiveRestore(args[1:], stdout, stderr)
}

func runArchiveRestore(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("archive restore", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rootFlag := fs.String("project-root", "", "project root (default $EVOLVE_PROJECT_ROOT, else .)")
	worktree := fs.String("worktree", "", "directory of the restored worktree (default .evolve/worktrees/<leaf>)")
	cycleArg, flagArgs := splitLeadingArg(args)
	if err := fs.Parse(flagArgs); err != nil {
		return 10
	}
	cycle, err := strconv.Atoi(cycleArg)
	if err != nil || cycle <= 0 || fs.NArg() > 0 {
		fmt.Fprintf(stderr, "%s\nevolve archive restore: want one cycle number, got %q %q\n", archiveUsage, cycleArg, fs.Args())
		return 10
	}
	root := salvageProjectRoot(*rootFlag)
	evolveDir := paths.EvolveDirOf(root)
	o := gc.WorktreeOptions{
		ProjectRoot:  root,
		EvolveDir:    evolveDir,
		WorktreeBase: filepath.Join(evolveDir, "worktrees"),
		Exec:         sysexec.DefaultRunner,
	}
	res, err := gc.RestoreArchive(o, cycle, *worktree)
	if err != nil {
		fmt.Fprintf(stderr, "evolve archive restore: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "restored cycle %d from %s: branch %s at %s in %s\n", cycle, res.Leaf, res.Branch, res.Head, res.Worktree)
	return 0
}

func splitLeadingArg(args []string) (string, []string) {
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' {
		return args[0], args[1:]
	}
	return "", args
}
