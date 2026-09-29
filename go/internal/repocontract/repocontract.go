// Package repocontract is the one decision on whether ship's repo-contract
// fixed scanner pack runs for a tree, shared by ship's gate and the build floor.
package repocontract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const evolveLoopModule = "github.com/mickeyyaya/evolve-loop/go"

func ModuleDir(root string) string {
	return filepath.Join(root, "go")
}

func GateOn(gate string) bool {
	return gate != "" && gate != "off"
}

func PackRuns(gate, root string) (runs bool, note string) {
	if !GateOn(gate) {
		return false, ""
	}
	if dir := ModuleDir(root); modulePath(filepath.Join(dir, "go.mod")) != evolveLoopModule {
		return false, fmt.Sprintf("%s does not declare %s, so the fixed scanner pack's guard suites are not in this tree; the pack is skipped", dir, evolveLoopModule)
	}
	if gate != "enforce" {
		return true, fmt.Sprintf("unknown stage %q — treating as enforce (a typo must not silently disable a red-main guard)", gate)
	}
	return true, ""
}

func modulePath(goMod string) string {
	data, err := os.ReadFile(goMod)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module ")
		if !ok {
			continue
		}
		if fields := strings.Fields(rest); len(fields) > 0 {
			return strings.Trim(fields[0], `"`)
		}
	}
	return ""
}
