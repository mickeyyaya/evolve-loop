# Factory pipeline findings: what building the evolve-loop factory taught (2026-09)

> **Purpose.** This report records what was learned while building the evolve-loop "factory". The factory is an autonomous, multi-phase LLM software pipeline (scout → triage → tdd → build → audit → ship). A Go orchestrator runs it as fleet waves of isolated lanes. Every finding cites the source it rests on. Findings live in `docs/`, never in code comments (`AGENTS.md` §5, `docs/conventions/code-comments.md`).
>
> **Version:** v1, 2026-09-26. **Status:** living document; §6 says how to extend it.

## 1. Purpose and scope

**What the pipeline is.** Each cycle walks a fixed spine of phase agents. Scout finds work, triage commits to it, tdd writes failing tests, build implements, audit grades the result (a different model family grades the builder), and ship lands it. A retrospective runs on FAIL, and a memo on PASS. Deterministic Go owns ordering, gates, ledgers and landing. LLM agents own the judgment (`AGENTS.md`; `docs/operations/pipeline-factory-rules.md`). The loop runs lanes in parallel, each in its own git worktree, from a runtime plane that the operator's console never writes to (`docs/architecture/adr/0080-runtime-console-plane-separation.md`).

**What this report records:** the load-bearing principles and the failures that taught them (§2), a failure taxonomy (§3), only the outcomes the sources measured (§4), and open problems and source disagreements (§5).

**How it was assembled.** From 58 files:
- 23 incident records on this tree, plus one on an unmerged branch.
- The regression coverage index.
- 15 ADR decision sections.
- 5 canonical policy files: `AGENTS.md`, `CLAUDE.md`, the operating policy, the factory rules and the runtime reference.
- 7 research and review records.
- The six per-package design notes the comment workstream has produced so far.

No number appears here that a cited source does not state.

## 2. Principles that proved load-bearing

Each principle names the failure class that taught it, the evidence, and the mechanism that now enforces it.

### P1. One belief, one home; everything else is a projection
- **Taught by:** "replicated beliefs without a coherence protocol", the root of 12 June defects. Six of them were a phase's identity hand-authored on six or more surfaces (`docs/research/campaign-retrospective-cycles-215-231-2026-06-06.md` §1–3). July confirmed it (`docs/research/lessons-and-resolutions-2026-07.md` §1).
- **Evidence since:**
  - Two pane-stripping seams drifted apart for six cycles (`docs/architecture/packages/internal-recovery.md`, cycle 1123).
  - The writer and the reader of the inbox retirement directories disagreed, so every shipped item read as `unknown` (F18, `docs/research/verification-wave-findings-2026-09-14.md`).
  - A safety guard and its extractor parsed the same bytes with two grammars (`docs/research/lessons-and-resolutions-2026-08.md` §2.1).
- **Enforced by:**
  - The operating-policy standard "single source with projection" (`docs/operations/operating-policy.md` §3.5).
  - Named single homes: `phasecontract.RegistryKey` and `phasecontract.OwedPath` (ADR-0100), `StripAgentContent`, `retirementStates` with `inboxbatch.CycleDirs`, and the one manifest projected as `IsProtectedSurface` and `IsProtectedScope` (F29).
  - Projection tests that run in both directions (`docs/architecture/adr/0100-declared-deliverables-gate.md`).

### P2. Deterministic work belongs to the host; the model owns judgment only
- **Taught by:** agents computing what the kernel already knew.
  - An auditor improvised the suite root and FAILed two correct builds (campaign retrospective, I-8(e), class B).
  - The triage agent almost never wrote its decision companion (`docs/architecture/packages/internal-triagecap.md`, cycles 308–322).
  - Triage's inbox claim was left to the persona and cost a re-dispatch in four cycles (`docs/incidents/2026-09-26-triage-claim-left-to-the-agent.md`, on branch `fix-host-claim`, not yet on this tree).
