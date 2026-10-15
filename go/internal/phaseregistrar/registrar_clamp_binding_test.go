package phaseregistrar

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
