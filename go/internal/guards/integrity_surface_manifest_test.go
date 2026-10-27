package guards

import (
	"strings"
	"testing"
)

var preRefactorFragments = []string{
	"/go/acs/regression/",
	"/go/internal/acssuite/",
	"/go/internal/guards/",
	"/go/internal/flagregistry/registry_table.go",
	"/go/internal/flagregistry/registry_ceiling_test.go",
	"/knowledge-base/research/flag-campaign-plan.json",
	"/skills/audit/",
	"/skills/adversarial-testing/",
	"/.claude/settings.json",
	"/.evolve/policy.json",
}

func TestProtectedSurfaceManifest_KeepsEveryPreRefactorFragment(t *testing.T) {
	var manifest []ProtectedSurfaceEntry = ProtectedSurfaceManifest
	got := make(map[string]bool, len(manifest))
	for _, e := range manifest {
		got[e.Fragment] = true
	}
	for _, frag := range preRefactorFragments {
		if !got[frag] {
			t.Errorf("pre-refactor fragment %q missing from ProtectedSurfaceManifest — "+
				"the SSOT refactor must be behavior-preserving (removals/edits of "+
				"pre-L4 entries are a regression)", frag)
		}
	}
}

func TestProtectedSurfaceManifest_EveryEntryDenies(t *testing.T) {
	if len(ProtectedSurfaceManifest) == 0 {
		t.Fatal("ProtectedSurfaceManifest is empty — the control plane has no boundary")
	}
	for _, e := range ProtectedSurfaceManifest {
		t.Run(e.Fragment, func(t *testing.T) {
			if e.Fragment != strings.ToLower(e.Fragment) {
				t.Errorf("fragment %q is not lower-case; IsProtectedSurface folds the PATH "+
					"to lower, so an upper-case fragment can never match", e.Fragment)
			}
			if strings.Contains(e.Fragment, "\\") {
				t.Errorf("fragment %q contains a backslash; fragments are slash-normalized", e.Fragment)
			}
			if !strings.HasPrefix(e.Fragment, "/") {
				t.Errorf("fragment %q lacks a leading slash; anchored dir/file fragments "+
					"start with / so 'go/acs' cannot match 'lego/acs'", e.Fragment)
			}
			if e.Rationale == "" {
				t.Errorf("fragment %q has no Rationale; every manifest entry must document "+
					"why its surface grades/gates a cycle", e.Fragment)
			}
			sample := e.Fragment
			if strings.HasSuffix(sample, "/") {
				sample += "x.go"
			}
			abs := "/users/x/.evolve/worktrees/cycle-9" + sample
			if !IsProtectedSurface(abs) {
				t.Errorf("IsProtectedSurface(%q) = false, want true (worktree-absolute form "+
					"of manifest entry %q must deny)", abs, e.Fragment)
			}
			rel := strings.TrimPrefix(sample, "/")
			if !IsProtectedSurface(rel) {
				t.Errorf("IsProtectedSurface(%q) = false, want true (repo-relative form of "+
					"manifest entry %q must deny)", rel, e.Fragment)
			}
		})
	}
}

func TestProtectedSurfaceManifest_NonProtectedPathsStillAllow(t *testing.T) {
	allowed := []string{
		"/wt/go/internal/core/observer.go",
		"/wt/go/internal/phases/ship/output.go",
		"/wt/go/acs/cycle21/predicates_test.go",
		"/wt/go/internal/flagregistry/registry.go",
		"/wt/README.md",
	}
	for _, p := range allowed {
		if IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = true, want false (the manifest must stay "+
				"NARROW; over-blocking breaks legitimate cycle writes)", p)
		}
	}
}

