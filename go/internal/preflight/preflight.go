// Package preflight ports legacy/scripts/dispatch/preflight-environment.sh.
//
// Single capability-detection probe (v8.25.0). Probes the host environment
// ONCE at dispatcher start and emits a JSON capability profile. The
// dispatcher reads the profile and auto-configures the auto-relaxable flags.
//
// Design principle: Discover, Decide, Log, Verify.
// This package implements Discover + Decide + Log. Verify happens elsewhere
// (phase-gate, ledger-SHA, role-gate, ship-gate).
package preflight

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	// sbx is the single source of truth for nested-Claude detection + the
	// inner-sandbox wrap policy, shared with the bridge launch path. Aliased
	// because a local variable in Probe() is named `sandbox`.
	sbx "github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
)

// Profile is the JSON shape emitted to stdout / .evolve/environment.json.
// Schema version 3 (matches preflight-environment.sh).
type Profile struct {
	SchemaVersion int        `json:"schema_version"`
	ProbedAt      string     `json:"probed_at"`
	Host          Host       `json:"host"`
	ClaudeCode    ClaudeCode `json:"claude_code"`
	Sandbox       Sandbox    `json:"sandbox"`
	Filesystem    Filesystem `json:"filesystem"`
	CLIBinaries   CLIBins    `json:"cli_binaries"`
	AutoConfig    AutoConfig `json:"auto_config"`
}

// Host captures os/version/shell.
type Host struct {
	OS        string `json:"os"`
	OSVersion string `json:"os_version"`
	Shell     string `json:"shell"`
}

// ClaudeCode captures nested-claude detection.
type ClaudeCode struct {
	Nested        bool    `json:"nested"`
	ClaudecodeEnv *string `json:"claudecode_env"`
}

// Sandbox captures sandbox-exec / bwrap availability.
type Sandbox struct {
	SandboxExecAvailable bool   `json:"sandbox_exec_available"`
	BwrapAvailable       bool   `json:"bwrap_available"`
	ExpectedToWork       bool   `json:"expected_to_work"`
	Reason               string `json:"reason"`
}

// Filesystem captures writability probes.
type Filesystem struct {
	StateDirWritable           bool   `json:"state_dir_writable"`
	InProjectWorktreesWritable bool   `json:"in_project_worktrees_writable"`
	TmpdirWritable             bool   `json:"tmpdir_writable"`
	CacheDirWritable           bool   `json:"cache_dir_writable"`
	StateDir                   string `json:"state_dir"`
}

// CLIBins maps each CLI to its resolved path (nil when absent).
type CLIBins struct {
	Claude *string `json:"claude"`
	Gemini *string `json:"gemini"`
	Codex  *string `json:"codex"`
	Agy    *string `json:"agy"`
	JQ     *string `json:"jq"`
	Git    *string `json:"git"`
}

// AutoConfig records the dispatcher's derived posture.
type AutoConfig struct {
	SandboxFallbackOnEPERM string `json:"EVOLVE_SANDBOX_FALLBACK_ON_EPERM"`
	WorktreeBase           string `json:"worktree_base"`
	WorktreeBaseReason     string `json:"worktree_base_reason"`
	InnerSandbox           bool   `json:"inner_sandbox"`
	InnerSandboxReason     string `json:"inner_sandbox_reason"`
	Reasoning              string `json:"reasoning"`
}

// Options exposes seams for testing.
type Options struct {
	ProjectRoot string
	PluginRoot  string
	Now         func() time.Time
	LookPath    func(string) (string, error)
	// Env stubs os.Getenv; used to control TMPDIR/HOME/SHELL etc in tests.
	Env func(string) string
	// IsNested overrides detection (defaults to checking $CLAUDECODE != "").
	IsNested func() bool
	// OSType overrides runtime detection ("darwin", "linux", "other").
	OSType string
	// SandboxCapable injects the MEASURED sandbox-apply capability
	// (capable, checked) — the ground-truth replacement for the nested env-var
	// guess. Production wires it to sandbox.Probe()'s measurement at the
	// composition root; nil (the default) leaves the result UNMEASURED so the
	// legacy unmeasured decision behavior is retained.
	SandboxCapable func() (capable bool, checked bool)
	// WorktreeBase is the operator override for the per-cycle worktree base,
	// resolved from policy.json (worktree.base) at the composition root. Empty ⇒
	// preflight selects a default writable base. Replaces the former
	// EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064).
	WorktreeBase string
}

