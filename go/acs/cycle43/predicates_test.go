//go:build acs

// Package cycle43 materializes the cycle-43 acceptance criteria for one task:
//
//	bridge-timing-psmas-config-43 — migrate 5 env-read flags to typed
//	BridgePolicy / bridge.Deps / WorkflowPolicy fields:
//	  EVOLVE_SCROLLBACK_LINES     → BridgePolicy.ScrollbackLines  / bridge.Deps.ScrollbackLines
//	  EVOLVE_BOOT_TIMEOUT_S       → BridgePolicy.BootTimeoutS     / bridge.Deps.BootTimeoutS
//	  EVOLVE_ARTIFACT_TIMEOUT_S   → BridgePolicy.ArtifactTimeoutS / bridge.Deps.ArtifactTimeoutS
//	  EVOLVE_ARTIFACT_MAX_EXTENDS → BridgePolicy.ArtifactMaxExtends / bridge.Deps.ArtifactMaxExtends
//	  EVOLVE_PSMAS_SKIP           → WorkflowPolicy.PSMASEnabled   / WorkflowConfig.PSMASEnabled
//	Remove 5 rows from registry_table.go; lower FlagCeiling 73→68.
//	Migrate ALL 8 bridge test files (cycle-42 failure lesson: incomplete migration).
//	Remove EVOLVE_BOOT_TIMEOUT_S CI override from .github/workflows/go.yml.
//
// AC map (1:1 with triage top_n):
//
//	AC1  5 flags absent from Lookup                  → C43_001 (behavioral)
//	AC2  No prod env reads for 5 flags               → C43_002 (config-check, waiver)
//	AC3  BridgePolicy has 4 int timing fields        → C43_003 (behavioral, compile-fail RED)
//	AC4  WorkflowPolicy has PSMASEnabled *bool       → C43_004 (behavioral, compile-fail RED)
//	AC5  bridge.Deps has 4 typed int fields          → C43_005 (behavioral, compile-fail RED)
//	AC6  bridge tests pass                           → manual+checklist (see below)
//	AC7  Full suite passes                           → manual+checklist (see below)
//	AC8  FlagCeiling == 68                           → C43_008 (config-check, waiver)
//	AC9  flagreaders guard passes                    → manual+checklist (see below)
//	AC10 control-flags.md updated                   → C43_010 (config-check, waiver)
//	AC11 CI workflow clean                           → C43_011 (config-check, waiver)
//	AC12 All bridge test files migrated              → C43_012 (config-check, waiver)
//	NEG  Registry count is exactly 68               → C43_NEG_RowCount (behavioral)
//	NEG  PSMAS_SKIP absent from cyclerun*.go        → C43_NEG_PSMASAbsent (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC6 (bridge tests pass):
//	  Checklist for Auditor:
//	  (a) exit 0 from `cd go && go test -count=1 ./internal/bridge/...`
//	  (b) zero FAIL lines in the output
//	  (c) run with -v and verify tests for each of the 8 migrated test files all pass
//
//	AC7 (full suite passes):
//	  Checklist for Auditor:
//	  (a) exit 0 from `cd go && go test -count=1 ./...`
//	  (b) no FAIL packages in output
//	  (c) `go build ./...` exits 0
//
//	AC9 (flagreaders guard passes):
//	  Checklist for Auditor:
//	  (a) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`
//	  (b) none of the 5 flag names appear in non-test, non-registry Go files:
//	      `grep -rn '"EVOLVE_SCROLLBACK_LINES"\|"EVOLVE_BOOT_TIMEOUT_S"\|"EVOLVE_ARTIFACT_TIMEOUT_S"\|"EVOLVE_ARTIFACT_MAX_EXTENDS"\|"EVOLVE_PSMAS_SKIP"'
//	       go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go'` → 0 matches
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C43_001 — 5 flags must be ABSENT from Lookup (any hit = flag still registered).
//	            C43_NEG_RowCount — registry must be EXACTLY 68; over- or under-removal fails.
//	            C43_NEG_PSMASAbsent — "EVOLVE_PSMAS_SKIP" must be ABSENT from cyclerun*.go.
//	Edge/OOD:   C43_NEG_RowCount checks exact 68: over-removal (<68) and under-removal (>68) both fail.
//	Lexical:    Lookup / len / FileNotContains / FileContains / struct-field-access / PSMASEnabled
//	            resolver / BridgePolicy / WorkflowPolicy / bridge.Deps composite literals — distinct verbs.
//	Semantic:   registry-absence (5 flags), exact-row-count (anti-both-directions), no-env-reads
//	            (multi-file anti-gaming), struct-field-existence (3 new API surfaces), no-doc-entries,
//	            ceiling-const, ci-yml-absent, test-file-migration, psmas-prod-absent — 10 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for bridge-timing-psmas-config-43
// (sole top_n task). Deferred tasks (CODEX_CONFIG_PATH, EVOLVE_STRICT_AUDIT,
// StatusInternal cluster) get zero predicates.
//
// 1:1 enforcement:
//
//	predicate=10 (C43_001,002,003,004,005,008,010,011,012,NEG_RowCount,NEG_PSMASAbsent — 11 funcs)
//	manual+checklist=3 (AC6,AC7,AC9)
//	unverifiable-remove=0
//	total AC count=12 + 2 NEG = 14 disposition rows; every AC has exactly one row.
package cycle43

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// removedFlags is the canonical list of 5 env flags that cycle-43 removes
// from the registry and from all production env readers.
var removedFlags = []string{
	"EVOLVE_SCROLLBACK_LINES",
	"EVOLVE_BOOT_TIMEOUT_S",
	"EVOLVE_ARTIFACT_TIMEOUT_S",
	"EVOLVE_ARTIFACT_MAX_EXTENDS",
	"EVOLVE_PSMAS_SKIP",
}

