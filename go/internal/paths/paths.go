// Package paths resolves the canonical .evolve/* filesystem layout
// honored across the Go pipeline. It centralizes what was previously
// duplicated across cmd_subagent.go (three verbatim copies of a 20-line
// env-resolution block) and ~10 inline `filepath.Join(..., ".evolve",
// ...)` magic strings scattered across cmd_*.go and internal packages.
//
// Resolution order for each field, highest priority first:
//
//  1. The corresponding EVOLVE_*_OVERRIDE env var, if set.
//  2. A path derived from PluginRoot (profiles, adapters, capability)
//     or ProjectRoot (state, ledger, llm_config).
//  3. cwd when env is empty (ProjectRoot only).
//
// CapabilityDir is intentionally NEVER honored by
// EVOLVE_ADAPTERS_DIR_OVERRIDE — it mirrors the bash REAL_ADAPTERS_DIR
// invariant so capability manifests reflect actual installed
// capabilities, not a test-seam sentinel dir. See the matching test
// TestResolve_CapabilityDirAlwaysPluginRoot.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// Layout is the resolved set of .evolve/* paths for a single command
// invocation. Constructed once at entry and passed by value to call
// sites that need any of its fields.
type Layout struct {
	// ProjectRoot is the user's project directory — the parent of
	// .evolve/. From EVOLVE_PROJECT_ROOT or cwd.
	ProjectRoot string

	// PluginRoot is the directory where the evolve-loop plugin is
	// installed. Same as ProjectRoot when running from a checkout;
	// different when run as an installed plugin. From EVOLVE_PLUGIN_ROOT
	// or ProjectRoot.
	PluginRoot string

	// EvolveDir is <ProjectRoot>/.evolve.
	EvolveDir string

	// StateFile is <EvolveDir>/state.json — persistent batch-scope
	// state (currentBatch, failedApproaches, lastCycleNumber).
	StateFile string

	// CycleStateFile is <EvolveDir>/cycle-state.json — transient
	// per-cycle state (phase, active_agent, workspace_path,
	// completed_phases).
	CycleStateFile string

	// LedgerFile is <EvolveDir>/ledger.jsonl — hash-chained audit
	// trail. Overridable via EVOLVE_LEDGER_OVERRIDE.
	LedgerFile string

	// LLMConfigFile is <EvolveDir>/llm_config.json — role→cli+model
	// routing. Overridable via EVOLVE_LLM_CONFIG_PATH.
	LLMConfigFile string

	// ProfilesDir is <PluginRoot>/.evolve/profiles — sandbox + LLM
	// profile JSONs. Overridable via EVOLVE_PROFILES_DIR_OVERRIDE.
	ProfilesDir string

	// AdaptersDir is <PluginRoot>/adapters — CLI adapter scripts and
	// metadata. Overridable via EVOLVE_ADAPTERS_DIR_OVERRIDE.
	AdaptersDir string

	// CapabilityDir is <PluginRoot>/adapters — capability manifest
	// source. NEVER overridden; mirrors bash REAL_ADAPTERS_DIR so
	// capability manifests reflect the real install path even when
	// EVOLVE_ADAPTERS_DIR_OVERRIDE points elsewhere for testing.
	CapabilityDir string
}

// Resolve computes the Layout from the supplied environment lookup and
// fallback cwd. lookupEnv is the seam for testability — production
// callers pass os.Getenv (or use ResolveFromEnv). cwd is the fallback
// for ProjectRoot when EVOLVE_PROJECT_ROOT is empty.
//
// Resolve is total: it never returns an error and never reads the
// filesystem. Field validation (existence, permissions) is the caller's
// responsibility — Resolve only constructs the canonical paths.
//
// The returned Layout is cwd-bound at construction time: relative
// fields (when cwd is empty and no env overrides are set) capture the
// process's working directory then. Callers must not os.Chdir between
// Resolve() and any filesystem write using a Layout field, or those
// writes will land relative to the new cwd instead of the original.
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

	evolveDir := filepath.Join(projectRoot, ".evolve")

	profilesDir := lookupEnv("EVOLVE_PROFILES_DIR_OVERRIDE")
	if profilesDir == "" {
		profilesDir = filepath.Join(pluginRoot, ".evolve", "profiles")
	}
	adaptersDir := lookupEnv("EVOLVE_ADAPTERS_DIR_OVERRIDE")
	if adaptersDir == "" {
		adaptersDir = filepath.Join(pluginRoot, "adapters")
	}
	// CapabilityDir never honors EVOLVE_ADAPTERS_DIR_OVERRIDE — mirrors
	// bash REAL_ADAPTERS_DIR so capability manifests reflect the real
	// install path even when the override points to a test sentinel dir.
	capabilityDir := filepath.Join(pluginRoot, "adapters")

	llmConfigFile := lookupEnv("EVOLVE_LLM_CONFIG_PATH")
	if llmConfigFile == "" {
		llmConfigFile = filepath.Join(evolveDir, "llm_config.json")
	}
	ledgerFile := lookupEnv("EVOLVE_LEDGER_OVERRIDE")
	if ledgerFile == "" {
		ledgerFile = filepath.Join(evolveDir, "ledger.jsonl")
	}

	return Layout{
		ProjectRoot:    projectRoot,
		PluginRoot:     pluginRoot,
		EvolveDir:      evolveDir,
		StateFile:      filepath.Join(evolveDir, "state.json"),
		CycleStateFile: filepath.Join(evolveDir, "cycle-state.json"),
		LedgerFile:     ledgerFile,
		LLMConfigFile:  llmConfigFile,
		ProfilesDir:    profilesDir,
		AdaptersDir:    adaptersDir,
		CapabilityDir:  capabilityDir,
	}
}

// ResolveFromEnv is the production-default convenience: Resolve with
// os.Getenv as the lookup and os.Getwd() as the cwd fallback. cwd
// errors are swallowed (resolved to "") to keep the signature pure;
// callers that need to surface cwd failures should call Resolve
// directly with an explicit cwd.
func ResolveFromEnv() Layout {
	cwd, _ := os.Getwd()
	return Resolve(os.Getenv, cwd)
}

// absFn is a seam so tests can exercise the filepath.Abs error branch.
// filepath.Abs only fails when os.Getwd fails (cwd deleted/unmounted).
var absFn = filepath.Abs

// AbsoluteRoot resolves p — a project root, possibly relative or "." — to an
// absolute path. A relative root silently breaks path-crossing contracts: a
// worktree-phase agent (cwd=worktree) and the in-process bridge (cwd=main repo)
// resolve the same relative artifact path to DIFFERENT absolute locations, which
// caused cycle-119's ExitArtifactTimeout. This is the single shared helper
// applied at every command entrypoint (replacing the one-off closure in
// cmd_loop.go and the cwd-bound envOrCwd pattern).
//
// On the rare filepath.Abs error it WARNs via warn (when non-nil) and returns p
// UNCHANGED rather than silently proceeding with a broken relative path — fail
// loud, since continuing would reproduce the very timeout this guards against.
// label names the source (e.g. "--project-root") for the warning.
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