// Probe runs the discover+decide pipeline and returns the populated Profile.
func Probe(opts Options) Profile {
	getEnv := opts.Env
	if getEnv == nil {
		getEnv = os.Getenv
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.LookPath == nil {
		opts.LookPath = exec.LookPath
	}
	if opts.ProjectRoot == "" {
		opts.ProjectRoot, _ = os.Getwd()
	}
	osType := opts.OSType
	if osType == "" {
		switch runtime.GOOS {
		case "darwin":
			osType = "darwin"
		case "linux":
			osType = "linux"
		default:
			osType = "other"
		}
	}

	// Host
	osVersion := unameR()
	shell := filepath.Base(getEnvDefault(getEnv, "SHELL", "/bin/sh"))

	// Nested
	nested := false
	if opts.IsNested != nil {
		nested = opts.IsNested()
	} else {
		nested = sbx.DetectNested(getEnv)
	}
	var claudecodePtr *string
	if v := getEnv("CLAUDECODE"); v != "" {
		claudecodePtr = &v
	}

	// Sandbox capability
	sandboxExec := false
	bwrap := false
	if _, err := opts.LookPath("sandbox-exec"); err == nil {
		sandboxExec = true
	}
	if _, err := opts.LookPath("bwrap"); err == nil {
		bwrap = true
	}
	// Measured capability (opt-in seam; nil ⇒ legacy unmeasured decision).
	var capable, capChecked bool
	if opts.SandboxCapable != nil {
		capable, capChecked = opts.SandboxCapable()
	}

	// Host readiness and launch use the same measured wrapping decision. Session
	// markers describe context; they do not establish sandbox capability.
	innerSandbox, innerReason := sbx.ShouldWrap(nested, sbx.ProbeResult{
		OS:        osType,
		Available: (osType == "darwin" && sandboxExec) || (osType == "linux" && bwrap),
		Capable:   capable, CapabilityChecked: capChecked,
	})
	sandbox := Sandbox{
		SandboxExecAvailable: sandboxExec, BwrapAvailable: bwrap,
		ExpectedToWork: innerSandbox, Reason: innerReason,
	}

	// Filesystem
	stateDir := filepath.Join(opts.ProjectRoot, ".evolve")
	_ = os.MkdirAll(stateDir, 0o755)
	fs := Filesystem{
		StateDir:                   stateDir,
		StateDirWritable:           probeWritable(stateDir),
		InProjectWorktreesWritable: probeWritable(filepath.Join(stateDir, "worktrees")),
	}
	projectHash := projectHash8(opts.ProjectRoot)

	probeTmpDir := ""
	if tmp := getEnv("TMPDIR"); tmp != "" {
		probeTmpDir = filepath.Join(strings.TrimRight(tmp, "/"), "evolve-loop", projectHash)
		fs.TmpdirWritable = probeWritable(probeTmpDir)
	}
	probeCache := cacheDirPath(osType, getEnv, projectHash)
	if probeCache != "" {
		fs.CacheDirWritable = probeWritable(probeCache)
	}

	// Worktree base selection
	wtBase, wtReason := selectWorktreeBase(opts, nested, fs, filepath.Join(stateDir, "worktrees"), probeTmpDir, probeCache)

	// CLI binaries
	bins := CLIBins{
		Claude: lookPathPtr(opts.LookPath, "claude"),
		Gemini: lookPathPtr(opts.LookPath, "gemini"),
		Codex:  lookPathPtr(opts.LookPath, "codex"),
		Agy:    lookPathPtr(opts.LookPath, "agy"),
		JQ:     lookPathPtr(opts.LookPath, "jq"),
		Git:    lookPathPtr(opts.LookPath, "git"),
	}

	// Auto-config
	autoEPERM := "0"
	if nested && !innerSandbox {
		autoEPERM = "1"
	}

	var autoReasoning string
	if wtBase != "" {
		autoReasoning = fmt.Sprintf(
			"Nested LLM-CLI hint: %v. Worktree base: %s (%s). Inner sandbox: %v (%s). Mandatory profiles require their own successful wrapper.",
			nested, wtBase, wtReason, innerSandbox, innerReason)
	} else {
		autoReasoning = fmt.Sprintf(
			"ERROR: no writable worktree base. Tried in-project (%v), TMPDIR (%v), cache dir (%v). OPERATOR ACTION: set worktree.base in .evolve/policy.json to a writable directory, or run from a different shell with broader permissions. Last-resort: use the explicit no-worktree operator mode (loses per-cycle isolation, NOT recommended).",
			fs.InProjectWorktreesWritable, fs.TmpdirWritable, fs.CacheDirWritable)
	}

	return Profile{
		SchemaVersion: 3,
		ProbedAt:      opts.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Host: Host{
			OS: osType, OSVersion: osVersion, Shell: shell,
		},
		ClaudeCode:  ClaudeCode{Nested: nested, ClaudecodeEnv: claudecodePtr},
		Sandbox:     sandbox,
		Filesystem:  fs,
		CLIBinaries: bins,
		AutoConfig: AutoConfig{
			SandboxFallbackOnEPERM: autoEPERM,
			WorktreeBase:           wtBase,
			WorktreeBaseReason:     wtReason,
			InnerSandbox:           innerSandbox,
			InnerSandboxReason:     innerReason,
			Reasoning:              autoReasoning,
		},
	}
}

// MeasuredSandboxCapability returns the cached sandbox.Probe() measurement of
// whether the OS sandbox actually applies on this host. Production call sites
// wire it into Options.SandboxCapable so the host-capability report reflects a
// measured fact rather than the nested env-var guess.
func MeasuredSandboxCapability() (capable bool, checked bool) {
	pr := sbx.Probe()
	return pr.Capable, pr.CapabilityChecked
}

func selectWorktreeBase(opts Options, nested bool, fs Filesystem, inProject, tmpdir, cacheDir string) (string, string) {
	// 1. Operator override (policy.json worktree.base)
	if v := opts.WorktreeBase; v != "" {
		if probeWritable(v) {
			return v, "operator-provided worktree.base (writable)"
		}
		// fall-through with WARN attached to reason field
		// (mirrors bash setting WORKTREE_BASE_REASON but proceeding)
	}
	// 2. Standalone + in-project
	if !nested && fs.InProjectWorktreesWritable {
		return inProject, "standalone shell: in-project location preferred (easy operator inspection)"
	}
	// 3. TMPDIR
	if fs.TmpdirWritable && tmpdir != "" {
		return tmpdir, "TMPDIR (sandbox-friendly default for nested-Claude)"
	}
	// 4. Cache dir
	if fs.CacheDirWritable && cacheDir != "" {
		return cacheDir, "user cache dir (TMPDIR unavailable)"
	}
	// 5. Last resort in-project
	if fs.InProjectWorktreesWritable {
		return inProject, "in-project (TMPDIR/cache unavailable; isolation degraded if parent sandbox blocks at exec time)"
	}
	return "", ""
}

// probeWritable mimics bash probe_writable: mkdir -p, touch sentinel, rm sentinel.
func probeWritable(dir string) bool {
	if dir == "" {
		return false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	probe := filepath.Join(dir, fmt.Sprintf(".preflight-probe.%d", os.Getpid()))
	f, err := os.Create(probe)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(probe)
	return true
}

func projectHash8(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:])[:8]
}

