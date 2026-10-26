package guards

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var protectedGateFiles = []struct {
	path  string
	class GuaranteeClass
}{
	{"go/internal/flagregistry/registry_table.go", AuditVerdictGuarantee},
	{"go/internal/flagregistry/registry_ceiling_test.go", AuditVerdictGuarantee},
	{"knowledge-base/research/flag-campaign-plan.json", AuditVerdictGuarantee},
	{".claude/settings.json", GuardGuarantee},
	{".evolve/policy.json", PolicyGuarantee},
	{"go/internal/core/workspace_guard.go", GuardGuarantee},
	{"go/internal/core/activating_fields_guard_test.go", GuardGuarantee},
	{"go/internal/core/orchestrator_guard_test.go", GuardGuarantee},
	{"go/internal/core/phase_advisor_guard_test.go", GuardGuarantee},
	{"go/internal/core/flow_span_guard_test.go", GuardGuarantee},
	{"go/internal/core/ledger_runid_writers_guard_test.go", GuardGuarantee},
	{"go/internal/phases/ship/binary_staging_guard.go", GuardGuarantee},
	{"go/internal/phases/ship/binary_staging_guard_test.go", GuardGuarantee},
	{"go/internal/phases/ship/committed_doc_guard.go", GuardGuarantee},
	{"go/internal/phases/ship/committed_doc_guard_test.go", GuardGuarantee},
	{"go/internal/phases/ship/foreign_predicate_guard.go", GuardGuarantee},
	{"go/internal/phases/ship/foreign_predicate_guard_test.go", GuardGuarantee},
	{"go/internal/bridge/driver_tmux_submitverify_guard_test.go", GuardGuarantee},
	{"go/internal/bridge/integration_wait_guard_test.go", GuardGuarantee},
	{"go/internal/sysexec/command_guard_test.go", GuardGuarantee},
	{"go/internal/sysexec/llm_launch_guard_test.go", GuardGuarantee},
	{"go/internal/orchestrator/guard_test.go", GuardGuarantee},
	{"go/test/structure/nopoll_guard_test.go", GuardGuarantee},
	{"go/internal/core/disposition_gate.go", GateGuarantee},
	{"go/internal/core/disposition_gate_test.go", GateGuarantee},
	{"go/internal/core/build_explanation_handoff.go", GateGuarantee},
	{"go/internal/core/build_floor_reviewer.go", GateGuarantee},
	{"go/internal/core/floor_lists_base.go", GateGuarantee},
	{"go/internal/core/floor_lists_base_test.go", GateGuarantee},
	{"go/internal/core/floor_lists_regression.go", GateGuarantee},
	{"go/internal/core/floor_lists_regression_test.go", GateGuarantee},
	{"go/internal/core/cover_changed_floor.go", GateGuarantee},
	{"go/internal/profiles/gate_inputs_realtree_test.go", GuardGuarantee},
	{"go/internal/core/ste_floor.go", GateGuarantee},
	{"go/internal/stelint/added.go", GateGuarantee},
	{"go/internal/core/build_explanation_floor_test.go", GateGuarantee},
	{"go/internal/core/reviewer.go", GateGuarantee},
	{"go/internal/core/orchestrator.go", AuditVerdictGuarantee},
	{"go/internal/core/cyclerun.go", AuditVerdictGuarantee},
	{"go/internal/core/cyclerun_dispatch.go", AuditVerdictGuarantee},
	{"go/internal/core/cyclerun_review.go", AuditVerdictGuarantee},
	{"go/internal/core/cyclerun_postreview.go", AuditVerdictGuarantee},
	{"go/internal/core/leak_recovery.go", GateGuarantee},
	{"go/internal/core/leak_ownership.go", GateGuarantee},
	{"go/internal/core/cyclerun_remediate.go", AuditVerdictGuarantee},
	{"go/internal/core/continuation_ancestor_predicates.go", AuditVerdictGuarantee},
	{"go/internal/core/evaluate_batch.go", AuditVerdictGuarantee},
	{"go/internal/core/failure_learning.go", AuditVerdictGuarantee},
	{"go/internal/core/errors.go", GateGuarantee},
	{"go/internal/core/failure_diag.go", GateGuarantee},
	{"go/cmd/evolve/cmd_cycle_config.go", GateGuarantee},
	{"go/internal/phases/audit/defect_ledger.go", AuditVerdictGuarantee},
	{"go/internal/phases/audit/ciparity.go", GateGuarantee},
	{"go/internal/subagent/run.go", LedgerGuarantee},
	{"go/internal/core/ship_recovery.go", GateGuarantee},
	{"go/internal/core/resume_execution.go", AuditVerdictGuarantee},
	{"go/internal/core/resume_bootstrap.go", AuditVerdictGuarantee},
	{"go/internal/core/ports.go", SandboxGuarantee},
	{"go/internal/core/bridge_completion.go", GateGuarantee},
	{"go/internal/phases/runner/runner.go", SandboxGuarantee},
	{"go/internal/phases/runner/dispatch.go", SandboxGuarantee},
	{"go/internal/phases/runner/verdict_engine.go", AuditVerdictGuarantee},
	{"go/cmd/evolve/dispatch_gate.go", DispatchCredentialGuarantee},
	{"go/cmd/evolve/dispatch_gate_test.go", DispatchCredentialGuarantee},
	{"go/internal/looppreflight/checks.go", SandboxGuarantee},
	{"go/internal/looppreflight/drivers.go", SandboxGuarantee},
	{"go/internal/preflight/preflight.go", SandboxGuarantee},
	{"go/internal/phases/audit/audit.go", AuditVerdictGuarantee},
	{"go/internal/phases/audit/classification.go", AuditVerdictGuarantee},
	{"go/internal/phases/retro/retro.go", AuditVerdictGuarantee},
	{"go/internal/phases/ship/native.go", GateGuarantee},
	{"go/internal/phases/ship/ship.go", GateGuarantee},
	{"go/internal/phases/ship/gitops.go", GateGuarantee},
	{"go/internal/phases/ship/gitops_landing.go", GateGuarantee},
	{"go/internal/phases/ship/landing_intent.go", GateGuarantee},
	{"go/internal/phases/audit/explanation_review_gate.go", AuditVerdictGuarantee},
	{"go/internal/phases/audit/solution_gate.go", GateGuarantee},
	{"go/internal/core/solution_floor.go", GateGuarantee},
	{"go/internal/phases/retro/explanation_review_gate.go", AuditVerdictGuarantee},
	{"go/internal/phases/ship/native_explanation_gate.go", GateGuarantee},
	{"agents/evolve-builder.md", InstructionGuarantee},
	{"agents/evolve-builder-reference.md", InstructionGuarantee},
	{"agents/evolve-auditor.md", AuditVerdictGuarantee},
	{"agents/evolve-auditor-reference.md", AuditVerdictGuarantee},
	{"agents/evolve-retrospective.md", InstructionGuarantee},
	{"agents/evolve-memo.md", InstructionGuarantee},
	{"schemas/handoff/build-report.schema.json", InstructionGuarantee},
	{"schemas/handoff/audit-report.schema.json", AuditVerdictGuarantee},
	{"schemas/handoff/retrospective-report.schema.json", InstructionGuarantee},
	{".evolve/profiles/builder.json", SandboxGuarantee},
	{".evolve/profiles/auditor.json", SandboxGuarantee},
	{"go/internal/core/build_handoff_floor.go", GateGuarantee},
	{"go/internal/core/phase_bindings_selfcheck.go", GateGuarantee},
	{"go/internal/core/blocker_proof.go", GateGuarantee},
	{"go/internal/core/build_blocker_exit.go", GateGuarantee},
	{"go/internal/core/floor_blocker_notes.go", GateGuarantee},
	{"go/internal/addedtests/budget.go", GateGuarantee},
	{"go/internal/inboxrank/weightsort_ban_test.go", GuardGuarantee},
	{"agents/evolve-code-reviewer.md", AuditVerdictGuarantee},
	{".evolve/profiles/code-reviewer.json", SandboxGuarantee},
	{"go/internal/router/code_review_pin_test.go", GuardGuarantee},
	{"go/internal/guards/integrity_surface.go", ManifestGuarantee},
	{"go/internal/bridge/sandbox_paths.go", SandboxGuarantee},
	{"go/internal/bridge/sandbox_wrap.go", SandboxGuarantee},
	{"go/internal/bridge/sandbox_terminal.go", SandboxGuarantee},
	{"go/internal/bridge/read_isolation.go", SandboxGuarantee},
	{"go/internal/bridge/protected_write.go", SandboxGuarantee},
	{"go/internal/bridge/no_nested_agency.go", SandboxGuarantee},
	{"go/internal/bridge/dispatch_credential.go", DispatchCredentialGuarantee},
	{"go/internal/bridge/hermetic_env.go", SandboxGuarantee},
	{"go/internal/bridge/context_access.go", SandboxGuarantee},
	{"go/internal/bridge/launch.go", SandboxGuarantee},
	{"go/internal/bridge/launch_llm.go", SandboxGuarantee},
	{"go/internal/bridge/driver_tmux_boot.go", SandboxGuarantee},
	{"go/internal/policy/core.go", PolicyGuarantee},
	{"go/internal/policy/strict_keys.go", PolicyGuarantee},
	{"go/internal/core/statemachine.go", PhaseOrderGuarantee},
	{"go/internal/bridge/dispatch_signal_guard_test.go", GuardGuarantee},
	{"go/internal/bridge/resolve_guard_test.go", GuardGuarantee},
	{"go/internal/bridge/selfmatch_guard_test.go", GuardGuarantee},
	{"go/internal/bridge/selfmatch_guard_prefilter_test.go", GuardGuarantee},
	{"go/internal/bridge/tmux_own_server_guard_test.go", GuardGuarantee},
	{"go/internal/bridge/driver_claudep.go", SandboxGuarantee},
	{"go/internal/bridge/driver_claudetmux.go", SandboxGuarantee},
	{"go/internal/bridge/driver_codextmux.go", SandboxGuarantee},
	{"go/internal/bridge/driver_tmux_prepare.go", SandboxGuarantee},
	{"go/internal/bridge/manifest.go", SandboxGuarantee},
	{"go/internal/bridge/dispatch_reap.go", GuardGuarantee},
	{"go/internal/phaseobserver/phaseobserver.go", GuardGuarantee},
	{"go/internal/inboxbatch/consoleroute.go", GuardGuarantee},
	{"go/internal/inboxmover/inboxmover.go", GuardGuarantee},
	{"go/internal/inboxmover/lifecycle/lifecycle.go", GuardGuarantee},
	{"go/internal/inboxmover/lifecycle/claim.go", GuardGuarantee},
	{"go/internal/inboxmover/lifecycle/route.go", GuardGuarantee},
	{"go/internal/inboxmover/lifecycle/edit.go", GuardGuarantee},
}