- **Enforced by:**
  - Core Rule 5 (`AGENTS.md`).
  - ADR-0044's "deterministic-first, LLM-last": the LLM advisor is only the escalation tail, and its diagnosis is promoted into a deterministic registry (`docs/architecture/adr/0044-unified-phase-recovery-protocol.md`).
  - ADR-0088's `Conclude`, which computes the verdict from link statuses and never reads prose (`docs/architecture/adr/0088-audit-chain-of-reasoning.md`).
  - ADR-0072's "orchestrator decides, Go enforces the floor", and agents never writing the signal stream (ADR-0101, decision 7).
  - `ProjectDecisionJSON`, and the host performing declared effects before every judge (F36). This matches the external finding that idempotency belongs at the effect layer, because LLM retries diverge (`docs/research/ai-factory-pipeline-resilience-2026-06-02.md`).

### P3. A proxy is evidence, never the verdict
- **Taught by** (class named in `docs/incidents/2026-08-12-proxy-as-verdict-findings.md`):
  - "closed" matched inside "disclosed" and caused four batch halts.
  - A `git-common-dir` parent was used as the state root.
  - HEAD movement was read as "this cycle shipped" (`docs/incidents/2026-09-12-shipped-via-build-sibling-landing.md`).
  - Pane text was read as a quota wall (`docs/incidents/2026-08-15-false-walls-and-repick-class.md`).
- **Enforced by:**
  - The ADR-0088 seven-link chain of reasoning. A missing link fails harder than a negative finding, and a link reported coherent without its evidence is downgraded.
  - The cycle's own ship latch, `CycleState.Shipped`.
  - `WallCorroborator`, which makes one live probe before accepting a wall.
- **First live result:** the chain, still in shadow, named cycle 1466 `deceptive` and cycle 1470 `derailed` for the right reasons (`docs/incidents/2026-08-15-false-walls-and-repick-class.md` §4).

### P4. Gate what production actually produces, and tell the producer what is owed
- **Taught by:**
  - A scanner bound untracked runtime mints and blocked three audit-green ships (`docs/incidents/2026-08-09-zero-ship-batch.md`).
  - The registry declared handoff files that 0 of 15 builds and 0 of 29 scouts produced (ADR-0100).
  - A disposition schema was never shown to the agents who had to write it (2026-08-09).
  - Prompt compaction left the auditor with 27% of its persona (`docs/incidents/2026-08-10-persona-strip-lobotomy.md`).
  - A minted phase was dispatched without its artifact path (`docs/incidents/2026-08-10-continuation-absorbing-fail.md`).
- **Enforced by:**
  - ADR-0084 I1: scanners bind only tracked state.
  - ADR-0084 I2: literal examples single-sourced against the production reader (`docs/architecture/adr/0084-gate-integrity-invariants.md`).
  - ADR-0100's `agent_owed`/`harness_produced` partition and declared effects, with owed paths rendered into the prompt.
  - The persona keep-guard, and the rule "grade the dispatched prompt, not the source file".

### P5. Verify the gate before the code
- **Taught by:** the pipeline manufacturing its own FAILs.
  - 7 of 11 consecutive FAILs (cycles 1596–1606) were the harness rejecting its own output (`docs/incidents/2026-09-03-auditor-mutates-the-worktree.md`).
  - Class A in `docs/incidents/2026-08-17-failure-rate-review-1481-1503.md`.
  - A ship gate that tested the project root and seeded from an empty index (`docs/incidents/2026-09-14-lane-ship-gate-package-scoped-tests.md`).
- **Mitigating fact:** these gates failed safe. They produced false FAILs, never false PASSes (`docs/incidents/2026-08-16-17-pipeline-hardening-campaign.md` §2).
- **Enforced by:**
  - The "gate-proof-before-code" guardrail (`docs/incidents/2026-08-10-continuation-absorbing-fail.md`, lesson 5; `CLAUDE.md`).
  - ADR-0084 I3: no gate fails without persisted evidence.
  - Checking in the live failing artifact as the regression fixture.

### P6. One verifier for the engine and the gate
- **Taught by:** verdict parity failures.
  - The runner classified the bytes the gate then salvaged, so a green cycle sealed FAIL (F22, cycle 1685).
  - A stale `acs-verdict.json` replayed into every repair round (cycle 1603, `docs/incidents/REGRESSION-COVERAGE-INDEX.md`).
  - Ship bound a sibling lane's audit (`docs/incidents/2026-08-26-audit-binding-fail-open.md`).
  - A verdict synthesized from scrollback contradicted the PASS on disk for 38 cycles (`docs/architecture/adr/0072-system-failure-policy-and-halt.md`).