func TestProtectedSurfaceManifest_CoversExplanationTrustBoundary(t *testing.T) {
	for _, path := range []string{
		"go/internal/explanationdocs/explanationdocs.go",
		"go/internal/core/build_explanation_handoff.go",
		"go/internal/core/build_floor_reviewer.go",
		"go/internal/core/reviewer.go",
		"go/internal/core/orchestrator.go",
		"go/internal/core/cyclerun.go",
		"go/internal/core/cyclerun_dispatch.go",
		"go/internal/core/cyclerun_review.go",
		"go/internal/core/cyclerun_remediate.go",
		"go/internal/core/evaluate_batch.go",
		"go/internal/core/failure_learning.go",
		"go/internal/core/ship_recovery.go",
		"go/internal/core/ports.go",
		"go/internal/reportdoc/reportdoc.go",
		"go/internal/phases/runner/runner.go",
		"go/internal/bridge/sandbox_wrap.go",
		"go/internal/bridge/driver_tmux_boot.go",
		"go/internal/bridge/launch.go",
		"go/internal/adapters/bridge/bridge.go",
		"go/internal/adapters/sandbox/sandbox.go",
		"go/internal/adapters/sandbox/confinement.go",
		"go/internal/looppreflight/checks.go",
		"go/internal/looppreflight/drivers.go",
		"go/internal/preflight/preflight.go",
		"go/internal/phases/audit/explanation_review_gate.go",
		"go/internal/phases/audit/audit.go",
		"go/internal/phases/retro/explanation_review_gate.go",
		"go/internal/phases/retro/retro.go",
		"go/internal/phases/ship/native_explanation_gate.go",
		"go/internal/phases/ship/native.go",
		"go/internal/phases/ship/ship.go",
		"go/internal/phases/ship/gitops.go",
		"agents/evolve-builder-reference.md",
		"agents/evolve-auditor-reference.md",
		"agents/evolve-retrospective.md",
		"schemas/handoff/build-report.schema.json",
		"schemas/handoff/audit-report.schema.json",
		"schemas/handoff/retrospective-report.schema.json",
		".evolve/profiles/builder.json",
		".evolve/profiles/auditor.json",
		".evolve/build-explanation-contracts/cycle-42.json",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("explanation trust-boundary path %q is not protected", path)
		}
	}
}

func TestProtectedSurfaceManifest_CoversTheCorrectionCompletionPredicate(t *testing.T) {
	const predicate = "go/internal/core/bridge_completion.go"
	for _, p := range []string{predicate, "/users/x/.evolve/worktrees/cycle-9/" + predicate} {
		if !IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = false: the predicate that lets a correction complete on a carried deliverable "+
				"is control plane — a lane that widened it could carry its own stale deliverable past the bridge's baseline guard", p)
		}
	}
}

func TestProtectedSurfaceManifest_CoversTheCommentProofTheCommitGateTrusts(t *testing.T) {
	for _, path := range []string{
		"go/internal/commentaudit/equivalence.go",
		"go/internal/commentaudit/history.go",
		"go/internal/commentaudit/across.go",
		"go/internal/commitgate/comment_only.go",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("IsProtectedSurface(%q) = false: the comment-only proof decides the commit gate's review waiver and its history refusals, so no cycle may edit it", path)
		}
	}
	if IsProtectedSurface("go/cmd/commentaudit/main.go") {
		t.Error("the commentaudit command's thin main stays cycle territory; only the rule package is protected")
	}
}

func TestProtectedSurfaceManifest_CoversTheBuildHandoffFloor(t *testing.T) {
	for _, path := range []string{
		"go/internal/core/build_handoff_floor.go",
		"/wt/go/internal/core/build_handoff_floor.go",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("%q holds the build handoff floor's mandatory half and the review-input projection; a cycle must not be able to empty the floor that grades it", path)
		}
	}
}

func TestProtectedSurfaceManifest_CoversTheMainTreeOwnershipSpecification(t *testing.T) {
	for _, path := range []string{
		"go/internal/core/leak_ownership.go",
		"/wt/go/internal/core/leak_ownership.go",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("%q decides which leaked main-tree paths the tree-diff guard exempts as a live sibling's; a cycle must not be able to widen the exemption that judges it", path)
		}
	}
}

