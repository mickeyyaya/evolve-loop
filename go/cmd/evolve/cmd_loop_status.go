package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dashboard"
)

func isLoopStatusInvocation(args []string) bool {
	return len(args) > 0 && args[0] == "status" && (len(args) == 1 || strings.HasPrefix(args[1], "-"))
}

func runLoopStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve loop status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot string
	var asJSON bool
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	fs.BoolVar(&asJSON, "json", false, "emit {\"loop\": …} instead of the text report")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve loop status: unexpected argument %q (usage: evolve loop status [--json] [--project-root P])\n", fs.Arg(0))
		return 10
	}
	root, err := loopStopRoot(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop status: cwd: %v\n", err)
		return 1
	}
	if err := statusSnapshotReadable(root); err != nil {
		fmt.Fprintf(stderr, "evolve loop status: cannot read the snapshot: %v\n", err)
		return 2
	}
	loop := dashboard.Collect(root, time.Now()).Loop
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]dashboard.LoopStatus{"loop": loop}); err != nil {
			fmt.Fprintf(stderr, "evolve loop status: encode: %v\n", err)
			return 1
		}
		return 0
	}
	writeStatusLoopLine(stdout, loop)
	heartbeat := "none"
	if !loop.LeaseHeartbeat.IsZero() {
		heartbeat = loop.LeaseHeartbeat.UTC().Format(time.RFC3339)
	}
	fmt.Fprintf(stdout, "lease:   heartbeat=%s\n", heartbeat)
	return 0
}