- **Enforced by:**
  - `deliverable.Reviewer.VerifyForClassification`, shared by the engine and the gate.
  - The ADR-0072 coherence check, with an always-on halt floor.
  - Round-suffixed verdict archives.
  - Run-scoped audit bindings that fail closed.
  - One completion contract per decision (`docs/incidents/2026-09-14-router-proposal-read-the-scrollback.md`).

### P7. A cycle cannot edit, or draw, what grades it
- **Taught by:**
  - A build agent edited the gate that graded it, and the audit approved (`docs/architecture/adr/0064-pipeline-integrity-boundary.md`).
  - Batch-5 lanes drew work whose fix was on the control plane (`docs/architecture/adr/0074-typed-signal-contracts-routing-authority.md`).
  - Nine lanes drew one protected item (`docs/incidents/2026-09-14-triage-refusal-poison-loop.md`).
  - 18 FAILs traced to five items (F29).
  - A lane changed a protected file through a shell tool (`docs/incidents/2026-09-26-lane-edited-the-control-plane-through-a-shell-tool.md`).
- **Enforced by:**
  - `guards.ProtectedSurfaceManifest`, 99 fragments per `docs/operations/pipeline-factory-rules.md` §1, which feeds three refusals: plan time, claim time and triage commit time.
  - The role guard, and a protected-surface check on the build floor.
  - The P2 ship backstop, which now recovers into build.
  - Typed routing: `route`, and `pipeline-*` kinds routed to the console.
  - Deterministic refusals routed to the console on the first hit.
  - A Claude-family floor, so the builder's family never judges its own work (`docs/architecture/adr/0104-fallback-is-a-property-of-the-bridge-handle.md`).

### P8. Recorded state must be entailed by evidence
- **Taught by:** state that no longer matched what had happened.
  - PASS was recorded while the commit never landed (July lessons, meta-lesson 3).
  - Retirement did not ride the landing, so shipped work was re-picked (2026-08-15).
  - Triage bookkeeping defeated consumption (`docs/incidents/2026-08-24-wave2-consumption-id-linkage.md`).
  - Three dispatch stores each re-dispatched retired work (2026-08-16/17, meta-finding 1).
  - A shipped item was re-pinned to a lane (`docs/incidents/2026-09-15-shipped-item-re-pinned-and-sealed-lease-blocks-refresh.md`).
- **Enforced by:**
  - ADR-0074 I3: transitions are transactional with their evidence.
  - Consumption carried in the ship commit itself.
  - The ship latch.
  - Ledger records derived from diffs, not labels (operating policy §3.9).
  - Every writer of the hash-chained ledger going through the chained append (`docs/incidents/2026-08-11-verify-wave-halts-and-ledger-forensics.md`).

### P9. Settings are config, not code; no feature flags
- **Taught by:**
  - Phase identity lived in Go flow control (`docs/architecture/adr/0058-config-drive-transition-kernel.md`).
  - A `floor` key with no struct field enforced nothing (`docs/architecture/packages/internal-policy.md`).
  - One overloaded dial held a proven fast-fail in shadow (F27).
- **Enforced by:**
  - Operating policy §3.4.
  - Compiled defaults in `internal/policy`, overridden only by `.evolve/policy.json`.
  - `internal/config` as the one reader of routing dials, with a flag-ceiling ratchet (`docs/architecture/packages/internal-config.md`).
  - "No flags" and "no new dial" as ADR decisions (ADR-0099, ADR-0100).
  - Split dials: `spine_floor`, `fatal_pane`.
- **Exceptions:**
  - The trust anchors stay in Go on purpose: the legality graph and the audit PASS/WARN floor (ADR-0058 decisions 1 and 8).
  - Some routing env dials remain (for example `EVOLVE_DYNAMIC_ROUTING`, `CLAUDE.md`). `internal/config` is their only reader, and the ratchet forbids new ones.

