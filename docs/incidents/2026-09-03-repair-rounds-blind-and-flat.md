# 2026-09-03 — Repair rounds were blind and flat: the auditor's findings never reached the builder, and the declared tier escalation could never fire

**Class:** pipeline-integrity (repair-loop convergence) · **Fix:** ADR-0096 · **Research:**
[ship-rate-harness-reliability-2026-09-02.md](../research/ship-rate-harness-reliability-2026-09-02.md) (gaps G1, G2, G10; proposals R1, R2)

## Issue

Over cycles 1560–1605 the autonomous ship rate was 9 / 46 (19.6 %) against a ≥ 60 % target, and
ship probability by audit-round count ran 100 % (1 round) → 50 % → 17 % → 0 % (4+). The in-cycle
repair loop (ADR-0092/0093) re-dispatched tdd/build after an audit FAIL, but every repair round was
a byte-identical retry: same CLI, same tier (`balanced`), same effort (`medium`), same prompt
structure — with the auditor's actual findings missing from the brief.

Live evidence:

- Cycle 1605, `audit-report.round2.md:109`: *"the round-2 repair treated the round-1 FAIL as a
  documentation finding — it rewrote one Rationale sentence in the explanation doc and left the
  report section and the dead seam untouched."* Round 3 headline: *"H1 (HIGH) — caller-proof hard
  floor violated for the third consecutive round."*
- Cycle 1596, `failure-decision.json`: *"the fourth-round Builder received one truncated round-2
  defect; … the inherited-dispositions block each occur zero times"* in `build-prompt.txt`.
- `llm-calls.ndjson` for 1605: build ×3 at `codex-tmux/balanced`, tdd ×3 at
  `claude-tmux/balanced`; `.evolve/profiles/builder.json` `effort_level: "medium"` on all three.

## Gap — why the existing net missed it

1. **A declared control with no producer.** `builder.json` and `tdd-engineer.json` have carried
   `model_tier_overrides.audit_retry_2plus: "deep"` since the override table landed. The only
   consumer, `subagent/modeltier.go activeSituation`, returned `""` for every cycle > 1 (its
   comment said the producer "remains inert until plumbed"); three tests guarded that the key's
   VALUE was a canonical tier, none that it fired. Worse, that resolver is not on the production
   dispatch path at all: `phases/runner` resolves the tier via `llmroute.Resolve` plus the advisor
   overlay `PhaseRequest.ModelRoutingTier`, and `subagent.Run` skips the adaptive resolver
   whenever `resolvellm` returns the profile tier — which it always does. A fix that plumbed the
   situation into `subagent.ResolveModelTier` would have shipped inert; it was built, then
   discarded on that finding.
2. **The brief read the wrong artifact.** `core/repair_eligibility.go seedAuditRepairContext`
   seeded the rebuild from `audit-fail-reason.json` — the coherence floor's deterministic gate
   strings ("EGPS: ship_eligible=false", "apicover -enforce flagged 2 line(s)"). The auditor's
   HIGH findings with root cause and `path:line` live in `audit-report.md`, which nothing parsed.
   The dashboard work (ADR-0095) had to build that parser for the operator's panel, which is how
   the absence on the agent side became visible.
3. **No forensic trail.** `build-prompt.txt` / `tdd-prompt.txt` were overwritten in place each
   round (only `acs-verdict.json` and `audit-report.md` were round-archived), so "what was round
   2 actually told?" could not be answered after the fact.

## Solution

- **R1 — escalate at the seam that fires.** `core.Orchestrator.repairRoundTier`
  (`retry_tier_escalation.go`, beside the ADR-0076 D floor) raises a tdd/build re-dispatch to the
  profile's declared `audit_retry_2plus` tier while `CycleState.AuditRepairActive` is set,
  clamped through `router.ClampPlanModelRouting`, applied as `PhaseRequest.ModelRoutingTier` on
  both dispatch surfaces (`cyclerun_dispatch.go`, `resume.go`). Effort follows the tier via the
  new profile `effort_overrides` (bridge `launch.go effortForTier`): builder deep → `high`,
  tdd-engineer deep → `xhigh`.
- **R2 — brief the findings, name the persisted ones.** `core.composeRepairBrief`
  (`repair_brief.go`) appends to the gate reasons the rejecting round's findings
  (CRITICAL/HIGH/MEDIUM, via `reportdoc.Findings` — the grammar shared with the dashboard and the
  audit gate) and marks each finding whose lead clause (`reportdoc.FindingKey`) also appears in
  `audit-report.round<N-1>.md` as **PERSISTED**. `archiveRepairPrompts` retires the previous
  attempt's prompt to `<phase>-prompt.round<N>.txt`.
- **Wiring proofs** (the I2 invariant, both through `RunCycle` with a live-shape auditor):
  `TestRepairRoundDispatch_RaisesBuildTierThroughLiveLoop` asserts the round-2 build request
  carries `ModelRoutingTier=deep` (and round 1 does not);
  `TestRepairRoundDispatch_BriefCarriesAuditorFindingsAndArchivesPrompt` asserts the round-2
  `Context[audit_repair_findings]` names the auditor's H1 and that `build-prompt.round1.txt`
  exists. Unit pins: declared-key-required, raise-only, envelope clamp through the real guardrail,
  PERSISTED marking, LOW omitted, no-clobber archival.

## Regression coverage

| Defect | Test |
|---|---|
| `audit_retry_2plus` declared but never produced | `core/repair_tier_escalation_test.go` (unit + live-loop wiring) |
| repair brief drops the auditor's findings | `core/repair_brief_test.go` `TestComposeRepairBrief_*`, `TestRepairRoundDispatch_BriefCarries…` |
| findings-only report (no gate reasons) rebuilt blind | `TestComposeRepairBrief_FindingsAloneStillSeed` |
| round prompts overwritten in place | `TestArchiveRepairPrompts_RenamesOnceNeverClobbers` + the live-loop proof |
| effort flat across an escalated tier | `bridge/effort_overrides_test.go` |

## What to watch

The dashboard's repair-round histogram (ADR-0095) is the instrument: if R1/R2 work, the
3-round column stops being a graveyard and the "carried" counts in the round history fall.
