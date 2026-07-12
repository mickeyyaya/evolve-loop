//go:build acs

// Package cycle748 materializes the cycle-748 acceptance criteria for the sole
// committed top_n task push-ci-watch-remote-parity (triage-report.md ## top_n;
// scout-report's three selections were all out of this fleet lane's assigned
// set, so per R9.3 no predicates bind to them and no deferred-floor predicates
// exist).
//
// Task source: .evolve/inbox/2026-07-07T06-32-00Z-push-ci-watch-remote-parity.json
// (weight 0.90). Incident: main stayed red 2026-07-06 11:50→20:16+ across 8
// pushes — nothing in the loop reads GitHub CI's verdict on a push, and
// release preflight never checks the remote CI conclusion on the release
// commit (v22.0.0 was cut on red CI, release_hardening memory).
//
// AC map (1:1), derived from the inbox item's acceptance list:
//
//	AC1 failed CI run ⇒ critical inbox item naming the failing test
//	    (faked gh runner)                                → C748_001 + C748_002 (negative)
//	AC2 evolve release refuses to tag on non-green release-commit CI;
//	    explicit override exists and logs loudly         → C748_003 + C748_004
//	AC3 CI verdict appears in the cycle dossier          → C748_005
//	AC4 knobs live in policy.json; zero new env flags    → C748_006 + C748_007 (config-check)
//	AC5 go vet / -race / apicover -enforce green         → manual+checklist (auditor
//	    runs the repo-wide CI-parity gates on touched pkgs per ADR-0069)
//
// Each behavioral predicate shells `go test -race -count=1 -v -run '^<name>$'`
// over the unit-test contract in the target package, which EXERCISES the SUT
// (faked gh runner seams, real temp inbox dirs, real policy.json documents) —
// behavioral via subprocess, no source-grep predicates (cycle-85 rule). The
// `-v` + "--- PASS:" guard rejects a rename/no-tests-matched silent green.
// C748_007 is the sole declared config-check (an ABSENCE assertion: the
// feature must introduce no EVOLVE_* env flag).
package cycle748

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	ciwatchPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	preflightPkg = "github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
	dossierPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	policyPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// runGoTest executes the named unit test under -race and requires an explicit
// verbose PASS marker so the predicate fails on: compile failure, test
// failure, a race report, a missing package, OR the test not existing
// (rename gaming).
func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

// AC1 (positive) — the incident twin: a push whose CI run FAILS yields a
// critical fix-forward inbox item that names the failing job/test (bounded log
// excerpt), driven through a faked gh-runner seam and a temp inbox dir.
func TestC748_001_FailedCIRunFilesCriticalInboxItem(t *testing.T) {
	runGoTest(t, ciwatchPkg, "TestCIWatch_FailedRunFilesCriticalInboxItem")
}

// AC1 (negative — strongest anti-no-op signal): a GREEN CI run files NO inbox
// item. A stub that unconditionally writes an escalation would pass C748_001
// and must fail here.
func TestC748_002_GreenCIRunFilesNoInboxItem(t *testing.T) {
	runGoTest(t, ciwatchPkg, "TestCIWatch_GreenRunFilesNoInboxItem")
}

// AC2 (positive gate) — release preflight REFUSES to proceed when the release
// commit's go CI run conclusion is not success (faked run-state seam; no
// network in tests).
func TestC748_003_ReleaseRefusesTagOnRedCI(t *testing.T) {
	runGoTest(t, preflightPkg, "TestRun_RefusesTagOnRedReleaseCommitCI")
}

// AC2 (override edge) — the explicit operator override lets a red-CI release
// proceed AND logs loudly (the override must be visible in preflight output,
// never silent).
func TestC748_004_ReleaseCIOverrideLogsLoudly(t *testing.T) {
	runGoTest(t, preflightPkg, "TestRun_CIOverrideAllowsRedCIAndLogsLoudly")
}

// AC3 — the CI-watch verdict recorded by the post-push watch is ingested into
// the cycle dossier (round-trips through dossier build; absent verdict is
// never fabricated).
func TestC748_005_CIVerdictAppearsInCycleDossier(t *testing.T) {
	runGoTest(t, dossierPkg, "TestBuild_IngestsCIWatchVerdict")
}

// AC4 (behavioral half) — all knobs (enabled, timeout, poll interval) resolve
// from a policy.json ci_watch block: absent block ⇒ compiled defaults
// (watch enabled — gates default ON as compiled Go defaults, per the
// observer/fleet pattern), present block overrides, malformed values are
// rejected explicitly rather than silently zeroed.
func TestC748_006_CIWatchKnobsResolveFromPolicyJSON(t *testing.T) {
	runGoTest(t, policyPkg, "TestCIWatchPolicy_KnobsFromPolicyJSON")
}

// AC4 (absence half) — zero new env flags: the flag registry table must not
// gain a CI-watch env var. This is an inherent config-presence(absence) check
// over the single SSOT registry table, not a feature-behavior grep.
// acs-predicate: config-check
func TestC748_007_NoNewCIWatchEnvFlag(t *testing.T) {
	root := acsassert.RepoRoot(t)
	table := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	if !acsassert.FileNotContains(t, table, "CI_WATCH") {
		t.Errorf("flagregistry registry_table.go gained a CI_WATCH env flag — AC4 requires policy.json knobs only (zero new env flags)")
	}
}
