package guards

import (
	"path/filepath"
	"strings"
)

// ProtectedSurfaceEntry is one manifest row: a lower-case, slash-normalized path fragment and why it is control plane.
type ProtectedSurfaceEntry struct {
	Fragment  string // "/dir/" for a directory subtree, "/dir/file.ext" for a single file
	Rationale string
}

// ProtectedSurfaceManifest is the single source of truth for the pipeline control plane no autonomous cycle may modify.
// It is compiled rather than config, so a cycle cannot write the knob that disarms it.
// See ADR-0064.
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
	{Fragment: "/skills/solution-scout/", Rationale: "the document-cycle discovery persona the kernel preloads on its own authority (policy.CompiledDefaultOverlaySkills, ADR-0099 slice 3)"},
	{Fragment: "/skills/solution-build/", Rationale: "the document-cycle build persona the kernel preloads on its own authority (policy.CompiledDefaultOverlaySkills, ADR-0099 slice 3)"},
	{Fragment: "/skills/solution-audit/", Rationale: "the document-cycle audit grading persona the kernel preloads on its own authority — the same class as /skills/audit/ (policy.CompiledDefaultOverlaySkills, ADR-0099 slice 3)"},
	{Fragment: "/.claude/settings.json", Rationale: "PreToolUse hook wiring (repo + global ~/.claude)"},
	{Fragment: "/.evolve/policy.json", Rationale: "gate-default overrides (eval/contract/swarm gates)"},

	// A directory entry when every file in it is control plane; file entries inside ordinary cycle territory.
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
	// A cycle that could edit the disposition gate could route its own failure classification.
	{Fragment: "/go/internal/core/disposition_gate.go", Rationale: "ADR-0074 disposition-contract gate (retro's verdict-on-the-verdict verifier + digest cross-check)"},
	{Fragment: "/go/internal/core/disposition_gate_test.go", Rationale: "the disposition gate's own regression pins"},

	// The Build explanation trust boundary: a cycle that could edit these could approve its own
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
	{Fragment: "/go/internal/core/cyclerun_review.go", Rationale: "post-Build explanation refresh eligibility decision and review-to-guard ordering"},
	{Fragment: "/go/internal/core/cyclerun_postreview.go", Rationale: "fresh-cycle post-Build explanation refresh call site (applyPostReviewGuards -> explanationdocs.RefreshResult); carved out of cyclerun_review.go by #549"},
	{Fragment: "/go/internal/core/cyclerun_remediate.go", Rationale: "Build explanation correction and remediation projection"},
	{Fragment: "/go/internal/core/continuation_stamp.go", Rationale: "continuation explanation-history ownership transition"},
	{Fragment: "/go/internal/core/evaluate_batch.go", Rationale: "parallel evaluator explanation handoff projection"},
	{Fragment: "/go/internal/core/failure_learning.go", Rationale: "failed-cycle explanation handoff projection"},
	{Fragment: "/go/internal/core/errors.go", Rationale: "the Bridge port's error sentinels and the integrity predicates on them (ErrArtifactTimeout, isArtifactTimeout — the timeout-only gate unit 02 injects — IsInfraTeardownError, IsOptionalSkippableError)"},
	{Fragment: "/go/internal/core/failure_diag.go", Rationale: "failed-cycle failure-diagnostic seam: the unit-02 facade and the two injection sites of the timeout-only gate whose body is errors.go (ADR-0103)"},
	{Fragment: "/go/internal/core/failurediag/", Rationale: "the failure-diag sidecar writer and the delivery-failure classifier the failed-cycle handoff projects from (ADR-0103 unit 02)"},
	{Fragment: "/go/internal/core/carryover/", Rationale: "the carryover-todo lifecycle — mint admission, closeout merges, retirements and the state.json persist the failed-cycle handoff projects into (ADR-0103 unit 03)"},
	{Fragment: "/go/internal/core/carryover_lifecycle.go", Rationale: "the unit-03 seam: the lifecycle's one wired construction and the facades the persist callers, ship and the ACS-named tests keep (ADR-0103)"},
	{Fragment: "/go/internal/core/failurelearning/", Rationale: "the failure-learning engine — the failed-approach recorder, the deterministic floor, remediation filing and the recurrence closure the failed-cycle handoff projects from (ADR-0103 unit 03b)"},
	{Fragment: "/go/internal/core/failure_learning_engine.go", Rationale: "the unit-03b seam: the engine's one wired construction, the request projection and the two facades the spine, the floor-verdict producer and the by-name tests keep (ADR-0103)"},
	{Fragment: "/go/internal/config/", Rationale: "config resolution — the routing-config loader, its registry/env/policy dials, validators and the config.warning codes every routing consumer is injected from (ADR-0103 unit 08)"},
	{Fragment: "/go/cmd/evolve/cmd_cycle_config.go", Rationale: "the unit-08 seam: the loader's one wired construction, the policy-stages projection and the two stage forwarders the cycle/loop root keeps (ADR-0103)"},
	{Fragment: "/go/internal/observerengine/", Rationale: "the phase-observer engine — the stream-json tail/decoder, the stall rules, the incident responder and the envelope/report sinks the manual phase-observer subcommand runs (ADR-0103 unit 12)"},
	{Fragment: "/go/internal/phaseobserver/phaseobserver.go", Rationale: "the unit-12 seam: the engine's one wired construction, the Config→Settings projection and the Run facade phasecmd and the by-name tests keep (ADR-0103)"},
	{Fragment: "/go/internal/adapters/observer/core_adapter.go", Rationale: "the live core.Observer adapter: the Signal Center accessor, the layout projection and the two observer fault codes on the auto-spawn path (ADR-0103 unit 12)"},
	{Fragment: "/go/internal/core/defectledger/", Rationale: "the defect ledger — the anti-laundering schema, writer and readers, and the continuation disposition gate the audit phase, carryover and the adoption seeder project from (ADR-0103 unit 09)"},
	{Fragment: "/go/internal/phases/audit/defect_ledger.go", Rationale: "the unit-09 seam: the ledger's one wired construction, the request/rejection projections, the citation resolver and the facades disposition.go, the prompt builder and the ACS-named tests keep (ADR-0103)"},
	{Fragment: "/go/internal/bridge/launchoutcome/", Rationale: "the launch-outcome classifier — the one exit-code table, the cause-line miners, the request gauntlet and the BRIDGE_EXIT_* projection a launch failure's triage reads (ADR-0103 unit 10)"},
	{Fragment: "/go/internal/bridge/engine.go", Rationale: "the unit-10 seam: the Launch spine's named steps, the classification call, the launch-error persist and the facade the driver tests and the ACS-named tests keep (ADR-0103)"},
	{Fragment: "/go/internal/textcap/", Rationale: "the two rune-cap text rules the advisor prompt and the carryover lifecycle project (ADR-0103 unit 04)"},
	{Fragment: "/go/internal/core/advisor/", Rationale: "the phase advisor — routing/plan prompt composer, bridge launcher with the profile fallback chain, redacted capture and plan/proposal parser with the mint recursion guard (ADR-0103 unit 04)"},
	{Fragment: "/go/internal/core/phase_advisor.go", Rationale: "the unit-04 seam: the one wired construction, the Bridge→Launcher projection, and the facades the composition root, resume, the judge/adjudicator, the failure digest and the by-name tests keep (ADR-0103)"},
	{Fragment: "/go/internal/phases/audit/ciparitygate/", Rationale: "the CI-parity gates — go vet, acs-durable, the serialized -tags integration tier with its cross-lane lock and clean-env retake, apicover -enforce and new-package graduation: the deterministic gates that decide whether a cycle may ship (ADR-0103 unit 14)"},
	{Fragment: "/go/internal/phases/audit/ciparity.go", Rationale: "the unit-14 seam: the gates' one per-call construction from the host's runner and budget seams, the request projection, the change-set locator and the five facades the by-name tests and ACS predicates keep (ADR-0103)"},
	{Fragment: "/go/internal/subagent/subagentrun/", Rationale: "the evolve subagent run execution path — request admission, resolution, prompt delivery and staging, the bridge exec port, artifact verification and the agent_subprocess ledger record (ADR-0103 unit 16)"},
	{Fragment: "/go/internal/subagent/run.go", Rationale: "the unit-16 seam: the dispatcher's one wired construction from RunOptions, the request/result projections and the facades the root, dispatch-parallel, the Runner twin and the ACS binders keep (ADR-0103)"},
	{Fragment: "/go/internal/core/outcome/", Rationale: "the C1 phase-outcome recorder the failed-cycle handoff projects from (ADR-0103 unit 01)"},
	{Fragment: "/go/internal/inboxmover/lifecycle/", Rationale: "the inbox lifecycle mover — claim, promote, quarantine release, the cycle drain, orphan recovery and the processed-record primitives every root, the ship post-ship and the triage sandbox move through (ADR-0103 unit 06)"},
	{Fragment: "/go/internal/inboxmover/inboxmover.go", Rationale: "the unit-06 seam: Options, the resolved defaults, the ledger fallback, the one wired construction and the facades every production root, the ship phase and the ACS predicates keep (ADR-0103)"},
	{Fragment: "/go/internal/loopwave/", Rationale: "the loop wave engine — the wave gate, one dispatch body, the min-width repair, the fleet-config reload, the freshness-gated launcher, quota/budget sizing and the plan source (ADR-0103 unit 13)"},
	{Fragment: "/go/internal/loopchain/", Rationale: "the batch-chaining driver and the boundary binary-refresh engine (ADR-0103 unit 13)"},
	{Fragment: "/go/cmd/evolve/cmd_loop_wave.go", Rationale: "the unit-13 wave seam: the engine's one wired construction, the coordinator accessor and the facades the pool, the budget wrapper and the by-name tests keep (ADR-0103)"},
	{Fragment: "/go/cmd/evolve/cmd_loop_chain.go", Rationale: "the unit-13 chain seam: the eleven package-var projections, the two process adapters, the chain root's Center and the facades the ACS-named tests keep (ADR-0103)"},
	{Fragment: "/go/internal/core/ship_recovery.go", Rationale: "rebase recovery must invalidate stale explanation and route through Build"},
	{Fragment: "/go/internal/core/resume.go", Rationale: "resume entry point (RunCycleFromPhase) and resumed-deliverable explanation review parity (reviewResumedDeliverable)"},
	{Fragment: "/go/internal/core/resume_execution.go", Rationale: "resume sealing, projection, and post-Build refresh call sites (resumeExecution.run); carved out of resume.go by #549"},
	{Fragment: "/go/internal/core/resume_bootstrap.go", Rationale: "resume rebase-split recovery and explanation identity check (explanationdocs.RecoverRebaseSplit, requireResumeExplanationIdentity); carved out of resume.go by #549"},
	{Fragment: "/go/internal/core/ports.go", Rationale: "typed Bridge request sandbox requirement"},
	{Fragment: "/go/internal/core/phase.go", Rationale: "typed phase explanation handoff and contract-version fields"},
	{Fragment: "/go/internal/cyclestate/state.go", Rationale: "durable explanation contract version and Build binding state"},
	{Fragment: "/go/internal/phaseio/handoffs.go", Rationale: "typed cross-phase explanation handoff schema"},
	{Fragment: "/go/internal/phases/runner/runner.go", Rationale: "requiresExplanationSandbox decision (the assignment call site now lives in dispatch.go)"},
	{Fragment: "/go/internal/phases/runner/dispatch.go", Rationale: "mandatory versioned-Build sandbox propagation call site (dispatchPhaseAttempts sets RequireSandbox from requiresExplanationSandbox); carved out of runner.go by #549"},
	{Fragment: "/go/internal/phases/runner/verdict/", Rationale: "the phase runner's verdict engine — the settle ladder, the teardown reconcile arms, the stale-leftover gate, the ACS deterministic floor, the verdict-source rule and the ship guard every contracted phase's verdict passes through (ADR-0103 unit 11)"},
	{Fragment: "/go/internal/phases/runner/verdict_engine.go", Rationale: "the unit-11 seam: the engine's one wired construction, the Dispatch projection, the Center derivation from the Bridge and the settle-bound projections the settle tests keep (ADR-0103)"},
	{Fragment: "/go/internal/bridge/", Rationale: "Bridge registry, drivers, and OS sandbox fail-closed enforcement for versioned Build"},
	{Fragment: "/go/internal/adapters/bridge/", Rationale: "Bridge request adapter preserving mandatory Build sandbox propagation"},
	{Fragment: "/go/internal/adapters/sandbox/", Rationale: "OS-specific confinement policy and generated write boundary"},
	{Fragment: "/go/internal/looppreflight/checks.go", Rationale: "pre-spend required-sandbox readiness HALT"},
	{Fragment: "/go/internal/looppreflight/drivers.go", Rationale: "sandbox-enabled profile discovery used by readiness HALT"},
	{Fragment: "/go/internal/preflight/preflight.go", Rationale: "measured host sandbox capability used by readiness HALT"},
	{Fragment: "/go/internal/phases/audit/audit.go", Rationale: "Auditor explanation documentation host activation gate"},
	{Fragment: "/go/internal/phases/audit/classification.go", Rationale: "Auditor explanation-review gate call site (newAuditClassification -> validateExplanationReview); carved out of audit.go by #549"},
	{Fragment: "/go/internal/phases/retro/retro.go", Rationale: "Retrospective explanation-review gate call site"},
	{Fragment: "/go/internal/phases/ship/native.go", Rationale: "native Ship explanation re-verification call site"},
	{Fragment: "/go/internal/phases/ship/ship.go", Rationale: "Ship phase typed explanation binding propagation"},
	{Fragment: "/go/internal/phases/ship/gitops.go", Rationale: "verified worktree selection and the direct-path mutation (the ship-binding writer and the push moved to landing/, ADR-0103 unit 07)"},
	{Fragment: "/go/internal/phases/ship/landing/", Rationale: "the fleet landing — the ff-merge, the push with its inline push-race repair and the ship-binding writer the ship phase lands through (ADR-0103 unit 07)"},
	{Fragment: "/go/internal/phases/ship/gitops_landing.go", Rationale: "the unit-07 seam: the landing's one wired construction, the push projection and the writeShipBinding / isAncestor / captureGitOutput facades the ship paths keep (ADR-0103)"},
	{Fragment: "/go/internal/phases/audit/explanation_review_gate.go", Rationale: "Auditor qualitative explanation-review gate"},
	{Fragment: "/go/internal/phases/audit/solution_gate.go", Rationale: "ADR-0099 slice 2: the document deliverable audit gate default — a lane must not soften the contract its own solution is graded against"},
	{Fragment: "/go/internal/solutioncheck/", Rationale: "ADR-0099 slice 2: the ONE engine that judges a document deliverable — the gate above delegates to it, so it is the surface a lane would actually soften"},
	{Fragment: "/go/internal/core/solution_floor.go", Rationale: "ADR-0099 slice 2: SolutionViolations/DocumentCycle — the shared classification + collection the floor, the audit gate and ship call"},
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