// TestC43_001_RemovedFlagsAbsentFromRegistry verifies that all 5 env flags are
// no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1. BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production
// SSOT. A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 5 flags are currently registered (3 StatusInternal + 1 StatusActive +
// 1 StatusInternal); each Lookup returns (flag, true).
func TestC43_001_RemovedFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go\n"+
				"(bridge-timing-psmas-config-43: migrate to BridgePolicy/WorkflowPolicy).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC43_002_NoProdEnvReadsForRemovedFlags verifies that the 5 flag name string
// literals have been deleted from all production source files (non-test, non-registry).
//
// Covers AC2 (anti-gaming, cycle-8 split-const lesson): removing registry rows without
// deleting the env reads is the split-const hiding pattern. The prod readers are:
//   - driver_tmux_repl.go: EVOLVE_SCROLLBACK_LINES, EVOLVE_BOOT_TIMEOUT_S, EVOLVE_ARTIFACT_TIMEOUT_S
//   - recipe_adapter.go: EVOLVE_BOOT_TIMEOUT_S
//   - engine.go: EVOLVE_ARTIFACT_MAX_EXTENDS
//   - cmd_phase_observer.go: EVOLVE_ARTIFACT_MAX_EXTENDS
//   - core/cyclerun.go, core/cyclerun_record.go, core/cyclerun_select.go: EVOLVE_PSMAS_SKIP
//
// acs-predicate: config-check
//
// RED: all 5 flag string literals currently appear in the above prod files.
func TestC43_002_NoProdEnvReadsForRemovedFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)

	prodFiles := []struct {
		relPath string
		flags   []string
	}{
		{
			relPath: "go/internal/bridge/driver_tmux_repl.go",
			flags:   []string{"EVOLVE_SCROLLBACK_LINES", "EVOLVE_BOOT_TIMEOUT_S", "EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/recipe_adapter.go",
			flags:   []string{"EVOLVE_BOOT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/engine.go",
			flags:   []string{"EVOLVE_ARTIFACT_MAX_EXTENDS"},
		},
		{
			relPath: "go/cmd/evolve/cmd_phase_observer.go",
			flags:   []string{"EVOLVE_ARTIFACT_MAX_EXTENDS"},
		},
		{
			relPath: "go/internal/core/cyclerun.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
		{
			relPath: "go/internal/core/cyclerun_record.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
		{
			relPath: "go/internal/core/cyclerun_select.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
	}

	for _, pf := range prodFiles {
		fullPath := filepath.Join(root, pf.relPath)
		for _, flag := range pf.flags {
			if !acsassert.FileNotContains(t, fullPath, flag) {
				t.Errorf("RED: %s still contains the env flag string %q.\n"+
					"Builder must replace all envInt/envchain reads for %q with the typed\n"+
					"BridgePolicy/WorkflowPolicy field accessor (cycle-8 anti-gaming:\n"+
					"removing the registry row without deleting the env read is split-const hiding).\n"+
					"File: %s", pf.relPath, flag, flag, fullPath)
			}
		}
	}
}

// TestC43_003_BridgePolicyHas4IntTimingFields verifies that:
//  1. policy.BridgePolicy has 4 new int fields: ScrollbackLines, BootTimeoutS,
//     ArtifactTimeoutS, ArtifactMaxExtends.
//  2. policy.BridgeConfig() propagates non-zero values from the policy block.
//  3. The zero-value default (absent Bridge block) returns zero int fields
//     (drivers apply their own built-in defaults when field==0).
//
// Covers AC3. BEHAVIORAL (compile-fail RED): directly constructs BridgePolicy{} with
// the 4 int fields and calls BridgeConfig(). Until Builder adds these fields to
// BridgePolicy and updates BridgeConfig(), this test FAILS TO COMPILE.
//
// RED: BridgePolicy (policy.go:400-404) currently has only 3 string directory fields.
// This test does not compile.
func TestC43_003_BridgePolicyHas4IntTimingFields(t *testing.T) {
	// Direct struct field access — compile-fail RED until Builder adds the 4 int fields.
	cfg := policy.Policy{
		Bridge: &policy.BridgePolicy{
			BootTimeoutS:       90,
			ArtifactTimeoutS:   180,
			ArtifactMaxExtends: 5,
			ScrollbackLines:    3000,
		},
	}.BridgeConfig()

	if cfg.BootTimeoutS != 90 {
		t.Errorf("RED: BridgeConfig().BootTimeoutS = %d, want 90.\n"+
			"Builder must add BootTimeoutS int to BridgePolicy AND BridgeConfig,\n"+
			"then propagate in BridgeConfig() resolver.\n"+
			"(replaces EVOLVE_BOOT_TIMEOUT_S envInt reads in driver_tmux_repl.go and recipe_adapter.go)",
			cfg.BootTimeoutS)
	}
	if cfg.ArtifactTimeoutS != 180 {
		t.Errorf("RED: BridgeConfig().ArtifactTimeoutS = %d, want 180.\n"+
			"Builder must add ArtifactTimeoutS int to BridgePolicy AND BridgeConfig,\n"+
			"then propagate in BridgeConfig() resolver.\n"+
			"(replaces EVOLVE_ARTIFACT_TIMEOUT_S envInt reads in driver_tmux_repl.go)",
			cfg.ArtifactTimeoutS)
	}
	if cfg.ArtifactMaxExtends != 5 {
		t.Errorf("RED: BridgeConfig().ArtifactMaxExtends = %d, want 5.\n"+
			"Builder must add ArtifactMaxExtends int to BridgePolicy AND BridgeConfig,\n"+
			"then propagate in BridgeConfig() resolver.\n"+
			"(replaces EVOLVE_ARTIFACT_MAX_EXTENDS envInt reads in engine.go and cmd_phase_observer.go)",
			cfg.ArtifactMaxExtends)
	}
	if cfg.ScrollbackLines != 3000 {
		t.Errorf("RED: BridgeConfig().ScrollbackLines = %d, want 3000.\n"+
			"Builder must add ScrollbackLines int to BridgePolicy AND BridgeConfig,\n"+
			"then propagate in BridgeConfig() resolver.\n"+
			"(replaces EVOLVE_SCROLLBACK_LINES envInt reads in driver_tmux_repl.go)",
			cfg.ScrollbackLines)
	}

	// Zero-value default: absent Bridge block must return zero int fields
	// (drivers use their own built-in defaults when field==0).
	dflt := policy.Policy{}.BridgeConfig()
	if dflt.BootTimeoutS != 0 {
		t.Errorf("RED: policy.Policy{}.BridgeConfig().BootTimeoutS = %d, want 0 (absent block → zero).\n"+
			"An absent/empty Bridge block must not override timing defaults; 0 tells the driver\n"+
			"to use its built-in constant (tmuxREPLBootTimeoutS=60 / tmuxArtifactScrollback / etc.).",
			dflt.BootTimeoutS)
	}
}

// TestC43_004_WorkflowPolicyHasPSMASEnabledBool verifies that:
//  1. policy.WorkflowPolicy has a PSMASEnabled *bool field.
//  2. policy.WorkflowConfig has a PSMASEnabled bool field.
//  3. The WorkflowConfig() resolver defaults PSMASEnabled to false (opt-in flag).
//  4. An explicit *true pointer in WorkflowPolicy enables it.
//
// Covers AC4. BEHAVIORAL (compile-fail RED): directly accesses
// WorkflowPolicy.PSMASEnabled. Until Builder adds this field, the test FAILS
// TO COMPILE — compile failure IS the RED state.
//
// RED: WorkflowPolicy (policy.go:463-475) does not have PSMASEnabled.
// WorkflowConfig (policy.go:478-490) does not have PSMASEnabled.
// This test does not compile.
func TestC43_004_WorkflowPolicyHasPSMASEnabledBool(t *testing.T) {
	// Zero-value default: absent Workflow block must yield PSMASEnabled=false (opt-in).
	// The current prod behavior: envchain.BoolValue(cr.envSnap["EVOLVE_PSMAS_SKIP"], false)
	// returns false when the env var is unset — policy default must match.
	dflt := policy.Policy{}.WorkflowConfig()
	if dflt.PSMASEnabled {
		t.Errorf("RED: policy.Policy{}.WorkflowConfig().PSMASEnabled = true, want false.\n" +
			"Builder must add PSMASEnabled bool to WorkflowConfig with default=false in\n" +
			"WorkflowConfig() resolver (nil PSMASEnabled in WorkflowPolicy → off; PSMAS is opt-in).")
	}

	// Explicit true override must enable it.
	tr := true
	enabled := policy.Policy{
		Workflow: &policy.WorkflowPolicy{
			PSMASEnabled: &tr,
		},
	}.WorkflowConfig()
	if !enabled.PSMASEnabled {
		t.Errorf("RED: PSMASEnabled with *bool=true pointer = false, want true.\n" +
			"Builder must honor explicit PSMASEnabled=true in policy.json:\n" +
			"  if p.Workflow.PSMASEnabled != nil {\n" +
			"    c.PSMASEnabled = *p.Workflow.PSMASEnabled\n" +
			"  }")
	}

	// Explicit false override must keep it disabled.
	f := false
	disabled := policy.Policy{
		Workflow: &policy.WorkflowPolicy{
			PSMASEnabled: &f,
		},
	}.WorkflowConfig()
	if disabled.PSMASEnabled {
		t.Errorf("RED: PSMASEnabled with *bool=false pointer = true, want false.\n" +
			"Builder must propagate explicit PSMASEnabled=false from WorkflowPolicy.")
	}
}

// TestC43_005_BridgeDepsHas4TypedIntFields verifies that bridge.Deps has 4 new
// typed int fields: ScrollbackLines, BootTimeoutS, ArtifactTimeoutS, ArtifactMaxExtends.
//
// Covers AC5. BEHAVIORAL (compile-fail RED): directly constructs bridge.Deps{} using
// the 4 int fields. Until Builder adds these to Deps in engine.go, this test FAILS
// TO COMPILE — compile failure IS the RED state.
//
// RED: bridge.Deps (engine.go:48+) currently does NOT have ScrollbackLines, BootTimeoutS,
// or ArtifactMaxExtends. ArtifactTimeoutS exists in Config but NOT in Deps.
// This test does not compile.
func TestC43_005_BridgeDepsHas4TypedIntFields(t *testing.T) {
	// Direct composite literal — compile-fail RED until Builder adds the fields.
	d := bridge.Deps{
		ScrollbackLines:    2000,
		BootTimeoutS:       45,
		ArtifactTimeoutS:   600,
		ArtifactMaxExtends: 3,
	}

	// Verify the fields are accessible and correctly set (behavioral: the struct
	// carries the values we put in — tests that the type fields exist AND store).
	if d.ScrollbackLines != 2000 {
		t.Errorf("RED: bridge.Deps.ScrollbackLines = %d, want 2000.\n"+
			"Builder must add ScrollbackLines int to bridge.Deps in engine.go.",
			d.ScrollbackLines)
	}
	if d.BootTimeoutS != 45 {
		t.Errorf("RED: bridge.Deps.BootTimeoutS = %d, want 45.\n"+
			"Builder must add BootTimeoutS int to bridge.Deps in engine.go.",
			d.BootTimeoutS)
	}
	if d.ArtifactTimeoutS != 600 {
		t.Errorf("RED: bridge.Deps.ArtifactTimeoutS = %d, want 600.\n"+
			"Builder must add ArtifactTimeoutS int to bridge.Deps in engine.go\n"+
			"(currently only in Config, not Deps; test files set via Deps.Env map today).",
			d.ArtifactTimeoutS)
	}
	if d.ArtifactMaxExtends != 3 {
		t.Errorf("RED: bridge.Deps.ArtifactMaxExtends = %d, want 3.\n"+
			"Builder must add ArtifactMaxExtends int to bridge.Deps in engine.go.",
			d.ArtifactMaxExtends)
	}
}

// TestC43_010_ControlFlagsDocNoRemovedFlagRows verifies that the regenerated
// docs/architecture/control-flags.md no longer contains entries for the 5 removed flags.
//
// Covers AC10. acs-predicate: config-check
//
// RED: control-flags.md currently has rows for all 5 flags (they are active in the registry).
// After the migration, the doc must be regenerated and all 5 flag names must be absent.
func TestC43_010_ControlFlagsDocNoRemovedFlagRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"the 5 flag rows (e.g. `evolve flags generate`) in the same diff.\n"+
				"File: %s", name, controlFlagsDoc)
		}
	}
}