func TestProtectedSurfaceManifest_CoversTheLeakRecoveryRules(t *testing.T) {
	for _, path := range []string{
		"go/internal/core/leak_recovery.go",
		"/wt/go/internal/core/leak_recovery.go",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("%q holds the snapshot retry and abort, the no-checkout exemption, the fail-closed owner rule and quarantine-not-delete; a cycle must not be able to loosen the recovery that judges it", path)
		}
	}
}

func TestProtectedSurfaceManifest_CoversTheFloorsGoTestBudget(t *testing.T) {
	for _, path := range []string{
		"go/internal/core/phase_bindings_selfcheck.go",
		"/wt/go/internal/core/phase_bindings_selfcheck.go",
		"go/internal/addedtests/budget.go",
		"/wt/go/internal/addedtests/budget.go",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("%q sets the deadline the build floor runs a lane's packages under; a lane must not move the budget of the floor that grades it (inst-L1763b: lane 1763 raised it to survive)", path)
		}
	}
	if IsProtectedSurface("go/internal/addedtests/addedtests.go") {
		t.Error("only the budget is protected; the added-test grouping in addedtests.go stays cycle territory")
	}
}

func TestProtectedSurfaceManifest_CoversTheDerivedOutputCatalog(t *testing.T) {
	for _, path := range []string{
		"go/internal/derived/derived.go",
		"go/internal/derived/conflict.go",
		"/wt/go/internal/derived/regen.go",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("IsProtectedSurface(%q) = false: the derived catalog decides which rebase conflicts the protected recovery regenerates, so a cycle must not be able to widen it", path)
		}
	}
	if IsProtectedSurface("go/internal/derivedx/a.go") {
		t.Error("a sibling package that only shares the prefix is not the catalog")
	}
}

func TestProtectedSurfaceManifest_CoversTheBlockerProof(t *testing.T) {
	for _, path := range []string{
		"go/internal/core/blocker_proof.go",
		"go/internal/core/build_blocker_exit.go",
		"/wt/go/internal/core/floor_blocker_notes.go",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("%q decides whether a lane's claimed blocker is real; a lane that edits it can turn its own FAIL into a console route", path)
		}
	}
	if IsProtectedSurface("go/internal/core/blocker_proof_test.go") {
		t.Error("the prover's tests stay cycle territory")
	}
}

func TestProtectedSurfaceManifest_CoversTheDispatchCredential(t *testing.T) {
	for _, path := range []string{
		"go/internal/dispatchcred/dispatchcred.go",
		"/wt/go/internal/dispatchcred/dispatchcred.go",
		"go/cmd/evolve/dispatch_gate.go",
		"go/cmd/evolve/dispatch_gate_test.go",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("%q holds the dispatch credential or its check; a cycle must not be able to let its own cell dispatch an agent", path)
		}
	}
}

func TestProtectedSurfaceManifest_CoversTheIncidentRecordsTheDetectorAndItsStageKey(t *testing.T) {
	for _, path := range []string{
		".evolve/incidents/inc-2026-10-27-0a1b2c3d.json",
		"/users/x/.evolve/worktrees/cycle-9/.evolve/incidents/inc-x.json",
		"go/internal/selfheal/incident.go",
		"go/internal/policy/healing.go",
		"go/cmd/evolve/cmd_selfheal.go",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("IsProtectedSurface(%q) = false: the incident records, the detector with its four close conditions and the healing stage key are T0 (llm-isolation.md §18.2, §21); no agent may write them", path)
		}
	}
}

func TestProtectedSurfaceManifest_CoversTheDiagnosisSchemaAndTheProposeVerb(t *testing.T) {
	for _, path := range []string{
		"go/internal/phasecontract/diagnosis.go",
		"go/cmd/evolve/cmd_selfheal_propose.go",
		".evolve/incidents/proposals/inc-2026-10-27-0a1b2c3d.json",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("IsProtectedSurface(%q) = false: the diagnosis schema with its closed catalog, the propose verb and the proposal records are T0 (llm-isolation.md §18.6, §20.1)", path)
		}
	}
}
