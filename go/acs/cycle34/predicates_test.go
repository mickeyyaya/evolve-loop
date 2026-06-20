//go:build acs

// Package cycle34 materializes the cycle-34 acceptance criteria for:
//
//	gates-config-cluster-34 — migrate 4 gate-control env flags from
//	applyEnv reads in config.go to policy.GatesPolicy (Configuration Object);
//	delete env reads; wire at cmd_cycle.go composition root.
//	  - EVOLVE_CONTRACT_GATE  → policy.GatesPolicy.ContractGate
//	  - EVOLVE_EVAL_GATE      → policy.GatesPolicy.EvalGate
//	  - EVOLVE_TRIAGE_CAP_GATE → policy.GatesPolicy.TriageCapGate
//	  - EVOLVE_REVIEW_GATE    → policy.GatesPolicy.ReviewGate
//	Lower FlagCeiling 93→89; regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	gates-config-cluster-34:
//	  AC1   4 flags absent from Lookup               → C34_001 (behavioral)
//	  AC2   Registry row count == 89                 → C34_002 (behavioral, count)
//	  AC3   FlagCeiling const == 89                  → C34_003 (config-check, waiver)
//	  AC4   No env reads in config.go                → C34_004 (config-check, waiver)
//	  AC5   GatesConfig() defaults: enforce/enforce/enforce/off → C34_005 (behavioral)
//	  AC6   EVOLVE_WORKTREE_PATH still registered    → C34_006 (behavioral, PRE-EXISTING GREEN)
//	  AC7   flagreaders regression guard green        → manual+checklist (see below)
//	  NEG1  config.Load(emptyEnv) → ContractGate==StageEnforce → C34_NEG1 (behavioral, PRE-EXISTING GREEN)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle34 package;
//	    (b) exit 0 from `cd go && go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) none of the 4 flag name strings appear in any non-test, non-registry Go file:
//	        grep -rn '"EVOLVE_CONTRACT_GATE"\|"EVOLVE_EVAL_GATE"\|"EVOLVE_TRIAGE_CAP_GATE"\|"EVOLVE_REVIEW_GATE"'
//	         go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go'
//	         | grep -v 'acs/cycle34' → 0 matches;
//	    (d) all 4 flags had active env reads in config.go:applyEnv before the sweep;
//	        verify that their env reads (and only their env reads) are removed.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C34_001 — 4 flags must be ABSENT from Lookup (if Builder misses
//	           any one, Lookup returns ok=true and the test fails immediately).
//	           C34_004 — env-read literals must be ABSENT from config.go
//	           (if Builder removes registry rows without deleting call sites, the
//	           literal strings remain and these tests fail — the cycle-8 split-const
//	           anti-gaming check).
//	Edge/OOD:  C34_002 checks exact count 89; both over-removal (< 89) and
//	           under-removal (> 89) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / config.Load /
//	           direct struct-field access — distinct assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads,
//	           config-defaults (GatesConfig), worktree-path-present,
//	           load-default-behavior — 7 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (gates-config-cluster-34). Deferred tasks (WorkflowDefaults cluster,
// StatusInternal classification pass) get zero predicates.
//
// 1:1 enforcement:
//
//	predicate=7, manual+checklist=1, unverifiable-remove=0 → total AC=8 ✓
package cycle34

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// removedFlags is the canonical list of 4 flags that cycle-34 removes:
//   - EVOLVE_CONTRACT_GATE:    migrated to policy.GatesPolicy.ContractGate
//   - EVOLVE_EVAL_GATE:        migrated to policy.GatesPolicy.EvalGate
//   - EVOLVE_TRIAGE_CAP_GATE:  migrated to policy.GatesPolicy.TriageCapGate
//   - EVOLVE_REVIEW_GATE:      migrated to policy.GatesPolicy.ReviewGate
var removedFlags = []string{
	"EVOLVE_CONTRACT_GATE",
	"EVOLVE_EVAL_GATE",
	"EVOLVE_REVIEW_GATE",
	"EVOLVE_TRIAGE_CAP_GATE",
}