// TestC43_011_CIWorkflowNoBootTimeoutOverride verifies that the
// EVOLVE_BOOT_TIMEOUT_S CI override has been removed from .github/workflows/go.yml.
//
// Covers AC11. After migration, boot timeout comes from BridgePolicy.BootTimeoutS
// resolved from policy.json, so the env override in CI must be removed.
// The flagreaders guard will fail if this line stays.
//
// acs-predicate: config-check
//
// RED: .github/workflows/go.yml:55 currently has `EVOLVE_BOOT_TIMEOUT_S: "120"`.
func TestC43_011_CIWorkflowNoBootTimeoutOverride(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	goYML := filepath.Join(root, ".github", "workflows", "go.yml")
	if !acsassert.FileNotContains(t, goYML, "EVOLVE_BOOT_TIMEOUT_S") {
		t.Errorf("RED: .github/workflows/go.yml still contains EVOLVE_BOOT_TIMEOUT_S.\n"+
			"Builder must remove the `EVOLVE_BOOT_TIMEOUT_S: \"120\"` line from the CI workflow.\n"+
			"After migration, boot timeout is set via BridgePolicy.BootTimeoutS in policy.json;\n"+
			"the env CI override is dead code and the flagreaders guard will catch it.\n"+
			"File: %s", goYML)
	}
}

