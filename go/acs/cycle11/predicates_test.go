//go:build acs

package cycle11

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC11_001_AllObserverInactivityFlagsAbsentFromRegistry(t *testing.T) {
	allFlags := []string{
		"EVOLVE_OBSERVER_AUTOSPAWN",
		"EVOLVE_OBSERVER_ENABLED",
		"EVOLVE_OBSERVER_ENFORCE",
		"EVOLVE_OBSERVER_EOF_GRACE_S",
		"EVOLVE_OBSERVER_NUDGE_BODY",
		"EVOLVE_OBSERVER_NUDGE_S",
		"EVOLVE_OBSERVER_POLL_S",
		"EVOLVE_OBSERVER_STALL_S",
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

func TestC11_004_ObserverEnvReadsGoneFromPhaseObserverCmd(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	observerCmd := filepath.Join(root, "go", "internal", "cli", "phasecmd", "phase_observer.go")
	if !acsassert.FileNotContains(t, observerCmd, `envchain.Int("EVOLVE_OBSERVER_POLL_S"`) {
		t.Errorf("RED: cmd_phase_observer.go still reads EVOLVE_OBSERVER_POLL_S via envchain.\n"+
			"Builder must remove observerEnvConfig()'s 5 env reads (lines 99-103) and\n"+
			"replace them with policy.Load(projectRoot()).ObserverConfig() field reads.\n"+
			"File: %s", observerCmd)
	}
	if !acsassert.FileNotContains(t, observerCmd, `"EVOLVE_INACTIVITY_THRESHOLD_S"`) {
		t.Errorf("RED: cmd_phase_observer.go still has the EVOLVE_INACTIVITY_THRESHOLD_S fallback.\n"+
			"Builder must remove the envOr(...os.Getenv(\"EVOLVE_INACTIVITY_THRESHOLD_S\")) call\n"+
			"and replace it with policy.ObserverConfig().StallS (single default in code).\n"+
			"File: %s", observerCmd)
	}
}

func TestC11_005_InactivityEnvReadsGoneFromPhaseWatchdogCmd(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	watchdogCmd := filepath.Join(root, "go", "internal", "cli", "phasecmd", "phase_watchdog.go")
	if !acsassert.FileNotContains(t, watchdogCmd, `envchain.Int("EVOLVE_INACTIVITY_THRESHOLD_S"`) {
		t.Errorf("RED: cmd_phase_watchdog.go still reads EVOLVE_INACTIVITY_THRESHOLD_S via envchain.\n"+
			"Builder must remove watchdogEnvConfig()'s 5 env reads (lines 56-60) and\n"+
			"replace them with policy.Load(projectRoot()).ObserverConfig() field reads.\n"+
			"File: %s", watchdogCmd)
	}
	if !acsassert.FileNotContains(t, watchdogCmd, `envchain.Bool("EVOLVE_INACTIVITY_DISABLE"`) {
		t.Errorf("RED: cmd_phase_watchdog.go still reads EVOLVE_INACTIVITY_DISABLE via envchain.Bool.\n"+
			"Builder must replace with policy.ObserverConfig().WatchdogDisabled.\n"+
			"File: %s", watchdogCmd)
	}
}

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
	if !acsassert.FileContains(t, policyFile, "ObserverConfig()") {
		t.Errorf("RED: internal/policy/policy.go has no ObserverConfig() method.\n"+
			"Builder must add Policy.ObserverConfig() that returns ObserverPolicy with\n"+
			"defaults applied (Autospawn=true, PollS=5, StallS=600, NudgeS=300,\n"+
			"WatchdogPollS=15, WatchdogWarnPct=75, WatchdogGraceS=10).\n"+
			"File: %s", policyFile)
	}
}

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
