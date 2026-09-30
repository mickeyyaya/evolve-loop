//go:build acs

package cycle28

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var dispatchFlags = []string{
	"EVOLVE_DISPATCH_DEPTH",
	"EVOLVE_DISPATCH_LOG_TTL_DAYS",
	"EVOLVE_DISPATCH_PLAN_LOG",
	"EVOLVE_DISPATCH_POLICY",
	"EVOLVE_DISPATCH_REPEAT_THRESHOLD",
	"EVOLVE_TRACKER_TTL_DAYS",
}

func TestC28_001_DispatchFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range dispatchFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-28 dispatch-cluster-28).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC28_004_NoEnvReadsForRemovedFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file  string
		flags []string
	}{
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_control.go"),
			[]string{`"EVOLVE_DISPATCH_POLICY"`, `"EVOLVE_DISPATCH_REPEAT_THRESHOLD"`},
		},
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_prune_ephemeral.go"),
			[]string{`"EVOLVE_DISPATCH_LOG_TTL_DAYS"`, `"EVOLVE_TRACKER_TTL_DAYS"`},
		},
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_subagent.go"),
			[]string{`"EVOLVE_DISPATCH_PLAN_LOG"`},
		},
	}
	for _, c := range checks {
		for _, literal := range c.flags {
			if !acsassert.FileNotContains(t, c.file, literal) {
				t.Errorf("RED: %s still contains env-read literal %q.\n"+
					"Builder must delete the os.Getenv call for this flag and replace it\n"+
					"with the appropriate CLI flag or config-as-code field.\n"+
					"File: %s", filepath.Base(c.file), literal, c.file)
			}
		}
	}
}

func TestC28_005_DispatchConfigStructExistsInPolicy(t *testing.T) {
	pInfo, ok := reflect.TypeOf(policy.Policy{}).FieldByName("Dispatch")
	if !ok {
		t.Fatalf("RED: policy.Policy.Dispatch field missing.\n" +
			"Builder must add `Dispatch *DispatchConfig` to Policy in go/internal/policy/policy.go\n" +
			"(parallel to QuotaReset *QuotaResetConfig from cycle-26).")
	}
	if pInfo.Type.Kind() != reflect.Ptr {
		t.Fatalf("RED: policy.Policy.Dispatch is kind %v, want pointer (*DispatchConfig).\n"+
			"The field must be typed as `*DispatchConfig`.",
			pInfo.Type.Kind())
	}

	dispType := pInfo.Type.Elem()
	if _, ok := dispType.FieldByName("Policy"); !ok {
		t.Errorf("RED: DispatchConfig missing Policy field.\n" +
			"Builder must add `Policy string` to DispatchConfig.\n" +
			"Accepted values: \"off\"|\"verify\"|\"stop\"; default \"verify\".")
	}
	if f, ok := dispType.FieldByName("RepeatThreshold"); ok {
		if f.Type.Kind() != reflect.Int {
			t.Errorf("RED: DispatchConfig.RepeatThreshold is kind %v, want int.",
				f.Type.Kind())
		}
	} else {
		t.Errorf("RED: DispatchConfig missing RepeatThreshold field.\n" +
			"Builder must add `RepeatThreshold int` to DispatchConfig (default 3).")
	}
}

// TestC28_006_DispatchDepthSplitConstHasProtocolComment verifies that the
// dispatchDepthEnv const in go/internal/subagent/recursion.go carries the
// required protocol comment marking it as a legitimate IPC split-const.
//
// Covers AC6. The comment "SSOT IPC-protocol-allowed:" marks this const as the
// canonical IPC handoff name for the parent→child recursion-depth contract —
// allowing the flagreaders guard to recognize it as a split-const (not a stale
// env read) and exempting it from the env-reader ban.
//
// acs-predicate: config-check
//
// RED: recursion.go has `dispatchDepthEnv = "EVOLVE_DISPATCH_DEPTH"` as a bare
// const without the SSOT IPC-protocol-allowed comment. The const is live
// (required for IPC), but the protocol annotation is missing.
func TestC28_006_DispatchDepthSplitConstHasProtocolComment(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	recursionFile := filepath.Join(root, "go", "internal", "subagent", "recursion.go")
	if !acsassert.FileContains(t, recursionFile, "SSOT IPC-protocol-allowed:") {
		t.Errorf("RED: recursion.go does not contain 'SSOT IPC-protocol-allowed:' comment.\n"+
			"Builder must add a comment to the dispatchDepthEnv const declaration:\n"+
			"  // SSOT IPC-protocol-allowed: parent→child recursion-depth handoff\n"+
			"This marks the const as a legitimate IPC handoff (not a stale env read)\n"+
			"so the flagreaders guard recognizes it as split-const-exempt.\n"+
			"File: %s", recursionFile)
	}
}

// acs-predicate: config-check — doc regeneration is a required build step.
func TestC28_008_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range dispatchFlags {
		if !acsassert.FileNotContains(t, controlFlags, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 6 dispatch-cluster rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", name, controlFlags)
		}
	}
}

// acs-predicate: config-check
func TestC28_NEG1_CliDefaultsPreservedInPruneEphemeral(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	pruneFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_prune_ephemeral.go")
	if !acsassert.FileContains(t, pruneFile, `"dispatch-log-ttl-days"`) {
		t.Errorf("RED: cmd_prune_ephemeral.go does not register --dispatch-log-ttl-days CLI flag.\n"+
			"Builder must add: flag.IntVar(&logTTL, \"dispatch-log-ttl-days\", 30, \"...\")\n"+
			"(default 30 preserves the previous EVOLVE_DISPATCH_LOG_TTL_DAYS default).\n"+
			"File: %s", pruneFile)
	}
	if !acsassert.FileContains(t, pruneFile, `"tracker-ttl-days"`) {
		t.Errorf("RED: cmd_prune_ephemeral.go does not register --tracker-ttl-days CLI flag.\n"+
			"Builder must add: flag.IntVar(&trackerTTL, \"tracker-ttl-days\", 7, \"...\")\n"+
			"(default 7 preserves the previous EVOLVE_TRACKER_TTL_DAYS default).\n"+
			"File: %s", pruneFile)
	}
}

// acs-predicate: config-check
func TestC28_NEG2_NoResidualEnvLiteralsInLoopControl(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	loopCtlFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_control.go")
	for _, literal := range []string{
		"EVOLVE_DISPATCH_POLICY",
		"EVOLVE_DISPATCH_REPEAT_THRESHOLD",
	} {
		if !acsassert.FileNotContains(t, loopCtlFile, literal) {
			t.Errorf("RED: cmd_loop_control.go still contains %q (possibly in a comment or env read).\n"+
				"Builder must delete ALL references to this literal from the file.\n"+
				"Wire dispatch policy via pol.DispatchConfig().Policy and\n"+
				"repeat threshold via pol.DispatchConfig().RepeatThreshold instead.\n"+
				"File: %s", literal, loopCtlFile)
		}
	}
}