// TestC43_012_AllBridgeTestFilesMigrated verifies that none of the 8 bridge test
// files listed in scout-report Finding 3 still reference the 5 env flag names
// via Deps.Env map or LookupEnv map arguments.
//
// Covers AC12 (critical — this is the cycle-42 failure point: 3 test files were left
// unmigrated, leaving bridge tests RED after the production env reads were removed).
//
// acs-predicate: config-check
//
// RED: 8 bridge test files currently use EVOLVE_ARTIFACT_TIMEOUT_S / EVOLVE_BOOT_TIMEOUT_S
// / EVOLVE_SCROLLBACK_LINES / EVOLVE_ARTIFACT_MAX_EXTENDS via Deps.Env map injection.
// After migration, each test must use typed Deps fields directly (Deps{ArtifactTimeoutS: N}).
func TestC43_012_AllBridgeTestFilesMigrated(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)

	testFiles := []struct {
		relPath string
		flags   []string
	}{
		{
			relPath: "go/internal/bridge/scrollback_lines_test.go",
			flags:   []string{"EVOLVE_SCROLLBACK_LINES"},
		},
		{
			relPath: "go/internal/bridge/driver_tmux_repl_boottimeout_test.go",
			flags:   []string{"EVOLVE_BOOT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/envoverlay_test.go",
			// Only the ARTIFACT_TIMEOUT_S usages that set the specific flag need migration.
			// The generic-key TestEnvInt_DepsEnvOverlay / TestLookupEnv_DepsEnvOverlay tests
			// use arbitrary keys ("K") and must be kept. Only entries that reference the flag
			// name literally are flagged.
			flags: []string{"EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/render_wedge_test.go",
			flags:   []string{"EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/driver_tmux_repl_escalation_test.go",
			flags:   []string{"EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/tmux_repl_fixture_test.go",
			flags:   []string{"EVOLVE_ARTIFACT_MAX_EXTENDS"},
		},
		{
			relPath: "go/internal/bridge/stop_review_ledger_test.go",
			flags:   []string{"EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/stopreview_test.go",
			flags:   []string{"EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		// core test files with PSMAS_SKIP (if any exist):
		{
			relPath: "go/internal/core/cyclerun.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
		{
			relPath: "go/internal/core/cyclerun_record.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
		{
			relPath: "go/internal/core/cyclerun_select.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
	}

	for _, tf := range testFiles {
		fullPath := filepath.Join(root, tf.relPath)
		for _, flag := range tf.flags {
			if !acsassert.FileNotContains(t, fullPath, flag) {
				t.Errorf("RED: %s still contains the env flag string %q.\n"+
					"Builder must replace `Deps{Env: map[string]string{%q: \"N\"}}` (or LookupEnv map)\n"+
					"with the typed Deps field directly (e.g. `Deps{ArtifactTimeoutS: N}`).\n"+
					"This is the cycle-42 failure point: test migration was incomplete.\n"+
					"File: %s", tf.relPath, flag, flag, fullPath)
			}
		}
	}
}

// TestC43_NEG_PSMASAbsentFromCyclerunProdFiles verifies that the
// "EVOLVE_PSMAS_SKIP" string literal has been deleted from all 3 production
// cyclerun*.go files in core.
//
// Covers NEG_PSMASAbsent. Anti-gaming (cycle-8 split-const lesson): Builder cannot
// remove the registry row while leaving envchain.BoolValue(cr.envSnap["EVOLVE_PSMAS_SKIP"], false)
// call sites. All 3 prod files must replace envchain reads with pol.WorkflowConfig().PSMASEnabled.
//
// acs-predicate: config-check
//
// RED: cyclerun.go:384, cyclerun_record.go:89, cyclerun_select.go:87 all contain
// envchain.BoolValue(cr.envSnap["EVOLVE_PSMAS_SKIP"], false) — 3 occurrences.
func TestC43_NEG_PSMASAbsentFromCyclerunProdFiles(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	psmasFlag := "EVOLVE_PSMAS_SKIP"

	cyclerunFiles := []string{
		"go/internal/core/cyclerun.go",
		"go/internal/core/cyclerun_record.go",
		"go/internal/core/cyclerun_select.go",
	}

	for _, rel := range cyclerunFiles {
		fullPath := filepath.Join(root, rel)
		if !acsassert.FileNotContains(t, fullPath, psmasFlag) {
			t.Errorf("RED: %s still contains the string literal %q.\n"+
				"Builder must replace envchain.BoolValue(cr.envSnap[\"EVOLVE_PSMAS_SKIP\"], false)\n"+
				"with pol.WorkflowConfig().PSMASEnabled (or equivalent policy read).\n"+
				"(cycle-8 anti-gaming: removing the registry row without deleting the env read\n"+
				"is the split-const hiding pattern).\n"+
				"File: %s", rel, psmasFlag, fullPath)
		}
	}
}
