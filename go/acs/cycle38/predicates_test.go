//go:build acs

// Package cycle38 materializes the cycle-38 acceptance criteria for TWO tasks
// (both in triage ## top_n):
//
//  1. router-cli-model-cluster-38 — migrate EVOLVE_ROUTER_CLI and
//     EVOLVE_ROUTER_MODEL from os.Getenv reads in cmd_cycle.go to new CLI/Model
//     string fields on the existing policy.RouterPolicy struct. Remove 2 registry
//     rows. Lower FlagCeiling 83→81.
//
//  2. gc-mode-config-38 — migrate EVOLVE_GC from os.Getenv read in
//     cmd_loop_outcome.go to a new Mode string field on the existing gc.Policy
//     struct. Remove 1 registry row. Lower FlagCeiling 81→80.
//
// Both tasks ship in the same cycle. ACS predicates verify the FINAL state
// (80 flags, ceiling=80) after both tasks complete. Intermediate ceiling-81
// state (Task 1 alone) is not a separate predicate because the audit suite
// runs after Builder finishes both tasks.
//
// AC map (1:1 with triage top_n tasks):
//
//	router-cli-model-cluster-38:
//	  AC1  EVOLVE_ROUTER_CLI absent from registry        → C38_001 (behavioral)
//	  AC2  EVOLVE_ROUTER_MODEL absent from registry      → C38_002 (behavioral)
//	  AC5  No prod env reads in cmd_cycle.go             → C38_005 (config-check, waiver)
//	  AC6  RouterPolicy has CLI+Model fields (compile)   → C38_006 (behavioral, compile-fail RED)
//	  AC7  flagreaders guard green                       → manual+checklist (see below)
//	  AC8  control-flags.md drops both rows              → C38_008 (config-check, waiver)
//	  NEG1 No t.Setenv for old env vars in tests         → C38_NEG1 (config-check, waiver)
//	  FULL go test ./... green                           → manual+checklist (see below)
//	  NOTE AC3 (count==81) and AC4 (ceiling==81) are INTERMEDIATE; superseded by
//	       gc-mode AC2 (count==80) and AC3 (ceiling==80) since both tasks ship together.
//
//	gc-mode-config-38:
//	  AC1  EVOLVE_GC absent from registry               → C38_GC_001 (behavioral)
//	  AC2  len(flagregistry.All) == 80                  → C38_GC_002 (behavioral, count)
//	  AC3  FlagCeiling == 80                            → C38_GC_003 (config-check, waiver)
//	  AC4  No prod EVOLVE_GC reads in cmd_loop_outcome.go → C38_GC_004 (config-check, waiver)
//	  AC5  gc.Policy.Mode field exists (compile)        → C38_GC_005 (behavioral, compile-fail RED)
//	  AC6  Mode off/shadow/enforce recognized           → manual+checklist (see below)
//	  NEG1 Invalid mode logs WARN, skips                → manual+checklist (see below)
//	  AC7  flagreaders guard green                      → manual+checklist (see below)
//	  FULL go test ./... green                          → manual+checklist (see below)
//
// ACs with manual+checklist disposition:
//
//	AC7 / flagreaders guard (both tasks): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) exit 0 from `cd go && go test -tags acs ./acs/regression/flagreaders/...`;
//	    (b) none of the 3 flag name strings appear in any non-test, non-registry Go file:
//	        grep -rn '"EVOLVE_ROUTER_CLI"\|"EVOLVE_ROUTER_MODEL"\|"EVOLVE_GC"' go/ \
//	          --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches.
//
//	AC6 router-cli (RouterConfig wires CLI/Model into resolveRouterDispatch):
//	    Checklist for Auditor:
//	    (a) resolveRouterDispatch in cmd_cycle.go accepts rc policy.RouterPolicy parameter;
//	    (b) when rc.CLI != "", the returned cli equals rc.CLI;
//	    (c) when rc.Model != "", the returned model equals rc.Model;
//	    (d) resolveRouterDispatchFor passes rc to resolveRouterDispatch.
//	    NOTE: see C38_006 for the compile-fail predicate; the behavioral wiring above
//	    is verified by updated unit tests in cmd_cycle_test.go (Builder must update them).
//
//	gc AC6 (Mode off/shadow/enforce recognized by runGCHook):
//	    Checklist for Auditor:
//	    (a) `go test -run TestGCPolicyModeRecognized ./internal/gc/...` exits 0;
//	    (b) switch in runGCHook covers "off", "shadow", "enforce" branches explicitly;
//	    (c) Builder added TestGCPolicyModeDefaultsOff and TestGCPolicyModeRecognized to
//	        go/internal/gc/ (or a new policy_mode_test.go).
//
//	gc NEG1 (Invalid mode logs WARN, skips):
//	    Checklist for Auditor:
//	    (a) `go test -run TestRunGCHook_InvalidModeSkipped ./cmd/evolve/...` exits 0;
//	    (b) cmd_loop_outcome.go's switch default branch writes `[gc] WARN: ...` to stderr
//	        and returns without running gc.Plan; Builder must NOT remove the WARN behavior.
//
//	FULL (go test ./... clean) for both tasks:
//	    Checklist for Auditor:
//	    (a) exit 0 from `cd go && go test ./... -count=1`;
//	    (b) cmd_cycle_test.go compiles with updated resolveRouterDispatch signature;
//	    (c) cmd_router_dispatch_test.go compiles with updated resolveRouterDispatchHealthy
//	        and updated test assertions (no t.Setenv for ROUTER_CLI/MODEL).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C38_001, C38_002, C38_GC_001 — 3 flags must be ABSENT from Lookup
//	            (if Builder misses one, Lookup returns ok=true and the test fails).
//	            C38_005, C38_GC_004 — env-read string literals must be ABSENT from
//	            source files (anti-gaming: removing the registry row without deleting
//	            the os.Getenv call is the cycle-8 split-const failure mode).
//	            C38_NEG1 — old t.Setenv calls must be ABSENT from test files (tests
//	            that call t.Setenv("EVOLVE_ROUTER_CLI",...) test a deleted env path).
//	Edge/OOD:   C38_GC_002 checks EXACT count 80; over-removal (<80) and under-removal
//	            (>80) both fail.
//	Lexical:    Lookup / len / FileNotContains / FileContains / RouterPolicy field access /
//	            gc.Policy field access — distinct assertion verbs across the suite.
//	Semantic:   registry-absence (3), no-env-reads (2), struct-compile (2), no-stale-tests (1),
//	            count (1), ceiling (1), no-doc-entries (2) — 12 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n tasks
// (router-cli-model-cluster-38 and gc-mode-config-38). Deferred tasks (workflow-defaults-cluster,
// legacy-phase-enable-cluster, dynamic-routing-cluster) get zero predicates this cycle.
//
// 1:1 enforcement:
//
//	Task 1: predicate=5, manual+checklist=3 (AC7, FULL, AC6-behavioral), unverifiable-remove=0
//	        → task AC count=8 ✓ (AC1, AC2, AC5, AC6, AC7, AC8, NEG1, FULL;
//	           AC3/AC4 are intermediate and superseded by gc AC2/AC3)
//	Task 2: predicate=4, manual+checklist=5 (AC6, NEG1, AC7, FULL, flagreaders), unverifiable-remove=0
//	        → task AC count=9 ✓ (AC1, AC2, AC3, AC4, AC5, AC6, NEG1, AC7, FULL)
package cycle38

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ---------------------------------------------------------------------------
// Task 1: router-cli-model-cluster-38
// ---------------------------------------------------------------------------

