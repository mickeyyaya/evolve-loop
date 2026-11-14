package main

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

func (r gcRun) diskSignals(projectRoot string, pol gc.Policy) {
	floor, warn := pol.DiskFloorBytes(), pol.DiskWarnBytes()
	fmt.Fprintf(r.stdout, "evolve gc: free disk floor gc.disk_floor_gb is %s, warn level gc.disk_warn_gb is %s\n", gcSize(int64(floor)), gcSize(int64(warn)))
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
		fmt.Fprintf(r.stderr, "evolve gc: WARN: free disk %s at %s is under gc.disk_floor_gb (%s); the sweep still runs\n", gcSize(int64(free)), path, gcSize(int64(floor)))
	case free < warn:
		fmt.Fprintf(r.stderr, "evolve gc: WARN: free disk %s at %s is under gc.disk_warn_gb (%s), above the floor\n", gcSize(int64(free)), path, gcSize(int64(warn)))
	}
}
