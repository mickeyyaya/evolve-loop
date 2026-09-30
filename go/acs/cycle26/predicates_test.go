//go:build acs

package cycle26

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var removedFlags = []string{
	"EVOLVE_ACS_PREDICATE_TIMEOUT_S",
	"EVOLVE_QUOTA_RESET_AT",
	"EVOLVE_QUOTA_RESET_HOURS",
}

func TestC26_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-26 quota-config-object).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

func TestC26_004_NoEnvReadsForQuotaFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	quotaresetFile := filepath.Join(root, "go", "internal", "quotareset", "quotareset.go")
	for _, dead := range []string{
		`"EVOLVE_QUOTA_RESET_AT"`,
		`"EVOLVE_QUOTA_RESET_HOURS"`,
	} {
		if !acsassert.FileNotContains(t, quotaresetFile, dead) {
			t.Errorf("RED: quotareset.go still contains %q.\n"+
				"Builder must delete the getEnv call for this flag and replace it with\n"+
				"the typed opts field (opts.ResetAt or opts.DefaultHours).\n"+
				"File: %s", dead, quotaresetFile)
		}
	}
}

func TestC26_005_QuotaResetConfigFieldInPolicy(t *testing.T) {
	fInfo, ok := reflect.TypeOf(policy.Policy{}).FieldByName("QuotaReset")
	if !ok {
		t.Fatalf("RED: policy.Policy.QuotaReset field missing.\n" +
			"Builder must add `QuotaReset *QuotaResetConfig` to Policy in go/internal/policy/policy.go\n" +
			"(parallel to Fanout *FanoutPolicy and Observer *ObserverPolicy).")
	}
	if fInfo.Type.Kind() != reflect.Ptr {
		t.Errorf("RED: policy.Policy.QuotaReset is kind %v, want pointer (*QuotaResetConfig).\n"+
			"The field must be typed as `*QuotaResetConfig`, not %v.",
			fInfo.Type.Kind(), fInfo.Type.Kind())
	}

	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cmdFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_quota_reset.go")
	if !acsassert.FileContains(t, cmdFile, "QuotaReset") {
		t.Errorf("RED: cmd_quota_reset.go does not load policy or reference QuotaReset.\n"+
			"Builder must update runQuotaReset to:\n"+
			"  1. Load policy.json via policy.Load (or equivalent)\n"+
			"  2. Call pol.QuotaResetConfig() (or pol.QuotaReset) to obtain opts.ResetAt + opts.DefaultHours\n"+
			"  3. Pass those values to quotareset.Compute\n"+
			"File: %s", cmdFile)
	}
}

func TestC26_006_WorktreePathStillInRegistry(t *testing.T) {
	const worktreePath = "EVOLVE_WORKTREE_PATH"
	if _, ok := flagregistry.Lookup(worktreePath); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH removed.\n"+
			"Builder MUST NOT remove EVOLVE_WORKTREE_PATH from registry_table.go.\n"+
			"It is a live IPC handoff (agents/evolve-tester.md) pinned by TestC50_009.\n"+
			"This is the same mistake that killed cycles 17, 18, and 19.",
			worktreePath)
	}
}

func TestC26_008_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, flag := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 3 quota/ACS cluster rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}

func TestC26_NEG1_QuotaOptionsTypedFieldsExist(t *testing.T) {
	resetAtInfo, ok := reflect.TypeOf(quotareset.Options{}).FieldByName("ResetAt")
	if !ok {
		t.Fatalf("RED: quotareset.Options.ResetAt field missing.\n" +
			"Builder must add `ResetAt string` to Options in go/internal/quotareset/quotareset.go.\n" +
			"Source 1 in Compute must then use `strings.TrimSpace(opts.ResetAt)` instead of\n" +
			"getEnv(\"EVOLVE_QUOTA_RESET_AT\").")
	}
	if resetAtInfo.Type.Kind() != reflect.String {
		t.Errorf("RED: quotareset.Options.ResetAt is kind %v, want string.\n"+
			"The field must be typed as `string`.",
			resetAtInfo.Type.Kind())
	}

	hoursInfo, ok := reflect.TypeOf(quotareset.Options{}).FieldByName("DefaultHours")
	if !ok {
		t.Fatalf("RED: quotareset.Options.DefaultHours field missing.\n" +
			"Builder must add `DefaultHours float64` to Options in go/internal/quotareset/quotareset.go.\n" +
			"Source 3 in Compute must then use `if opts.DefaultHours > 0 { hours = opts.DefaultHours }`\n" +
			"instead of getEnv(\"EVOLVE_QUOTA_RESET_HOURS\").")
	}
	if hoursInfo.Type.Kind() != reflect.Float64 {
		t.Errorf("RED: quotareset.Options.DefaultHours is kind %v, want float64.\n"+
			"The field must be typed as `float64`.",
			hoursInfo.Type.Kind())
	}
}

func TestC26_NEG2_AcsPredTimeoutHasZeroEnvReaders(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)

	acsSuiteFile := filepath.Join(root, "go", "internal", "acssuite", "acssuite.go")
	if !acsassert.FileNotContains(t, acsSuiteFile, `"EVOLVE_ACS_PREDICATE_TIMEOUT_S"`) {
		t.Errorf("RED: acssuite.go unexpectedly reads EVOLVE_ACS_PREDICATE_TIMEOUT_S.\n"+
			"The runner should only use EVOLVE_ACS_GO_TIMEOUT_S.\n"+
			"If this check fails, investigate before removing the flag — it may not be dead.\n"+
			"File: %s", acsSuiteFile)
	}

	registryFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	if !acsassert.FileNotContains(t, registryFile, `"EVOLVE_ACS_PREDICATE_TIMEOUT_S"`) {
		t.Errorf("RED: registry_table.go still registers EVOLVE_ACS_PREDICATE_TIMEOUT_S.\n"+
			"Builder must delete the registry row for this dead flag.\n"+
			"A comment reference in changedpkgs.go:6 is acceptable (it is not an env read).\n"+
			"File: %s", registryFile)
	}
}
