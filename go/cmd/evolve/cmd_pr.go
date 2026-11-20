package main

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/landcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func landDeps() landcmd.Deps {
	return landcmd.Deps{
		ResolveRoot:     loopStopRoot,
		PrintVerbJSON:   printVerbJSON,
		NewSignalCenter: newRoleSignalCenter,
		DarwinSet:       darwinSetVerb.run,
		RefreshWorktree: core.DerivedWorktree,
	}
}

func runPR(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return landcmd.RunPR(args, stdin, stdout, stderr, landDeps())
}
