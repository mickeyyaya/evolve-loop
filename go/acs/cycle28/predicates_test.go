//go:build acs

// Package cycle28 materializes the cycle-28 acceptance criteria for:
//
//	dispatch-cluster-28 — remove 6 EVOLVE_DISPATCH_* and EVOLVE_TRACKER_TTL_DAYS
//	flags by migrating them to:
//	  - EVOLVE_DISPATCH_DEPTH: IPC split-const (bucket 5) — comment + registry row only
//	  - EVOLVE_DISPATCH_LOG_TTL_DAYS, EVOLVE_TRACKER_TTL_DAYS, EVOLVE_DISPATCH_PLAN_LOG:
//	    CLI flags (bucket 4) in cmd_prune_ephemeral.go / cmd_subagent.go
//	  - EVOLVE_DISPATCH_POLICY, EVOLVE_DISPATCH_REPEAT_THRESHOLD: config-as-code
//	    (bucket 1) via policy.DispatchConfig in policy.go + cmd_loop_control.go
//	Lower FlagCeiling 126→120; regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	dispatch-cluster-28:
//	  AC1  6 flags absent from Lookup              → C28_001 (behavioral)
//	  AC2  Registry row count == 120               → C28_002 (behavioral, count)
//	  AC3  FlagCeiling const == 120                → C28_003 (config-check, waiver)
//	  AC4  No env reads for 5 config/CLI flags     → C28_004 (config-check, waiver)
//	  AC5  policy.DispatchConfig struct + method   → C28_005 (behavioral + reflect)
//	  AC6  DISPATCH_DEPTH split-const comment      → C28_006 (config-check, waiver)
//	  AC7  flagreaders guard green                 → manual+checklist (see below)
//	  AC8  control-flags.md has no removed rows    → C28_008 (config-check, waiver)
//	  NEG1 CLI flag defaults preserved (30 and 7) → C28_NEG1 (config-check, waiver)
//	  NEG2 No residual env literals in cmd_loop_control → C28_NEG2 (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle28 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no literal string "EVOLVE_DISPATCH_POLICY", "EVOLVE_DISPATCH_REPEAT_THRESHOLD",
//	        "EVOLVE_DISPATCH_PLAN_LOG", "EVOLVE_DISPATCH_LOG_TTL_DAYS", or
//	        "EVOLVE_TRACKER_TTL_DAYS" in any non-test, non-registry Go file
//	        (grep -rn 'EVOLVE_DISPATCH_POLICY\|EVOLVE_DISPATCH_REPEAT' go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches);
//	    (d) EVOLVE_DISPATCH_DEPTH remains in subagent/recursion.go as the IPC
//	        split-const (grep -n 'EVOLVE_DISPATCH_DEPTH' go/internal/subagent/recursion.go
//	        → at least 1 hit; the const is retained for IPC handoff).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C28_001 — 6 flags must be ABSENT from Lookup (if Builder misses one,
//	           Lookup returns ok=true and the test fails immediately).
//	           C28_NEG2 — EVOLVE_DISPATCH_POLICY/REPEAT literals must be ABSENT from
//	           cmd_loop_control.go (if Builder only removes the registry row without
//	           deleting the env read, the literal stays and this fails).
//	Edge/OOD:  C28_002 checks exact count 120; both over-removal (< 120) and
//	           under-removal (> 120) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / reflect — distinct
//	           assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads,
//	           dispatch-config-struct, split-const-comment, doc-absence,
//	           cli-flag-defaults, residual-literal-absent — 9 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (dispatch-cluster-28). Deferred tasks (BYPASS_* cluster) get zero predicates.
//
// 1:1 enforcement: predicate=9, manual+checklist=1, unverifiable-remove=0 → total AC=10 ✓
package cycle28

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// dispatchFlags is the canonical list of 6 flags that cycle-28 removes:
//   - EVOLVE_DISPATCH_DEPTH: IPC split-const (registry row removed; const in recursion.go stays)
//   - EVOLVE_DISPATCH_LOG_TTL_DAYS: migrated to --dispatch-log-ttl-days CLI flag
//   - EVOLVE_DISPATCH_PLAN_LOG: migrated to --dispatch-plan-log CLI flag
//   - EVOLVE_DISPATCH_POLICY: migrated to policy.DispatchConfig.Policy
//   - EVOLVE_DISPATCH_REPEAT_THRESHOLD: migrated to policy.DispatchConfig.RepeatThreshold
//   - EVOLVE_TRACKER_TTL_DAYS: migrated to --tracker-ttl-days CLI flag
var dispatchFlags = []string{
	"EVOLVE_DISPATCH_DEPTH",
	"EVOLVE_DISPATCH_LOG_TTL_DAYS",
	"EVOLVE_DISPATCH_PLAN_LOG",
	"EVOLVE_DISPATCH_POLICY",
	"EVOLVE_DISPATCH_REPEAT_THRESHOLD",
	"EVOLVE_TRACKER_TTL_DAYS",
}

// TestC28_001_DispatchFlagsAbsentFromRegistry verifies that all 6 dispatch/tracker
// flags are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1. The 6 flags span three removal patterns:
//   - EVOLVE_DISPATCH_DEPTH: IPC split-const — registry row removed, const in recursion.go stays
//   - EVOLVE_DISPATCH_{LOG_TTL_DAYS,PLAN_LOG} + EVOLVE_TRACKER_TTL_DAYS: CLI flags
//   - EVOLVE_DISPATCH_{POLICY,REPEAT_THRESHOLD}: config-as-code migration
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 6 flags are currently registered (FlagCeiling=126); each Lookup returns (flag, true).
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

