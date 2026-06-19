//go:build acs

// Package cycle11 materializes the cycle-11 acceptance criteria for the
// committed top_n task:
//
//	consolidate-observer-inactivity-cluster — remove all 13 OBSERVER_*/INACTIVITY_*
//	registry rows; add ObserverPolicy struct to policy.go; update 3 production
//	read sites (cmd_phase_observer.go, cmd_phase_watchdog.go, cmd_cycle.go);
//	lower FlagCeiling 176→163.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	consolidate-observer-inactivity-cluster:
//	  AC1+NEG1+NEG2 All 13 OBSERVER_*/INACTIVITY_* flags absent from Lookup → C11_001 (behavioral)
//	  AC3           Registry row count == 163                                 → C11_002 (behavioral, count)
//	  AC2           FlagCeiling const == 163                                  → C11_003 (config-check, waiver)
//	  AC4(a)        cmd_phase_observer.go OBSERVER_* env reads gone           → C11_004 (config-check, waiver)
//	  AC4(b)        cmd_phase_watchdog.go INACTIVITY_* env reads gone         → C11_005 (config-check, waiver)
//	  AC4(c)        cmd_cycle.go OBSERVER_AUTOSPAWN os.Getenv gone            → C11_006 (config-check, waiver)
//	  AC9           ObserverPolicy struct present in internal/policy/policy.go → C11_007 (config-check, waiver)
//	  EDGE1         control-flags.md has no OBSERVER_*/INACTIVITY_* rows      → C11_008 (config-check, waiver)
//	  EDGE3         docs_contract_test.go INACTIVITY_*/OBSERVER_EOF_GRACE_S
//	                removed from allowedUndocumented                          → C11_009 (config-check, waiver)
//
// ACs with manual+checklist disposition (enforced by CI, no cycle predicate needed):
//
//	AC5  (flagregistry tests pass): TestAll_SortedByName + TestRegistry_FlagCeiling in CI
//	AC6  (full suite 0 FAIL): CI pipeline
//	AC7  (flagreaders guard passes): CI acs lane — go test -tags acs ./acs/regression/flagreaders/...
//	AC10 (registry sorted): TestAll_SortedByName in normal CI run
//
// ACs removed:
//
//	AC8  (ACS cycle11 predicates pass): self-referential — unverifiable-remove
//	EDGE2 (.apicover-enforce has cycle11): pre-existing GREEN (TDD adds ./acs/cycle11/ during RED phase)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:    C11_001 — Lookup returns ok=false for all 13 flags; cannot satisfy
//	             by adding magic strings — the registry row must be absent.
//	Edge/OOD:    OBSERVER_ENABLED + OBSERVER_ENFORCE are in C11_001 — they were dead
//	             (0 production reads); their registry absence is the OOD case (pure
//	             delete, no code migration).
//	Lexical:     Lookup / len() / FileNotContains / FileContains — four distinct verbs.
//	Semantic:    registry count, flag-absence, env-reads-deleted, struct-added,
//	             docs-updated — five distinct behavioral checks.
//
// 1:1 enforcement: predicate=9, manual+checklist=4, unverifiable-remove=1,
// pre-existing-GREEN=1 → total AC=15 ✓
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (consolidate-observer-inactivity-cluster). Deferred tasks (BRIDGE_* cluster,
// CHECKPOINT_*, etc.) get zero predicates this cycle.
package cycle11

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC11_001_AllObserverInactivityFlagsAbsentFromRegistry verifies that all 13
// OBSERVER_*/INACTIVITY_* flags are no longer registered after Builder removes
// their rows from registry_table.go.
//
// Covers AC1 (0 rows in registry_table.go), NEG1 (removed flags absent from
// Lookup), and NEG2 (OBSERVER_ENABLED/ENFORCE — dead flags with 0 production
// reads — are also absent). Includes all 13 flags: 8 OBSERVER_ rows and 5
// INACTIVITY_ rows as enumerated in scout-report §Key Findings.
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the row must be absent for Lookup
// to return ok=false. Both the 11 config flags (had live reads) and the 2 dead
// flags (ENABLED, ENFORCE — no reads) are verified.
//
// RED: all 13 flags are currently registered; each Lookup returns (flag, true).
func TestC11_001_AllObserverInactivityFlagsAbsentFromRegistry(t *testing.T) {
	// All 13 OBSERVER_*/INACTIVITY_* rows from scout-report §Key Findings.
	// Lexical diversity: Lookup is the uniform verb; semantic diversity is the
	// full cluster (config flags + dead flags = two sub-cases).
	allFlags := []string{
		// 8 OBSERVER_ rows (scout-report §Key Findings)
		"EVOLVE_OBSERVER_AUTOSPAWN",
		"EVOLVE_OBSERVER_ENABLED", // dead: 0 production reads (comment only)
		"EVOLVE_OBSERVER_ENFORCE", // dead: 0 production reads (comment only)
		"EVOLVE_OBSERVER_EOF_GRACE_S",
		"EVOLVE_OBSERVER_NUDGE_BODY",
		"EVOLVE_OBSERVER_NUDGE_S",
		"EVOLVE_OBSERVER_POLL_S",
		"EVOLVE_OBSERVER_STALL_S",
		// 5 INACTIVITY_ rows (scout-report §Key Findings)
		"EVOLVE_INACTIVITY_DISABLE",
		"EVOLVE_INACTIVITY_GRACE_S",
		"EVOLVE_INACTIVITY_POLL_S",
		"EVOLVE_INACTIVITY_THRESHOLD_S",
		"EVOLVE_INACTIVITY_WARN_PCT",
	}
	for _, name := range allFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-11 OBSERVER+INACTIVITY consolidation).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC11_002_RegistryRowCountIs163 verifies that after removing all 13
// OBSERVER_*/INACTIVITY_* rows the total registry count is exactly 163.
//
// Covers AC3 (163 rows total). Both over-removal (< 163) and under-removal
// (> 163) fail the assertion.
//
// BEHAVIORAL: calls flagregistry.All directly (the production SSOT slice).
// No source-file grepping; a magic-string patch cannot satisfy this.
//
// RED: len(flagregistry.All) is currently 176, which is 13 rows above 163.
func TestC11_002_RegistryRowCountIs163(t *testing.T) {
	const want = 163
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove all 13 OBSERVER_*/INACTIVITY_* rows from registry_table.go.\n"+
			"Both over-removal (< 163) and under-removal (> 163) fail.\n"+
			"Expected: 176 − 13 = 163.",
			got, want)
	}
}

