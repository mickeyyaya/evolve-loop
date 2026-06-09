package looppreflight

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/doctor"
	"github.com/mickeyyaya/evolve-loop/go/internal/preflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// fixedNow is a deterministic clock for tests.
func fixedNow() time.Time { return time.Unix(0, 0).UTC() }

// goodPipelineOptions returns an Options on which EVERY check passes: each spine
// phase has a factory + contract, the one profile loads and its CLI resolves to
// a known driver, every binary probes found, the host has full capabilities,
// disk is ample, and no stale bridge sessions linger. Every external lookup is a
// seam so the test needs no real registry/driver/profile-dir/host state.
// SkipBoot keeps the real bridge boot inert. Tests override individual seams to
// exercise one failure at a time.
func goodPipelineOptions(t *testing.T) Options {
	t.Helper()
	return Options{
		ProjectRoot:   t.TempDir(),
		EvolveDir:     t.TempDir(),
		Now:           fixedNow,
		SkipBoot:      true,
		SpinePhases:   []string{"build", "scout"},
		FactoryKnown:  func(string) bool { return true },
		ContractKnown: func(string) bool { return true },
		ProfileLister: func() ([]string, error) { return []string{"builder"}, nil },
		ProfileGetter: func(name string) (profiles.Profile, error) {
			return profiles.Profile{Name: name, CLI: "claude-tmux"}, nil
		},
		DriverKnown: func(string) bool { return true },
		ProbeCLI: func(bin string) (doctor.Result, error) {
			return doctor.Result{Tool: bin, Found: true, Path: "/usr/bin/" + bin, Method: "path"}, nil
		},
		HostProbe: func() preflight.Profile {
			return preflight.Profile{Sandbox: preflight.Sandbox{ExpectedToWork: true, SandboxExecAvailable: true}}
		},
		DirWritable:   func(string) bool { return true },
		DiskFreeBytes: func(string) (uint64, error) { return 50 << 30, nil }, // 50 GiB
		TmuxSessions:  func() ([]string, error) { return nil, nil },
		// Freeze seams (ADR-0044 C5): benign defaults so unrelated tests
		// never stat the real ~/.codex or exec real brew.
		SelfUpdateEvidence: func(string) (bool, string, error) { return false, "", nil },
		PinnedLister:       func() ([]string, error) { return nil, nil },
	}
}

// findCheck returns the CheckResult with the given name, or fails the test.
func findCheck(t *testing.T, r Result, name string) CheckResult {
	t.Helper()
	for _, c := range r.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no check named %q in result (checks: %+v)", name, r.Checks)
	return CheckResult{}
}

func TestRun_PipelineStructure_AllGood(t *testing.T) {
	r, err := Run(goodPipelineOptions(t))
	if err != nil {
		t.Fatalf("Run returned harness error: %v", err)
	}
	if r.Halted() {
		t.Fatalf("expected no halt, got OverallLevel=%s checks=%+v", r.OverallLevel, r.Checks)
	}
	c := findCheck(t, r, "pipeline-structure")
	if c.Level != LevelPass {
		t.Fatalf("pipeline-structure: want LevelPass, got %s (%s)", c.Level, c.Detail)
	}
}

func TestRun_PipelineStructure_MissingContract_Halts(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ContractKnown = func(name string) bool { return name != "scout" }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run returned harness error: %v", err)
	}
	if !r.Halted() {
		t.Fatalf("expected halt for missing contract, got %s", r.OverallLevel)
	}
	c := findCheck(t, r, "pipeline-structure")
	if c.Level != LevelHalt {
		t.Fatalf("want LevelHalt, got %s", c.Level)
	}
	if !strings.Contains(c.Detail, "scout") {
		t.Fatalf("detail should name the phase with the missing contract; got %q", c.Detail)
	}
}

func TestRun_PipelineStructure_UnknownCLI_Halts(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "bogus-cli"}, nil
	}
	opts.DriverKnown = func(cli string) bool { return cli != "bogus-cli" }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run returned harness error: %v", err)
	}
	if !r.Halted() {
		t.Fatalf("expected halt for unknown CLI, got %s", r.OverallLevel)
	}
	c := findCheck(t, r, "pipeline-structure")
	if !strings.Contains(c.Detail, "bogus-cli") {
		t.Fatalf("detail should name the unresolved CLI; got %q", c.Detail)
	}
}

// A single check accumulates ALL gaps — the operator sees every problem at once,
// not just the first.
func TestRun_PipelineStructure_AccumulatesAllGaps(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ContractKnown = func(name string) bool { return name != "build" }
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux", CLIFallback: []string{"ghost-tmux"}}, nil
	}
	opts.DriverKnown = func(cli string) bool { return cli != "ghost-tmux" }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run returned harness error: %v", err)
	}
	c := findCheck(t, r, "pipeline-structure")
	if !strings.Contains(c.Detail, "build") {
		t.Fatalf("detail should report the missing contract; got %q", c.Detail)
	}
	if !strings.Contains(c.Detail, "ghost-tmux") {
		t.Fatalf("detail should report the unresolved fallback CLI; got %q", c.Detail)
	}
}

func TestRun_EmptyProjectRoot_IsHarnessError(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProjectRoot = ""
	if _, err := Run(opts); err == nil {
		t.Fatalf("expected a harness error for empty ProjectRoot, got nil")
	}
}