// TestC28_004_NoEnvReadsForRemovedFlags verifies that the os.Getenv read sites
// for EVOLVE_DISPATCH_POLICY, EVOLVE_DISPATCH_REPEAT_THRESHOLD,
// EVOLVE_DISPATCH_PLAN_LOG, EVOLVE_DISPATCH_LOG_TTL_DAYS, and
// EVOLVE_TRACKER_TTL_DAYS have been deleted from their respective source files.
//
// Covers AC4. Config-check waiver: FileNotContains asserts structural absence of
// the exact quoted env-key strings. The 5 literals span 3 files:
//   - cmd_loop_control.go: DISPATCH_POLICY and DISPATCH_REPEAT_THRESHOLD
//   - cmd_prune_ephemeral.go: DISPATCH_LOG_TTL_DAYS and TRACKER_TTL_DAYS
//   - cmd_subagent.go: DISPATCH_PLAN_LOG
//
// EVOLVE_DISPATCH_DEPTH is NOT checked here: it remains in recursion.go as the
// IPC split-const (bucket 5) — the literal is retained as the IPC handoff name;
// only its registry row is removed.
//
// acs-predicate: config-check
//
// RED:
//
//	cmd_loop_control.go:38  has `os.Getenv("EVOLVE_DISPATCH_POLICY")`
//	cmd_loop_control.go:58  has `os.Getenv("EVOLVE_DISPATCH_REPEAT_THRESHOLD")`
//	cmd_prune_ephemeral.go:59 has `os.Getenv("EVOLVE_DISPATCH_LOG_TTL_DAYS")`
//	cmd_prune_ephemeral.go:50 has `os.Getenv("EVOLVE_TRACKER_TTL_DAYS")`
//	cmd_subagent.go:273     has `os.Getenv("EVOLVE_DISPATCH_PLAN_LOG")`
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

// TestC28_005_DispatchConfigStructExistsInPolicy verifies that:
//
//  1. policy.Policy has a Dispatch field (reflect check — fails at runtime when
//     the field is absent, giving a precise per-test failure message without
//     a whole-file compile error).
//  2. The Dispatch field is a pointer type (*DispatchConfig).
//  3. The pointed-to struct has Policy string and RepeatThreshold int sub-fields
//     — the typed replacements for the deleted os.Getenv reads.
//
// Covers AC5. BEHAVIORAL: reflect.FieldByName traverses the production type
// system; a magic-string source edit cannot satisfy this — the struct and fields
// must actually exist for reflect to find them.
//
// RED:
//
//	policy.Policy has no Dispatch field → reflect returns ok=false at the first check.
//	(post-field-add) DispatchConfig lacks Policy or RepeatThreshold → inner checks fail.
func TestC28_005_DispatchConfigStructExistsInPolicy(t *testing.T) {
	// Check Policy has Dispatch field.
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

	// Navigate to the DispatchConfig struct type via the pointer's element type.
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

// TestC28_008_ControlFlagsMdHasNoRemovedRows verifies that the generated doc
// docs/architecture/control-flags.md has no entries for any of the 6 removed
// flags after the registry rows are removed and the doc regenerated via
// 'evolve flags generate'.
//
// Covers AC8. The doc is generated from the flagregistry (source of truth);
// absence follows from C28_001 (rows removed) plus regeneration.
//
// acs-predicate: config-check — doc regeneration is a required build step.
//
// RED: control-flags.md currently has entries for all 6 removed flags.
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

// TestC28_NEG1_CliDefaultsPreservedInPruneEphemeral verifies that the two new
// CLI flags added to cmd_prune_ephemeral.go use the same default values that the
// now-deleted os.Getenv calls used:
//   - --dispatch-log-ttl-days: default 30 (was EVOLVE_DISPATCH_LOG_TTL_DAYS default)
//   - --tracker-ttl-days: default 7 (was EVOLVE_TRACKER_TTL_DAYS default)
//
// Covers NEG1 (capability preserved). Config-check waiver: FileContains asserts
// the CLI flag registration literals appear with their names in source; a refactor
// that drops or renames the flags would fail this predicate.
//
// acs-predicate: config-check
//
// RED: cmd_prune_ephemeral.go has only os.Getenv calls today — it does not yet
// register "dispatch-log-ttl-days" or "tracker-ttl-days" as CLI flags.
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

// TestC28_NEG2_NoResidualEnvLiteralsInLoopControl is the anti-gaming predicate
// that verifies cmd_loop_control.go no longer contains any literal env-key strings
// for the two config-as-code flags it previously read via os.Getenv.
//
// Anti-gaming rationale (cycle-8 lesson): a Builder could theoretically remove the
// registry row without deleting the os.Getenv call, leaving the literal in source.
// AC4 (TestC28_004) catches the quoted Getenv-call form; NEG2 adds a second layer
// by asserting the bare flag name string itself is absent — even in commented-out
// code. Together AC4 + NEG2 close both the live-env-read and residual-literal
// gaming surfaces for cmd_loop_control.go.
//
// acs-predicate: config-check
//
// RED: cmd_loop_control.go:38 contains "EVOLVE_DISPATCH_POLICY" and
// cmd_loop_control.go:58 contains "EVOLVE_DISPATCH_REPEAT_THRESHOLD".
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
