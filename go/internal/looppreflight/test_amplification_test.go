package looppreflight

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/preflight"
	"github.com/mickeyyaya/evolveloop/go/internal/profiles"
)

func TestAmplifyRun_HostCapabilitiesMultipleWarningsStayNonHalting(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux", Sandbox: &profiles.SandboxConfig{Enabled: true}}, nil
	}
	opts.HostProbe = func() preflight.Profile {
		return preflight.Profile{Sandbox: preflight.Sandbox{ExpectedToWork: false, Reason: "sandbox unavailable"}}
	}
	opts.DiskFreeBytes = func(string) (uint64, error) { return 100 << 20, nil }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Halted() {
		t.Fatalf("warning-only host degradations must not halt, got %s", r.OverallLevel)
	}
	c := findCheck(t, r, "host-capabilities")
	if c.Level != LevelWarn {
		t.Fatalf("combined host degradations should warn, got %s (%q)", c.Level, c.Detail)
	}
	detail := strings.ToLower(c.Detail)
	for _, want := range []string{"sandbox", "disk"} {
		if !strings.Contains(detail, strings.ToLower(want)) {
			t.Fatalf("combined warning detail should include %q, got %q", want, c.Detail)
		}
	}
}

func TestAmplifyRun_PipelineStructureUnknownFallbackHalts(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux", CLIFallback: []string{"not-a-driver"}}, nil
	}
	opts.DriverKnown = func(cli string) bool { return cli != "not-a-driver" }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !r.Halted() {
		t.Fatalf("unknown fallback driver must halt, got %s", r.OverallLevel)
	}
	c := findCheck(t, r, "pipeline-structure")
	if c.Level != LevelHalt || !strings.Contains(c.Detail, "not-a-driver") {
		t.Fatalf("pipeline-structure should name unknown fallback, got %s (%q)", c.Level, c.Detail)
	}
}