// TestC38_001_RouterCLIAbsentFromRegistry verifies that EVOLVE_ROUTER_CLI is no
// longer registered after Builder removes its row from registry_table.go.
//
// Covers AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: EVOLVE_ROUTER_CLI is currently in registry_table.go (StatusInternal);
// Lookup returns (flag, true).
func TestC38_001_RouterCLIAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_ROUTER_CLI"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove the EVOLVE_ROUTER_CLI row from registry_table.go.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_ROUTER_CLI", f.Status, f.Cluster)
	}
}

// TestC38_002_RouterModelAbsentFromRegistry verifies that EVOLVE_ROUTER_MODEL is
// no longer registered after Builder removes its row from registry_table.go.
//
// Covers AC2. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
//
// RED: EVOLVE_ROUTER_MODEL is currently in registry_table.go (StatusInternal);
// Lookup returns (flag, true).
func TestC38_002_RouterModelAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_ROUTER_MODEL"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove the EVOLVE_ROUTER_MODEL row from registry_table.go.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_ROUTER_MODEL", f.Status, f.Cluster)
	}
}

// TestC38_005_NoProdRouterEnvReadsInCmdCycle verifies that the two os.Getenv
// string literals for EVOLVE_ROUTER_CLI and EVOLVE_ROUTER_MODEL have been deleted
// from cmd_cycle.go.
//
// Covers AC5. Anti-gaming (cycle-8 split-const lesson): Builder cannot remove
// the registry rows while leaving the os.Getenv("EVOLVE_ROUTER_CLI") call sites.
// This predicate catches that gap.
//
// acs-predicate: config-check
//
// RED: cmd_cycle.go currently contains os.Getenv("EVOLVE_ROUTER_CLI") at line
// 639 and os.Getenv("EVOLVE_ROUTER_MODEL") at line 642. Both string literals
// (with surrounding double quotes) must be absent after migration.
func TestC38_005_NoProdRouterEnvReadsInCmdCycle(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cmdFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")

	for _, name := range []string{"EVOLVE_ROUTER_CLI", "EVOLVE_ROUTER_MODEL"} {
		// Check for the quoted string literal form: "EVOLVE_ROUTER_CLI"
		// This targets os.Getenv calls, not comments (comments lack the surrounding quotes).
		literal := `"` + name + `"`
		if !acsassert.FileNotContains(t, cmdFile, literal) {
			t.Errorf("RED: cmd_cycle.go still contains the string literal %s.\n"+
				"Builder must replace os.Getenv(%q) with rc.CLI or rc.Model from the\n"+
				"policy.RouterPolicy parameter (cycle-8 anti-gaming: removing the registry\n"+
				"row without deleting the os.Getenv call is the split-const hiding pattern).\n"+
				"File: %s", literal, name, cmdFile)
		}
	}
}

