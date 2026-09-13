package inboxbatch

// layout.go — the ONE home of the inbox claim layout. A claim moves an item
// from the inbox root into processing/cycle-<N>/; the writer (inboxmover.Claim),
// every reader (inboxmover.Locate, core's dispatch-time claim scan) and the
// name parser derive that path from here, so the gate that judges a claim can
// never read a shape the claim did not write. It lives in this leaf package
// because core cannot import inboxmover (the ledger adapter imports core).

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const processingDirName = "processing"
const cycleDirPrefix = "cycle-"

// ProcessingDir is where claimed items live, grouped by claiming cycle.
func ProcessingDir(inboxDir string) string {
	return filepath.Join(inboxDir, processingDirName)
}

// ProcessingCycleDir is the directory a claim by cycle moves an item into.
func ProcessingCycleDir(inboxDir string, cycle int) string {
	return filepath.Join(ProcessingDir(inboxDir), cycleDirPrefix+strconv.Itoa(cycle))
}

// ParseProcessingCycle inverts ProcessingCycleDir's basename: the claiming
// cycle of a "cycle-<N>" directory name, or ok=false for anything else
// (notes, stray files, a name with extra segments, and "cycle-0": cycles start
// at 1 and 0 is the "pending at the root" sentinel of inboxmover.Location).
func ParseProcessingCycle(dirName string) (cycle int, ok bool) {
	if !strings.HasPrefix(dirName, cycleDirPrefix) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(dirName, cycleDirPrefix))
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// ProcessingCycleDirs lists the existing claim dirs, ascending by cycle —
// the one walk every reader of processing/ uses (inboxmover.Locate, orphan
// recovery, dispatch state, failure counts). Anything that is not a parseable
// cycle dir is skipped; a missing processing/ is an empty list.
func ProcessingCycleDirs(inboxDir string) []string {
	entries, err := os.ReadDir(ProcessingDir(inboxDir))
	if err != nil {
		return nil
	}
	byCycle := map[int]string{}
	var cycles []int
	for _, e := range entries {
		if cycle, ok := ParseProcessingCycle(e.Name()); ok && e.IsDir() {
			byCycle[cycle] = ProcessingCycleDir(inboxDir, cycle)
			cycles = append(cycles, cycle)
		}
	}
	sort.Ints(cycles)
	var dirs []string
	for _, c := range cycles {
		dirs = append(dirs, byCycle[c])
	}
	return dirs
}
