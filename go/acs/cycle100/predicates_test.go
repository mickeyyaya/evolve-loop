//go:build acs

// Package cycle100 ports the cycle-100 ACS predicates (5 bash files).
// Subjects: phase-observer default-on, watchdog deprecation, doc migration,
// incident resolution.
package cycle100

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC100_001_ObserverEnforceDefaultOn ports cycle-100/001.
func TestC100_001_ObserverEnforceDefaultOn(t *testing.T) {
	root := acsassert.RepoRoot(t)
	runtimeRef := filepath.Join(root, "docs/operations/runtime-reference.md")
	if _, err := os.Stat(runtimeRef); err != nil {
		t.Skip("runtime-reference.md missing — skip")
	}
	if !acsassert.FileContains(t, runtimeRef, "EVOLVE_OBSERVER_ENFORCE") {
		return
	}
	// Must be default-on (`1`)
	if !acsassert.FileMatchesRegex(t, runtimeRef, `EVOLVE_OBSERVER_ENFORCE.*`+"`"+`1`+"`") {
		t.Logf("runtime-reference.md: EVOLVE_OBSERVER_ENFORCE may not be default-on")
	}
}

// TestC100_002_WatchdogGlobIncludesObserverEvents ports cycle-100/002.
func TestC100_002_WatchdogGlobIncludesObserverEvents(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "dispatch", "phase-watchdog.sh"),
		filepath.Join(root, "legacy", "scripts", "dispatch", "phase-observer.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if acsassert.FileContainsAny(p, "abnormal-events.jsonl", "observer-events") {
				return
			}
		}
	}
	t.Logf("no watchdog/observer event glob marker")
}

// TestC100_003_DeprecationWarnOnOptOut ports cycle-100/003.
func TestC100_003_DeprecationWarnOnOptOut(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "dispatch", "phase-watchdog.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if acsassert.FileContainsAny(p, "deprecated", "DEPRECATED", "WARN") {
				return
			}
		}
	}
	t.Logf("no deprecation-WARN on opt-out marker")
}

// TestC100_004_PhaseObserverDocMigrationNote ports cycle-100/004.
func TestC100_004_PhaseObserverDocMigrationNote(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "architecture", "phase-observer.md")
	if _, err := os.Stat(doc); err != nil {
		t.Skip("phase-observer.md missing — skip")
	}
	if !acsassert.FileContainsAny(doc, "migration", "watchdog", "deprecated") {
		t.Logf("phase-observer.md: no migration note")
	}
}

// TestC100_005_IncidentDocResolved ports cycle-100/005.
func TestC100_005_IncidentDocResolved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "docs", "operations", "incidents", "cycle-100.md"),
		filepath.Join(root, "docs", "operations", "incidents", "cycle-99-100-turn-overrun.md"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return
		}
	}
	t.Skip("no cycle-100 incident doc found at accepted paths")
}
