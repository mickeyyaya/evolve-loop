//go:build acs

// Package cycle1145 materialises the cycle-1145 acceptance criteria for the two
// fleet-scoped SSOT tasks triage committed to THIS cycle:
//
//   - artifact-name-ssot-scout-report-backfill  → route the hardcoded
//     "scout-report.md" literals through phasecontract's ArtifactName SSOT
//   - required-roles-ssot-subagent-dispatch     → derive subagent dispatch's
//     agentRoles allow-list from the phasecontract registry
//
// The third fleet-scoped id (cycle-docs-floor-architecture-changes) was
// DEFERRED by triage and therefore carries ZERO predicates here (R9.3:
// predicates bind only to triage-committed work; a predicate gating deferred
// work starves the committed task — the cycle-280 failure mode).
//
// Predicate strategy. Both tasks are duplication-removal refactors, so the two
// crux predicates split along the only two axes that can actually observe the
// defect:
//
//   - 001 is an ABSENCE check (acsassert-style FileNotContains semantics over
//     the production tree, the sanctioned form per go/acs/README.md "Absence
//     checks"): the quoted literal must survive in exactly ONE declaration
//     site. Duplication is inherently a source-level property — no runtime
//     observation can distinguish five copies of an equal string from one
//     shared one — so this is the predicate that red-fails today.
//     003 is its anti-gaming twin: greening 001 by deleting or renaming the
//     SSOT declaration itself fails 003.
//   - 002 is BEHAVIORAL and exercises evalgate (one of the five literal sites)
//     end-to-end through its exported core.DeliverableReviewer: the gate must
//     still find and parse a scout report named by the registry, and must fail
//     open when it is named anything else. It is pre-existing GREEN and stays
//     green only if the refactor preserves the resolved value — the regression
//     half of "consolidate the literal".
//   - 004 is BEHAVIORAL and red-fails today: every LLM-dispatchable agent the
//     phasecontract registry declares must be accepted by the subagent dispatch
//     allow-list. "router" is registered (agents/evolve-router.md,
//     .evolve/profiles/router.json) yet absent from run.go:120's hand-typed
//     agentRoles slice — that IS the drift the task removes.
//   - 005/006 bound the fix from both sides: 005 pins the five roles that exist
//     ONLY in the dispatch list (inspirer, evaluator, plan-reviewer, memo,
//     tester) so a naive slice→registry swap cannot silently drop dispatch
//     capability; 006 pins that "ship" — registered but NoArtifact, a native
//     host-side phase with no profile — stays REJECTED, so the derivation
//     cannot over-reach and break the existing profile-conformance invariant.
//
// Roots: production source and profiles are read under acsassert.RepoRoot (the
// worktree — where Builder's change lands and is committed), never main.
package cycle1145

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// scoutArtifactLiteral is the quoted form of the scout deliverable filename.
// The QUOTED form is deliberate: prose comments that mention scout-report.md
// are documentation, not a competing declaration, and must not trip the check.
const scoutArtifactLiteral = `"scout-report.md"`

// registryDeclSite is the ONE file allowed to declare the literal — the
// phasecontract registry, which is the SSOT by construction.
const registryDeclSite = "internal/phasecontract/contract_registry.go"