// TestC34_001_RemovedFlagsAbsentFromRegistry verifies that all 4 removed flags
// are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1. BEHAVIORAL: calls flagregistry.Lookup() for each flag — the
// production SSOT. A source edit alone cannot satisfy this; the registry row
// must be absent for Lookup to return ok=false.
//
// RED: all 4 flags are currently registered (FlagCeiling=93); each Lookup
// returns (flag, true).
func TestC34_001_RemovedFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-34 gates-config-cluster-34).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC34_002_RegistryRowCountIs89 verifies that after removing all 4 rows the
// total registry count is exactly 89.
//
// Covers AC2. BEHAVIORAL: calls len(flagregistry.All) — the production count.
// Over-removal (< 89) and under-removal (> 89) both fail.
//
// RED: registry currently has 93 rows (FlagCeiling=93); count is 93.
func TestC34_002_RegistryRowCountIs89(t *testing.T) {
	got := len(flagregistry.All)
	if got != 89 {
		t.Errorf("RED: len(flagregistry.All) = %d, want 89 (93 − 4 removed gate flags).\n"+
			"Builder must remove exactly 4 rows from registry_table.go:\n"+
			"  EVOLVE_CONTRACT_GATE, EVOLVE_EVAL_GATE, EVOLVE_REVIEW_GATE, EVOLVE_TRIAGE_CAP_GATE",
			got)
	}
}

// TestC34_003_FlagCeilingConstIs89 verifies that the FlagCeiling ratchet constant
// has been updated from 93 to 89 in registry_ceiling_test.go.
//
// Covers AC3. The ratchet prevents accidental registry growth; lowering it by 4
// (93−4=89) is mandatory alongside the row removal.
//
// acs-predicate: config-check
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 93.
func TestC34_003_FlagCeilingConstIs89(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 89") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 89'.\n"+
			"Builder must lower the FlagCeiling constant from 93 to 89 in the same diff\n"+
			"as removing the 4 gate rows (93 − 4 = 89).\n"+
			"File: %s", ceilingFile)
	}
}

// TestC34_004_NoGateEnvReadsInConfigGo verifies that all 4 gate-flag env-read
// string literals have been deleted from config.go:applyEnv.
//
// Covers AC4. Anti-gaming (cycle-8 split-const lesson): Builder cannot remove
// the registry rows while leaving env["EVOLVE_CONTRACT_GATE"] (and siblings) in
// config.go:applyEnv. This predicate catches that gap for all 4 flags.
//
// acs-predicate: config-check
//
// RED: config.go currently reads all 4 flags at lines ~517–542 in applyEnv.
// All 4 flag name strings must be absent after migration.
func TestC34_004_NoGateEnvReadsInConfigGo(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	configFile := filepath.Join(root, "go", "internal", "config", "config.go")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, configFile, name) {
			t.Errorf("RED: config.go still contains the string %q.\n"+
				"Builder must delete the entire if-block for env[%q] in applyEnv\n"+
				"(cycle-8 anti-gaming: removing the registry row without deleting the env read\n"+
				"is the split-const hiding pattern).\n"+
				"File: %s", name, name, configFile)
		}
	}
}

