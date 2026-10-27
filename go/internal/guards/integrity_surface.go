package guards

import (
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type GuaranteeClass string

const (
	GuardGuarantee              GuaranteeClass = "guard"
	GateGuarantee               GuaranteeClass = "ship-or-commit-gate"
	SandboxGuarantee            GuaranteeClass = "sandbox-rule"
	LedgerGuarantee             GuaranteeClass = "ledger"
	PolicyGuarantee             GuaranteeClass = "policy-parser"
	PhaseOrderGuarantee         GuaranteeClass = "phase-order"
	AuditVerdictGuarantee       GuaranteeClass = "audit-verdict"
	DispatchCredentialGuarantee GuaranteeClass = "dispatch-credential"
	InstructionGuarantee        GuaranteeClass = "kernel-instruction"
	ManifestGuarantee           GuaranteeClass = "manifest"
)

func GuaranteeClasses() []GuaranteeClass {
	return []GuaranteeClass{GuardGuarantee, GateGuarantee, SandboxGuarantee, LedgerGuarantee, PolicyGuarantee, PhaseOrderGuarantee, AuditVerdictGuarantee, DispatchCredentialGuarantee, InstructionGuarantee, ManifestGuarantee}
}

type ProtectedSurfaceEntry struct {
	Fragment  string
	Class     GuaranteeClass
	Rationale string
}

var ProtectedSurfaceManifest = []ProtectedSurfaceEntry{
	{Fragment: "/go/acs/regression/", Class: AuditVerdictGuarantee, Rationale: "standing deterministic gates"},
	{Fragment: "/go/internal/acssuite/", Class: AuditVerdictGuarantee, Rationale: "the gate runner"},
	{Fragment: "/go/internal/phases/audit/probe_quarantine", Class: AuditVerdictGuarantee, Rationale: "the gate runner's input filter — decides which files the EGPS suite is allowed to see"},
	{Fragment: "/go/internal/guards/", Class: GuardGuarantee, Rationale: "the guards (incl. this manifest + role.go)"},
	{Fragment: "/go/internal/flagregistry/registry_table.go", Class: AuditVerdictGuarantee, Rationale: "the campaign metric SSOT"},
	{Fragment: "/go/internal/flagregistry/registry_ceiling_test.go", Class: AuditVerdictGuarantee, Rationale: "the ceiling ratchet gate"},
	{Fragment: "/knowledge-base/research/flag-campaign-plan.json", Class: AuditVerdictGuarantee, Rationale: "the campaign contract"},
	{Fragment: "/skills/audit/", Class: AuditVerdictGuarantee, Rationale: "the audit grading rubric"},
	{Fragment: "/skills/adversarial-testing/", Class: AuditVerdictGuarantee, Rationale: "the adversarial anti-gaming rubric (M4 goal-integrity)"},
	{Fragment: "/skills/fable/", Class: InstructionGuarantee, Rationale: "the operating-discipline overlay preloaded into deep/top-tier phase agents (policy.ResolveOverlays → bridge skill-overlay injection); its SKILL.md persona is integrity-load-bearing once injected into every phase prompt (audit-F1)"},
	{Fragment: "/skills/solution-scout/", Class: InstructionGuarantee, Rationale: "the document-cycle discovery persona the kernel preloads on its own authority (policy.CompiledDefaultOverlaySkills, ADR-0099 slice 3)"},
	{Fragment: "/skills/solution-build/", Class: InstructionGuarantee, Rationale: "the document-cycle build persona the kernel preloads on its own authority (policy.CompiledDefaultOverlaySkills, ADR-0099 slice 3)"},
	{Fragment: "/skills/solution-audit/", Class: AuditVerdictGuarantee, Rationale: "the document-cycle audit grading persona the kernel preloads on its own authority — the same class as /skills/audit/ (policy.CompiledDefaultOverlaySkills, ADR-0099 slice 3)"},
	{Fragment: "/skills/engineering-craft/", Class: InstructionGuarantee, Rationale: "the craft persona the kernel preloads into every code cycle's source-writing dispatch at any tier (policy.CompiledDefaultOverlaySkills); a tampered copy would rewrite how every builder and test writer works"},
	{Fragment: "/skills/code-review-simplify/", Class: InstructionGuarantee, Rationale: "the self-review persona the kernel preloads into every code cycle's source-writing dispatch at any tier (policy.CompiledDefaultOverlaySkills); a tampered copy would rewrite how every builder and test writer reviews its own diff"},
	{Fragment: "/skills/docs-write/", Class: InstructionGuarantee, Rationale: "the document-writing persona the kernel preloads into every document-cycle scout, build and audit dispatch and every code cycle's source-writing dispatch at any tier (policy.CompiledDefaultOverlaySkills); a tampered copy would rewrite how every agent writes the documents a person reads"},
	{Fragment: "/.claude/settings.json", Class: GuardGuarantee, Rationale: "PreToolUse hook wiring (repo + global ~/.claude)"},
	{Fragment: "/.evolve/policy.json", Class: PolicyGuarantee, Rationale: "gate-default overrides (eval/contract/swarm gates)"},

	{Fragment: "/go/internal/guidance/", Class: InstructionGuarantee, Rationale: "the rule registry check of CI and the generator of the L0 region that every agent of every family loads (ADR-0145)"},
	{Fragment: "/guidance/rules/", Class: InstructionGuarantee, Rationale: "the rule registry: its L0 rules render into AGENTS.md, CLAUDE.md and GEMINI.md, so a tampered rule rewrites the invariants of every agent (ADR-0145)"},
	{Fragment: "/go/internal/commitgate/", Class: GateGuarantee, Rationale: "the pre-commit quality gate (attestation writer the manual-ship reader trusts)"},
	{Fragment: "/go/internal/launchrecord/", Class: GateGuarantee, Rationale: "the bridge launch record of evolve agent run (ADR-0154 X11): the commit gate attests a review run from it, so a cycle that weakened its tree binding or its run id check could attest a review that never saw the staged tree"},
	{Fragment: "/go/internal/commentaudit/", Class: GateGuarantee, Rationale: "the comment rules the commit gate trusts: the comment-only proof behind its review waiver, the added-comment and removed-history refusals, the history archive's append-only check, and the build comment floor (ADR-0115)"},
	{Fragment: "/go/internal/phaseintegrity/", Class: LedgerGuarantee, Rationale: "the per-phase integrity chain's DigestSource (ADR-0065)"},
	{Fragment: "/go/internal/binaryguard/", Class: GuardGuarantee, Rationale: "the compiled-binary commit backstop at the commit chokepoint"},
	{Fragment: "/go/internal/guardslog/", Class: GuardGuarantee, Rationale: "the shared guards-log writer (the guards' evidence trail)"},
	{Fragment: "/go/internal/cli/guardcmd/", Class: GuardGuarantee, Rationale: "the CLI wiring of the trust-kernel guard + gate subcommands"},
	{Fragment: "/go/internal/core/workspace_guard.go", Class: GuardGuarantee, Rationale: "workspace-pollution guard (archives a polluted workspace)"},
	{Fragment: "/go/internal/core/activating_fields_guard_test.go", Class: GuardGuarantee, Rationale: "ADR-0058 registry trust anchor (drift-loud activating-fields guard)"},
	{Fragment: "/go/internal/core/orchestrator_guard_test.go", Class: GuardGuarantee, Rationale: "inserted-phase tree-diff guard pin (cycle-274 G)"},
	{Fragment: "/go/internal/core/phase_advisor_guard_test.go", Class: GuardGuarantee, Rationale: "advisor routing guard pin"},
	{Fragment: "/go/internal/core/flow_span_guard_test.go", Class: GuardGuarantee, Rationale: "the I-8 typed-flow-decision guard (ADR-0154 X8): it fails when LastBalancedSpan is named outside internal/core/advisor/span.go, so each flow seat reads its declared decision file and only the legacy stage reaches the stdout span; a cycle that quietly widened it could let a flow decision come from the prose of an agent again"},
	{Fragment: "/go/internal/core/ledger_runid_writers_guard_test.go", Class: GuardGuarantee, Rationale: "the agent_subprocess run_id writer guard: closes the writer set and requires each writer to call core.RunIDFromWorkspace. Since #503 a run-scoped binding REFUSES an entry with no run_id, so a cycle that quietly delisted a writer here would make everything that writer emits unbindable and hard-stop ship at AUDIT_BINDING_NO_AUDITOR (cycle-1571 H1)"},
	{Fragment: "/go/internal/phases/ship/binary_staging_guard.go", Class: GuardGuarantee, Rationale: "staging-time backstop against compiled-binary commits (tracked-binary-in-acs-dir class fix)"},
	{Fragment: "/go/internal/phases/ship/binary_staging_guard_test.go", Class: GuardGuarantee, Rationale: "the staging backstop's own regression pin"},
	{Fragment: "/go/internal/phases/ship/committed_doc_guard.go", Class: GuardGuarantee, Rationale: "the landing-time refusal of a diff that deletes committed documentation under docs/ or knowledge-base/; a cycle that weakened it could land the deletion of the repo's primary asset"},
	{Fragment: "/go/internal/phases/ship/committed_doc_guard_test.go", Class: GuardGuarantee, Rationale: "the committed-documentation refusal's own regression pin"},
	{Fragment: "/go/internal/phases/ship/foreign_predicate_guard.go", Class: GuardGuarantee, Rationale: "the landing-time refusal of a cycle landing that adds the predicate package of another cycle (logic-first-delivery-design §7.9 A2); a cycle that weakened it could land a superseded attempt's predicates beside its own"},
	{Fragment: "/go/internal/phases/ship/foreign_predicate_guard_test.go", Class: GuardGuarantee, Rationale: "the foreign predicate package refusal's own regression pin"},
	{Fragment: "/go/internal/bridge/driver_tmux_submitverify_guard_test.go", Class: GuardGuarantee, Rationale: "submit-verify's own keep-guard: pins that every real tmux driver declares an input-line marker (a family that omits it goes inert) and that a failed verification stays loud — a cycle editing this could re-open the cycles 1505/1510/1517 unsubmitted-prompt stall unobserved"},
	{Fragment: "/go/internal/bridge/integration_wait_guard_test.go", Class: GuardGuarantee, Rationale: "the bounded-wait guard of the bridge integration tests: it fails when a test passes context.Background() or context.TODO() to a launch or wait entry; a cycle editing this could re-open the #1132 macOS hang, where a stuck agent waited until the 20 min package timeout panic"},
	{Fragment: "/go/internal/sysexec/command_guard_test.go", Class: GuardGuarantee, Rationale: "the K10 process guard: it fails on any production exec.Command or exec.CommandContext call outside sysexec.Command, so every process the pipeline starts has a Cancel and a WaitDelay; a cycle that quietly widened it could start processes that outlive their cancel"},
	{Fragment: "/go/internal/sysexec/llm_launch_guard_test.go", Class: GuardGuarantee, Rationale: "the I-1 single-launcher guard (ADR-0154 X3): it fails on any start of an LLM binary outside the bridge launch API, and matrix row N2 runs the same scan; a cycle that quietly widened its exemptions could start an LLM with no cell environment, no unset TMUX and no sandbox decision"},
	{Fragment: "/go/internal/orchestrator/guard_test.go", Class: GuardGuarantee, Rationale: "the wave orchestrator's own process guard: it fails on any poll (time.Sleep, Tick, NewTicker), any process start, any stop by pid and any broad or name-matched kill in the package, so the scheduler only decides and never stops a lane by itself; a cycle that quietly widened it could let the orchestrator start or kill processes"},
	{Fragment: "/go/test/structure/nopoll_guard_test.go", Class: GuardGuarantee, Rationale: "the no-poll guard of the event channels (ADR-0127): it fails on any time.Sleep, time.Tick, time.NewTicker, time.After or time.NewTimer in the directories of its list (internal/events today), so every wait arms a kernel watch and a one-shot deadline; a cycle that quietly widened it could bring back the poll loops that the channels replace"},
	{Fragment: "/go/internal/core/disposition_gate.go", Class: GateGuarantee, Rationale: "ADR-0074 disposition-contract gate (retro's verdict-on-the-verdict verifier + digest cross-check)"},
	{Fragment: "/go/internal/core/disposition_gate_test.go", Class: GateGuarantee, Rationale: "the disposition gate's own regression pins"},

	{Fragment: "/go/internal/explanationdocs/", Class: AuditVerdictGuarantee, Rationale: "Build explanation contract, host snapshots, and deterministic verifier"},
	{Fragment: "/go/internal/reportdoc/", Class: AuditVerdictGuarantee, Rationale: "strict visible-Markdown parser shared by explanation review gates"},
	{Fragment: "/go/internal/core/build_explanation_handoff.go", Class: GateGuarantee, Rationale: "orchestrator activation and post-review sealing lifecycle"},
	{Fragment: "/go/internal/core/build_floor_reviewer.go", Class: GateGuarantee, Rationale: "mandatory deterministic Build explanation floor, and the floor coverage pass's go-test argv (coverTestArgs, under addedtests.PackageTimeout) and its verification-lock wait (coverLockWait = verifylock.MaxWait)"},
	{Fragment: "/go/internal/core/floor_lists_base.go", Class: GateGuarantee, Rationale: "the build floor reads go/.apicover-enforce and go/.cover-strict from the base and the tree together (gate-input-inventory D3); a cycle that weakened it could drop enforcement from its own floor"},
	{Fragment: "/go/internal/core/floor_lists_base_test.go", Class: GateGuarantee, Rationale: "the base-and-tree floor list merge's own regression pins"},
	{Fragment: "/go/internal/core/floor_lists_regression.go", Class: GateGuarantee, Rationale: "the build floor refuses a lane that drops an enforced package or lowers a coverage floor of the base (gate-input-inventory D3); a cycle that weakened it could land the weaker list on main"},
	{Fragment: "/go/internal/core/floor_lists_regression_test.go", Class: GateGuarantee, Rationale: "the floor list regression refusal's own pins"},
	{Fragment: "/go/internal/core/cover_changed_floor.go", Class: GateGuarantee, Rationale: "the build floor's cover-changed and cover-strict checks, which read the merged floor lists"},
	{Fragment: "/go/internal/profiles/gate_inputs_realtree_test.go", Class: GuardGuarantee, Rationale: "the write-scope regression test over docs/architecture/gate-input-inventory.md, with the pinned authority rows and open seams; a cycle that relabelled a row or a pin would silence it"},
	{Fragment: "/go/internal/core/ste_floor.go", Class: GateGuarantee, Rationale: "the build floor's STE check: at docs_floor.ste_stage enforce it sends a doc whose added lines fail the lint back to the build; a cycle that weakened it could land the doc CI refuses"},
	{Fragment: "/go/internal/stelint/added.go", Class: GateGuarantee, Rationale: "the one added-line scope of the STE lint, shared by the build floor and evolve docs ste-lint --added (the CI job)"},
	{Fragment: "/go/internal/core/build_explanation_floor_test.go", Class: GateGuarantee, Rationale: "composition tripwire proving optional reviewers cannot replace the explanation floor"},
	{Fragment: "/go/internal/core/reviewer.go", Class: GateGuarantee, Rationale: "mandatory explanation-review chain composition"},
	{Fragment: "/go/internal/core/orchestrator.go", Class: AuditVerdictGuarantee, Rationale: "fresh-cycle explanation activation and Build-context sealing call sites"},
	{Fragment: "/go/internal/core/cyclerun.go", Class: AuditVerdictGuarantee, Rationale: "fresh-cycle explanation contract-version stamp"},
	{Fragment: "/go/internal/core/cyclerun_dispatch.go", Class: AuditVerdictGuarantee, Rationale: "fresh-cycle downstream explanation projection call site"},
	{Fragment: "/go/internal/core/cyclerun_review.go", Class: AuditVerdictGuarantee, Rationale: "post-Build explanation refresh eligibility decision and review-to-guard ordering"},
	{Fragment: "/go/internal/core/cyclerun_postreview.go", Class: AuditVerdictGuarantee, Rationale: "fresh-cycle post-Build explanation refresh call site (applyPostReviewGuards -> explanationdocs.RefreshResult); carved out of cyclerun_review.go by #549"},
	{Fragment: "/go/internal/core/leak_recovery.go", Class: GateGuarantee, Rationale: "build-leak recovery and its fail-closed rules: the pre-phase snapshot retry and the abort when it still fails in a checkout, the no-checkout exemption decided once per cycle, leaveForOwner failing a keyed path no live lane or console lease holds, and quarantine-not-delete for a differing main-tree copy"},
	{Fragment: "/go/internal/core/leak_ownership.go", Class: GateGuarantee, Rationale: "the main-tree ownership Specification: what leak recovery may claim, which leaked paths the tree-diff guard exempts as a live sibling's (leakExemptions, heldBySibling), the mint-registry read with its quarantine, and the explanationdocs.IsCycleChangeRecord owner rule"},
	{Fragment: "/go/internal/core/cyclerun_remediate.go", Class: AuditVerdictGuarantee, Rationale: "Build explanation correction and remediation projection"},
	{Fragment: "/go/internal/core/continuation_ancestor_predicates.go", Class: AuditVerdictGuarantee, Rationale: "continuation archives the ancestor's explanation records and predicate package (design A2)"},
	{Fragment: "/go/internal/core/evaluate_batch.go", Class: AuditVerdictGuarantee, Rationale: "parallel evaluator explanation handoff projection"},
	{Fragment: "/go/internal/core/failure_learning.go", Class: AuditVerdictGuarantee, Rationale: "failed-cycle explanation handoff projection"},
	{Fragment: "/go/internal/core/errors.go", Class: GateGuarantee, Rationale: "the Bridge port's error sentinels and the integrity predicates on them (ErrArtifactTimeout, isArtifactTimeout — the timeout-only gate unit 02 injects — IsInfraTeardownError, IsOptionalSkippableError)"},
	{Fragment: "/go/internal/core/failure_diag.go", Class: GateGuarantee, Rationale: "failed-cycle failure-diagnostic seam: the unit-02 facade and the two injection sites of the timeout-only gate whose body is errors.go (ADR-0103)"},
	{Fragment: "/go/cmd/evolve/cmd_cycle_config.go", Class: GateGuarantee, Rationale: "the unit-08 seam: the loader's one wired construction, the policy-stages projection and the two stage forwarders the cycle/loop root keeps (ADR-0103)"},
	{Fragment: "/go/internal/core/defectledger/", Class: AuditVerdictGuarantee, Rationale: "the defect ledger — the anti-laundering schema, writer and readers, and the continuation disposition gate the audit phase, carryover and the adoption seeder project from (ADR-0103 unit 09)"},
	{Fragment: "/go/internal/phases/audit/defect_ledger.go", Class: AuditVerdictGuarantee, Rationale: "the unit-09 seam: the ledger's one wired construction, the request/rejection projections, the citation resolver and the facades disposition.go, the prompt builder and the ACS-named tests keep (ADR-0103)"},
	{Fragment: "/go/internal/phases/audit/ciparitygate/", Class: GateGuarantee, Rationale: "the CI-parity gates — go vet, acs-durable, the serialized -tags integration tier with its cross-lane lock and clean-env retake, apicover -enforce and new-package graduation: the deterministic gates that decide whether a cycle may ship (ADR-0103 unit 14)"},
	{Fragment: "/go/internal/phases/audit/ciparity.go", Class: GateGuarantee, Rationale: "the unit-14 seam: the gates' one per-call construction from the host's runner and budget seams, the request projection, the change-set locator and the five facades the by-name tests and ACS predicates keep (ADR-0103)"},
	{Fragment: "/go/internal/subagent/subagentrun/", Class: LedgerGuarantee, Rationale: "the evolve subagent run execution path — request admission, resolution, prompt delivery and staging, the bridge exec port, artifact verification and the agent_subprocess ledger record (ADR-0103 unit 16)"},
	{Fragment: "/go/internal/subagent/run.go", Class: LedgerGuarantee, Rationale: "the unit-16 seam: the dispatcher's one wired construction from RunOptions, the request/result projections and the facades the root, dispatch-parallel, the Runner twin and the ACS binders keep (ADR-0103)"},
	{Fragment: "/go/internal/triagedecision/", Class: GateGuarantee, Rationale: "ADR-0106 H1: the one reader of the triage report the host derives the decision from before any judge — a lane that could edit it could commit itself to less than its pin"},
	{Fragment: "/go/internal/recoveryguard/", Class: GuardGuarantee, Rationale: "ADR-0106 F2: the kernel fence around a recovery dispatch — the only backstop against an agent that may rewrite deliverables"},
	{Fragment: "/go/internal/core/ship_recovery.go", Class: GateGuarantee, Rationale: "rebase recovery must invalidate stale explanation and route through Build"},
	{Fragment: "/go/internal/derived/", Class: GateGuarantee, Rationale: "ADR-0128 Q1: the derived-output catalog decides which rebase conflicts the protected recovery regenerates instead of sending to the debugger — a lane that could edit it could auto-resolve its own genuine conflict"},
	{Fragment: "/go/internal/core/resume_execution.go", Class: AuditVerdictGuarantee, Rationale: "resume sealing, projection, and post-Build refresh call sites (resumeExecution.run); carved out of resume.go by #549"},
	{Fragment: "/go/internal/core/resume_bootstrap.go", Class: AuditVerdictGuarantee, Rationale: "resume rebase-split recovery and explanation identity check (explanationdocs.RecoverRebaseSplit, requireResumeExplanationIdentity); carved out of resume.go by #549"},
	{Fragment: "/go/internal/core/ports.go", Class: SandboxGuarantee, Rationale: "typed Bridge request sandbox requirement"},
	{Fragment: "/go/internal/core/bridge_completion.go", Class: GateGuarantee, Rationale: "ADR-0113: the one predicate that lets a correction re-dispatch complete on worktree evidence with its deliverable carried unchanged — a lane that could widen it could carry its own stale deliverable past the bridge's pre-dispatch baseline guard"},
	{Fragment: "/go/internal/phases/runner/runner.go", Class: SandboxGuarantee, Rationale: "requiresExplanationSandbox decision (the assignment call site now lives in dispatch.go)"},
	{Fragment: "/go/internal/phases/runner/dispatch.go", Class: SandboxGuarantee, Rationale: "mandatory versioned-Build sandbox propagation call site (the RequireSandbox assignment is pinned by explanationLifecycleAssignPins in build_explanation_wiring_test.go); carved out of runner.go by #549"},
	{Fragment: "/go/internal/phases/runner/verdict/", Class: AuditVerdictGuarantee, Rationale: "the phase runner's verdict engine — the settle ladder, the teardown reconcile arms, the stale-leftover gate, the ACS deterministic floor, the verdict-source rule and the ship guard every contracted phase's verdict passes through (ADR-0103 unit 11)"},
	{Fragment: "/go/internal/phases/runner/verdict_engine.go", Class: AuditVerdictGuarantee, Rationale: "the unit-11 seam: the engine's one wired construction, the Dispatch projection, the Center derivation from the Bridge and the settle-bound projections the settle tests keep (ADR-0103)"},
	{Fragment: "/go/internal/adapters/bridge/", Class: SandboxGuarantee, Rationale: "Bridge request adapter preserving mandatory Build sandbox propagation"},
	{Fragment: "/go/internal/adapters/sandbox/", Class: SandboxGuarantee, Rationale: "OS-specific confinement policy and generated write boundary"},
	{Fragment: "/go/internal/dispatchcred/", Class: DispatchCredentialGuarantee, Rationale: "the dispatch credential that no agent cell can read (ADR-0154 I-3): a cycle that could edit it could let a cell dispatch an agent"},
	{Fragment: "/go/internal/agentclass/", Class: DispatchCredentialGuarantee, Rationale: "the agent class of each evolve verb (ADR-0156, cli-contract.md §4.1): a cycle that could mark a verb class A could let a cell run it without the dispatch credential"},
	{Fragment: "/go/internal/agentrequest/", Class: DispatchCredentialGuarantee, Rationale: "the request channel of a cell (llm-isolation.md §7.2.1): a cycle that could widen its validator or skip its judgement could let a cell dispatch a phase agent"},
	{Fragment: "/go/cmd/evolve/request_channel.go", Class: DispatchCredentialGuarantee, Rationale: "the judge and dispatch seats of the request channel (llm-isolation.md §7.2.1): a cycle that edits them could approve each request or run another role"},
	{Fragment: "/go/cmd/evolve/request_channel_test.go", Class: DispatchCredentialGuarantee, Rationale: "the guard that an approved request runs only through the orchestrator judgement and the requested role (TestAgentRequest_AnApprovedRequestIsRunByThePipeline)"},
	{Fragment: "/go/internal/core/request_handoff.go", Class: DispatchCredentialGuarantee, Rationale: "the call of the request channel after each phase dispatch (llm-isolation.md §7.2.1)"},
	{Fragment: "/go/cmd/evolve/dispatch_gate.go", Class: DispatchCredentialGuarantee, Rationale: "the one gate that checks the dispatch credential for each verb that is not class A (ADR-0154 I-3, cli-contract.md §4.4)"},
	{Fragment: "/go/cmd/evolve/dispatch_gate_test.go", Class: DispatchCredentialGuarantee, Rationale: "the guard that each registry command has a class and each verb that is not class A refuses without the credential (cli-contract.md §4.4)"},
	{Fragment: "/go/cmd/evolve/agent_classes_doc_test.go", Class: DispatchCredentialGuarantee, Rationale: "the guard that the class table matches cli-contract.md §4.1 and §5 (TestAgentClasses_MatchTheContract)"},
	{Fragment: "/go/internal/inputindex/", Class: SandboxGuarantee, Rationale: "the artifact index and lineage of each phase cell (ADR-0154 I-4, I-5, X5): the sandbox grants reads of exactly the indexed paths, so a cycle that widened the lineage or listed raw output could let a cell read unrelated cycles or the raw logs of other agents"},
	{Fragment: "/go/internal/looppreflight/checks.go", Class: SandboxGuarantee, Rationale: "pre-spend required-sandbox readiness HALT"},
	{Fragment: "/go/internal/looppreflight/drivers.go", Class: SandboxGuarantee, Rationale: "sandbox-enabled profile discovery used by readiness HALT"},
	{Fragment: "/go/internal/preflight/preflight.go", Class: SandboxGuarantee, Rationale: "measured host sandbox capability used by readiness HALT"},
	{Fragment: "/go/internal/phases/audit/audit.go", Class: AuditVerdictGuarantee, Rationale: "Auditor explanation documentation host activation gate"},
	{Fragment: "/go/internal/phases/audit/classification.go", Class: AuditVerdictGuarantee, Rationale: "Auditor explanation-review gate call site (newAuditClassification -> validateExplanationReview); carved out of audit.go by #549"},
	{Fragment: "/go/internal/phases/retro/retro.go", Class: AuditVerdictGuarantee, Rationale: "Retrospective explanation-review gate call site"},
	{Fragment: "/go/internal/phases/ship/native.go", Class: GateGuarantee, Rationale: "native Ship explanation re-verification call site"},
	{Fragment: "/go/internal/phases/ship/ship.go", Class: GateGuarantee, Rationale: "Ship phase typed explanation binding propagation"},
	{Fragment: "/go/internal/phases/ship/gitops.go", Class: GateGuarantee, Rationale: "verified worktree selection and the direct-path mutation (the ship-binding writer and the push moved to landing/, ADR-0103 unit 07)"},
	{Fragment: "/go/internal/phases/ship/landing/", Class: GateGuarantee, Rationale: "the fleet landing — the ff-merge, the push with its inline push-race repair and the ship-binding writer the ship phase lands through (ADR-0103 unit 07)"},
	{Fragment: "/go/internal/phases/ship/gitops_landing.go", Class: GateGuarantee, Rationale: "the unit-07 seam: the landing's one wired construction, the push projection and the writeShipBinding / isAncestor / captureGitOutput facades the ship paths keep (ADR-0103)"},
	{Fragment: "/go/internal/phases/ship/landing_intent.go", Class: GateGuarantee, Rationale: "the two-phase landing: the write-ahead intent, the resume that skips the gates, and the unwind before Audit (ADR-0039 §8.1)"},
	{Fragment: "/.evolve/landing/", Class: GateGuarantee, Rationale: "the host-owned landing intents: a prepared intent lets ship resume a landing with no gate, so no phase may write one (ADR-0039 §8.1)"},
	{Fragment: "/go/internal/overlap/", Class: GateGuarantee, Rationale: "the overlap proof of the landing queue: its tier decides whether a composed candidate keeps its audit verdict with no re-audit (T1), so a cycle that could edit it could skip its own re-audit (ADR-0128 §6, §8)"},
	{Fragment: "/go/internal/phases/audit/explanation_review_gate.go", Class: AuditVerdictGuarantee, Rationale: "Auditor qualitative explanation-review gate"},
	{Fragment: "/go/internal/phases/audit/solution_gate.go", Class: GateGuarantee, Rationale: "ADR-0099 slice 2: the document deliverable audit gate default — a lane must not soften the contract its own solution is graded against"},
	{Fragment: "/go/internal/solutioncheck/", Class: GateGuarantee, Rationale: "ADR-0099 slice 2: the ONE engine that judges a document deliverable — the gate above delegates to it, so it is the surface a lane would actually soften"},
	{Fragment: "/go/internal/core/solution_floor.go", Class: GateGuarantee, Rationale: "ADR-0099 slice 2: SolutionViolations/DocumentCycle — the shared classification + collection the floor, the audit gate and ship call"},
	{Fragment: "/go/internal/phases/retro/explanation_review_gate.go", Class: AuditVerdictGuarantee, Rationale: "Retrospective explanation-review and correction-todo gate"},
	{Fragment: "/go/internal/phases/ship/native_explanation_gate.go", Class: GateGuarantee, Rationale: "canonical native Ship explanation re-verification gate"},
	{Fragment: "/agents/evolve-builder.md", Class: InstructionGuarantee, Rationale: "Builder authority and explanation deliverable instructions"},
	{Fragment: "/agents/evolve-builder-reference.md", Class: InstructionGuarantee, Rationale: "Builder explanation document schema"},
	{Fragment: "/agents/evolve-auditor.md", Class: AuditVerdictGuarantee, Rationale: "Auditor explanation-review authority"},
	{Fragment: "/agents/evolve-auditor-reference.md", Class: AuditVerdictGuarantee, Rationale: "Auditor explanation-review output schema"},
	{Fragment: "/agents/evolve-retrospective.md", Class: InstructionGuarantee, Rationale: "Retro explanation verification and correction contract"},
	{Fragment: "/agents/evolve-memo.md", Class: InstructionGuarantee, Rationale: "Memo explanation handoff rendering contract"},
	{Fragment: "/schemas/handoff/build-report.schema.json", Class: InstructionGuarantee, Rationale: "Builder explanation report section activation schema"},
	{Fragment: "/schemas/handoff/audit-report.schema.json", Class: AuditVerdictGuarantee, Rationale: "Auditor explanation review section activation schema"},
	{Fragment: "/schemas/handoff/retrospective-report.schema.json", Class: InstructionGuarantee, Rationale: "Retrospective explanation review section activation schema"},
	{Fragment: "/.evolve/profiles/builder.json", Class: SandboxGuarantee, Rationale: "Builder write boundary for explanation artifacts"},
	{Fragment: "/.evolve/profiles/auditor.json", Class: SandboxGuarantee, Rationale: "Auditor read-only boundary for Builder explanation artifacts"},
	{Fragment: "/.evolve/build-explanation-contracts/", Class: AuditVerdictGuarantee, Rationale: "host-owned activation and Build result snapshots"},
	{Fragment: "/go/internal/core/build_handoff_floor.go", Class: GateGuarantee, Rationale: "build handoff floor: mandatory half, names walker, review-input projection (ADR-0117)"},
	{Fragment: "/go/internal/core/phase_bindings_selfcheck.go", Class: GateGuarantee, Rationale: "the build floor's go-test runners and their argvs (unit, tagged added-test): a lane must not move the deadline of the floor that grades it (inst-L1763b; cycles 1787/1791/1792/1798 timed out at a 120s literal here)"},
	{Fragment: "/go/internal/core/blocker_proof.go", Class: GateGuarantee, Rationale: "the blocker prover: the deterministic proof that a phase's gate failures need a protected path or come from two conflicting cycle predicates; a lane that weakens it escapes a real FAIL into a console route"},
	{Fragment: "/go/internal/core/build_blocker_exit.go", Class: GateGuarantee, Rationale: "the build floor's proven-blocker exit: which floor failures a blocker can explain and the blocked-needs-console ending"},
	{Fragment: "/go/internal/core/floor_blocker_notes.go", Class: GateGuarantee, Rationale: "the floor's uncovered-block lines that the blocker prover reads as evidence; a lane that changes them changes what the proof sees"},
	{Fragment: "/go/internal/addedtests/budget.go", Class: GateGuarantee, Rationale: "PackageTimeout, the one go-test deadline the build floor, its coverage pass and ship's repo contract run under: moving it moves the floor's budget"},
	{Fragment: "/go/internal/inboxrank/weightsort_ban_test.go", Class: GuardGuarantee, Rationale: "the one-ranking guard: no inbox ordering outside inboxrank (ADR-0121); a lane that could edit it could reorder the queue it is graded from"},
	{Fragment: "/skills/quality-index/", Class: AuditVerdictGuarantee, Rationale: "the one definition of \"qualified\" the code-review loop and the audit share: the dimensions, the bars, the N/A rules and the grammars (ADR-0124)"},
	{Fragment: "/go/internal/qualityindex/", Class: AuditVerdictGuarantee, Rationale: "the quality index's code home: the vocabulary, the threshold resolver, the Scores and Review Plan parsers and Qualifies (ADR-0124)"},
	{Fragment: "/skills/architecture-review/", Class: AuditVerdictGuarantee, Rationale: "the structural rubric the independent code review judges with (ADR-0124)"},
	{Fragment: "/go/internal/codereview/", Class: AuditVerdictGuarantee, Rationale: "the code-review phase's kernel half: the report grammar, the finding rows and the REVIEW_FINDINGS signal (ADR-0124)"},
	{Fragment: "/agents/evolve-code-reviewer.md", Class: AuditVerdictGuarantee, Rationale: "the independent reviewer's persona: what it reviews and how it reports (ADR-0124)"},
	{Fragment: "/.evolve/phases/code-review/", Class: AuditVerdictGuarantee, Rationale: "the code-review phase's spec: its sections, grammar and inputs (ADR-0124)"},
	{Fragment: "/.evolve/profiles/code-reviewer.json", Class: SandboxGuarantee, Rationale: "the reviewer's read-only, Claude-floor dispatch profile (ADR-0124)"},
	{Fragment: "/go/internal/router/code_review_pin_test.go", Class: GuardGuarantee, Rationale: "pins the registry's conditional_mandatory[code-review] rule; the registry file itself stays editable for routing work (ADR-0124)"},
	{Fragment: "/go/internal/guards/integrity_surface.go", Class: ManifestGuarantee, Rationale: "the protected manifest itself: a lane that could edit it could lift the fence of any gate (ADR-0157)"},
	{Fragment: "/go/internal/bridge/sandbox_paths.go", Class: SandboxGuarantee, Rationale: "the sandbox deny and write-grant rules of a cell (ADR-0157)"},
	{Fragment: "/go/internal/bridge/sandbox_wrap.go", Class: SandboxGuarantee, Rationale: "the OS sandbox prefix and the fail-closed check sandboxRequiredButUnavailable (ADR-0157)"},
	{Fragment: "/go/internal/bridge/sandbox_terminal.go", Class: SandboxGuarantee, Rationale: "the sandbox terminal path check (ADR-0157)"},
	{Fragment: "/go/internal/bridge/read_isolation.go", Class: SandboxGuarantee, Rationale: "the read isolation of a cell (ADR-0154 I-4)"},
	{Fragment: "/go/internal/bridge/protected_write.go", Class: SandboxGuarantee, Rationale: "the write deny of the protected manifest paths for a cell"},
	{Fragment: "/go/internal/bridge/no_nested_agency.go", Class: SandboxGuarantee, Rationale: "the refusal of an LLM binary inside a cell (ADR-0154 I-1)"},
	{Fragment: "/go/internal/bridge/dispatch_credential.go", Class: DispatchCredentialGuarantee, Rationale: "the check that the sandbox of a cell denies the dispatch credential (ADR-0154 I-3)"},
	{Fragment: "/go/internal/bridge/hermetic_env.go", Class: SandboxGuarantee, Rationale: "the hermetic environment of a cell: the allowed and dropped names"},
	{Fragment: "/go/internal/bridge/context_access.go", Class: SandboxGuarantee, Rationale: "the context-access grant reports that bound what a cell reads"},
	{Fragment: "/go/internal/bridge/launch.go", Class: SandboxGuarantee, Rationale: "the launch flags that carry --require-sandbox into the driver Config"},
	{Fragment: "/go/internal/bridge/launch_llm.go", Class: SandboxGuarantee, Rationale: "the one launch API of an LLM (ADR-0154 I-1, X3)"},
	{Fragment: "/go/internal/bridge/driver_tmux_boot.go", Class: SandboxGuarantee, Rationale: "the tmux boot that applies sandboxPrefixForLaunch to each tmux family"},
	{Fragment: "/go/internal/bridge/dispatch_signal_guard_test.go", Class: GuardGuarantee, Rationale: "the kill-guard pin: a test that leaves SignalProcess nil never reaches kill(2)"},
	{Fragment: "/go/internal/bridge/realtmux_sender_guard_test.go", Class: GuardGuarantee, Rationale: "the sender guard of the real tmux tests: it fails when a test Deps over execTmux{} has no SignalProcess, so the reap at kill-session would reach the default sender (the PR #1190 macOS panic)"},
	{Fragment: "/go/internal/bridge/resolve_guard_test.go", Class: GuardGuarantee, Rationale: "the one-resolver guard: only bridge.Resolve builds a driver name"},
	{Fragment: "/go/internal/bridge/selfmatch_guard_test.go", Class: GuardGuarantee, Rationale: "the self-match guard: the text of a tracked file never takes the shape of a live dialog rule"},
	{Fragment: "/go/internal/bridge/selfmatch_guard_prefilter_test.go", Class: GuardGuarantee, Rationale: "the self-match prefilter gives the exhaustive verdict for each rule"},
	{Fragment: "/go/internal/bridge/tmux_own_server_guard_test.go", Class: GuardGuarantee, Rationale: "the own-server guard of the tmux integration tests: a test never shares or kills the tmux server of the host"},
	{Fragment: "/go/internal/bridge/driver_claudep.go", Class: SandboxGuarantee, Rationale: "the claude -p driver: the ANTHROPIC_API_KEY credential-isolation abort and the fail-closed sandbox refusal"},
	{Fragment: "/go/internal/bridge/driver_claudetmux.go", Class: SandboxGuarantee, Rationale: "the claude tmux driver: the ANTHROPIC_API_KEY credential-isolation abort"},
	{Fragment: "/go/internal/bridge/driver_codextmux.go", Class: SandboxGuarantee, Rationale: "the codex tmux driver: the OPENAI_API_KEY credential-isolation abort"},
	{Fragment: "/go/internal/bridge/driver_tmux_prepare.go", Class: SandboxGuarantee, Rationale: "reusedSessionRefusal: a named tmux session that cannot prove the sandbox is never reused"},
	{Fragment: "/go/internal/bridge/manifest.go", Class: SandboxGuarantee, Rationale: "validateEnvPassthrough: a CLI manifest cannot pass a credential or a pipeline control variable into a cell"},
	{Fragment: "/go/internal/bridge/dispatch_reap.go", Class: GuardGuarantee, Rationale: "the ownership proof that decides which processes a dispatch reap may signal"},
	{Fragment: "/go/internal/proctree/", Class: GuardGuarantee, Rationale: "the process tree that the reap and the kill sites read to prove ownership"},
	{Fragment: "/go/internal/phaseobserver/phaseobserver.go", Class: GuardGuarantee, Rationale: "the kill site of the phase observer: it signals a process group"},
	{Fragment: "/go/internal/inboxbatch/consoleroute.go", Class: GuardGuarantee, Rationale: "ConsoleRouted: the routing refusal that keeps a protected or pipeline-kind item off the lanes"},
	{Fragment: "/go/internal/inboxmover/inboxmover.go", Class: GuardGuarantee, Rationale: "the wiring of WithProtectedPath into the claim of each production root"},
	{Fragment: "/go/internal/inboxmover/lifecycle/lifecycle.go", Class: GuardGuarantee, Rationale: "ErrConsoleRouted and WithProtectedPath: the claim refusal of an operator-owned item"},
	{Fragment: "/go/internal/inboxmover/lifecycle/claim.go", Class: GuardGuarantee, Rationale: "the lane claim that refuses a console-routed item"},
	{Fragment: "/go/internal/inboxmover/lifecycle/route.go", Class: GuardGuarantee, Rationale: "the route decision that the claim refusal reads"},
	{Fragment: "/go/internal/inboxmover/lifecycle/edit.go", Class: GuardGuarantee, Rationale: "the item edit that refuses to relax a console route"},
	{Fragment: "/go/internal/heldproposal/", Class: GateGuarantee, Rationale: "the held-proposal decision, the hold record and the operator approval (ADR-0157): a lane that could edit it could approve its own change"},
	{Fragment: "/go/internal/phases/ship/held_proposal_gate.go", Class: GateGuarantee, Rationale: "the landing check that holds a protected or pipeline-kind lane change until the operator approves its tree (ADR-0157)"},
	{Fragment: "/go/cmd/evolve/cmd_landing.go", Class: GateGuarantee, Rationale: "evolve landing approve and reject: the only writer of an operator approval (ADR-0157)"},
	{Fragment: "/go/internal/policy/held_proposals.go", Class: PolicyGuarantee, Rationale: "the held_proposals.stage key and its fail-closed read (ADR-0157)"},
	{Fragment: "/go/internal/adapters/ledger/", Class: LedgerGuarantee, Rationale: "the hash-chained ledger: the append, the seal, the anchor and the rebaseline (ADR-0157)"},
	{Fragment: "/go/internal/ledgerverify/", Class: LedgerGuarantee, Rationale: "the verifier of the ledger hash chain (ADR-0157)"},
	{Fragment: "/go/internal/ledgertip/", Class: LedgerGuarantee, Rationale: "the ledger tip that binds the chain head (ADR-0157)"},
	{Fragment: "/go/internal/policy/core.go", Class: PolicyGuarantee, Rationale: "the policy parser Load (ADR-0157)"},
	{Fragment: "/go/internal/policy/strict_keys.go", Class: PolicyGuarantee, Rationale: "the strict policy: refuseUnknownKeys refuses a key that the schema does not know (ADR-0157)"},
	{Fragment: "/go/internal/core/statemachine.go", Class: PhaseOrderGuarantee, Rationale: "the phase-order state machine CanTransition that clamps each plan of the advisor (ADR-0155 F3.2)"},
	{Fragment: "/.evolve/incidents/", Class: LedgerGuarantee, Rationale: "the incident records of the self-healing loop: only the detector writes them, so no agent can open, close or edit its own incident (ADR-0155, llm-isolation.md §18.2)"},
	{Fragment: "/go/internal/selfheal/", Class: AuditVerdictGuarantee, Rationale: "the incident detector and the four close conditions of I-14: an agent that could edit them could close its own incident (ADR-0155, llm-isolation.md §21)"},
	{Fragment: "/go/cmd/evolve/cmd_selfheal.go", Class: AuditVerdictGuarantee, Rationale: "evolve selfheal incidents: it chooses the evidence of the detector, builds the verifier of the four close conditions and writes the records (ADR-0155, llm-isolation.md §18.5)"},
	{Fragment: "/go/internal/policy/healing.go", Class: PolicyGuarantee, Rationale: "the healing.detector.stage key and its validation (ADR-0155, llm-isolation.md §20.2)"},
	{Fragment: "/go/internal/phasecontract/diagnosis.go", Class: GateGuarantee, Rationale: "the evolve.diagnosis/1 schema with the closed action catalog and the low-confidence escalation: an agent that could widen it could propose an action outside the catalog (ADR-0155, llm-isolation.md §18.4, §20.1)"},
	{Fragment: "/go/cmd/evolve/cmd_selfheal_propose.go", Class: AuditVerdictGuarantee, Rationale: "evolve selfheal propose: it builds the evidence index, runs the orchestrator seat, checks each diagnosis.json and writes the proposal records (ADR-0155, llm-isolation.md §18.6)"},
}

func IsProtectedSurface(path string) bool {
	p, ok := normalizeSurfacePath(path)
	if !ok {
		return false
	}
	pd := p + "/"
	for _, e := range ProtectedSurfaceManifest {
		if strings.Contains(pd, e.Fragment) {
			return true
		}
	}
	return false
}

func IsProtectedScope(path string) bool {
	return IsProtectedSurface(path)
}

func TouchesProtectedSurface(path string) bool {
	if IsProtectedSurface(path) {
		return true
	}
	p, ok := normalizeSurfacePath(path)
	if !ok {
		return false
	}
	dir := strings.TrimSuffix(p, "/") + "/"
	for i := 0; i < len(dir)-1; i++ {
		if dir[i] == '/' && startsAFragment(dir[i:]) {
			return true
		}
	}
	return false
}

func startsAFragment(tail string) bool {
	for _, e := range ProtectedSurfaceManifest {
		if strings.HasPrefix(e.Fragment, tail) {
			return true
		}
	}
	return false
}

func normalizeSurfacePath(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	slashed := filepath.ToSlash(raw)
	p := path.Clean("/" + slashed)
	if strings.HasSuffix(slashed, "/") && p != "/" {
		p += "/"
	}
	return strings.ToLower(p), true
}

var pathInTextRE = regexp.MustCompile(`[A-Za-z0-9_.@-]+(?:/[A-Za-z0-9_.@-]+)+/?`)

func ProtectedPathsIn(text string) []string {
	var out []string
	for _, tok := range pathInTextRE.FindAllString(text, -1) {
		candidate := strings.TrimRight(tok, ".")
		if slices.ContainsFunc(goModuleDirs, func(dir string) bool { return strings.HasPrefix(candidate, dir) }) {
			candidate = "go/" + candidate
		}
		if IsProtectedScope(candidate) && !slices.Contains(out, candidate) {
			out = append(out, candidate)
		}
	}
	slices.Sort(out)
	return out
}

var fileInTextRE = regexp.MustCompile(`[A-Za-z0-9_@-]{2,}\.[A-Za-z][A-Za-z0-9]*`)

var goModuleDirs = []string{"internal/", "cmd/", "pkg/", "acs/", "test/"}

func PathsIn(text string) []string {
	var out []string
	for _, re := range []*regexp.Regexp{pathInTextRE, fileInTextRE} {
		for _, tok := range re.FindAllString(text, -1) {
			candidate := strings.TrimRight(tok, ".")
			if pathShaped(candidate) && !slices.ContainsFunc(out, func(p string) bool { return strings.Contains(p, candidate) }) {
				out = append(out, candidate)
			}
		}
	}
	return out
}

func pathShaped(tok string) bool {
	return slices.ContainsFunc(strings.Split(strings.TrimSuffix(tok, "/"), "/"), func(seg string) bool { return len(seg) > 1 })
}