var freedByTheNarrowFence = []string{
	"go/internal/bridge/engine.go",
	"go/internal/bridge/driver_codex.go",
	"go/internal/bridge/autorespond.go",
	"go/internal/bridge/channel/supervisor.go",
	"go/internal/bridge/launchoutcome/classify.go",
	"go/internal/core/advisor/advisor.go",
	"go/internal/core/phase_advisor.go",
	"go/internal/core/carryover/carryover.go",
	"go/internal/core/continuation_stamp.go",
	"go/internal/core/resume.go",
	"go/internal/core/phase.go",
	"go/internal/inboxmover/lifecycle/absorb.go",
	"go/internal/inboxmover/lifecycle/file.go",
	"go/internal/loopwave/admission.go",
	"go/internal/loopchain/loopchain.go",
	"go/internal/config/config.go",
	"go/internal/observerengine/observerengine.go",
	"go/internal/textcap/textcap.go",
}

func TestProtectedGateFiles_EachKeepsItsEntryAndItsClass(t *testing.T) {
	for _, f := range protectedGateFiles {
		if !IsProtectedSurface(f.path) {
			t.Errorf("gate file %s lost its protected-surface entry; it enforces a %s guarantee (ADR-0157)", f.path, f.class)
		}
		if got := classOf(f.path); got != f.class {
			t.Errorf("gate file %s has class %q, want %q", f.path, got, f.class)
		}
	}
}