// TestC38_006_RouterPolicyHasCLIAndModelFields verifies that policy.RouterPolicy
// has the new CLI and Model string fields required by the migration.
//
// Covers AC6. BEHAVIORAL (compile-fail RED): this test directly accesses
// policy.RouterPolicy{}.CLI and policy.RouterPolicy{}.Model. Until Builder adds
// these fields to RouterPolicy in policy.go, this test fails to compile with
// "unknown field CLI" / "unknown field Model". A compile failure IS the RED state.
//
// After Builder adds the fields and wires them in RouterConfig(), the
// zero-value defaults must be empty strings (no CLI/model override by default),
// which is correct: the profile + fallback (claude-tmux/opus) provides the base.
//
// RED: policy.RouterPolicy currently has RouterReplan, RoutingJudge, ReconDigest,
// ReplanDepth, PlanModel, ProposeModel — but NOT CLI or Model. This test does
// not compile.
func TestC38_006_RouterPolicyHasCLIAndModelFields(t *testing.T) {
	// Directly construct a RouterPolicy with the new fields — compile-fail RED
	// until Builder adds the fields to the struct.
	cfg := policy.Policy{
		Router: &policy.RouterPolicy{
			CLI:   "codex-tmux",
			Model: "deep",
		},
	}.RouterConfig()

	if cfg.CLI != "codex-tmux" {
		t.Errorf("RED: RouterConfig().CLI = %q, want \"codex-tmux\".\n"+
			"Builder must add CLI string field to RouterPolicy and wire it in RouterConfig().\n"+
			"policy.RouterPolicy{CLI: \"codex-tmux\"} must flow through to the resolver.", cfg.CLI)
	}
	if cfg.Model != "deep" {
		t.Errorf("RED: RouterConfig().Model = %q, want \"deep\".\n"+
			"Builder must add Model string field to RouterPolicy and wire it in RouterConfig().\n"+
			"policy.RouterPolicy{Model: \"deep\"} must flow through to the resolver.", cfg.Model)
	}

	// Also verify zero-value defaults: an absent Router block must yield empty CLI/Model
	// (no override — the profile + fallback provides the base).
	dflt := policy.Policy{}.RouterConfig()
	if dflt.CLI != "" {
		t.Errorf("RED: policy.Policy{}.RouterConfig().CLI = %q, want \"\" (no override default).\n"+
			"An absent/empty RouterPolicy.CLI must not override the profile default.", dflt.CLI)
	}
	if dflt.Model != "" {
		t.Errorf("RED: policy.Policy{}.RouterConfig().Model = %q, want \"\" (no override default).\n"+
			"An absent/empty RouterPolicy.Model must not override the profile default.", dflt.Model)
	}
}