// TestC11_003_FlagCeilingConstIs163 verifies that the FlagCeiling ratchet
// constant in registry_ceiling_test.go has been lowered from 176 to 163
// in the same diff as the row removal.
//
// // acs-predicate: config-check — the constant value is the canonical ratchet
// config; still reading 176 after the 13-row removal breaks the ratchet guarantee.
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 176.
func TestC11_003_FlagCeilingConstIs163(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 163") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 163'.\n"+
			"Builder must lower the FlagCeiling constant from 176 to 163 in the same diff\n"+
			"as removing the 13 OBSERVER_*/INACTIVITY_* rows (176 − 13 = 163).\n"+
			"File: %s", ceilingFile)
	}
}

// TestC11_004_ObserverEnvReadsGoneFromPhaseObserverCmd verifies that the
// observerEnvConfig() function's envchain.Int / os.Getenv calls for all
// EVOLVE_OBSERVER_* and EVOLVE_INACTIVITY_THRESHOLD_S have been removed from
// cmd_phase_observer.go.
//
// The scout identified 5 env reads at lines 99-103:
//   - envchain.Int("EVOLVE_OBSERVER_POLL_S", ...)
//   - atoiOr(envOr("EVOLVE_OBSERVER_STALL_S", os.Getenv("EVOLVE_INACTIVITY_THRESHOLD_S")), ...)
//   - envchain.Int("EVOLVE_OBSERVER_NUDGE_S", ...)
//   - os.Getenv("EVOLVE_OBSERVER_NUDGE_BODY")
//   - envchain.Int("EVOLVE_OBSERVER_EOF_GRACE_S", ...)
//
// These are replaced by policy.Load(projectRoot()).ObserverConfig() fields.
//
// // acs-predicate: config-check — the env-read ABSENCE is the structural contract.
//
// RED: cmd_phase_observer.go:99 currently has envchain.Int("EVOLVE_OBSERVER_POLL_S",...).
func TestC11_004_ObserverEnvReadsGoneFromPhaseObserverCmd(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	observerCmd := filepath.Join(root, "go", "cmd", "evolve", "cmd_phase_observer.go")
	// Check that the key envchain sentinel read for OBSERVER_POLL_S is gone
	// (proxy for all 5 env reads in observerEnvConfig being removed).
	if !acsassert.FileNotContains(t, observerCmd, `envchain.Int("EVOLVE_OBSERVER_POLL_S"`) {
		t.Errorf("RED: cmd_phase_observer.go still reads EVOLVE_OBSERVER_POLL_S via envchain.\n"+
			"Builder must remove observerEnvConfig()'s 5 env reads (lines 99-103) and\n"+
			"replace them with policy.Load(projectRoot()).ObserverConfig() field reads.\n"+
			"File: %s", observerCmd)
	}
	// Also verify the two-key STALL_S/INACTIVITY_THRESHOLD_S fallback is gone.
	if !acsassert.FileNotContains(t, observerCmd, `"EVOLVE_INACTIVITY_THRESHOLD_S"`) {
		t.Errorf("RED: cmd_phase_observer.go still has the EVOLVE_INACTIVITY_THRESHOLD_S fallback.\n"+
			"Builder must remove the envOr(...os.Getenv(\"EVOLVE_INACTIVITY_THRESHOLD_S\")) call\n"+
			"and replace it with policy.ObserverConfig().StallS (single default in code).\n"+
			"File: %s", observerCmd)
	}
}

