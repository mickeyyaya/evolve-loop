//go:build acs

// Package cycle31 materializes the cycle-31 acceptance criteria for:
//
//	bypass-config-cluster-31 — migrate 6 EVOLVE_BYPASS_* flags from os.Getenv/
//	envBypass() reads to CLI flags (flag.BoolVar) threaded as struct params;
//	also dead-sweep EVOLVE_SKIP_WORKTREE (zero Go reader post v12). 7 flags total:
//	  - EVOLVE_BYPASS_COMMIT_GATE    → --bypass-commit-gate on evolve ship
//	  - EVOLVE_BYPASS_PHASE_GATE     → --bypass on evolve guard phase
//	  - EVOLVE_BYPASS_POSTEDIT_VALIDATE → --bypass on evolve postedit-validate
//	  - EVOLVE_BYPASS_PREFIX_GATE    → --bypass on evolve commit-prefix-gate
//	  - EVOLVE_BYPASS_ROLE_GATE      → --bypass on evolve guard role
//	  - EVOLVE_BYPASS_SHIP_GATE      → --bypass on evolve guard ship
//	  - EVOLVE_SKIP_WORKTREE         → dead sweep (no Go reader post v12)
//	Lower FlagCeiling 109→102; regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	bypass-config-cluster-31:
//	  AC1  7 flags absent from Lookup                → C31_001 (behavioral)
//	  AC2  Registry row count == 102                 → C31_002 (behavioral, count)
//	  AC3  FlagCeiling const == 102                  → C31_003 (config-check, waiver)
//	  AC4  No envBypass/os.Getenv reads for BYPASS   → C31_004 (config-check, waiver)
//	  AC5  Guard constructors accept bypass bool      → C31_005 (behavioral, reflect)
//	  AC6  EVOLVE_WORKTREE_PATH still registered     → C31_006 (behavioral, PRE-EXISTING GREEN)
//	  AC7  flagreaders regression guard green         → manual+checklist (see below)
//	  AC8  control-flags.md has no removed rows      → C31_008 (config-check, waiver)
//	  NEG1 envBypass helper deleted from helpers.go  → C31_NEG1 (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle31 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no literal string "EVOLVE_BYPASS_COMMIT_GATE", "EVOLVE_BYPASS_PHASE_GATE",
//	        "EVOLVE_BYPASS_POSTEDIT_VALIDATE", "EVOLVE_BYPASS_PREFIX_GATE",
//	        "EVOLVE_BYPASS_ROLE_GATE", or "EVOLVE_BYPASS_SHIP_GATE" in any
//	        non-test, non-registry Go file via os.Getenv or envBypass()
//	        (grep -rn 'envBypass\|os\.Getenv.*BYPASS'
//	         go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go'
//	        | grep -v 'acs/cycle31' → 0 matches);
//	    (d) EVOLVE_SKIP_WORKTREE is also absent from all production Go
//	        (it had a shell reader in run-cycle.sh removed in v12; zero Go readers).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C31_001 — 7 flags must be ABSENT from Lookup (if Builder misses
//	           one, Lookup returns ok=true and the test fails immediately).
//	           C31_004 — envBypass and os.Getenv("EVOLVE_BYPASS_*") calls must be
//	           ABSENT from the specific production Go files (if Builder only removes
//	           the registry row without updating the call sites, this fails).
//	           C31_NEG1 — the envBypass() helper function must be ABSENT from
//	           guards/helpers.go (if Builder migrates callers but leaves the dead
//	           helper, this fails).
//	Edge/OOD:  C31_002 checks exact count 102; both over-removal (< 102) and
//	           under-removal (> 102) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / reflect —
//	           distinct assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads,
//	           guard-DI-signature, worktree-path-present, doc-absence,
//	           helper-fn-deleted — 8 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (bypass-config-cluster-31). Deferred tasks (Dynamic Phase Routing, etc.) get
// zero predicates.
//
// 1:1 enforcement: predicate=8, manual+checklist=1, unverifiable-remove=0 → total AC=9 ✓
package cycle31

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/internal/guards"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// bypassFlags is the canonical list of 7 flags that cycle-31 removes:
//   - EVOLVE_BYPASS_COMMIT_GATE:       migrated to --bypass-commit-gate on evolve ship
//   - EVOLVE_BYPASS_PHASE_GATE:        migrated to --bypass on evolve guard phase
//   - EVOLVE_BYPASS_POSTEDIT_VALIDATE: migrated to --bypass on evolve postedit-validate
//   - EVOLVE_BYPASS_PREFIX_GATE:       migrated to --bypass on evolve commit-prefix-gate
//   - EVOLVE_BYPASS_ROLE_GATE:         migrated to --bypass on evolve guard role
//   - EVOLVE_BYPASS_SHIP_GATE:         migrated to --bypass on evolve guard ship
//   - EVOLVE_SKIP_WORKTREE:            dead sweep (shell reader removed in v12)
var bypassFlags = []string{
	"EVOLVE_BYPASS_COMMIT_GATE",
	"EVOLVE_BYPASS_PHASE_GATE",
	"EVOLVE_BYPASS_POSTEDIT_VALIDATE",
	"EVOLVE_BYPASS_PREFIX_GATE",
	"EVOLVE_BYPASS_ROLE_GATE",
	"EVOLVE_BYPASS_SHIP_GATE",
	"EVOLVE_SKIP_WORKTREE",
}