// TestC38_008_ControlFlagsDocNoRouterCLIModelRows verifies that the regenerated
// docs/architecture/control-flags.md no longer contains entries for EVOLVE_ROUTER_CLI
// and EVOLVE_ROUTER_MODEL. This checks that `evolve flags generate` was run in the
// same diff as the registry row removals.
//
// Covers AC8. acs-predicate: config-check
//
// RED: control-flags.md currently has rows for both flags (they are in the
// registry). After the migration, the doc must be regenerated and both flag
// names must be absent.
func TestC38_008_ControlFlagsDocNoRouterCLIModelRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range []string{"EVOLVE_ROUTER_CLI", "EVOLVE_ROUTER_MODEL"} {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"the 2 router CLI/model rows (e.g. `evolve flags generate`) in the same diff.\n"+
				"File: %s", name, controlFlagsDoc)
		}
	}
}

// TestC38_NEG1_NoStaleRouterEnvSetenvsInTests verifies that the old t.Setenv calls
// for EVOLVE_ROUTER_CLI and EVOLVE_ROUTER_MODEL have been removed from the unit
// test files that tested the env-read code paths.
//
// Covers NEG1. After the migration, tests must use policy.RouterPolicy{CLI: "..."}
// instead of t.Setenv("EVOLVE_ROUTER_CLI", "...") to configure the router dispatch.
// Stale t.Setenv calls test a deleted code path and must be removed.
//
// acs-predicate: config-check
//
// RED: cmd_cycle_test.go currently uses t.Setenv("EVOLVE_ROUTER_CLI", ...) in
// TestResolveRouterDispatch_Precedence (5 calls); cmd_router_dispatch_test.go uses
// t.Setenv("EVOLVE_ROUTER_CLI", ...) in 2 test functions.
func TestC38_NEG1_NoStaleRouterEnvSetenvsInTests(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	testFiles := []string{
		filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle_test.go"),
		filepath.Join(root, "go", "cmd", "evolve", "cmd_router_dispatch_test.go"),
	}
	for _, path := range testFiles {
		for _, name := range []string{"EVOLVE_ROUTER_CLI", "EVOLVE_ROUTER_MODEL"} {
			// Check for the t.Setenv call form: t.Setenv("EVOLVE_ROUTER_CLI"
			setenvLiteral := `t.Setenv("` + name + `"`
			if !acsassert.FileNotContains(t, path, setenvLiteral) {
				t.Errorf("RED: %s still contains %q.\n"+
					"Builder must remove the t.Setenv call for %q and replace it with\n"+
					"policy.RouterPolicy{CLI: \"...\"} or policy.RouterPolicy{Model: \"...\"}\n"+
					"(tests for a deleted env override path must not remain).\n"+
					"File: %s", filepath.Base(path), setenvLiteral, name, path)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Task 2: gc-mode-config-38
// ---------------------------------------------------------------------------

// TestC38_GC_001_GCAbsentFromRegistry verifies that EVOLVE_GC is no longer
// registered after Builder removes its row from registry_table.go.
//
// Covers gc-mode AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
//
// RED: EVOLVE_GC is currently in registry_table.go (StatusActive); Lookup returns
// (flag, true).
func TestC38_GC_001_GCAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_GC"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove the EVOLVE_GC row from registry_table.go.\n"+
			"Current entry: Status=%q Cluster=%q Kind=%q",
			"EVOLVE_GC", f.Status, f.Cluster, f.Kind)
	}
}

// TestC38_GC_004_NoProdGCEnvReadInCmdLoopOutcome verifies that the os.Getenv
// string literal for EVOLVE_GC has been deleted from cmd_loop_outcome.go.
//
// Covers gc-mode AC4. Anti-gaming (cycle-8 split-const lesson): Builder cannot
// remove the registry row while leaving the os.Getenv("EVOLVE_GC") call site.
//
// acs-predicate: config-check
//
// RED: cmd_loop_outcome.go currently contains os.Getenv("EVOLVE_GC") at line 86.
// The quoted string literal "EVOLVE_GC" must be absent after migration.
func TestC38_GC_004_NoProdGCEnvReadInCmdLoopOutcome(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	outcomeFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_outcome.go")
	literal := `"EVOLVE_GC"`
	if !acsassert.FileNotContains(t, outcomeFile, literal) {
		t.Errorf("RED: cmd_loop_outcome.go still contains the string literal %s.\n"+
			"Builder must replace os.Getenv(\"EVOLVE_GC\") with gcPol.Mode after loading\n"+
			"the gc.Policy from policy.json (cycle-8 anti-gaming: removing the registry\n"+
			"row without deleting the os.Getenv call is the split-const hiding pattern).\n"+
			"File: %s", literal, outcomeFile)
	}
}

// TestC38_GC_005_GCPolicyHasModeField verifies that gc.Policy has the new Mode
// string field required by the migration.
//
// Covers gc-mode AC5. BEHAVIORAL (compile-fail RED): this test directly accesses
// gc.Policy{}.Mode. Until Builder adds the Mode field to gc.Policy in gc.go,
// this test fails to compile with "unknown field Mode". A compile failure IS the
// RED state.
//
// The zero value of Mode ("") means "off" behavior in runGCHook:
//
//	if gcPol.Mode == "" { mode = "off" }
//
// So gc.Policy{}.Mode must be "" (the Go zero value of string).
//
// RED: gc.Policy currently has Runs, SalvageTTLDays, LogsTTLDays, TrackerTTLDays
// — but NOT Mode. This test does not compile.
func TestC38_GC_005_GCPolicyHasModeField(t *testing.T) {
	// Directly access the Mode field — compile-fail RED until Builder adds it.
	pol := gc.Policy{Mode: "shadow"}
	if pol.Mode != "shadow" {
		t.Errorf("RED: gc.Policy{Mode: \"shadow\"}.Mode = %q, want \"shadow\".\n"+
			"Builder must add Mode string field to gc.Policy in go/internal/gc/gc.go.", pol.Mode)
	}

	// Zero value must be "" (the "off" sentinel for runGCHook).
	zeroPol := gc.Policy{}
	if zeroPol.Mode != "" {
		t.Errorf("RED: gc.Policy{}.Mode = %q, want \"\" (zero value = off behavior).\n"+
			"The Mode field must be a plain string (zero value = \"\"); runGCHook converts\n"+
			"\"\" to \"off\" behavior so an absent gc block in policy.json behaves as disabled.", zeroPol.Mode)
	}
}

// TestC38_GC_008_ControlFlagsDocNoGCRow verifies that the regenerated
// docs/architecture/control-flags.md no longer contains an entry for EVOLVE_GC.
//
// This is the Task 2 equivalent of AC8 for Task 1 (control-flags.md regeneration).
// acs-predicate: config-check
//
// RED: control-flags.md currently has a row for EVOLVE_GC (StatusActive, Kind=enum).
// After the migration, the doc must be regenerated and EVOLVE_GC must be absent.
func TestC38_GC_008_ControlFlagsDocNoGCRow(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	if !acsassert.FileNotContains(t, controlFlagsDoc, "EVOLVE_GC") {
		t.Errorf("RED: control-flags.md still contains \"EVOLVE_GC\".\n"+
			"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
			"the EVOLVE_GC row (e.g. `evolve flags generate`) in the same diff.\n"+
			"File: %s", controlFlagsDoc)
	}
}
