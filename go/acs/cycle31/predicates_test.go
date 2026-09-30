//go:build acs

package cycle31

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var bypassFlags = []string{
	"EVOLVE_BYPASS_COMMIT_GATE",
	"EVOLVE_BYPASS_PHASE_GATE",
	"EVOLVE_BYPASS_POSTEDIT_VALIDATE",
	"EVOLVE_BYPASS_PREFIX_GATE",
	"EVOLVE_BYPASS_ROLE_GATE",
	"EVOLVE_BYPASS_SHIP_GATE",
	"EVOLVE_SKIP_WORKTREE",
}

func TestC31_001_BypassFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range bypassFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-31 bypass-config-cluster-31).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC31_004_NoEnvBypassReadsInProductionGo(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file    string
		absents []string
	}{
		{
			filepath.Join(root, "go", "internal", "guards", "phase.go"),
			[]string{"EVOLVE_BYPASS_PHASE_GATE"},
		},
		{
			filepath.Join(root, "go", "internal", "guards", "role.go"),
			[]string{"EVOLVE_BYPASS_ROLE_GATE"},
		},
		{
			filepath.Join(root, "go", "internal", "guards", "ship.go"),
			[]string{"EVOLVE_BYPASS_SHIP_GATE"},
		},
		{
			filepath.Join(root, "go", "internal", "cli", "guardcmd", "postedit_validate.go"),
			[]string{"EVOLVE_BYPASS_POSTEDIT_VALIDATE"},
		},
		{
			filepath.Join(root, "go", "internal", "cli", "guardcmd", "commit_prefix_gate.go"),
			[]string{"EVOLVE_BYPASS_PREFIX_GATE"},
		},
		{
			filepath.Join(root, "go", "internal", "phases", "ship", "commitgate.go"),
			[]string{"EVOLVE_BYPASS_COMMIT_GATE"},
		},
		{
			filepath.Join(root, "go", "internal", "phases", "ship", "gitops.go"),
			[]string{"EVOLVE_BYPASS_PREFIX_GATE"},
		},
	}
	for _, c := range checks {
		for _, pattern := range c.absents {
			if !acsassert.FileNotContains(t, c.file, pattern) {
				t.Errorf("RED: %s still contains env-bypass read for %q.\n"+
					"Builder must remove os.Getenv / envBypass() call for this flag\n"+
					"and replace it with the CLI flag bool threaded via the struct param.\n"+
					"File: %s",
					filepath.Base(c.file), pattern, c.file)
			}
		}
	}
}

func TestC31_005_GuardConstructorsAcceptBypassBool(t *testing.T) {
	type guardFnSpec struct {
		name       string
		fn         any
		wantNumIn  int
		bypassArgN int
	}

	specs := []guardFnSpec{
		{
			name:       "guards.NewPhase",
			fn:         guards.NewPhase,
			wantNumIn:  2,
			bypassArgN: 1,
		},
		{
			name:       "guards.NewRole",
			fn:         guards.NewRole,
			wantNumIn:  2,
			bypassArgN: 1,
		},
		{
			name:       "guards.NewShip",
			fn:         guards.NewShip,
			wantNumIn:  1,
			bypassArgN: 0,
		},
	}

	for _, s := range specs {
		ft := reflect.TypeOf(s.fn)
		if ft == nil || ft.Kind() != reflect.Func {
			t.Errorf("RED: %s is not a func — reflect.TypeOf returned %v", s.name, ft)
			continue
		}
		if ft.NumIn() != s.wantNumIn {
			t.Errorf("RED: %s takes %d param(s), want %d.\n"+
				"Builder must add `bypass bool` as a constructor parameter.\n"+
				"Current signature removes the env-bypass path (envBypass) in favor of DI.",
				s.name, ft.NumIn(), s.wantNumIn)
			continue
		}
		bypassType := ft.In(s.bypassArgN)
		if bypassType.Kind() != reflect.Bool {
			t.Errorf("RED: %s param[%d] has kind %v, want bool.\n"+
				"The bypass parameter must be a plain bool (not a pointer or interface).",
				s.name, s.bypassArgN, bypassType.Kind())
		}
	}
}

func TestC31_006_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

// acs-predicate: config-check — doc regeneration is a required build step.
func TestC31_008_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range bypassFlags {
		if !acsassert.FileNotContains(t, controlFlags, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 7 bypass/dead rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", name, controlFlags)
		}
	}
}

// acs-predicate: config-check
func TestC31_NEG1_EnvBypassHelperDeleted(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	helpersFile := filepath.Join(root, "go", "internal", "guards", "helpers.go")
	if !acsassert.FileNotContains(t, helpersFile, "envBypass") {
		t.Errorf("RED: guards/helpers.go still contains the 'envBypass' function.\n"+
			"Builder must DELETE the envBypass() helper (not just remove callers)\n"+
			"after migrating all Phase/Role/Ship guard callers to the bypass bool DI param.\n"+
			"The helper is the sole env-bypass mechanism for guards; its presence means\n"+
			"the migration is incomplete even if individual callers are updated.\n"+
			"File: %s", helpersFile)
	}
}
