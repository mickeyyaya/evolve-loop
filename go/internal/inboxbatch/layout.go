package inboxbatch

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
	return CycleDir(ProcessingDir(inboxDir), strconv.Itoa(cycle))
}

// CycleDir is the one spelling of a cycle-nested lifecycle directory, <parent>/cycle-<cycle>.
func CycleDir(parent, cycle string) string {
	return filepath.Join(parent, cycleDirPrefix+cycle)
}

// ParseProcessingCycle returns the cycle a "cycle-<N>" name claims, N >= 1.
// N = 0 is rejected because it is inboxmover.Location's "pending at the root" sentinel.
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

// ProcessingCycleDirs lists the existing claim dirs in ascending cycle order.
func ProcessingCycleDirs(inboxDir string) []string {
	return CycleDirs(ProcessingDir(inboxDir))
}

// CycleDirs lists parent's cycle-<N> subdirectories in ascending cycle order; a missing parent yields nil.
func CycleDirs(parent string) []string {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	byCycle := map[int]string{}
	var cycles []int
	for _, e := range entries {
		if cycle, ok := ParseProcessingCycle(e.Name()); ok && e.IsDir() {
			byCycle[cycle] = filepath.Join(parent, e.Name())
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