### P10. Unit-green is not live-green: every mechanism ships with a wiring proof
- **Taught by:**
  - `retrofile` and the recurrence escalator shipped inert (ADR-0074; `docs/architecture/adr/0076-failure-disposition-boundary-escalation.md`).
  - Each ADR-0076 convergence slice's first design had a dead link (July lessons §6b).
  - A tier-escalation fix would have shipped off the dispatch path (`docs/incidents/2026-09-03-repair-rounds-blind-and-flat.md`).
  - A release went red because local test runs skipped the RealTmux family and nobody watched CI after the push (`docs/incidents/2026-08-25-submitverify-false-wedge-release-red.md`).
- **Enforced by:**
  - Operating policy §3.3.
  - Composition-root predicates: `DeclaredDeliverablesGateWired`, `SignalCenterWired`.
  - Source-scan pins.
  - Build-confirmed mutants per fix, counted in the coverage index.
  - Fixtures built through the production writers.

### P11. Fail loudly, with coded, durable signals
- **Taught by:**
  - A gate swallowed its output (`debug:""`) and turned a false RED into a forensic dig (2026-08-09).
  - A phase's own FAIL reason persisted nowhere (`docs/incidents/2026-09-13-phase-own-fail-reason-invisible.md`).
  - The ADR-0101 survey found at least 15 unwired sinks, three severity vocabularies and 361 hand-written stderr sites (`docs/architecture/adr/0101-signal-center.md`).
- **Enforced by:**
  - The Signal Center: closed module and kind sets, registered codes, drift raised as a signal, `signals.ndjson` per cycle, and listeners that observe but never decide.
  - ADR-0103 units, each with one module tag and its own codes (`docs/architecture/adr/0103-component-breakdown-program.md`).
  - `evolve signals codes check` in the build.
- **Validated:** in the first verification wave, the root cause was read from four stream lines (F1). The rule that came out of it: "every producer that stamps a code retires a regex".

### P12. Isolate lanes and planes; write shared trees only at boundaries
- **Taught by:**
  - Mid-batch console writes killed three innocent cycles (operating policy §2.4).
  - Six interference classes, W1–W6 (ADR-0080).
  - A console PR merged mid-wave caused a rejected lane push (F11).
  - The ship gate inherited the lane's IPC environment (`docs/incidents/2026-09-14-ship-gate-inherits-the-lane-ipc-env.md`).
  - A read-only auditor rewrote the builder's tree (2026-09-03).
- **Enforced by:**
  - The runtime/console plane split, with the inbox as the only cross-plane channel.
  - Per-cycle worktrees.
  - `treefence` for read-only phases.
  - `ipcenv.Scrub`.
  - `landingTree`, which tests the tree that will actually be pushed.
  - The boundary-only write rule.

### P13. Halt on system failure; route or quarantine task failure
- **Taught by:**
  - A 38-cycle false-FAIL storm with no progress monovariant (ADR-0072).
  - A 10-cycle zero-ship batch that evaded an identity-keyed ceiling (2026-08-09).
- **Enforced by:**
  - ADR-0072 categories. `verdict-incoherence` and `infra-systemic` always halt and auto-file a P0. Task failures quarantine at a ceiling.
  - The consecutive-failures breaker, with a compiled default of 3 (`docs/operations/runtime-reference.md`).
  - The goal-stall union breaker (internal-policy).
  - The refusal disposition table (factory rules §4).
  - The operator guardrail: two consecutive zero-ship cycles stop the loop (`CLAUDE.md`).

  "Never stop the queue" still holds: the queue is still injected while the loop halts.

### P14. A retry must change its inputs; salvage before requeue
- **Taught by:**
  - Ship probability fell by audit round, 100% → 50% → 17% → 0%, while every repair round repeated the same CLI, tier and brief (`docs/research/ship-rate-harness-reliability-2026-09-02.md`).
  - Lanes restarted cold: "not a capability gap but an economics gap" (`docs/architecture/adr/0076-convergence-architecture.md`).
  - A complete, audited implementation was stranded across more than 10 worktrees (2026-08-12, A1).
- **Enforced by:**
  - Repair briefs that carry the auditor's findings and escalate the tier (ADR-0096 via 2026-09-03).
  - Standing audit findings on re-entry (F16, F19).
  - Green-before-handoff, continuation-on-fail and recurrence tier escalation (ADR-0076).
  - Salvage before requeue (operating policy §1.4).