func cacheDirPath(osType string, getEnv func(string) string, hash string) string {
	home := getEnv("HOME")
	if home == "" {
		return ""
	}
	switch osType {
	case "darwin":
		return filepath.Join(home, "Library", "Caches", "evolve-loop", hash)
	case "linux":
		base := getEnv("XDG_CACHE_HOME")
		if base == "" {
			base = filepath.Join(home, ".cache")
		}
		return filepath.Join(base, "evolve-loop", hash)
	default:
		return filepath.Join(home, ".cache", "evolve-loop", hash)
	}
}

func unameR() string {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimRight(string(out), "\n")
}

func getEnvDefault(getEnv func(string) string, key, dflt string) string {
	if v := getEnv(key); v != "" {
		return v
	}
	return dflt
}

func lookPathPtr(look func(string) (string, error), name string) *string {
	p, err := look(name)
	if err != nil || p == "" {
		return nil
	}
	return &p
}

// MarshalJSON returns the profile's JSON bytes (compact, matches the bash
// jq -n output topology).
func (p Profile) MarshalJSON() ([]byte, error) {
	type alias Profile
	return json.Marshal(alias(p))
}

// PrettyJSON returns the profile's JSON with 2-space indentation.
func (p Profile) PrettyJSON() string {
	b, _ := json.MarshalIndent(p, "", "  ")
	return string(b)
}

// Summary returns the human-readable summary form (--summary mode).
func (p Profile) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Environment Profile (probed %s)\n", p.ProbedAt)
	fmt.Fprintf(&b, "  Host:             %s %s (%s)\n", p.Host.OS, p.Host.OSVersion, p.Host.Shell)
	fmt.Fprintf(&b, "  Nested-Claude:    %v\n", p.ClaudeCode.Nested)
	fmt.Fprintf(&b, "  Sandbox works:    %v (%s)\n", p.Sandbox.ExpectedToWork, p.Sandbox.Reason)
	fmt.Fprintf(&b, "  State writable:   %v\n", p.Filesystem.StateDirWritable)
	fmt.Fprintf(&b, "  Worktree probes:  in-project=%v tmpdir=%v cache=%v\n",
		p.Filesystem.InProjectWorktreesWritable, p.Filesystem.TmpdirWritable, p.Filesystem.CacheDirWritable)
	fmt.Fprintf(&b, "  Auto-config:\n")
	fmt.Fprintf(&b, "    EVOLVE_SANDBOX_FALLBACK_ON_EPERM=%s\n", p.AutoConfig.SandboxFallbackOnEPERM)
	wt := p.AutoConfig.WorktreeBase
	if wt == "" {
		wt = "<NONE>"
	}
	fmt.Fprintf(&b, "    worktree_base=%s\n", wt)
	fmt.Fprintf(&b, "    worktree_base_reason: %s\n", p.AutoConfig.WorktreeBaseReason)
	fmt.Fprintf(&b, "    inner_sandbox=%v\n", p.AutoConfig.InnerSandbox)
	fmt.Fprintf(&b, "    inner_sandbox_reason: %s\n", p.AutoConfig.InnerSandboxReason)
	fmt.Fprintf(&b, "    Reasoning: %s\n", p.AutoConfig.Reasoning)
	return b.String()
}

// WriteToFile persists the profile to <projectRoot>/.evolve/environment.json
// using atomic rename. Best-effort: silent failure is acceptable per bash
// contract (file is a cache, not a source of truth).
func (p Profile) WriteToFile(projectRoot string) error {
	target := filepath.Join(projectRoot, ".evolve", "environment.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp := target + ".tmp"
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}
