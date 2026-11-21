package main

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/landcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var prRefreshWorktree = core.DerivedWorktree

func landDeps() landcmd.Deps {
	return landcmd.Deps{
		ResolveRoot:     loopStopRoot,
		PrintVerbJSON:   printVerbJSON,
		NewSignalCenter: newRoleSignalCenter,
		DarwinSet:       darwinSetVerb.run,
		RefreshWorktree: prRefreshWorktree,
	}
}

func runPR(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return landcmd.RunPR(args, stdin, stdout, stderr, landDeps())
}