// TestC1145_001_ScoutReportNameDeclaredOnlyInRegistry is the crux predicate for
// artifact-name-ssot-scout-report-backfill: after the backfill, no production
// (non-test) Go file under go/internal may declare the scout artifact filename
// as its own string literal. Every consumer must resolve it from
// phasecontract.For("scout").ArtifactName.
//
// RED today: internal/cyclesimulator, internal/topngate, internal/phases/scout,
// internal/evalgate and internal/router each carry an independent copy.
func TestC1145_001_ScoutReportNameDeclaredOnlyInRegistry(t *testing.T) {
	root := acsassert.RepoRoot(t)
	internalDir := filepath.Join(root, "go", "internal")

	var offenders []string
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(filepath.Join(root, "go"), path)
		if relErr != nil {
			return relErr
		}
		if filepath.ToSlash(rel) == registryDeclSite {
			return nil // the SSOT declaration itself
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(body), scoutArtifactLiteral) {
			offenders = append(offenders, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", internalDir, err)
	}

	if len(offenders) > 0 {
		t.Errorf("the scout artifact filename %s is independently declared in %d production file(s) outside the phasecontract SSOT: %s — route each through phasecontract.For(\"scout\").ArtifactName",
			scoutArtifactLiteral, len(offenders), strings.Join(offenders, ", "))
	}
}

// TestC1145_002_EvalGateResolvesScoutReportByRegistryName is the behavioral
// half: evalgate's materialization gate is one of the literal sites, and it is
// only useful if it actually opens the file the scout phase writes. This
// exercises the exported reviewer (the real system under test, not a grep) in
// both directions:
//
//	positive — report named phasecontract.For("scout").ArtifactName → the gate
//	           reads it, sees an unmaterialized slug, and BLOCKS at enforce.
//	negative — the same bytes under any other name → the gate finds nothing and
//	           fails open (Approve=true). This is what proves the positive case
//	           is filename-sensitive rather than trivially true.
func TestC1145_002_EvalGateResolvesScoutReportByRegistryName(t *testing.T) {
	contract, ok := phasecontract.For("scout")
	if !ok {
		t.Fatal("phasecontract has no registered contract for phase \"scout\"")
	}

	// The "- **Slug:**" bullet form is what evalgate.SelectedSlugs actually
	// parses out of the "## Selected Tasks" section (the "### Task N:" header
	// carries no slug for this consumer).
	const unmaterialized = "cycle1145-deliberately-unmaterialized-slug"
	report := "# Scout Report\n\n## Selected Tasks\n\n### Task 1: " + unmaterialized +
		"\n- **Slug:** " + unmaterialized + "\n- **Complexity:** S\n"

	review := func(t *testing.T, filename string) core.ReviewResult {
		t.Helper()
		workspace := t.TempDir()
		projectRoot := t.TempDir() // no .evolve/evals/ → nothing is materialized
		if err := os.WriteFile(filepath.Join(workspace, filename), []byte(report), 0o644); err != nil {
			t.Fatalf("writing %s: %v", filename, err)
		}
		return evalgate.NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
			Phase:       string(core.PhaseScout),
			Workspace:   workspace,
			ProjectRoot: projectRoot,
		})
	}

	got := review(t, contract.ArtifactName)
	if got.Approve {
		t.Errorf("evalgate approved a scout phase whose report (%s, the registry ArtifactName) selects unmaterialized slug %q — the gate did not read the registry-named artifact",
			contract.ArtifactName, unmaterialized)
	} else if !strings.Contains(got.Reason, unmaterialized) {
		t.Errorf("evalgate blocked but its reason does not name the offending slug %q: %q", unmaterialized, got.Reason)
	}

	// Negative / sensitivity control: any other filename must NOT be picked up.
	if other := review(t, "scout-report-cycle1145-not-the-contract.md"); !other.Approve {
		t.Errorf("evalgate blocked on a report that is NOT at the contracted artifact name (reason=%q) — the gate is matching something other than the registry filename",
			other.Reason)
	}
}

// TestC1145_003_ScoutContractDeclaresRuntimeTruthName is the anti-gaming twin
// of 001. The cheapest way to green an absence check is to delete or rename the
// thing being counted, so pin that the surviving declaration is still the
// registry's and still carries the runtime-truth value the pipeline writes.
func TestC1145_003_ScoutContractDeclaresRuntimeTruthName(t *testing.T) {
	contract, ok := phasecontract.For("scout")
	if !ok {
		t.Fatal("phasecontract has no registered contract for phase \"scout\"")
	}
	if contract.ArtifactName != "scout-report.md" {
		t.Errorf("phasecontract.For(\"scout\").ArtifactName = %q, want \"scout-report.md\" (the filename the scout phase actually writes)", contract.ArtifactName)
	}

	var found bool
	for _, a := range phasecontract.RequiredArtifacts() {
		if a == contract.ArtifactName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("phasecontract.RequiredArtifacts() = %v does not include the scout artifact %q — the completeness half of the SSOT was dropped",
			phasecontract.RequiredArtifacts(), contract.ArtifactName)
	}
}