// TestC11_005_InactivityEnvReadsGoneFromPhaseWatchdogCmd verifies that the
// watchdogEnvConfig() function's envchain.Int / envchain.Bool calls for all
// EVOLVE_INACTIVITY_* flags have been removed from cmd_phase_watchdog.go.
//
// The scout identified 5 env reads at lines 56-60:
//   - envchain.Int("EVOLVE_INACTIVITY_THRESHOLD_S", ...)
//   - envchain.Int("EVOLVE_INACTIVITY_POLL_S", ...)
//   - envchain.Int("EVOLVE_INACTIVITY_WARN_PCT", ...)
//   - envchain.Int("EVOLVE_INACTIVITY_GRACE_S", ...)
//   - envchain.Bool("EVOLVE_INACTIVITY_DISABLE", ...)
//
// These are replaced by policy.Load(projectRoot()).ObserverConfig() fields.
//
// // acs-predicate: config-check — the env-read ABSENCE is the structural contract.
//
// RED: cmd_phase_watchdog.go:56 currently has envchain.Int("EVOLVE_INACTIVITY_THRESHOLD_S",...).
func TestC11_005_InactivityEnvReadsGoneFromPhaseWatchdogCmd(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	watchdogCmd := filepath.Join(root, "go", "cmd", "evolve", "cmd_phase_watchdog.go")
	if !acsassert.FileNotContains(t, watchdogCmd, `envchain.Int("EVOLVE_INACTIVITY_THRESHOLD_S"`) {
		t.Errorf("RED: cmd_phase_watchdog.go still reads EVOLVE_INACTIVITY_THRESHOLD_S via envchain.\n"+
			"Builder must remove watchdogEnvConfig()'s 5 env reads (lines 56-60) and\n"+
			"replace them with policy.Load(projectRoot()).ObserverConfig() field reads.\n"+
			"File: %s", watchdogCmd)
	}
	// Also verify the Bool read for DISABLE is gone.
	if !acsassert.FileNotContains(t, watchdogCmd, `envchain.Bool("EVOLVE_INACTIVITY_DISABLE"`) {
		t.Errorf("RED: cmd_phase_watchdog.go still reads EVOLVE_INACTIVITY_DISABLE via envchain.Bool.\n"+
			"Builder must replace with policy.ObserverConfig().WatchdogDisabled.\n"+
			"File: %s", watchdogCmd)
	}
}

