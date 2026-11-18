package main

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/inboxcmd"
)

func inboxDeps() inboxcmd.Deps {
	return inboxcmd.Deps{
		QueueGate:          inboxQueueGate,
		LaneForbidden:      laneForbidden,
		PrintVerbJSON:      printVerbJSON,
		HumanWriter:        humanWriter,
		CheckRootHasEvolve: checkRootHasEvolve,
		NewSignalCenter:    newRoleSignalCenter,
	}
}

func runInbox(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return inboxcmd.Run(args, stdin, stdout, stderr, inboxDeps())
}

func runInboxMover(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return inboxcmd.RunMover(args, stdin, stdout, stderr, inboxDeps())
}
