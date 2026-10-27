package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var envVarRE = regexp.MustCompile(`EVOLVE_[A-Z][A-Z0-9_]*`)

var allowedUndocumented = map[string]bool{
	"EVOLVE_TEST_COST_THRESHOLD":           true,
	"EVOLVE_TEST_COST_GUARD_STRICT":        true,
	"EVOLVE_TESTING":                       true,
	"EVOLVE_BUILD_MODEL":                   true,
	"EVOLVE_SCOUT_MODEL":                   true,
	"EVOLVE_INTENT_MODEL":                  true,
	"EVOLVE_TRIAGE_MODEL":                  true,
	"EVOLVE_TDD_MODEL":                     true,
	"EVOLVE_AUDIT_MODEL":                   true,
	"EVOLVE_BUILD_PERMISSION_MODE":         true,
	"EVOLVE_SCOUT_PERMISSION_MODE":         true,
	"EVOLVE_INTENT_PERMISSION_MODE":        true,
	"EVOLVE_TRIAGE_PERMISSION_MODE":        true,
	"EVOLVE_TDD_PERMISSION_MODE":           true,
	"EVOLVE_AUDIT_PERMISSION_MODE":         true,
	"EVOLVE_BUILD_PLAN_INPUT":              true,
	"EVOLVE_BUILD_PLAN_OUTPUT":             true,
	"EVOLVE_TDD_ENGINEER_MODEL":            true,
	"EVOLVE_TDD_ENGINEER_PERMISSION_MODE":  true,
	"EVOLVE_PLAN_REVIEWER_PERMISSION_MODE": true,
	"EVOLVE_SKIP_CYCLE_HEALTH":             true,

	"EVOLVE_HANG_CLASSIFIER":           true,
	"EVOLVE_PHASE_":                    true,
	"EVOLVE_PHASE_BUILD_BIN":           true,
	"EVOLVE_PLUGIN_ROOT":               true,
	"EVOLVE_PROJECT_ROOT":              true,
	"EVOLVE_QUOTA_RESET_AT":            true,
	"EVOLVE_QUOTA_RESET_HOURS":         true,
	"EVOLVE_RELEASE_REQUIRE_PREFLIGHT": true,
	"EVOLVE_RESUME_ALLOW_HEAD_MOVED":   true,
	"EVOLVE_SHIP_SCRIPT":               true,
	"EVOLVE_TRACKER_TTL_DAYS":          true,
	"EVOLVE_USE_PHASE_REGISTRY":        true,
}

func TestEnvVars_DocumentedInCLAUDEmd(t *testing.T) {
	t.Parallel()
	repoRoot := findRepoRoot(t)
	var claudeBody string
	for _, rel := range []string{"CLAUDE.md", filepath.Join("docs", "operations", "runtime-reference.md")} {
		body, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		claudeBody += string(body) + "\n"
	}

	used := collectEnvVarsFromCode(t, filepath.Join(repoRoot, "go"))
	var missing []string
	for v := range used {
		if allowedUndocumented[v] {
			continue
		}
		if !strings.Contains(claudeBody, v) {
			missing = append(missing, v)
		}
	}
	sort.Strings(missing)
	if len(missing) == 0 {
		return
	}
	msg := fmt.Sprintf("docs-contract: %d EVOLVE_* env vars used in code but not in CLAUDE.md or docs/operations/runtime-reference.md:\n  %s\n\n"+
		"Either add rows to the runtime-reference.md 'Current behavior' table OR add entries to allowedUndocumented "+
		"in docs_contract_test.go (with a one-line rationale comment per entry).",
		len(missing), strings.Join(missing, "\n  "))
	if os.Getenv("EVOLVE_DOCS_CONTRACT_STRICT") == "1" {
		t.Error(msg)
	} else {
		t.Log(msg)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := cwd
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find CLAUDE.md walking up from %s", cwd)
	return ""
}

func collectEnvVarsFromCode(t *testing.T, goRoot string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	err := filepath.Walk(goRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.Contains(path, "/acs/") || strings.Contains(path, "/archive/") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, m := range envVarRE.FindAll(b, -1) {
			out[string(m)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", goRoot, err)
	}
	return out
}