### P15. Treat every CLI as a drifting external interface
- **Taught by:**
  - A Claude update flipped the trust-dialog default to "No, exit" (`docs/incidents/2026-09-01-claude-2252-trust-default-flip.md`).
  - Plan-mode dialogs matched no rule (coverage index, 2026-08-27).
  - A session wall rendered with U+00A0 went unrecognized (F8).
  - A rejected codex model idled out the artifact window (F7).
  - Fixture text on the pane forged quota walls (2026-08-15).
  - A 529 error went unrecognized (`docs/incidents/2026-08-18-transient-529-inside-artifact-timeout.md`).
- **Enforced by:**
  - Bottom-anchored manifest rules, and a test that every rule compiles.
  - Validating regexes against the real pane.
  - Detection designed so the cheap failure is the likely one: a false positive pauses, a missed signal livelocks (operating policy §6).
  - The `model_unsupported` rule, which fails over at once.
  - ADR-0104: the fallback walk is a property of the bridge handle, and every chain ends with every available CLI.

## 3. Failure taxonomy

| Class | How it presented | Example sources | Fix mechanism | Pin (one per row; full set in the coverage index) |
|---|---|---|---|---|
| Replicated-belief drift | `load agent: no such file`; artifact timeouts; shipped items re-offered | campaign retrospective I-3–I-8; F18 | derive from one home (P1) | `dispatchstate_test.go` (F18) |
| Pipeline-manufactured FAIL | green work rejected: untracked stub, "disclosed", a one-byte `}`, parser shape | 2026-08-09; 2026-08-11; 2026-08-17 class A; 2026-09-03 | tracked-only scans, word-bounded matchers, `treefence` | `phasecoherence/unpaired_tracked_test.go` |
| Verdict incoherence | FAIL recorded while PASS sits on disk; repaired PASS downgraded | ADR-0072; F22; cycle 1603 | one verifier; coherence floor; round archives | `core/audit_round_artifacts_test.go` |
| Proxy used as a label | `SHIPPED_VIA_BUILD` with no ship; "phase-infra class" on a gate refusal | 2026-09-12; 2026-09-13 | own ship latch; C1 diagnostics | `core/shipped_via_build_own_ship_test.go` |
| Declared but unproduced, or produced but unread | a tier override with no producer; `retrofile` with zero callers; extinct handoff files | 2026-09-03 repair rounds; ADR-0074; ADR-0100 | registry partition; wiring proofs | `core/repair_tier_escalation_test.go` |
| Uncommunicated contract | 27% of the persona dispatched; schema never shown; path never disclosed | 2026-08-10 (both) | keep-guard; literal examples; contract footer | `phasecoherence/persona_strip_operational_test.go` |
| Retired-work re-dispatch | lanes burn on shipped or parked items | 2026-08-15; 2026-08-24; 2026-08-16/17 | consumption rides the landing; retirement across all stores | `phases/ship/consume_lanescope_union_test.go` |
| Control-plane draw or edit | a protected item re-drawn nine times; a shell edit reached audit | 2026-09-14 poison loop; F29; 2026-09-26 | three refusals, the build-floor check, first-hit console route | `core/build_floor_protected_integration_test.go` |
| Shared-tree or environment leak | innocent lanes killed; env-sensitive tests red only inside lanes | ADR-0080; 2026-09-14 IPC env | plane split, `ipcenv.Scrub`, `landingTree` | `TestRunNative_GateTestsTheLaneWorktreeNotTheProjectRoot` |
| CLI surface drift | modal hangs; walls unrecognized or forged; dead model | 2026-09-01; 2026-08-27; 2026-08-15; F7/F8 | anchored rules, corroboration, `model_unsupported` | `bridge/autorespond_planmode_test.go` |
| Missing fallback | a retro timeout sealed the cycle; a dead pane idled 900 s | 2026-09-14 retro; F27 | ADR-0104 walking handle; own `fatal_pane` dial | `TestEveryAgentProfileHasAFallbackChain` |
| Non-convergent or silent retry | 10 failed cycles before a halt; flat repair rounds; silent decline of an unknown class | 2026-08-09; 2026-09-03; F19 | volume breaker; findings plus tier on retry; coded decision signals | `core/blocker_breaker_consecutive_test.go` |
| Stale-artifact reuse | a retry certifies the prior attempt's leftover | coverage index (2026-08-25) | pre-dispatch baseline identity | `bridge/completion_baseline_test.go` |

