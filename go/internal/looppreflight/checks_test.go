package looppreflight

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/doctor"
	"github.com/mickeyyaya/evolve-loop/go/internal/preflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// --- checkLLMCLIStatus -----------------------------------------------------

func TestRun_LLMCLIStatus_AllPresent(t *testing.T) {
	r, err := Run(goodPipelineOptions(t))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "llm-cli-status")
	if c.Level != LevelPass {
		t.Fatalf("want LevelPass, got %s (%s)", c.Level, c.Detail)
	}
}

func TestRun_LLMCLIStatus_MissingBinary_Halts(t *testing.T) {
	opts := goodPipelineOptions(t)
	// claude-tmux → binary "claude". Report it missing, with a probe trail.
	opts.ProbeCLI = func(bin string) (doctor.Result, error) {
		if bin == "claude" {
			return doctor.Result{Tool: bin, Found: false, Checked: []string{
				"exec.LookPath(claude) → not found",
				"/usr/local/bin/claude → not present",
			}}, nil
		}
		return doctor.Result{Tool: bin, Found: true, Path: "/usr/bin/" + bin}, nil
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !r.Halted() {
		t.Fatalf("expected halt for missing CLI binary, got %s", r.OverallLevel)
	}
	c := findCheck(t, r, "llm-cli-status")
	if c.Level != LevelHalt {
		t.Fatalf("want LevelHalt, got %s", c.Level)
	}
	if !strings.Contains(c.Detail, "claude") {
		t.Fatalf("detail should name the missing binary; got %q", c.Detail)
	}
	if !strings.Contains(c.Detail, "not present") {
		t.Fatalf("detail should carry the probe trail; got %q", c.Detail)
	}
}

// claude-tmux and claude-p both map to the "claude" binary — probe it once.
func TestRun_LLMCLIStatus_DedupsBinaries(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux", CLIFallback: []string{"claude-p"}}, nil
	}
	var probed []string
	opts.ProbeCLI = func(bin string) (doctor.Result, error) {
		probed = append(probed, bin)
		return doctor.Result{Tool: bin, Found: true, Path: "/usr/bin/" + bin}, nil
	}
	if _, err := Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	claudeCount := 0
	for _, b := range probed {
		if b == "claude" {
			claudeCount++
		}
	}
	if claudeCount != 1 {
		t.Fatalf("expected the claude binary probed exactly once, probed=%v", probed)
	}
}

// --- checkHostCapabilities -------------------------------------------------

func TestRun_HostCapabilities_AllGood(t *testing.T) {
	r, err := Run(goodPipelineOptions(t))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "host-capabilities")
	if c.Level != LevelPass {
		t.Fatalf("want LevelPass, got %s (%s)", c.Level, c.Detail)
	}
}

func TestRun_HostCapabilities_NoTmux_Halts(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProbeCLI = func(bin string) (doctor.Result, error) {
		if bin == "tmux" {
			return doctor.Result{Tool: bin, Found: false, Checked: []string{"exec.LookPath(tmux) → not found"}}, nil
		}
		return doctor.Result{Tool: bin, Found: true, Path: "/usr/bin/" + bin}, nil
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "host-capabilities")
	if c.Level != LevelHalt {
		t.Fatalf("want LevelHalt for missing tmux, got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Detail, "tmux") {
		t.Fatalf("detail should mention tmux; got %q", c.Detail)
	}
}

func TestRun_HostCapabilities_EvolveDirUnwritable_Halts(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.DirWritable = func(string) bool { return false }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "host-capabilities")
	if c.Level != LevelHalt {
		t.Fatalf("want LevelHalt for unwritable .evolve, got %s", c.Level)
	}
}

// Fresh-cycle Build explanation enforcement requires a real filesystem
// sandbox, so an unavailable host capability must halt before phase spend.
func TestRun_HostCapabilities_SandboxWantedButUnavailable_Halts(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux", Sandbox: &profiles.SandboxConfig{Enabled: true}}, nil
	}
	opts.HostProbe = func() preflight.Profile {
		// NON-nested (ClaudeCode.Nested zero): a standalone host with a broken
		// sandbox is genuinely unconfined — the halt case. The nested variant
		// warns instead: TestRun_HostCapabilities_SandboxWantedNested_WarnsNotHalts.
		return preflight.Profile{Sandbox: preflight.Sandbox{ExpectedToWork: false, Reason: "sandbox binary present but sandbox_apply failed (standalone host)"}}
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !r.Halted() {
		t.Fatalf("required sandbox absence must halt; got %s", r.OverallLevel)
	}
	c := findCheck(t, r, "host-capabilities")
	if c.Level != LevelHalt {
		t.Fatalf("want LevelHalt, got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(strings.ToLower(c.Detail), "sandbox") {
		t.Fatalf("detail should mention sandbox; got %q", c.Detail)
	}
	if !strings.Contains(c.Detail, "required Build sandbox") {
		t.Errorf("HALT must name the required Build sandbox; got %q", c.Detail)
	}
	if strings.Contains(c.Detail, "degrades gracefully") {
		t.Errorf("WARN must drop the reassuring 'degrades gracefully' phrasing; got %q", c.Detail)
	}
}

func TestRun_HostCapabilities_NoSandboxProfiles_Passes(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux"}, nil
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "host-capabilities")
	if c.Level != LevelPass || strings.Contains(strings.ToLower(c.Detail), "sandbox") {
		t.Fatalf("profiles without sandbox should not warn, got %s (%q)", c.Level, c.Detail)
	}
}

