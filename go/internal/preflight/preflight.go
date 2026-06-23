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
	sbx "github.com/mickeyyaya/evolveloop/go/internal/adapters/sandbox"
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
	sandbox := decideSandbox(osType, nested, sandboxExec, bwrap)

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
	wtBase, wtReason := selectWorktreeBase(opts, getEnv, nested, fs, filepath.Join(stateDir, "worktrees"), probeTmpDir, probeCache)

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
	if nested {
		autoEPERM = "1"
	}
	// The InnerSandbox boolean is the SSOT wrap policy (sbx.ShouldWrap) — the
	// SAME decision the bridge launch path applies, so preflight's promise and
	// the hot path can't drift. The reason strings stay preflight-local
	// (operator-facing host-profile context, not duplicated decision logic).
	innerProbe := sbx.ProbeResult{OS: osType, Available: (osType == "darwin" && sandboxExec) || (osType == "linux" && bwrap)}
	innerSandbox, wrapReason := sbx.ShouldWrap(nested, innerProbe)
	innerReason := "standalone shell with working sandbox: defense-in-depth enabled"
	switch {
	case nested:
		innerReason = "nested-Claude: outer Claude Code OS sandbox + Tier-1 hooks suffice; inner sandbox-exec adds friction without protection (intersect-only nesting)"
	case !sandbox.ExpectedToWork:
		innerReason = "sandbox not expected to work on this host: " + sandbox.Reason
	case !innerSandbox:
		// Exhaustiveness guard: ShouldWrap declined to wrap for a reason the
		// cases above don't cover. Derive the reason from the SSOT decision so
		// innerReason can never drift from innerSandbox (keeps the host-profile
		// honest even if decideSandbox's ExpectedToWork ever decouples from the
		// binary probe). Unreachable with today's Probe inputs.
		innerReason = "inner sandbox not applied: " + wrapReason
	}

	var autoReasoning string
	if wtBase != "" {
		if nested {
			autoReasoning = fmt.Sprintf(
				"nested-Claude detected. Sandbox startup-fallback enabled (EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1). Worktree relocated to sandbox-friendly path: %s (%s). Inner sandbox-exec DISABLED (%s). Tier-1 kernel hooks (phase-gate, role-gate, ledger SHA) keep enforcing.",
				wtBase, wtReason, innerReason)
		} else {
			autoReasoning = fmt.Sprintf(
				"standalone shell. Worktree base: %s (%s). Inner sandbox-exec: %v (%s).",
				wtBase, wtReason, innerSandbox, innerReason)
		}
	} else {
		autoReasoning = fmt.Sprintf(
			"ERROR: no writable worktree base. Tried in-project (%v), TMPDIR (%v), cache dir (%v). OPERATOR ACTION: set EVOLVE_WORKTREE_BASE to a writable directory, or run from a different shell with broader permissions. Last-resort: use the explicit no-worktree operator mode (loses per-cycle isolation, NOT recommended).",
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

func decideSandbox(osType string, nested, sbExec, bwrap bool) Sandbox {
	s := Sandbox{SandboxExecAvailable: sbExec, BwrapAvailable: bwrap}
	switch osType {
	case "darwin":
		if sbExec {
			if nested {
				s.ExpectedToWork = false
				s.Reason = "Darwin nested-Claude: sandbox_apply() returns EPERM (rc=71)"
			} else {
				s.ExpectedToWork = true
				s.Reason = "Darwin standalone: sandbox-exec available and parent unsandboxed"
			}
		} else {
			s.Reason = "Darwin: sandbox-exec binary not on PATH"
		}
	case "linux":
		if bwrap {
			s.ExpectedToWork = true
			s.Reason = "Linux: bwrap available; nested namespaces supported"
		} else {
			s.Reason = "Linux: bwrap binary not on PATH"
		}
	default:
		s.Reason = fmt.Sprintf("Unsupported OS: %s — sandbox not enforced", osType)
	}
	return s
}

func selectWorktreeBase(opts Options, getEnv func(string) string, nested bool, fs Filesystem, inProject, tmpdir, cacheDir string) (string, string) {
	// 1. Operator override
	if v := getEnv("EVOLVE_WORKTREE_BASE"); v != "" {
		if probeWritable(v) {
			return v, "operator-provided EVOLVE_WORKTREE_BASE (writable)"
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
