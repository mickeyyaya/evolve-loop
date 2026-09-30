//go:build acs

package cycle30

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var retryFlags = []string{
	"EVOLVE_CONTRACT_CORRECTION_RETRIES",
	"EVOLVE_PHASE_LATENCY_CEILING",
	"EVOLVE_PHASE_LATENCY_CEILING_S",
	"EVOLVE_PHASE_MAX_ATTEMPTS",
	"EVOLVE_RETRY_BACKOFF_BASE_S",
	"EVOLVE_SKIP_CYCLE_HEALTH",
}

func TestC30_001_RetryFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range retryFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-30 recovery-retry-config-cluster-30).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC30_004_NoEnvKeyConstantsInProductionGo(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file    string
		absents []string
	}{
		{
			filepath.Join(root, "go", "internal", "envchain", "keys.go"),
			[]string{
				"KeyPhaseMaxAttempts",
				"KeyRetryBackoffBaseS",
				"KeyPhaseLatencyCeilingS",
				"KeyContractCorrectionRetries",
			},
		},
		{
			filepath.Join(root, "go", "internal", "core", "retry_backoff.go"),
			[]string{
				"KeyPhaseMaxAttempts",
				"KeyRetryBackoffBaseS",
				"KeyContractCorrectionRetries",
			},
		},
		{
			filepath.Join(root, "go", "internal", "cyclehealth", "cyclehealth.go"),
			[]string{"KeyPhaseLatencyCeilingS"},
		},
	}
	for _, c := range checks {
		for _, id := range c.absents {
			if !acsassert.FileNotContains(t, c.file, id) {
				t.Errorf("RED: %s still contains envchain key constant %q.\n"+
					"Builder must delete the constant definition from envchain/keys.go\n"+
					"and replace every call site with the appropriate RetryConfig field.\n"+
					"File: %s", filepath.Base(c.file), id, c.file)
			}
		}
	}
}

func TestC30_005_RetryConfigStructExistsInPolicy(t *testing.T) {
	pInfo, ok := reflect.TypeOf(policy.Policy{}).FieldByName("Retry")
	if !ok {
		t.Fatalf("RED: policy.Policy.Retry field missing.\n" +
			"Builder must add `Retry *RetryPolicy` to Policy in go/internal/policy/policy.go\n" +
			"(parallel to Workflow *WorkflowPolicy from cycle-29).")
	}
	if pInfo.Type.Kind() != reflect.Ptr {
		t.Fatalf("RED: policy.Policy.Retry is kind %v, want pointer (*RetryPolicy).\n"+
			"The field must be typed as `*RetryPolicy`.",
			pInfo.Type.Kind())
	}

	retryType := pInfo.Type.Elem()
	intFields := []string{"PhaseMaxAttempts", "RetryBackoffBaseS", "PhaseLatencyCeilingS", "ContractCorrectionRetries"}
	for _, fname := range intFields {
		if f, ok := retryType.FieldByName(fname); ok {
			if f.Type.Kind() != reflect.Int {
				t.Errorf("RED: RetryPolicy.%s is kind %v, want int.", fname, f.Type.Kind())
			}
		} else {
			t.Errorf("RED: RetryPolicy missing %s field.\n"+
				"Builder must add `%s int` to RetryPolicy.", fname, fname)
		}
	}

	rc := policy.Policy{}.RetryConfig()
	if rc.PhaseMaxAttempts != 2 {
		t.Errorf("RED: RetryConfig().PhaseMaxAttempts = %d, want 2 (matches legacy EVOLVE_PHASE_MAX_ATTEMPTS default).",
			rc.PhaseMaxAttempts)
	}
	if rc.RetryBackoffBaseS != 5 {
		t.Errorf("RED: RetryConfig().RetryBackoffBaseS = %d, want 5 (matches legacy EVOLVE_RETRY_BACKOFF_BASE_S default).",
			rc.RetryBackoffBaseS)
	}
	if rc.PhaseLatencyCeilingS != 900 {
		t.Errorf("RED: RetryConfig().PhaseLatencyCeilingS = %d, want 900 (matches legacy EVOLVE_PHASE_LATENCY_CEILING_S default).",
			rc.PhaseLatencyCeilingS)
	}
	if rc.ContractCorrectionRetries != 2 {
		t.Errorf("RED: RetryConfig().ContractCorrectionRetries = %d, want 2 (matches legacy EVOLVE_CONTRACT_CORRECTION_RETRIES default).",
			rc.ContractCorrectionRetries)
	}

	rc2 := policy.Policy{Retry: &policy.RetryPolicy{}}.RetryConfig()
	if rc2.PhaseMaxAttempts != 2 {
		t.Errorf("edge: empty RetryPolicy{}.PhaseMaxAttempts resolved to %d, want 2.\n"+
			"A zero int must fall through to the default (2), not override it.",
			rc2.PhaseMaxAttempts)
	}
	if rc2.RetryBackoffBaseS != 5 {
		t.Errorf("edge: empty RetryPolicy{}.RetryBackoffBaseS resolved to %d, want 5.\n"+
			"A zero int must fall through to the default (5), not override it.",
			rc2.RetryBackoffBaseS)
	}
	if rc2.PhaseLatencyCeilingS != 900 {
		t.Errorf("edge: empty RetryPolicy{}.PhaseLatencyCeilingS resolved to %d, want 900.\n"+
			"A zero int must fall through to the default (900), not override it.",
			rc2.PhaseLatencyCeilingS)
	}
	if rc2.ContractCorrectionRetries != 2 {
		t.Errorf("edge: empty RetryPolicy{}.ContractCorrectionRetries resolved to %d, want 2.\n"+
			"A zero int must fall through to the default (2), not override it.",
			rc2.ContractCorrectionRetries)
	}
}

func TestC30_006_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

// acs-predicate: config-check — doc regeneration is a required build step.
func TestC30_008_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range retryFlags {
		if !acsassert.FileNotContains(t, controlFlags, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 6 recovery-retry rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", name, controlFlags)
		}
	}
}

// acs-predicate: config-check
func TestC30_NEG1_NoResidualResolverFunctionsInRetryBackoff(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	retryBackoffFile := filepath.Join(root, "go", "internal", "core", "retry_backoff.go")
	for _, fnName := range []string{
		"resolvePhaseMaxAttempts",
		"resolveRetryBackoffBase",
		"resolveContractCorrectionRetries",
	} {
		if !acsassert.FileNotContains(t, retryBackoffFile, fnName) {
			t.Errorf("RED: retry_backoff.go still contains resolver function %q.\n"+
				"Builder must DELETE this function (not rename it) and replace its\n"+
				"callers with the appropriate policy.RetryConfig field access.\n"+
				"File: %s", fnName, retryBackoffFile)
		}
	}
}

// acs-predicate: config-check
func TestC30_NEG2_NoCycleHealthDirectEnvRead(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cycleHealthFile := filepath.Join(root, "go", "internal", "cyclehealth", "cyclehealth.go")
	if !acsassert.FileNotContains(t, cycleHealthFile, "PhaseEnvKey") {
		t.Errorf("RED: cyclehealth.go still contains PhaseEnvKey (the per-phase env-override read).\n"+
			"Builder must simplify perPhaseCeiling() to return globalCeiling directly\n"+
			"(scout BA1: no per-phase override was ever set in practice; the read is dead).\n"+
			"File: %s", cycleHealthFile)
	}
}
