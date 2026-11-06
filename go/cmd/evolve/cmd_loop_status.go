package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dashboard"
)

type loopStatusJSON struct {
	ProjectRoot string               `json:"project_root"`
	Loop        dashboard.LoopStatus `json:"loop"`
}

func loopStatusArgs(args []string) ([]string, bool) {
	fs := flag.NewFlagSet("evolve loop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	registerLoopArgFlags(fs)
	registerPerAgentOverrides(fs)
	if fs.Parse(args) != nil || fs.Arg(0) != "status" {
		return nil, false
	}
	flagEnd := len(args) - fs.NArg()
	return append(append([]string{}, args[:flagEnd]...), args[flagEnd+1:]...), true
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
	if rc := checkRootHasEvolve("loop status", root, stderr); rc != 0 {
		return rc
	}
	loop := dashboard.Collect(root, time.Now()).Loop
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(loopStatusJSON{ProjectRoot: root, Loop: loop}); err != nil {
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