// IsProtectedSurface reports whether path is on the control plane. path may be absolute or
// repo-relative: a fragment matches anywhere in it, so the boundary holds in any worktree.
func IsProtectedSurface(path string) bool {
	p, ok := normalizeSurfacePath(path)
	if !ok {
		return false
	}
	// The appended slash lets a slashless directory name ("go/internal/bridge") match its fragment.
	// It changes no other match, because it can only complete a fragment that ends in a slash.
	pd := p + "/"
	for _, e := range ProtectedSurfaceManifest {
		if strings.Contains(pd, e.Fragment) {
			return true
		}
	}
	return false
}

// IsProtectedScope reports whether path is, or as a directory spelling contains, protected surface.
// Routing and triage's breaker compose it with the build sandbox (lanerouting); the ship tripwire
// and role guard keep IsProtectedSurface. Membership implies scope.
func IsProtectedScope(path string) bool {
	if IsProtectedSurface(path) {
		return true
	}
	p, ok := normalizeSurfacePath(path)
	if !ok {
		return false
	}
	if dir, isDir := directorySpelling(p); isDir {
		for _, e := range ProtectedSurfaceManifest {
			if fragmentInside(dir, e.Fragment) {
				return true
			}
		}
	}
	return false
}

// normalizeSurfacePath is the one spelling both projections match. The leading slash lets a
// repo-relative path match its fragment; case-folding covers case-insensitive filesystems.
func normalizeSurfacePath(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.ToLower(p), true
}

// directorySpelling reports whether p is spelled as a directory (a trailing slash, or a last segment with
// no dot after its first character, so ".evolve" counts) and returns it with one trailing slash.
func directorySpelling(p string) (string, bool) {
	if strings.HasSuffix(p, "/") {
		return p, true
	}
	last := p[strings.LastIndex(p, "/")+1:]
	if last == "" || strings.Contains(last[1:], ".") {
		return "", false
	}
	return p + "/", true
}

// fragmentInside reports whether fragment lies under dir wherever dir is rooted: some suffix of dir that
// starts at a slash prefixes the fragment. The bare root "/" is excluded, since every fragment starts with it.
func fragmentInside(dir, fragment string) bool {
	for i := 0; i < len(dir)-1; i++ {
		if dir[i] == '/' && strings.HasPrefix(fragment, dir[i:]) {
			return true
		}
	}
	return false
}