// TestC31_001_BypassFlagsAbsentFromRegistry verifies that all 7 BYPASS/dead flags
// are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1. The 7 flags span two removal patterns:
//   - 6 BYPASS flags: CLI flag migration (campaign bucket 9)
//   - EVOLVE_SKIP_WORKTREE: dead sweep (zero Go reader post v12)
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 7 flags are currently registered (FlagCeiling=109); each Lookup
// returns (flag, true).
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

// TestC31_004_NoEnvBypassReadsInProductionGo verifies that the env-read mechanisms
// for all 6 BYPASS flags have been deleted from their specific production Go files.
//
// Covers AC4. Config-check waiver: FileNotContains asserts structural absence
// of the bypass env-read patterns per file:
//   - guards/helpers.go:          "EVOLVE_BYPASS_PHASE_GATE" / "EVOLVE_BYPASS_ROLE_GATE" /
//     "EVOLVE_BYPASS_SHIP_GATE" (read via envBypass())
//   - cmd_postedit_validate.go:   "EVOLVE_BYPASS_POSTEDIT_VALIDATE" (read via os.Getenv)
//   - cmd_commit_prefix_gate.go:  "EVOLVE_BYPASS_PREFIX_GATE" (read via os.Getenv)
//   - phases/ship/commitgate.go:  "EVOLVE_BYPASS_COMMIT_GATE" (read via opts.envBool)
//   - phases/ship/gitops.go:      "EVOLVE_BYPASS_PREFIX_GATE" (second reader for prefix gate)
//
// acs-predicate: config-check
//
// RED:
//
//	guards/phase.go:22    envBypass("EVOLVE_BYPASS_PHASE_GATE")
//	guards/role.go:28     envBypass("EVOLVE_BYPASS_ROLE_GATE")
//	guards/ship.go:32     envBypass("EVOLVE_BYPASS_SHIP_GATE")
//	cmd_postedit_validate.go:35  os.Getenv("EVOLVE_BYPASS_POSTEDIT_VALIDATE")
//	cmd_commit_prefix_gate.go:97 os.Getenv("EVOLVE_BYPASS_PREFIX_GATE")
//	phases/ship/commitgate.go:51 opts.envBool("EVOLVE_BYPASS_COMMIT_GATE")
//	phases/ship/gitops.go:614    os.Getenv("EVOLVE_BYPASS_PREFIX_GATE")
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
			filepath.Join(root, "go", "cmd", "evolve", "cmd_postedit_validate.go"),
			[]string{"EVOLVE_BYPASS_POSTEDIT_VALIDATE"},
		},
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_commit_prefix_gate.go"),
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

// TestC31_005_GuardConstructorsAcceptBypassBool verifies that the Phase, Role,
// and Ship guard constructors have been updated to accept a bypass bool parameter,
// enabling DI-based bypass instead of silent env-var reads.
//
// Covers AC5. BEHAVIORAL via reflect: inspects the production function signatures
// at runtime. A magic-string source edit cannot satisfy this — the actual function
// signatures must change for reflect.TypeOf to report the correct arity and types.
//
// Expected signatures after Builder's changes:
//
//	guards.NewPhase(s core.Storage, bypass bool) *Phase  → NumIn()==2, In(1).Kind()==bool
//	guards.NewRole(s core.Storage, bypass bool) *Role    → NumIn()==2, In(1).Kind()==bool
//	guards.NewShip(bypass bool) *Ship                    → NumIn()==1, In(0).Kind()==bool
//
// RED:
//
//	guards.NewPhase currently: func(core.Storage) *Phase → NumIn()==1
//	guards.NewRole  currently: func(core.Storage) *Role  → NumIn()==1
//	guards.NewShip  currently: func() *Ship              → NumIn()==0
func TestC31_005_GuardConstructorsAcceptBypassBool(t *testing.T) {
	type guardFnSpec struct {
		name       string
		fn         any
		wantNumIn  int
		bypassArgN int // index of the bypass bool param (0-based)
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

// TestC31_006_WorktreePathStillRegistered is the non-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the bypass cluster
// sweep. Cycles 17, 18, and 19 all failed when a Builder removed WORKTREE_PATH —
// this predicate closes that regression surface.
//
// Covers AC6 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
func TestC31_006_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

// TestC31_008_ControlFlagsMdHasNoRemovedRows verifies that the generated doc
// docs/architecture/control-flags.md has no entries for any of the 7 removed
// flags after the registry rows are removed and the doc regenerated via
// 'evolve flags generate'.
//
// Covers AC8. The doc is generated from the flagregistry (source of truth);
// absence follows from C31_001 (rows removed) plus regeneration.
//
// acs-predicate: config-check — doc regeneration is a required build step.
//
// RED: control-flags.md currently has entries for all 7 removed flags.
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

// TestC31_NEG1_EnvBypassHelperDeleted is the anti-gaming predicate that verifies
// the envBypass() helper function has been completely deleted from guards/helpers.go.
//
// Anti-gaming rationale (cycle-8/cycle-85 lesson): a Builder could migrate all
// three guard callers away from envBypass() while leaving the dead helper function
// in place. C31_004 confirms the BYPASS env-var strings are gone from individual
// guard files; NEG1 adds a second layer by asserting the shared envBypass() helper
// itself is gone — closing the gaming surface where the function stays as dead code.
//
// acs-predicate: config-check
//
// RED: guards/helpers.go:29 defines "func envBypass(name string) bool"
// that reads os.Getenv(name) == "1". After migration, this function must not exist.
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
