// Package repocontract owns ship's repo-contract fixed scanner pack: its suite
// list and whether it runs for a tree, shared by ship's gate and the build floor.
package repocontract

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type thisPackage struct{}

func evolveLoopModule() string {
	module, _, _ := strings.Cut(reflect.TypeOf(thisPackage{}).PkgPath(), "/internal/")
	return module
}

func Packages() []string {
	return []string{
		"./internal/phasespec/...",
		"./internal/profiles/...",
		"./internal/phasecoherence/...",
		"./internal/routingtest/...",
		"./internal/rawgitratchet/...",
		"./internal/sizeratchet/...",
		"./internal/testmainexit/...",
		"./internal/repocontract/...",
		"./internal/policy/...",
		"./internal/guards/...",
		"./internal/acssuite/...",
		"./internal/fleet/...",
		"./internal/evalqualitycheck/...",
		"./internal/inboxrank/...",
	}
}

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
	if dir := ModuleDir(root); modulePath(filepath.Join(dir, "go.mod")) != evolveLoopModule() {
		return false, fmt.Sprintf("%s does not declare %s, so the fixed scanner pack's guard suites are not in this tree; the pack is skipped", dir, evolveLoopModule())
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
		if fields := strings.Fields(line); len(fields) > 1 && fields[0] == "module" {
			return strings.Trim(fields[1], "\"`")
		}
	}
	return ""
}