var absentGateFileOnPurpose = map[string]string{
	"knowledge-base/research/flag-campaign-plan.json": "the pre-refactor fragment list keeps this entry, and the file is outside the tree",
}

func TestProtectedGateFiles_EachExistsInTheTree(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	for _, f := range protectedGateFiles {
		if absentGateFileOnPurpose[f.path] != "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f.path))); err != nil {
			t.Errorf("protected gate file %s is absent (%v); a stale entry protects nothing, so remove it or fix the path", f.path, err)
		}
	}
}

func TestGuaranteeClasses_EveryManifestEntryNamesOneOfTheTenClasses(t *testing.T) {
	classes := GuaranteeClasses()
	if len(classes) != 10 {
		t.Fatalf("GuaranteeClasses() has %d classes, want the 10 classes of ADR-0157", len(classes))
	}
	for _, e := range ProtectedSurfaceManifest {
		if !slices.Contains(classes, e.Class) {
			t.Errorf("entry %s has class %q, which is not a guarantee class of ADR-0157", e.Fragment, e.Class)
		}
	}
}

func TestNarrowFence_FreedFilesAreLaneWork(t *testing.T) {
	for _, p := range freedByTheNarrowFence {
		if IsProtectedScope(p) {
			t.Errorf("%s is protected again; ADR-0157 frees it because it holds no gate, so a new fence needs a new decision", p)
		}
	}
}

func classOf(path string) GuaranteeClass {
	p, _ := normalizeSurfacePath(path)
	best := ProtectedSurfaceEntry{}
	for _, e := range ProtectedSurfaceManifest {
		if strings.Contains(p+"/", e.Fragment) && len(e.Fragment) > len(best.Fragment) {
			best = e
		}
	}
	return best.Class
}
