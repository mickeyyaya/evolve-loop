package bridge

import (
	"context"
	"testing"
)

// probe_test.go — tests for resolveTier + Engine.Probe (the embedded
// manifest set), using a controlled LookPath so tiering is deterministic
// regardless of which CLIs are actually installed on the host.

func hasBinaryIn(set ...string) func(string) bool {
	m := map[string]bool{}
	for _, s := range set {
		m[s] = true
	}
	return func(b string) bool { return m[b] }
}

func TestResolveTier(t *testing.T) {
	full := Manifest{Binary: "claude", DefaultTier: "full", TierDependencies: map[string][]string{"full": {"claude", "tmux"}}}
	cases := []struct {
		name  string
		m     Manifest
		avail []string
		want  string
	}{
		{"stub → none", Manifest{Binary: "x", Stub: true}, []string{"x"}, "none"},
		{"binary absent → none", full, nil, "none"},
		{"all deps present → declared", full, []string{"claude", "tmux"}, "full"},
		{"missing dep → degraded", full, []string{"claude"}, "degraded"},
		{"empty default_tier → full", Manifest{Binary: "c", TierDependencies: map[string][]string{"full": {"c"}}}, []string{"c"}, "full"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveTier(tc.m, hasBinaryIn(tc.avail...)); got != tc.want {
				t.Fatalf("resolveTier = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEngineProbe_NoBinaries_AllNone(t *testing.T) {
	eng := NewEngine(Deps{LookPath: func(string) (string, error) { return "", errNoBin }})
	p, err := eng.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe err: %v", err)
	}
	if len(p.CLIs) == 0 {
		t.Fatal("Probe should enumerate the embedded manifests")
	}
	for cli, tier := range p.CLIs {
		if tier != "none" {
			t.Fatalf("cli %s tier = %q, want none (no binaries available)", cli, tier)
		}
	}
}

func TestEngineProbe_ClaudeTmuxAvailable(t *testing.T) {
	// claude + tmux present → claude-tmux resolves to its declared tier (hybrid).
	eng := NewEngine(Deps{LookPath: func(b string) (string, error) {
		if b == "claude" || b == "tmux" {
			return "/usr/bin/" + b, nil
		}
		return "", errNoBin
	}})
	p, _ := eng.Probe(context.Background())
	if tier := p.CLIs["claude-tmux"]; tier == "none" || tier == "" {
		t.Fatalf("claude-tmux tier = %q, want non-none with claude+tmux present", tier)
	}
	if p.Version == "" {
		t.Fatal("Probe should report an OS version string")
	}
}

var errNoBin = &probeErr{"not found"}

type probeErr struct{ s string }

func (e *probeErr) Error() string { return e.s }
