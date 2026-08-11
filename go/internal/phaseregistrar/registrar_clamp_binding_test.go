package phaseregistrar

// registrar_clamp_binding_test.go — cross-binding pin for the two enforcement
// styles of `writes_source ⟹ sandboxed dispatch profile` (ADR-0073): the
// registrar enforces it by CONSTRUCTION (Register forces Sandbox.Enabled into
// the profile it persists), the discovery clamp by VERIFICATION
// (phasespec.SandboxedProfilePredicate over the same on-disk profile). This
// test drives a real writes_source mint through Register and asserts the
// persisted profile passes the discovery-side predicate — so the two paths
// can never drift into a state where a legitimately-minted writer is
// stripped (or a strip-evading profile shape passes).

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestRegisterMintedWriter_PassesDiscoveryClampPredicate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := Registrar{ProfilesDir: dir}
	res, err := r.Register(phaseconfig.PhaseConfig{
		PhaseSpec: phasespec.PhaseSpec{Name: "coverage-writer", WritesSource: true},
		Dispatch:  phaseconfig.Dispatch{CLI: "claude"},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	profile := strings.TrimPrefix(res.Spec.AgentName(), "evolve-")
	pred := phasespec.SandboxedProfilePredicate(dir)
	if !pred(profile) {
		t.Fatalf("registrar-minted writes_source profile %q does not satisfy the discovery clamp predicate — a legitimately minted writer would be stripped at the next boot (files: %v)",
			profile, mustList(t, dir))
	}

	clamped, _ := phasespec.ClampDiscoveredSpecs([]phasespec.PhaseSpec{res.Spec}, pred)
	if !clamped[0].WritesSource {
		t.Fatal("discovery clamp stripped a registrar-minted writer — construction and verification drifted")
	}
}

func mustList(t *testing.T, dir string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}
