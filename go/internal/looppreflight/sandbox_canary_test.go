package looppreflight

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/preflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// TestSandboxNestedFallbackCanary — the verified-fallback canary gates on the
// nested_fallback dial: off ⇒ no canary (the dormant default); shadow ⇒ WARN
// when the outer environment fails to block an out-of-allowlist write; enforce
// ⇒ HALT. A verified (blocked) write, or a standalone session, ⇒ Pass.
func TestSandboxNestedFallbackCanary(t *testing.T) {
	sandboxProfile := func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "claude-tmux", Sandbox: &profiles.SandboxConfig{Enabled: true}}, nil
	}
	nestedHost := func() preflight.Profile {
		return preflight.Profile{
			ClaudeCode: preflight.ClaudeCode{Nested: true},
			Sandbox:    preflight.Sandbox{ExpectedToWork: false, Reason: "nested EPERM"},
		}
	}

	cases := []struct {
		name      string
		stage     config.Stage
		nested    bool
		noSandbox bool // leave the default (no-sandbox) profile ⇒ sandboxWanted=false
		blocked   bool // canary verdict: true = outer blocked the write (verified)
		wantLevel CheckLevel
	}{
		{"off → no canary, pass", config.StageOff, true, false, false, LevelPass},
		{"shadow + unverified → warn", config.StageShadow, true, false, false, LevelWarn},
		{"shadow + verified → pass", config.StageShadow, true, false, true, LevelPass},
		{"enforce + unverified → halt", config.StageEnforce, true, false, false, LevelHalt},
		{"enforce + verified → pass", config.StageEnforce, true, false, true, LevelPass},
		{"enforce + standalone → pass (not engaged)", config.StageEnforce, false, false, false, LevelPass},
		{"enforce + no-sandbox-profile → pass (not engaged)", config.StageEnforce, true, true, false, LevelPass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := goodPipelineOptions(t)
			if !tc.noSandbox {
				opts.ProfileGetter = sandboxProfile
			}
			if tc.nested {
				opts.HostProbe = nestedHost
			}
			opts.NestedFallbackStage = tc.stage
			opts.SandboxCanaryProbe = func() bool { return tc.blocked }
			r, err := Run(opts)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			c := findCheck(t, r, "sandbox-nested-fallback")
			if c.Level != tc.wantLevel {
				t.Errorf("level = %s, want %s (detail: %q)", c.Level, tc.wantLevel, c.Detail)
			}
			if tc.wantLevel == LevelHalt && !r.Halted() {
				t.Error("enforce-unverified must make the batch halt")
			}
		})
	}
}
