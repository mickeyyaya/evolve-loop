package looppreflight

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/bridge"
	"github.com/mickeyyaya/evolveloop/go/internal/preflight"
	"github.com/mickeyyaya/evolveloop/go/internal/profiles"
)

func TestRun_BridgeBoot_Skipped_Warns(t *testing.T) {
	opts := goodPipelineOptions(t) // SkipBoot: true
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "bridge-boot")
	if c.Level != LevelWarn {
		t.Fatalf("skipped boot should warn, got %s (%s)", c.Level, c.Detail)
	}
	if r.Halted() {
		t.Fatalf("skipped boot must not halt")
	}
}

func TestRun_BridgeBoot_Success_Passes(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.SkipBoot = false
	opts.BootTester = func(ctx context.Context, driver string, sandbox bool) (int, string) {
		return bridge.ExitOK, ""
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "bridge-boot")
	if c.Level != LevelPass {
		t.Fatalf("want LevelPass, got %s (%s)", c.Level, c.Detail)
	}
}

func TestRun_BridgeBoot_Timeout_Halts(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.SkipBoot = false
	opts.BootTester = func(ctx context.Context, driver string, sandbox bool) (int, string) {
		return bridge.ExitREPLBootTimeout, "...waiting for prompt marker\nboot timed out after 60s"
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !r.Halted() {
		t.Fatalf("boot timeout must halt, got %s", r.OverallLevel)
	}
	c := findCheck(t, r, "bridge-boot")
	if c.Level != LevelHalt {
		t.Fatalf("want LevelHalt, got %s", c.Level)
	}
	if !strings.Contains(c.Detail, "claude-tmux") {
		t.Fatalf("detail should name the driver; got %q", c.Detail)
	}
	if !strings.Contains(c.Detail, "boot timed out") {
		t.Fatalf("detail should carry the scrollback tail; got %q", c.Detail)
	}
}

// Only *-tmux drivers have a bootable REPL; a -p driver must be skipped.
func TestRun_BridgeBoot_OnlyTmuxDrivers(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.SkipBoot = false
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-p", CLIFallback: []string{"claude-tmux"}}, nil
	}
	var booted []string
	opts.BootTester = func(ctx context.Context, driver string, sandbox bool) (int, string) {
		booted = append(booted, driver)
		return bridge.ExitOK, ""
	}
	if _, err := Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(booted) != 1 || booted[0] != "claude-tmux" {
		t.Fatalf("only claude-tmux should boot, booted=%v", booted)
	}
}

func TestRun_BridgeBoot_SandboxEngageDerived(t *testing.T) {
	base := func(t *testing.T, expectWork bool) (sandboxArg bool) {
		opts := goodPipelineOptions(t)
		opts.SkipBoot = false
		opts.ProfileGetter = func(name string) (profiles.Profile, error) {
			return profiles.Profile{Name: name, CLI: "claude-tmux", Sandbox: &profiles.SandboxConfig{Enabled: true}}, nil
		}
		opts.HostProbe = func() preflight.Profile {
			return preflight.Profile{Sandbox: preflight.Sandbox{ExpectedToWork: expectWork, SandboxExecAvailable: expectWork}}
		}
		opts.BootTester = func(ctx context.Context, driver string, sandbox bool) (int, string) {
			sandboxArg = sandbox
			return bridge.ExitOK, ""
		}
		if _, err := Run(opts); err != nil {
			t.Fatalf("Run: %v", err)
		}
		return sandboxArg
	}

	if !base(t, true) {
		t.Fatalf("sandbox-enabled profile + host sandbox works → boot should engage sandbox")
	}
	if base(t, false) {
		t.Fatalf("host sandbox not expected to work → boot should NOT engage sandbox")
	}
}

func TestBootRCName(t *testing.T) {
	cases := []struct {
		name    string
		rc      int
		contain string
	}{
		{"ExitREPLBootTimeout", bridge.ExitREPLBootTimeout, "ExitREPLBootTimeout"},
		{"ExitMissingBinary", bridge.ExitMissingBinary, "ExitMissingBinary"},
		{"ExitBadFlags", bridge.ExitBadFlags, "ExitBadFlags"},
		{"WorkspaceSetupFailed", exitWorkspaceSetupFailed, "workspace setup failed"},
		{"UnknownCode1", 1, "boot failure"},
		{"UnknownCode99", 99, "boot failure"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := bootRCName(tc.rc)
			if !strings.Contains(got, tc.contain) {
				t.Errorf("bootRCName(%d) = %q; want substring %q", tc.rc, got, tc.contain)
			}
		})
	}
}

func TestNewDefaultBootTester_ReturnsNonNil(t *testing.T) {
	tester := newDefaultBootTester(t.TempDir(), io.Discard)
	if tester == nil {
		t.Fatal("newDefaultBootTester returned nil")
	}
}