## 4. Measured outcomes

Only numbers the sources state. Each is scoped to the window its source measured.

| Measure | Value | Source |
|---|---|---|
| June architecture campaign (cycles 215–231) | 17 started, 12 full pipelines; 12+ defects; 9 operator interventions; zero trust-kernel breaches | campaign retrospective §7, I-13 |
| False-FAIL storm (cycles 862–899) | 38 cycles; one task re-attempted 5×, another 8× | ADR-0072 |
| July batch pass rate | ~65% (24 PASS / 13 FAIL, batches 1–3); "238 prompt-injected lessons did not stop a single recurrence" | `docs/research/lessons-and-resolutions-2026-07.md` §1 |
| Batches 6–8 pass rate | stayed ~45% against a projected ~90% | ADR-0076 (convergence) |
| 2026-08-09 batch | 10 cycles, 0 ships; the new breaker stops such a run at 3 | 2026-08-09 incident; runtime reference |
| Persona strip | auditor kept 27%; 15 of 30 FAILs (cycles 1390–1429); 0 of 11 continuation passes | 2026-08-10 persona strip |
| Eval quality check | silently vacuous for 281 of 625 evals | ADR-0084 |
| Cycles 1481–1503 | 23 lanes, 3 ships; ≈74% raw fail rate, ≈50% on genuine attempts; zero forged PASSes | 2026-08-17 review §2–3 |
| Ship rate, cycles 1560–1605 | 9/46 = 19.6% against a ≥60% SLO; audit was the failing phase in 30 of 37 FAILs | ship-rate research §1 |
| Cycles 1596–1606 | 0 ships in 11; 7 of the 11 FAILs were the harness rejecting its own output | 2026-09-03 auditor incident |
| Triage refusals, cycles 1620–1688 | 18 FAILs from five items; lane-dispatchable queue went from 65 to 47 after the fix | verification wave F29 |
| Signal Center, wave 1 | registered codes went from 192 to 194; registry drift 0 | verification wave §2, §4 |
| Decomposition, after eight units | about 8% of core's stderr lines coded; `orchestrator.go` grew from 1169 to 1184 lines | `docs/architecture/decomposition/00-program-review-2026-09-14.md` |
| Regression coverage sweep (2026-05-29) | 73 failure modes: 40 pinned, 20 partial, 10 none, 3 untestable | coverage index summary |

**Not yet in `docs/`: the current ship streak.**
- The streak goal was 5 consecutive ships (verification wave, header).
- By 2026-09-26 it had become a "six-consecutive-ships campaign" (2026-09-26 control-plane incident).
- No source in `docs/` records the streak count. Recording it is an open item.

## 5. Open problems

Taken from the sources' "filed, not done" and open sections.

- **Center-less roots.** The resume and recovery paths and the operator-driven commands do not reach the signal stream (program review §1).
- **Gaps after F37.** Tracked as F38 (`docs/incidents/2026-09-26-lane-edited-the-control-plane-through-a-shell-tool.md`):
  - The role guard is blind to shell writes.
  - A stale cycle base after some rebases.
  - A blind rebuild when the build floor is off.
  - The manifest is not in a leaf package.
- **Routing residuals** (verification wave):
  - F35c: a declared directory inside a protected fragment.
  - F32: a lane has no alternate when its item is refused.
  - F30: scope-aware triage termination.
  - F13: route "unappliable by this phase" failures to the owning phase.
- **ADR-0100, filed not done:**
  - `dispatchEvaluateBatch` bypasses every reviewer.
  - A phase minted mid-cycle fails open.
  - Worktrees carry a tracked copy of the inbox.
