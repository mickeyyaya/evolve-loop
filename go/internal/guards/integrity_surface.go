package guards

import (
	"path/filepath"
	"strings"
)

// ProtectedSurfaceEntry is one row of ProtectedSurfaceManifest: a
// slash-normalized, lower-case path fragment plus the rationale for why the
// surface it names is pipeline control plane. Fragment is matched ANYWHERE in
// the slash-normalized, case-folded path (see IsProtectedSurface), so a
// directory entry keeps its trailing slash and a file entry names the full
// filename.
type ProtectedSurfaceEntry struct {
	Fragment  string // "/dir/" for a directory subtree, "/dir/file.ext" for a single file
	Rationale string // why this surface grades/gates a cycle
}

// ProtectedSurfaceManifest is the SINGLE SOURCE OF TRUTH for the pipeline
// INTEGRITY CONTROL PLANE: the deterministic gates that grade a cycle, the
// campaign metric SSOT, the guards themselves, the campaign contract, the
// grading rubrics, and the PreToolUse hook wiring.
//
// No autonomous phase agent may modify these — a cycle must never be able to
// edit the gate that judges it. This is the structural fix for the cycle-20
// breach, where the build agent edited
// go/acs/regression/flagreaders/readers_test.go (and the registry SSOT) to bless
// its own split-const dodge, and the audit approved the self-edit.
//
// Legitimate control-plane changes (e.g. an operator hardening a gate) go through
// human-gated `evolve ship --class manual` OUTSIDE any cycle, where the role
// guard's CycleID==0 path allows them — never an autonomous `--class cycle`.
//
// Fragments are matched anywhere in the slash-normalized path, so the boundary
// holds regardless of the file's physical root (a per-cycle worktree, the branch
// root, or main). The intentionally NARROW fragments (e.g. go/acs/regression/ but
// NOT go/acs/, registry_table.go but NOT the whole flagregistry package) preserve
// legitimate cycle behavior: a cycle still writes its own go/acs/cycleN/
// predicates and ordinary source.
//
// The manifest is deliberately a COMPILED Go value, not config: the boundary
// must not be config-softenable (a .evolve/policy.json knob here would let a
// cycle write the knob that disarms the guard). Its durable tripwire is
// go/acs/regression/protectedsurface, which walks the repo for gate-shaped Go
// files and RED-fails when one is not covered here — so the perimeter cannot
// silently rot as the trust kernel grows (L4, architecture review 2026-07-16).
var ProtectedSurfaceManifest = []ProtectedSurfaceEntry{
	{Fragment: "/go/acs/regression/", Rationale: "standing deterministic gates"},
	{Fragment: "/go/internal/acssuite/", Rationale: "the gate runner"},
	{Fragment: "/go/internal/phases/audit/probe_quarantine", Rationale: "the gate runner's input filter — decides which files the EGPS suite is allowed to see"},
	{Fragment: "/go/internal/guards/", Rationale: "the guards (incl. this manifest + role.go)"},
	{Fragment: "/go/internal/flagregistry/registry_table.go", Rationale: "the campaign metric SSOT"},
	{Fragment: "/go/internal/flagregistry/registry_ceiling_test.go", Rationale: "the ceiling ratchet gate"},
	{Fragment: "/knowledge-base/research/flag-campaign-plan.json", Rationale: "the campaign contract"},
	{Fragment: "/skills/audit/", Rationale: "the audit grading rubric"},
	{Fragment: "/skills/adversarial-testing/", Rationale: "the adversarial anti-gaming rubric (M4 goal-integrity)"},
	{Fragment: "/skills/fable/", Rationale: "the operating-discipline overlay preloaded into deep/top-tier phase agents (policy.ResolveOverlays → bridge skill-overlay injection); its SKILL.md persona is integrity-load-bearing once injected into every phase prompt (audit-F1)"},
	{Fragment: "/.claude/settings.json", Rationale: "PreToolUse hook wiring (repo + global ~/.claude)"},
	{Fragment: "/.evolve/policy.json", Rationale: "gate-default overrides (eval/contract/swarm gates)"},

	// L4 additions (architecture review 2026-07-16): gate-shaped surfaces the
	// pre-L4 list left silently in-cycle-writable. Dir entries where every file
	// in the package is control plane; file entries where the package (core,
	// phases/ship) is otherwise legitimate cycle territory.
	{Fragment: "/go/internal/commitgate/", Rationale: "the pre-commit quality gate (attestation writer the manual-ship reader trusts)"},
	{Fragment: "/go/internal/phaseintegrity/", Rationale: "the per-phase integrity chain's DigestSource (ADR-0065)"},
	{Fragment: "/go/internal/binaryguard/", Rationale: "the compiled-binary commit backstop at the commit chokepoint"},
	{Fragment: "/go/internal/guardslog/", Rationale: "the shared guards-log writer (the guards' evidence trail)"},
	{Fragment: "/go/internal/cli/guardcmd/", Rationale: "the CLI wiring of the trust-kernel guard + gate subcommands"},
	{Fragment: "/go/internal/core/workspace_guard.go", Rationale: "workspace-pollution guard (archives a polluted workspace)"},
	{Fragment: "/go/internal/core/activating_fields_guard_test.go", Rationale: "ADR-0058 registry trust anchor (drift-loud activating-fields guard)"},
	{Fragment: "/go/internal/core/orchestrator_guard_test.go", Rationale: "inserted-phase tree-diff guard pin (cycle-274 G)"},
	{Fragment: "/go/internal/core/phase_advisor_guard_test.go", Rationale: "advisor routing guard pin"},
	{Fragment: "/go/internal/core/ledger_runid_writers_guard_test.go", Rationale: "the agent_subprocess run_id writer guard: closes the writer set and requires each writer to call core.RunIDFromWorkspace. Since #503 a run-scoped binding REFUSES an entry with no run_id, so a cycle that quietly delisted a writer here would make everything that writer emits unbindable and hard-stop ship at AUDIT_BINDING_NO_AUDITOR (cycle-1571 H1)"},
	{Fragment: "/go/internal/phases/ship/binary_staging_guard.go", Rationale: "staging-time backstop against compiled-binary commits (tracked-binary-in-acs-dir class fix)"},
	{Fragment: "/go/internal/phases/ship/binary_staging_guard_test.go", Rationale: "the staging backstop's own regression pin"},
	{Fragment: "/go/internal/bridge/driver_tmux_submitverify_guard_test.go", Rationale: "submit-verify's own keep-guard: pins that every real tmux driver declares an input-line marker (a family that omits it goes inert) and that a failed verification stays loud — a cycle editing this could re-open the cycles 1505/1510/1517 unsubmitted-prompt stall unobserved"},
	// ADR-0074 S2 (operator-enrolled 2026-07-22, cycle-1034 salvage): the
	// disposition gate grades the retro that grades failed cycles — a cycle
	// able to edit it could route its own failure classification.
	{Fragment: "/go/internal/core/disposition_gate.go", Rationale: "ADR-0074 disposition-contract gate (retro's verdict-on-the-verdict verifier + digest cross-check)"},
	{Fragment: "/go/internal/core/disposition_gate_test.go", Rationale: "the disposition gate's own regression pins"},

	// Build explanation documentation is a host-activated trust boundary:
	// Builder produces it, then Audit, Ship, and Retro independently verify the
	// host-sealed provenance.
	// A cycle that could edit any of these narrow surfaces could approve its own
	// explanation or downgrade the active contract.
	{Fragment: "/go/internal/explanationdocs/", Rationale: "Build explanation contract, host snapshots, and deterministic verifier"},
	{Fragment: "/go/internal/reportdoc/", Rationale: "strict visible-Markdown parser shared by explanation review gates"},
	{Fragment: "/go/internal/core/build_explanation_handoff.go", Rationale: "orchestrator activation and post-review sealing lifecycle"},
	{Fragment: "/go/internal/core/build_floor_reviewer.go", Rationale: "mandatory deterministic Build explanation floor"},
	{Fragment: "/go/internal/core/build_explanation_floor_test.go", Rationale: "composition tripwire proving optional reviewers cannot replace the explanation floor"},
	{Fragment: "/go/internal/core/reviewer.go", Rationale: "mandatory explanation-review chain composition"},
	{Fragment: "/go/internal/core/orchestrator.go", Rationale: "fresh-cycle explanation activation and Build-context sealing call sites"},
	{Fragment: "/go/internal/core/cyclerun.go", Rationale: "fresh-cycle explanation contract-version stamp"},
	{Fragment: "/go/internal/core/cyclerun_dispatch.go", Rationale: "fresh-cycle downstream explanation projection call site"},
	{Fragment: "/go/internal/core/cyclerun_review.go", Rationale: "fresh-cycle post-Build explanation refresh call site"},
	{Fragment: "/go/internal/core/cyclerun_remediate.go", Rationale: "Build explanation correction and remediation projection"},
	{Fragment: "/go/internal/core/continuation_stamp.go", Rationale: "continuation explanation-history ownership transition"},
	{Fragment: "/go/internal/core/evaluate_batch.go", Rationale: "parallel evaluator explanation handoff projection"},
	{Fragment: "/go/internal/core/failure_learning.go", Rationale: "failed-cycle explanation handoff projection"},
	{Fragment: "/go/internal/core/ship_recovery.go", Rationale: "rebase recovery must invalidate stale explanation and route through Build"},
	{Fragment: "/go/internal/core/resume.go", Rationale: "resume activation, sealing, projection, and post-Build refresh call sites"},
	{Fragment: "/go/internal/core/ports.go", Rationale: "typed Bridge request sandbox requirement"},
	{Fragment: "/go/internal/core/phase.go", Rationale: "typed phase explanation handoff and contract-version fields"},
	{Fragment: "/go/internal/cyclestate/state.go", Rationale: "durable explanation contract version and Build binding state"},
	{Fragment: "/go/internal/phaseio/handoffs.go", Rationale: "typed cross-phase explanation handoff schema"},
	{Fragment: "/go/internal/phases/runner/runner.go", Rationale: "mandatory versioned-Build sandbox propagation call site"},
	{Fragment: "/go/internal/bridge/", Rationale: "Bridge registry, drivers, and OS sandbox fail-closed enforcement for versioned Build"},
	{Fragment: "/go/internal/adapters/bridge/", Rationale: "Bridge request adapter preserving mandatory Build sandbox propagation"},
	{Fragment: "/go/internal/adapters/sandbox/", Rationale: "OS-specific confinement policy and generated write boundary"},
	{Fragment: "/go/internal/looppreflight/checks.go", Rationale: "pre-spend required-sandbox readiness HALT"},
	{Fragment: "/go/internal/looppreflight/drivers.go", Rationale: "sandbox-enabled profile discovery used by readiness HALT"},
	{Fragment: "/go/internal/preflight/preflight.go", Rationale: "measured host sandbox capability used by readiness HALT"},
	{Fragment: "/go/internal/phases/audit/audit.go", Rationale: "Auditor explanation-review gate call site"},
	{Fragment: "/go/internal/phases/retro/retro.go", Rationale: "Retrospective explanation-review gate call site"},
	{Fragment: "/go/internal/phases/ship/native.go", Rationale: "native Ship explanation re-verification call site"},
	{Fragment: "/go/internal/phases/ship/ship.go", Rationale: "Ship phase typed explanation binding propagation"},
	{Fragment: "/go/internal/phases/ship/gitops.go", Rationale: "verified worktree selection, mutation, and immutable ship-binding writer"},
	{Fragment: "/go/internal/phases/audit/explanation_review_gate.go", Rationale: "Auditor qualitative explanation-review gate"},
	{Fragment: "/go/internal/phases/retro/explanation_review_gate.go", Rationale: "Retrospective explanation-review and correction-todo gate"},
	{Fragment: "/go/internal/phases/ship/native_explanation_gate.go", Rationale: "canonical native Ship explanation re-verification gate"},
	{Fragment: "/agents/evolve-builder.md", Rationale: "Builder authority and explanation deliverable instructions"},
	{Fragment: "/agents/evolve-builder-reference.md", Rationale: "Builder explanation document schema"},
	{Fragment: "/agents/evolve-auditor.md", Rationale: "Auditor explanation-review authority"},
	{Fragment: "/agents/evolve-auditor-reference.md", Rationale: "Auditor explanation-review output schema"},
	{Fragment: "/agents/evolve-retrospective.md", Rationale: "Retro explanation verification and correction contract"},
	{Fragment: "/agents/evolve-memo.md", Rationale: "Memo explanation handoff rendering contract"},
	{Fragment: "/schemas/handoff/build-report.schema.json", Rationale: "Builder explanation report section activation schema"},
	{Fragment: "/schemas/handoff/audit-report.schema.json", Rationale: "Auditor explanation review section activation schema"},
	{Fragment: "/schemas/handoff/retrospective-report.schema.json", Rationale: "Retrospective explanation review section activation schema"},
	{Fragment: "/.evolve/profiles/builder.json", Rationale: "Builder write boundary for explanation artifacts"},
	{Fragment: "/.evolve/profiles/auditor.json", Rationale: "Auditor read-only boundary for Builder explanation artifacts"},
	{Fragment: "/.evolve/build-explanation-contracts/", Rationale: "host-owned activation and Build result snapshots"},
}

// IsProtectedSurface reports whether path targets the pipeline integrity control
// plane. path may be absolute or repo-relative; matching is on a slash-normalized
// fragment, so the boundary holds regardless of the file's physical root.
func IsProtectedSurface(path string) bool {
	if path == "" {
		return false
	}
	p := filepath.ToSlash(path)
	// Ensure a leading slash so a leading segment ("go/acs/regression/...") still
	// matches a "/go/acs/regression/" fragment.
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// Case-fold: macOS/Windows filesystems are case-insensitive, so "Go/ACS/..."
	// is the same path as "go/acs/..."; fragments are already lower-case.
	p = strings.ToLower(p)
	for _, e := range ProtectedSurfaceManifest {
		if strings.Contains(p, e.Fragment) {
			return true
		}
	}
	return false
}