func TestRun_HostCapabilities_LowDisk_Warns(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.DiskFreeBytes = func(string) (uint64, error) { return 100 << 20, nil } // 100 MiB
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Halted() {
		t.Fatalf("low disk must not halt; got %s", r.OverallLevel)
	}
	c := findCheck(t, r, "host-capabilities")
	if c.Level != LevelWarn {
		t.Fatalf("want LevelWarn, got %s", c.Level)
	}
}

// A disk-probe error is non-fatal: the check must not halt or warn on it.
func TestRun_HostCapabilities_DiskProbeError_Ignored(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.DiskFreeBytes = func(string) (uint64, error) { return 0, errors.New("statfs boom") }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "host-capabilities")
	if c.Level != LevelPass {
		t.Fatalf("disk-probe error should be ignored (pass), got %s (%s)", c.Level, c.Detail)
	}
}

// A NESTED launch is not an unconfined launch: the outer LLM-CLI session
// already imposes OS sandbox + Tier-1 hooks, which is exactly why the
// dispatch-time guard (bridge.sandboxRequiredButUnavailable) and the wrap
// policy (sandbox.ShouldWrap) both treat nested as requirement-satisfied.
// The 2026-09-01 first live launch after #518 hit this: preflight HALTed a
// nested host that dispatch would have happily (and safely) run. Preflight
// must agree with the gate it fronts: WARN with the doctrine, never HALT.
func TestRun_HostCapabilities_SandboxWantedNested_WarnsNotHalts(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux", Sandbox: &profiles.SandboxConfig{Enabled: true}}, nil
	}
	opts.HostProbe = func() preflight.Profile {
		return preflight.Profile{
			Sandbox:    preflight.Sandbox{ExpectedToWork: false, Reason: "Darwin nested-Claude: sandbox_apply() returns EPERM (rc=71)"},
			ClaudeCode: preflight.ClaudeCode{Nested: true},
		}
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Halted() {
		t.Fatalf("a nested launch is confined by the outer session and must not HALT; got %s", r.OverallLevel)
	}
	c := findCheck(t, r, "host-capabilities")
	if c.Level != LevelWarn {
		t.Fatalf("want LevelWarn surfacing the outer-confinement doctrine, got %s (%s)", c.Level, c.Detail)
	}
	// Honest-WARN register (sandbox-confinement-ssot.md slice 4, re-asserted by
	// the 2026-09-01 architecture review): state the POSTURE, never a
	// reassuring conclusion — the inner layer is unconfined, and the outer
	// session is unverified unless the canary (sibling check) verifies it.
	if !strings.Contains(c.Detail, "UNCONFINED at the inner layer") || !strings.Contains(c.Detail, "UNVERIFIED") {
		t.Fatalf("the WARN must state the honest posture; got %q", c.Detail)
	}
	if !strings.Contains(c.Detail, "sandbox-nested-fallback") {
		t.Fatalf("the WARN must point at the verifying sibling check; got %q", c.Detail)
	}
	for _, reassuring := range []string{"degrades gracefully", "runs via OUTER confinement", "already confine"} {
		if strings.Contains(c.Detail, reassuring) {
			t.Fatalf("reassuring phrasing %q must not appear (honest-WARN slice); got %q", reassuring, c.Detail)
		}
	}
}

// EVOLVE_SANDBOX=off is the host opt-out cell of the shared predicate: the
// dispatch gate honours it loudly and runs; preflight must WARN, not HALT
// (before the sandbox.ConfinementSatisfied extraction the two sites diverged
// in exactly this cell — bridge ran, preflight blocked).
func TestRun_HostCapabilities_SandboxWantedOptOut_WarnsNotHalts(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux", Sandbox: &profiles.SandboxConfig{Enabled: true}}, nil
	}
	opts.HostProbe = func() preflight.Profile {
		return preflight.Profile{Sandbox: preflight.Sandbox{ExpectedToWork: false, Reason: "sandbox binary present but sandbox_apply failed"}}
	}
	opts.SandboxMode = func() string { return "off" }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Halted() {
		t.Fatalf("explicit host opt-out must WARN loudly, not HALT (parity with the dispatch gate); got %s", r.OverallLevel)
	}
	c := findCheck(t, r, "host-capabilities")
	if c.Level != LevelWarn || !strings.Contains(c.Detail, "UNCONFINED") || !strings.Contains(c.Detail, "EVOLVE_SANDBOX=off") {
		t.Fatalf("want loud opt-out WARN, got %s (%s)", c.Level, c.Detail)
	}
}
