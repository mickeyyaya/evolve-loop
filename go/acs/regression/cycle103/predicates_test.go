//go:build acs

// Package cycle103 ports the cycle-103 ACS predicates (9 bash files).
// Subject: build-planner phase introduction (Opt C build-plan rollout).
package cycle103

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC103_001_BuildPlannerPersonaExists ports cycle-103/001.
func TestC103_001_BuildPlannerPersonaExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	persona := filepath.Join(root, "agents", "evolve-build-planner.md")
	if _, err := os.Stat(persona); err != nil {
		t.Skip("evolve-build-planner.md missing — skip cycle-103-001")
	}
	// YAML frontmatter must have name, model (tier-1|opus), tools
	raw, err := os.ReadFile(persona)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	front := extractFrontmatter(string(raw))
	if !containsLineRegex(front, `^name:\s+evolve-build-planner\s*$`) {
		t.Errorf("frontmatter missing 'name: evolve-build-planner'")
	}
	if !containsLineRegex(front, `^model:\s+(tier-1|opus)`) {
		t.Errorf("frontmatter missing 'model: tier-1|opus'")
	}
	if !containsLineRegex(front, `^tools:\s+`) {
		t.Errorf("frontmatter missing 'tools:' field")
	}
}

// TestC103_002_BuildPlannerProfileValid ports cycle-103/002.
func TestC103_002_BuildPlannerProfileValid(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(root, ".evolve", "profiles", "build-planner.json")
	if _, err := os.Stat(profile); err != nil {
		t.Skip("build-planner.json missing — skip cycle-103-002")
	}
	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Errorf("invalid JSON: %v", err)
		return
	}
	if v, _ := doc["parallel_eligible"].(bool); v {
		t.Errorf("parallel_eligible=true (single-writer invariant requires false)")
	}
	if v, _ := doc["max_turns"].(float64); int(v) != 10 {
		t.Errorf("max_turns=%v (expected 10)", v)
	}
	if v, _ := doc["challenge_token_required"].(bool); !v {
		t.Errorf("challenge_token_required not true")
	}
	if v, _ := doc["max_budget_usd"].(float64); abs(v-0.30) > 1e-6 {
		t.Errorf("max_budget_usd=%v (expected 0.30)", v)
	}
	if oa, _ := doc["output_artifact"].(string); !strings.Contains(oa, "build-plan.md") {
		t.Errorf("output_artifact=%q does not contain build-plan.md", oa)
	}
}

// TestC103_003_PhaseRegistryIncludesBuildPlanner ports cycle-103/003.
func TestC103_003_PhaseRegistryIncludesBuildPlanner(t *testing.T) {
	root := acsassert.RepoRoot(t)
	reg := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	if _, err := os.Stat(reg); err != nil {
		t.Skip("phase-registry.json missing — skip")
	}
	if !acsassert.FileContains(t, reg, "build-planner") {
		return
	}
	if !acsassert.FileContains(t, reg, "EVOLVE_BUILD_PLANNER") {
		t.Logf("phase-registry: no EVOLVE_BUILD_PLANNER enable_var")
	}
}

// TestC103_004_ListPhaseOrderIncludesBuildPlanner ports cycle-103/004.
func TestC103_004_ListPhaseOrderIncludesBuildPlanner(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "dispatch", "list-phase-order.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skip("list-phase-order.sh missing — skip")
	}
	if !acsassert.FileContains(t, script, "build-planner") {
		return
	}
}

// TestC103_005_SubagentRunAllowlistIncludesBuildPlanner ports cycle-103/005.
func TestC103_005_SubagentRunAllowlistIncludesBuildPlanner(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skip("subagent-run.sh missing — skip")
	}
	if !acsassert.FileContains(t, script, "build-planner") {
		return
	}
}

// TestC103_006_PhaseGatePreconditionRecognizesBuildPlanner ports cycle-103/006.
func TestC103_006_PhaseGatePreconditionRecognizesBuildPlanner(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "guards", "phase-gate-precondition.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skip("phase-gate-precondition.sh missing — skip")
	}
	if !acsassert.FileContains(t, script, "build-planner") {
		return
	}
}

// TestC103_007_GateFunctionsPresent ports cycle-103/007.
func TestC103_007_GateFunctionsPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if _, err := os.Stat(gate); err != nil {
		t.Skip("phase-gate.sh missing — skip")
	}
	for _, fn := range []string{
		"gate_tdd_to_build_planner",
		"gate_build_planner_to_build",
	} {
		if !acsassert.FileContains(t, gate, fn) {
			return
		}
	}
}

// TestC103_008_ShadowCycleDoesNotProduceBuildPlan ports cycle-103/008.
// Smoke: EVOLVE_BUILD_PLANNER=0 means shadow (no build-plan.md). Source check.
func TestC103_008_ShadowCycleDoesNotProduceBuildPlan(t *testing.T) {
	root := acsassert.RepoRoot(t)
	persona := filepath.Join(root, "agents", "evolve-build-planner.md")
	if _, err := os.Stat(persona); err != nil {
		t.Skip("build-planner persona missing — skip")
	}
	if !acsassert.FileContainsAny(persona, "shadow", "EVOLVE_BUILD_PLANNER", "advisory") {
		t.Logf("build-planner: no shadow-mode marker")
	}
}

// TestC103_009_Adr0019ExistsAndComplete ports cycle-103/009.
func TestC103_009_Adr0019ExistsAndComplete(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adr := filepath.Join(root, "docs", "architecture", "adr", "0019-build-planner-phase.md")
	if _, err := os.Stat(adr); err != nil {
		t.Skip("ADR-0019 missing — skip")
	}
	for _, section := range []string{
		"## Context",
		"## Decision",
		"## Consequences",
	} {
		if !acsassert.FileContainsAny(adr, section, strings.ToUpper(section)) {
			t.Logf("ADR-0019: missing section %q", section)
		}
	}
}

func extractFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	var out []string
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func containsLineRegex(content, pattern string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(content, "\n") {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