// rejectedAsUnknownAgent runs the real dispatch entry point with a deliberately
// invalid cycle and reports whether the role was rejected by the allow-list.
//
// Role validation is the FIRST check in DispatchParallel, so the two outcomes
// are cleanly separable without touching the filesystem or spawning anything:
// an allowed role falls through to the cycle check ("cycle must be >= 0"),
// while a disallowed one short-circuits with "unknown agent".
func rejectedAsUnknownAgent(t *testing.T, role string) bool {
	t.Helper()
	_, err := subagent.DispatchParallel(context.Background(), subagent.DispatchParallelRequest{
		Agent: role,
		Cycle: -1, // invalid on purpose: the next check after the allow-list
	}, subagent.DispatchParallelOptions{})
	if err == nil {
		t.Fatalf("DispatchParallel(%q, cycle=-1) returned no error; expected at least the cycle-range rejection", role)
	}
	return strings.Contains(err.Error(), "unknown agent")
}

// TestC1145_004_DispatchAllowListCoversEveryRegisteredAgent is the crux
// predicate for required-roles-ssot-subagent-dispatch: every agent the
// phasecontract registry declares as a real LLM deliverable producer must be
// dispatchable. Once agentRoles is derived from the registry this holds by
// construction; while it is a second hand-typed slice it can drift.
//
// RED today: "router" is registered in phasecontract (routing-plan.json,
// profile .evolve/profiles/router.json) but missing from run.go:120.
func TestC1145_004_DispatchAllowListCoversEveryRegisteredAgent(t *testing.T) {
	var missing []string
	for _, c := range phasecontract.Contracts() {
		if c.NoArtifact || c.AgentName == "" {
			continue // native host-side phase (ship) — not an LLM dispatch target
		}
		if rejectedAsUnknownAgent(t, c.AgentName) {
			missing = append(missing, c.Phase+"→"+c.AgentName)
		}
	}
	if len(missing) > 0 {
		t.Errorf("subagent dispatch rejects %d agent(s) the phasecontract registry declares: %s — derive the allow-list from the registry instead of re-typing it",
			len(missing), strings.Join(missing, ", "))
	}
}

// TestC1145_005_DispatchAllowListRetainsNonRegistryRoles guards the lossy half
// of the refactor. These five roles are dispatchable but have no phasecontract
// entry (they are not spine phases with report contracts), so replacing the
// slice with a bare registry projection would silently delete real dispatch
// capability. The derivation must be a UNION, not a substitution.
func TestC1145_005_DispatchAllowListRetainsNonRegistryRoles(t *testing.T) {
	// Every one of these ships a .evolve/profiles/<role>.json today.
	nonRegistry := []string{"inspirer", "evaluator", "plan-reviewer", "memo", "tester"}
	for _, role := range nonRegistry {
		if rejectedAsUnknownAgent(t, role) {
			t.Errorf("dispatch role %q is no longer accepted — it has a profile and no phasecontract entry, so the registry-derived allow-list dropped it", role)
		}
	}
}

// TestC1145_006_DispatchRejectsNonDispatchableAndUnknownRoles bounds the fix
// from the other side. "ship" is IN the registry but is a native host-side
// phase (NoArtifact, no .evolve/profiles/ship.json), so a derivation that
// blindly takes every registry AgentName would both accept an undispatchable
// role and break the existing profile-conformance invariant
// (TestAgentRoles_EveryRoleHasProfile). Case variants and junk must keep being
// rejected too — the allow-list stays exact-match.
func TestC1145_006_DispatchRejectsNonDispatchableAndUnknownRoles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	shipProfile := filepath.Join(root, ".evolve", "profiles", "ship.json")
	if _, err := os.Stat(shipProfile); err == nil {
		t.Fatalf("premise broken: %s now exists, so \"ship\" may legitimately be dispatchable — revisit this predicate", shipProfile)
	}

	for _, role := range []string{"ship", "Scout", "not-a-real-role", ""} {
		if !rejectedAsUnknownAgent(t, role) {
			t.Errorf("subagent dispatch accepted role %q — the allow-list over-reached beyond profile-backed, dispatchable roles", role)
		}
	}
}