// TestC34_005_GatesConfigDefaults verifies that policy.Policy{}.GatesConfig()
// returns the correct zero-value defaults for the four new fields:
//   - ContractGate  == "enforce"  (replicates config defaults() StageEnforce)
//   - EvalGate      == "enforce"  (replicates config defaults() StageEnforce)
//   - TriageCapGate == "enforce"  (replicates config defaults() StageEnforce)
//   - ReviewGate    == "off"      (replicates config defaults() StageOff)
//
// Covers AC5. BEHAVIORAL: directly calls the production GatesConfig() resolver
// on an empty Policy — the same code path the orchestrator uses at composition
// time (cmd_cycle.go, after pol := policy.Load(...)).
//
// RED: policy.Policy does not yet have GatesConfig() — this test fails to
// compile until Builder adds it (compile failure IS the RED state for
// new-method ACs).
func TestC34_005_GatesConfigDefaults(t *testing.T) {
	cfg := policy.Policy{}.GatesConfig()

	if cfg.ContractGate != "enforce" {
		t.Errorf("RED: GatesConfig().ContractGate = %q, want \"enforce\".\n"+
			"GatesPolicy.ContractGate default must be \"enforce\", matching\n"+
			"config.defaults() ContractGate: StageEnforce behavior.",
			cfg.ContractGate)
	}

	if cfg.EvalGate != "enforce" {
		t.Errorf("RED: GatesConfig().EvalGate = %q, want \"enforce\".\n"+
			"GatesPolicy.EvalGate default must be \"enforce\", matching\n"+
			"config.defaults() EvalGate: StageEnforce behavior.",
			cfg.EvalGate)
	}

	if cfg.TriageCapGate != "enforce" {
		t.Errorf("RED: GatesConfig().TriageCapGate = %q, want \"enforce\".\n"+
			"GatesPolicy.TriageCapGate default must be \"enforce\", matching\n"+
			"config.defaults() TriageCapGate: StageEnforce behavior.",
			cfg.TriageCapGate)
	}

	if cfg.ReviewGate != "off" {
		t.Errorf("RED: GatesConfig().ReviewGate = %q, want \"off\".\n"+
			"GatesPolicy.ReviewGate default must be \"off\", matching\n"+
			"config.defaults() ReviewGate: StageOff (zero value) behavior.",
			cfg.ReviewGate)
	}
}

// TestC34_006_WorktreePathStillRegistered is the non-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the cluster sweep.
// Cycles 17, 18, and 19 all failed when a Builder removed WORKTREE_PATH —
// this predicate closes that regression surface.
//
// Covers AC6 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
func TestC34_006_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

// TestC34_NEG1_DefaultContractGateIsEnforce verifies that config.Load with an
// empty env map returns ContractGate==StageEnforce (and EvalGate, TriageCapGate).
//
// This is the behavioral guard that removing env reads from config.go:applyEnv
// does NOT break the default values baked into config.defaults(). The defaults()
// function explicitly sets ContractGate=StageEnforce, EvalGate=StageEnforce,
// TriageCapGate=StageEnforce — removing applyEnv overrides preserves these.
//
// Covers NEG1. BEHAVIORAL: calls config.Load directly on the production loader.
//
// PRE-EXISTING GREEN: defaults() already sets these values; the test verifies
// the invariant survives the refactoring. Builder must NOT touch config.defaults()
// entries for gate fields (only applyEnv reads should be deleted).
func TestC34_NEG1_DefaultContractGateIsEnforce(t *testing.T) {
	// Empty env: no overrides. registryPath="" → readRegistry fails silently → defaults() used.
	cfg, _ := config.Load("", map[string]string{})

	if cfg.ContractGate != config.StageEnforce {
		t.Errorf("RED: config.Load(emptyEnv).ContractGate = %v, want StageEnforce (%v).\n"+
			"Removing env reads from applyEnv must not change the default.\n"+
			"config.defaults() must still set ContractGate: StageEnforce.",
			cfg.ContractGate, config.StageEnforce)
	}

	if cfg.EvalGate != config.StageEnforce {
		t.Errorf("RED: config.Load(emptyEnv).EvalGate = %v, want StageEnforce (%v).\n"+
			"Removing env reads from applyEnv must not change the default.",
			cfg.EvalGate, config.StageEnforce)
	}

	if cfg.TriageCapGate != config.StageEnforce {
		t.Errorf("RED: config.Load(emptyEnv).TriageCapGate = %v, want StageEnforce (%v).\n"+
			"Removing env reads from applyEnv must not change the default.",
			cfg.TriageCapGate, config.StageEnforce)
	}

	if cfg.ReviewGate != config.StageOff {
		t.Errorf("RED: config.Load(emptyEnv).ReviewGate = %v, want StageOff (%v).\n"+
			"ReviewGate is not set in defaults() so must be zero (StageOff) with empty env.",
			cfg.ReviewGate, config.StageOff)
	}
}
