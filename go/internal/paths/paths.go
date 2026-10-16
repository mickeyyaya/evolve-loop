// Package paths resolves the canonical .evolve/* filesystem layout honored
// across the Go pipeline.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type Layout struct {
	ProjectRoot string

	PluginRoot string

	EvolveDir string

	StateFile string

	CycleStateFile string

	LedgerFile string

	ProfilesDir string

	AdaptersDir string

	CapabilityDir string
}

func Resolve(lookupEnv func(string) string, cwd string) Layout {
	if lookupEnv == nil {
		lookupEnv = func(string) string { return "" }
	}

	projectRoot := lookupEnv("EVOLVE_PROJECT_ROOT")
	if projectRoot == "" {
		projectRoot = cwd
	}

	pluginRoot := lookupEnv("EVOLVE_PLUGIN_ROOT")
	if pluginRoot == "" {
		pluginRoot = projectRoot
	}

	evolveDir := EvolveDirOf(projectRoot)

	profilesDir := lookupEnv("EVOLVE_PROFILES_DIR_OVERRIDE")
	if profilesDir == "" {
		profilesDir = filepath.Join(pluginRoot, ".evolve", "profiles")
	}
	adaptersDir := lookupEnv("EVOLVE_ADAPTERS_DIR_OVERRIDE")
	if adaptersDir == "" {
		adaptersDir = filepath.Join(pluginRoot, "adapters")
	}
	capabilityDir := filepath.Join(pluginRoot, "adapters")

	ledgerFile := lookupEnv("EVOLVE_LEDGER_OVERRIDE")
	if ledgerFile == "" {
		ledgerFile = filepath.Join(evolveDir, "ledger.jsonl")
	}

	cycleStateFile := CycleStateFileFor(evolveDir, lookupEnv(ipcenv.CycleStateFileKey))

	return Layout{
		ProjectRoot:    projectRoot,
		PluginRoot:     pluginRoot,
		EvolveDir:      evolveDir,
		StateFile:      filepath.Join(evolveDir, "state.json"),
		CycleStateFile: cycleStateFile,
		LedgerFile:     ledgerFile,
		ProfilesDir:    profilesDir,
		AdaptersDir:    adaptersDir,
		CapabilityDir:  capabilityDir,
	}
}

func CycleStateFileFor(evolveDir, override string) string {
	if override != "" && isInside(evolveDir, override) {
		return override
	}
	return filepath.Join(evolveDir, "cycle-state.json")
}

func isInside(dir, path string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func ResolveFromEnv() Layout {
	cwd, _ := os.Getwd()
	return Resolve(os.Getenv, cwd)
}

var absFn = filepath.Abs

func AbsoluteRoot(label, p string, warn func(string)) string {
	abs, err := absFn(p)
	if err != nil {
		if warn != nil {
			warn(fmt.Sprintf("could not resolve %s %q to an absolute path (%v); worktree-phase paths may diverge across cwd boundaries", label, p, err))
		}
		return p
	}
	return abs
}

const LoopStopFile = "loop-stop"

func LoopStopPath(evolveDir string) string { return filepath.Join(evolveDir, LoopStopFile) }

func EvolveDirOf(projectRoot string) string { return filepath.Join(projectRoot, ".evolve") }

func RunWorkspace(projectRoot string, cycle int) string {
	return filepath.Join(EvolveDirOf(projectRoot), "runs", "cycle-"+strconv.Itoa(cycle))
}

const PolicyFile = "policy.json"

func PolicyPath(evolveDir string) string { return filepath.Join(evolveDir, PolicyFile) }