// TestC11_006_AutospawnOsGetenvGoneFromCycleCmd verifies that the
// os.Getenv("EVOLVE_OBSERVER_AUTOSPAWN") call at cmd_cycle.go:439 has been
// removed and replaced by policy.ObserverConfig().Autospawn.
//
// // acs-predicate: config-check — the os.Getenv ABSENCE is the structural contract.
//
// RED: cmd_cycle.go:439 currently has os.Getenv("EVOLVE_OBSERVER_AUTOSPAWN") != "0".
func TestC11_006_AutospawnOsGetenvGoneFromCycleCmd(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cycleCmd := filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")
	if !acsassert.FileNotContains(t, cycleCmd, `os.Getenv("EVOLVE_OBSERVER_AUTOSPAWN")`) {
		t.Errorf("RED: cmd_cycle.go still reads EVOLVE_OBSERVER_AUTOSPAWN via os.Getenv.\n"+
			"Builder must replace:\n"+
			"  os.Getenv(\"EVOLVE_OBSERVER_AUTOSPAWN\") != \"0\"\n"+
			"with:\n"+
			"  policy.ObserverConfig().Autospawn  (default true)\n"+
			"File: %s", cycleCmd)
	}
}

// TestC11_007_ObserverPolicyStructAddedToPolicy verifies that the ObserverPolicy
// struct has been added to internal/policy/policy.go.
//
// ObserverPolicy is the Configuration Object that replaces all 13 OBSERVER_*/
// INACTIVITY_* env vars. It is loaded from .evolve/policy.json "observer" block
// and injected into the 3 production read sites. Default values are encoded in
// Policy.ObserverConfig() (Autospawn=true, PollS=5, StallS=600, NudgeS=300,
// WatchdogPollS=15, WatchdogWarnPct=75, WatchdogGraceS=10).
//
// // acs-predicate: config-check — verifies the new config surface exists.
//
// RED: internal/policy/policy.go currently has no ObserverPolicy type.
func TestC11_007_ObserverPolicyStructAddedToPolicy(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	policyFile := filepath.Join(root, "go", "internal", "policy", "policy.go")
	if !acsassert.FileContains(t, policyFile, "ObserverPolicy") {
		t.Errorf("RED: internal/policy/policy.go has no ObserverPolicy struct.\n"+
			"Builder must add the ObserverPolicy struct and Policy.ObserverConfig() method\n"+
			"following the FanoutPolicy precedent (cycle-9).\n"+
			"Required fields: Autospawn bool, PollS int, StallS int, NudgeS int, NudgeBody string,\n"+
			"EOFGraceS int, WatchdogPollS int, WatchdogWarnPct int, WatchdogGraceS int, WatchdogDisabled bool.\n"+
			"File: %s", policyFile)
	}
	// Also verify the ObserverConfig() accessor method is present.
	if !acsassert.FileContains(t, policyFile, "ObserverConfig()") {
		t.Errorf("RED: internal/policy/policy.go has no ObserverConfig() method.\n"+
			"Builder must add Policy.ObserverConfig() that returns ObserverPolicy with\n"+
			"defaults applied (Autospawn=true, PollS=5, StallS=600, NudgeS=300,\n"+
			"WatchdogPollS=15, WatchdogWarnPct=75, WatchdogGraceS=10).\n"+
			"File: %s", policyFile)
	}
}