- **ADR-0064 Pillar 2** (the honest metric) is still in progress. The semantic gaming classes rest on the LLM lens.
- **The transient 529.** The diagnostic landed; the fix that avoids the wasted window has not (2026-08-18 status).
- **Pending operator and baseline items** (verification wave, items 8–10): the codex deep/top re-pin needs the operator's cost decision; ACS baselines have drifted on main (`cycle1253`, `cycle1632`); `TestChannel_EndToEnd` flakes on timing.
- **`recovery.phase_recovery` is still shadow**, and splitting further dials off it is open (`docs/architecture/packages/internal-config.md`).
- **Vocabulary unification.** `failureadapter` and `failurelog` are not one vocabulary yet. The ADR-0101 follow-up names the gap, and F19 records that 13 classes are joined to 7 categories.
- **The host-claim fix (F36)** is not yet merged to this tree.

### 5.1 Where the sources disagree

1. **Zero-ship halt.** The factory rules §3.8 list "two consecutive zero-ship cycles halt the batch" as a breaker. The runtime reference lists only the consecutive-failures ceiling, 3. `CLAUDE.md` and the 2026-08-10 incident treat the two-cycle rule as an operator guardrail. Whether it is mechanized is unclear.
2. **Ledger chain-safety.** The coverage index marks it ❌ GAP, while `docs/incidents/2026-08-11-verify-wave-halts-and-ledger-forensics.md` records it fixed by #450 and proven live.
3. **Coverage index upkeep.**
   - Its summary (14 incidents, 73 modes, dated 2026-05-29) predates about 30 later rows.
   - The row for cycles 1634/1636 appears twice.
   - The 2026-08-09 incident still lists the retro completion cutoff as open, but the index and the persona-strip incident record it fixed by #432.
4. **ADR-0044's status line** names the dial `EVOLVE_PHASE_RECOVERY`. `internal-config.md` records that env flag as retired and the dial as policy-only.
5. **ADR-0101** is still marked "Proposed", yet its slices landed and the factory rules treat it as enforced. Its `[orchestrator]` stderr count reads 284 in Context and 287 in Consequences.
6. **ADR numbers collide.** 0076 and 0082 are each used twice.
7. **Batch 5.** ADR-0074 says "three of its four FAILs" (cycles 1028–1037). The July lessons and the operating policy say "six of seven" (1028–1043). The two sources cover different windows.
8. **The ~90% projection** in the July lessons was not realized: ADR-0076 reports ~45% for batches 6–8.
9. **ADR-0100.** Its verification section exercises `handoff-build.json`, which Decision 1 removed from the registry. It is probably a test-local registry, but the ADR does not say.

## 6. How to extend this report

This is v1. It grows as the comment workstream lands per-package notes in `docs/architecture/packages/`.

**Procedure for each new package note:**
1. Read its Findings section.
2. For each finding that evidences a principle, add the note's path to that principle's evidence. Do not copy the finding text: the package note stays its home (P1).
3. A finding that fits no principle, and recurs in at least two sources, becomes a candidate principle with a new §3 row.
4. Add numbers only with a cited source.
5. Keep pins in `docs/incidents/REGRESSION-COVERAGE-INDEX.md`, the source of truth for pins. This report links it and does not duplicate it.
6. Record new source disagreements in §5.1.
7. Bump the version line.

**Rolled up in v1:**

| Package note | Principles it evidences |
|---|---|
| `internal-config.md` | P9 (flag ceiling, split dials), P4 (tests target the file the code reads) |
| `internal-policy.md` | P9 (dropped `floor` key, one parser), P1 (`BaseCLI`), P13 (union breaker, halt ceilings) |
| `internal-profiles.md` | P10 ("pin a rule, not a list"), P4 (tracked-profile funnel), P15 (triage moved to codex on evidence) |
| `internal-recovery.md` | P1 (`StripAgentContent`), P2 (LLM diagnosis promoted into a deterministic registry) |
| `internal-router.md` | P2 ("model proposes, kernel disposes"), P4 (signals with no producer), P7 (`control-plane-rebuild`) |
| `internal-triagecap.md` | P2 (`ProjectDecisionJSON`), P8 (`PruneConsumed`, `## superseded`), P11 (a malformed companion fell through silently) |

**Also open:**
- This report is not yet listed in `docs/research-index.md`.
- The incident on the `fix-host-claim` branch becomes a live link once that branch merges.
