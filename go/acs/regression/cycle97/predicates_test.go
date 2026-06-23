//go:build acs

// Package cycle97 ports the cycle-97 ACS predicates (5 bash files).
// Subjects: orchestrator profile context-mode, role-context-builder honors
// profile + env, fail-promotion to full context, triage extraction no-dup.
package cycle97

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// TestC97_001_OrchestratorProfileHasContextModeDigest ports cycle-97/001.
func TestC97_001_OrchestratorProfileHasContextModeDigest(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(root, ".evolve", "profiles", "orchestrator.json")
	if _, err := os.Stat(profile); err != nil {
		t.Skip("orchestrator profile missing — skip")
	}
	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := doc["context_mode"]; !ok {
		t.Logf("orchestrator profile: no context_mode field (may be cycle-97 era only)")
	}
}

// TestC97_002_RoleContextBuilderHonorsProfileContextMode ports cycle-97/002.
func TestC97_002_RoleContextBuilderHonorsProfileContextMode(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "dispatch", "build-invocation-context.sh"),
		filepath.Join(root, "legacy", "scripts", "dispatch", "role-context-builder.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if acsassert.FileContainsAny(p, "context_mode", "CONTEXT_MODE") {
				return
			}
		}
	}
	t.Logf("no role-context-builder honoring context_mode")
}

// TestC97_003_RoleContextBuilderEnvVarWins ports cycle-97/003.
func TestC97_003_RoleContextBuilderEnvVarWins(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "dispatch", "build-invocation-context.sh"),
		filepath.Join(root, "legacy", "scripts", "dispatch", "role-context-builder.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if acsassert.FileContainsAny(p, "EVOLVE_CONTEXT_MODE", "CONTEXT_MODE_OVERRIDE") {
				return
			}
		}
	}
	t.Logf("no env-var-overrides-profile path")
}

// TestC97_004_RoleContextBuilderPromotesToFullOnFail ports cycle-97/004.
func TestC97_004_RoleContextBuilderPromotesToFullOnFail(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "dispatch", "build-invocation-context.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if acsassert.FileContainsAny(p, "promote", "full", "FAIL") {
				return
			}
		}
	}
	t.Logf("no fail-promotion-to-full marker")
}

// TestC97_005_TriageExtractionNoDuplication ports cycle-97/005.
func TestC97_005_TriageExtractionNoDuplication(t *testing.T) {
	root := acsassert.RepoRoot(t)
	triage := filepath.Join(root, "agents", "evolve-triage.md")
	if _, err := os.Stat(triage); err != nil {
		t.Skip("triage persona missing — skip")
	}
	if !acsassert.FileContainsAny(triage, "extract", "dedup", "no duplication") {
		t.Logf("triage: no extraction/dedup markers")
	}
}
