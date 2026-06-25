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

// Profiles request sandboxing but the host won't sandbox → Warn (degrades
// gracefully), never Halt.
func TestRun_HostCapabilities_SandboxWantedButUnavailable_Warns(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux", Sandbox: &profiles.SandboxConfig{Enabled: true}}, nil
	}
	opts.HostProbe = func() preflight.Profile {
		return preflight.Profile{Sandbox: preflight.Sandbox{ExpectedToWork: false, Reason: "Darwin nested-Claude: EPERM"}}
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Halted() {
		t.Fatalf("sandbox-degraded must not halt; got %s", r.OverallLevel)
	}
	c := findCheck(t, r, "host-capabilities")
	if c.Level != LevelWarn {
		t.Fatalf("want LevelWarn, got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(strings.ToLower(c.Detail), "sandbox") {
		t.Fatalf("detail should mention sandbox; got %q", c.Detail)
	}
	// The WARN must be HONEST (P2 fix): it states phases run UNCONFINED at the
	// inner layer, not the reassuring "degrades gracefully" that hid the loss of
	// inner write-confinement.
	if !strings.Contains(c.Detail, "UNCONFINED at the inner layer") {
		t.Errorf("WARN must honestly state phases run UNCONFINED at the inner layer; got %q", c.Detail)
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
