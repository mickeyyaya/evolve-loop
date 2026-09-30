//go:build acs

package cycle29

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var workflowFlags = []string{
	"EVOLVE_AUDITOR_TIER_OVERRIDE",
	"EVOLVE_AUTO_PRUNE",
	"EVOLVE_DIFF_COMPLEXITY_DISABLE",
	"EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS",
	"EVOLVE_MAX_CYCLES_CAP",
}

func TestC29_001_WorkflowFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range workflowFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-29 workflow-config-cluster-29).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC29_004_NoEnvReadsForRemovedFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file  string
		flags []string
	}{
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go"),
			[]string{`"EVOLVE_AUTO_PRUNE"`, `"EVOLVE_MAX_CYCLES_CAP"`},
		},
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_control.go"),
			[]string{`"EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS"`},
		},
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_subagent.go"),
			[]string{`"EVOLVE_AUDITOR_TIER_OVERRIDE"`, `"EVOLVE_DIFF_COMPLEXITY_DISABLE"`},
		},
	}
	for _, c := range checks {
		for _, literal := range c.flags {
			if !acsassert.FileNotContains(t, c.file, literal) {
				t.Errorf("RED: %s still contains env-read literal %q.\n"+
					"Builder must delete the os.Getenv/envchain call for this flag and replace\n"+
					"it with the appropriate WorkflowConfig field via pol.WorkflowConfig().\n"+
					"File: %s", filepath.Base(c.file), literal, c.file)
			}
		}
	}
}

func TestC29_005_WorkflowConfigStructExistsInPolicy(t *testing.T) {
	pInfo, ok := reflect.TypeOf(policy.Policy{}).FieldByName("Workflow")
	if !ok {
		t.Fatalf("RED: policy.Policy.Workflow field missing.\n" +
			"Builder must add `Workflow *WorkflowPolicy` to Policy in go/internal/policy/policy.go\n" +
			"(parallel to Dispatch *DispatchConfig from cycle-28).")
	}
	if pInfo.Type.Kind() != reflect.Ptr {
		t.Fatalf("RED: policy.Policy.Workflow is kind %v, want pointer (*WorkflowPolicy).\n"+
			"The field must be typed as `*WorkflowPolicy`.",
			pInfo.Type.Kind())
	}

	wfType := pInfo.Type.Elem()
	requiredIntFields := []string{"MaxConsecutiveFails", "MaxCyclesCap"}
	for _, fname := range requiredIntFields {
		if f, ok := wfType.FieldByName(fname); ok {
			if f.Type.Kind() != reflect.Int {
				t.Errorf("RED: WorkflowPolicy.%s is kind %v, want int.", fname, f.Type.Kind())
			}
		} else {
			t.Errorf("RED: WorkflowPolicy missing %s field.\n"+
				"Builder must add `%s int` to WorkflowPolicy.", fname, fname)
		}
	}
	if f, ok := wfType.FieldByName("AutoPrune"); ok {
		if f.Type.Kind() != reflect.Ptr {
			t.Errorf("RED: WorkflowPolicy.AutoPrune is kind %v, want *bool (nil = default true).",
				f.Type.Kind())
		}
	} else {
		t.Errorf("RED: WorkflowPolicy missing AutoPrune field.\n" +
			"Builder must add `AutoPrune *bool` to WorkflowPolicy (nil = default true).")
	}
	if f, ok := wfType.FieldByName("DiffComplexityDisable"); ok {
		if f.Type.Kind() != reflect.Bool {
			t.Errorf("RED: WorkflowPolicy.DiffComplexityDisable is kind %v, want bool.",
				f.Type.Kind())
		}
	} else {
		t.Errorf("RED: WorkflowPolicy missing DiffComplexityDisable field.\n" +
			"Builder must add `DiffComplexityDisable bool` to WorkflowPolicy.")
	}
	if f, ok := wfType.FieldByName("AuditorTierOverride"); ok {
		if f.Type.Kind() != reflect.String {
			t.Errorf("RED: WorkflowPolicy.AuditorTierOverride is kind %v, want string.",
				f.Type.Kind())
		}
	} else {
		t.Errorf("RED: WorkflowPolicy missing AuditorTierOverride field.\n" +
			"Builder must add `AuditorTierOverride string` to WorkflowPolicy.")
	}

	wc := policy.Policy{}.WorkflowConfig()
	if wc.MaxConsecutiveFails != 1 {
		t.Errorf("RED: WorkflowConfig().MaxConsecutiveFails = %d, want 1 (matches legacy EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS default).",
			wc.MaxConsecutiveFails)
	}
	if wc.MaxCyclesCap != 25 {
		t.Errorf("RED: WorkflowConfig().MaxCyclesCap = %d, want 25 (matches legacy EVOLVE_MAX_CYCLES_CAP default).",
			wc.MaxCyclesCap)
	}
	if !wc.AutoPrune {
		t.Errorf("RED: WorkflowConfig().AutoPrune = false, want true (matches legacy EVOLVE_AUTO_PRUNE != '0' default).")
	}
}

func TestC29_007_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

func TestC29_E01_WorkflowConfigEmptyPolicyDefaults(t *testing.T) {
	wc := policy.Policy{Workflow: &policy.WorkflowPolicy{}}.WorkflowConfig()
	if wc.MaxConsecutiveFails != 1 {
		t.Errorf("E01: empty WorkflowPolicy{}.MaxConsecutiveFails resolved to %d, want 1.\n"+
			"A zero int must fall through to the default (1), not override it.",
			wc.MaxConsecutiveFails)
	}
	if wc.MaxCyclesCap != 25 {
		t.Errorf("E01: empty WorkflowPolicy{}.MaxCyclesCap resolved to %d, want 25.\n"+
			"A zero int must fall through to the default (25), not override it.",
			wc.MaxCyclesCap)
	}
	if !wc.AutoPrune {
		t.Errorf("E01: empty WorkflowPolicy{}.AutoPrune resolved to false, want true.\n" +
			"A nil *bool must fall through to the default (true).")
	}
	if wc.DiffComplexityDisable {
		t.Errorf("E01: empty WorkflowPolicy{}.DiffComplexityDisable resolved to true, want false.\n" +
			"A zero bool must remain false (disable=false = complexity check enabled by default).")
	}
	if wc.AuditorTierOverride != "" {
		t.Errorf("E01: empty WorkflowPolicy{}.AuditorTierOverride resolved to %q, want \"\".\n"+
			"An empty string must remain empty (no tier override by default).",
			wc.AuditorTierOverride)
	}
}
