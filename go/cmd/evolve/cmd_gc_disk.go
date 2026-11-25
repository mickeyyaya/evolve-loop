package main

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/diskfloor"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func (r gcRun) diskSignals(projectRoot string, pol policy.Policy) {
	res, err := pol.GCDiskFloor(r.stderr)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: %v\n", err)
		return
	}
	floor, warn := res.FloorBytes, res.WarnBytes
	fmt.Fprintf(r.stdout, "evolve gc: free disk floor %s is %s, warn level %s is %s\n", diskfloor.KeyFloorGB, gcSize(int64(floor)), diskfloor.KeyWarnGB, gcSize(int64(warn)))
	seen := map[string]bool{}
	for _, path := range []string{os.TempDir(), projectRoot} {
		if seen[path] {
			continue
		}
		seen[path] = true
		r.diskSignal(path, floor, warn)
	}
}

func (r gcRun) diskSignal(path string, floor, warn uint64) {
	free, err := gcDiskFreeBytes(path)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: free disk of %s unreadable: %v\n", path, err)
		return
	}
	switch {
	case free < floor:
		fmt.Fprintf(r.stderr, "evolve gc: WARN: free disk %s at %s is under %s (%s); the sweep still runs\n", gcSize(int64(free)), path, diskfloor.KeyFloorGB, gcSize(int64(floor)))
	case free < warn:
		fmt.Fprintf(r.stderr, "evolve gc: WARN: free disk %s at %s is under %s (%s), above the floor\n", gcSize(int64(free)), path, diskfloor.KeyWarnGB, gcSize(int64(warn)))
	}
}
