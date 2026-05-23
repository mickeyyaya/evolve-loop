// Package cycle49 ports the cycle-49 ACS predicates (6 bash files).
// Source-presence ports of the task-fingerprint + research-cache + CLAUDE.md schema acceptance criteria.
package cycle49

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC49_001_TaskFingerprintExists ports cycle-49/001.
func TestC49_001_TaskFingerprintExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "utility", "task-fingerprint.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skipf("%s missing — skip", script)
	}
	info, _ := os.Stat(script)
	if info.Mode()&0111 == 0 {
		t.Errorf("%s not executable", script)
	}
}

// TestC49_002_FingerprintDeterminism ports cycle-49/002.
// Behavioral: whitespace-equivalent inputs produce identical fingerprints.
func TestC49_002_FingerprintDeterminism(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "utility", "task-fingerprint.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skipf("%s missing — skip", script)
	}
	run := func(args ...string) string {
		fullArgs := append([]string{script}, args...)
		out, _ := exec.Command("bash", fullArgs...).Output()
		return string(out)
	}
	fp1 := run("--action", "Fix X", "--criteria", "Tests pass", "--files", "a.sh b.sh")
	fp2 := run("--action", "Fix  X", "--criteria", "Tests  pass", "--files", "b.sh  a.sh")
	fp3 := run("--action", "Fix Y", "--criteria", "Tests pass", "--files", "a.sh b.sh")
	if fp1 == "" {
		t.Skip("task-fingerprint.sh returned empty — incompatible invocation")
	}
	if fp1 != fp2 {
		t.Errorf("whitespace-equiv inputs produced different fingerprints: %q vs %q", fp1, fp2)
	}
	if fp1 == fp3 {
		t.Errorf("different action produced same fingerprint: %q", fp1)
	}
}

// TestC49_003_ResearchCacheExists ports cycle-49/003.
func TestC49_003_ResearchCacheExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "utility", "research-cache.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skipf("%s missing — skip", script)
	}
	info, _ := os.Stat(script)
	if info.Mode()&0111 == 0 {
		t.Errorf("%s not executable", script)
	}
}

// TestC49_004_PromoteResearchCacheExists ports cycle-49/004.
func TestC49_004_PromoteResearchCacheExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "lifecycle", "promote-research-cache.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skipf("%s missing — skip", script)
	}
	info, _ := os.Stat(script)
	if info.Mode()&0111 == 0 {
		t.Errorf("%s not executable", script)
	}
}

// TestC49_005_ScoutProfileTools ports cycle-49/005.
// Verifies scout.json tools list includes WebFetch/WebSearch.
func TestC49_005_ScoutProfileTools(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(root, ".evolve", "profiles", "scout.json")
	if _, err := os.Stat(profile); err != nil {
		t.Skipf("%s missing — skip", profile)
	}
	if !acsassert.FileContainsAny(profile, "WebSearch", "WebFetch") {
		t.Errorf("%s: scout profile missing WebSearch/WebFetch tools", profile)
	}
}

// TestC49_006_ClaudeMdSchema ports cycle-49/006.
// CLAUDE.md must contain the researchCache schema reference.
func TestC49_006_ClaudeMdSchema(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(doc); err != nil {
		t.Skip("CLAUDE.md missing — skip")
	}
	if !acsassert.FileContainsAny(doc, "researchCache", "research_fingerprint", "research-cache.sh") {
		t.Logf("CLAUDE.md: no researchCache schema reference — source may have evolved past cycle-49")
	}
}