// TestC11_008_ControlFlagsMdHasNoObserverInactivityRows verifies that the
// generated doc docs/architecture/control-flags.md has no EVOLVE_OBSERVER_*
// or EVOLVE_INACTIVITY_* entries after the 13 registry rows are removed and
// the doc regenerated.
//
// Covers EDGE1. The doc is generated from the flagregistry (source of truth);
// its absence of OBSERVER_*/INACTIVITY_* rows follows from C11_001 (rows removed)
// plus the regeneration step ('evolve flags generate').
//
// // acs-predicate: config-check — the doc regeneration is a required build step.
//
// RED: control-flags.md currently has EVOLVE_OBSERVER_* and EVOLVE_INACTIVITY_* entries.
func TestC11_008_ControlFlagsMdHasNoObserverInactivityRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	if !acsassert.FileNotContains(t, controlFlags, "EVOLVE_OBSERVER_AUTOSPAWN") {
		t.Errorf("RED: control-flags.md still contains EVOLVE_OBSERVER_AUTOSPAWN.\n"+
			"Builder must remove all 13 OBSERVER_*/INACTIVITY_* rows from registry_table.go\n"+
			"then regenerate the doc via 'evolve flags generate'.\n"+
			"File: %s", controlFlags)
	}
	if !acsassert.FileNotContains(t, controlFlags, "EVOLVE_INACTIVITY_THRESHOLD_S") {
		t.Errorf("RED: control-flags.md still contains EVOLVE_INACTIVITY_THRESHOLD_S.\n"+
			"Builder must regenerate control-flags.md after removing all 13 registry rows.\n"+
			"File: %s", controlFlags)
	}
}

// TestC11_009_DocsContractTestHasNoInactivityAllowedEntries verifies that
// docs_contract_test.go's allowedUndocumented map no longer contains entries
// for EVOLVE_INACTIVITY_* flags and EVOLVE_OBSERVER_EOF_GRACE_S.
//
// Covers EDGE3. The scout found 6 entries that must be removed when their
// registry rows go:
//   - EVOLVE_INACTIVITY_DISABLE (line 77)
//   - EVOLVE_INACTIVITY_GRACE_S (line 78)
//   - EVOLVE_INACTIVITY_POLL_S  (line 79)
//   - EVOLVE_INACTIVITY_WARN_PCT (line 80)
//   - EVOLVE_OBSERVER_EOF_GRACE_S (line 83)
//
// Note: EVOLVE_INACTIVITY_THRESHOLD_S is NOT in allowedUndocumented (scout confirms
// the 5-entry list at docs_contract_test.go:77-80,83).
//
// // acs-predicate: config-check — the allowedUndocumented map is a config surface.
//
// RED: docs_contract_test.go currently has all 5 entries in allowedUndocumented.
func TestC11_009_DocsContractTestHasNoInactivityAllowedEntries(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	docsContractTest := filepath.Join(root, "go", "cmd", "evolve", "docs_contract_test.go")
	if !acsassert.FileNotContains(t, docsContractTest, `"EVOLVE_INACTIVITY_DISABLE"`) {
		t.Errorf("RED: docs_contract_test.go still has EVOLVE_INACTIVITY_DISABLE in allowedUndocumented.\n"+
			"Builder must remove all 5 entries: INACTIVITY_DISABLE, INACTIVITY_GRACE_S,\n"+
			"INACTIVITY_POLL_S, INACTIVITY_WARN_PCT, and OBSERVER_EOF_GRACE_S.\n"+
			"These flags are being removed from the codebase, not just from the registry.\n"+
			"File: %s", docsContractTest)
	}
	if !acsassert.FileNotContains(t, docsContractTest, `"EVOLVE_OBSERVER_EOF_GRACE_S"`) {
		t.Errorf("RED: docs_contract_test.go still has EVOLVE_OBSERVER_EOF_GRACE_S in allowedUndocumented.\n"+
			"Builder must remove it when removing the OBSERVER_EOF_GRACE_S registry row.\n"+
			"File: %s", docsContractTest)
	}
}
