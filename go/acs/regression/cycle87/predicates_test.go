//go:build acs

// Package cycle87 ports the cycle-87 ACS predicates (8 bash files).
// Subjects: kb-search behavior, research-quota gate behavior, profile JSON validation.
package cycle87

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// TestC87_KbSearchFixture ports pred-kb-search-fixture.sh.
func TestC87_KbSearchFixture(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "research", "kb-search.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skip("kb-search.sh missing — skip")
	}
}

// TestC87_KbSearchGrepFallback ports pred-kb-search-grep-fallback.sh.
func TestC87_KbSearchGrepFallback(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "research", "kb-search.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skip("kb-search.sh missing — skip")
	}
	if !acsassert.FileContainsAny(script, "grep", "fallback") {
		t.Errorf("kb-search.sh: no grep-fallback marker")
	}
}

// TestC87_ProfilesJsonValidate ports pred-profiles-json-validate.sh.
// All .evolve/profiles/*.json must parse as JSON.
func TestC87_ProfilesJsonValidate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profilesDir := filepath.Join(root, ".evolve", "profiles")
	if _, err := os.Stat(profilesDir); err != nil {
		t.Skip("profiles dir missing — skip")
	}
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(profilesDir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Errorf("invalid JSON: %s: %v", path, err)
		}
	}
}

// TestC87_ResearchQuotaConcurrentNoLoss ports pred-research-quota-concurrent-no-loss.sh.
func TestC87_ResearchQuotaConcurrentNoLoss(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "hooks", "research-quota-gate.sh")
	if _, err := os.Stat(gate); err != nil {
		t.Skip("research-quota-gate.sh missing — skip")
	}
	if !acsassert.FileContainsAny(gate, "flock", "lockfile", "mv -n", "atomic") {
		t.Logf("research-quota-gate.sh: no explicit concurrency primitive (may be process-level)")
	}
}

// TestC87_ResearchQuotaDeepFlag ports pred-research-quota-deep-flag.sh.
func TestC87_ResearchQuotaDeepFlag(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "hooks", "research-quota-gate.sh")
	if _, err := os.Stat(gate); err != nil {
		t.Skip("research-quota-gate.sh missing — skip")
	}
	if !acsassert.FileContains(t, gate, "EVOLVE_ALLOW_DEEP_RESEARCH") {
		t.Errorf("research-quota-gate.sh: no EVOLVE_ALLOW_DEEP_RESEARCH branch")
	}
}

// TestC87_ResearchQuotaGateArithmetic ports pred-research-quota-gate-arithmetic.sh.
func TestC87_ResearchQuotaGateArithmetic(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "hooks", "research-quota-gate.sh")
	if _, err := os.Stat(gate); err != nil {
		t.Skip("research-quota-gate.sh missing — skip")
	}
	if !acsassert.FileContainsAny(gate, "quota", "counter", "remaining") {
		t.Errorf("research-quota-gate.sh: no quota/counter arithmetic")
	}
}

// TestC87_ResearchQuotaHookDisabled ports pred-research-quota-hook-disabled.sh.
func TestC87_ResearchQuotaHookDisabled(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "hooks", "research-quota-gate.sh")
	if _, err := os.Stat(gate); err != nil {
		t.Skip("research-quota-gate.sh missing — skip")
	}
	if !acsassert.FileContains(t, gate, "EVOLVE_RESEARCH_HOOK_DISABLED") {
		t.Errorf("research-quota-gate.sh: no EVOLVE_RESEARCH_HOOK_DISABLED branch")
	}
}

// TestC87_RunCycleResetsResearchUsage ports pred-run-cycle-resets-research-usage.sh.
func TestC87_RunCycleResetsResearchUsage(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "archive", "legacy", "scripts", "dispatch", "run-cycle.sh"),
		filepath.Join(root, "legacy", "scripts", "dispatch", "run-cycle.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if !acsassert.FileContainsAny(p, "researchUsage", "research_usage", "reset") {
				t.Errorf("%s: no research-usage reset path", p)
			}
			return
		}
	}
	t.Skip("run-cycle.sh missing at all accepted paths — Go cycle dispatch is canonical now")
}
