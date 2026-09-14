# Pipeline Factory Rules — what is secured, what agents own, and the anti-gaming protocol

> **Purpose.** The operator-facing digest of the rules the AI-driven pipeline enforces: which
> surfaces no agent may touch, what a lane handles on its own, and the protocol that stops an agent
> from gaming its own grades. Every rule names where it is enforced. The canonical, environment-
> independent policy is [operating-policy.md](operating-policy.md); this page projects it (plus
> ADR-0072/0074/0084/0099/0100/0101/0103 and `guards.ProtectedSurfaceManifest`) into one table set.
> A rendered copy is published as the "Pipeline Factory Rules" artifact (2026-09-14); **the repo is
> the source of truth** — when a rule and the code disagree, file a pipeline-integrity item.

| Tier | Meaning |
|---|---|
| **Secured** (operator-owned) | the control plane: gates, guards, rubrics, hooks, the ship path, releases, policy. An agent may propose; only a console session with a reviewed, gated `evolve ship --class manual` may change them. |
| **Delegated** (agents own) | everything a lane reaches through the spine — scout → triage → tdd → build → audit → ship — inside an isolated worktree, on inbox items routed `lane`, graded by gates the agent cannot edit. |
| **Protocol** (anti-gaming) | verdicts come from artifacts the pipeline verifies, never from what the agent says; every claim is cross-checked against evidence a different component produced; every failure is coded and signed on the stream. |

## 1. What is secured — the control plane

The list is a compiled manifest, `guards.ProtectedSurfaceManifest` (`go/internal/guards/integrity_surface.go`, 99 path fragments, each with its rationale). A cycle is forbidden to write these; the same list feeds three independent refusals (§3). A change lands only through the operator flow: isolated branch → red-first tests → simplifier + two reviews → `commit-gate` → `evolve ship --class manual` → CI.

| Surface | Why it is out of an agent's hands | Enforced by |
|---|---|---|
| `go/acs/regression/`, `internal/acssuite/` | the standing deterministic gates and their runner — the judges of every cycle | manifest · triage commit-time refusal · claim floor |
| `internal/guards/` | the guards themselves, this manifest, `role.go` | manifest (self-protecting) |
| `skills/audit/`, `skills/adversarial-testing/`, `skills/fable/`, `skills/solution-*` | the grading rubrics and the personas the kernel preloads on its own authority — an agent that edits its own rubric grades itself | manifest |
| `.claude/settings.json`, hook wiring | PreToolUse denies (in-process `Agent` during a live cycle, `git worktree`, bare commits) | manifest · ADR-0075 |
| Ship path: bare `git commit` / `git push origin main` | every landing carries an attestation, a gate run and a ship class (`cycle` = full audit binding, `manual` = operator) | ship-gate hook (denies) · `phases/ship/commitgate.go` |
| `.evolve/policy.json` gates & thresholds | eval / contract / repo-contract / EGPS gates default ON as compiled Go; the failure ceilings and breakers | `internal/policy` compiled defaults; operator override only |
| Registry SSOT (`phase-registry.json`, flag registry, campaign contract) | the phase catalogue, the metric ratchets — the loop's definition of "progress" | manifest · ratchet gate tests |
| Releases | `evolve release X.Y.Z`: preflight, changelog, atomic bump, 15-asset verify, auto-rollback; "publish" ≠ "push" | release preflight · operator word or 4 consecutive PASS |
| `internal/bridge/` | the drivers and the OS-sandbox fail-closed enforcement for versioned builds | manifest |

## 2. What agents handle — in the meantime

Agents own the work, not the judgment of the work. A lane draws an inbox item, plans it, writes the failing test first, builds, and is audited by a different model; the ship phase lands only what the gates verified. Nothing here needs an operator in the loop.

| Agents may | Bound by |
|---|---|
| Draw inbox items routed `lane` (or unrouted items whose fix surface is not protected) | plan-time gate + claim floor refuse `console-*` and protected derivations (ADR-0074); weights are priority, routes are authority |
| Write source in the cycle's own worktree | read-only phases are fenced (writes restored); writes outside the worktree are a guard abort |
| Author evals / predicates for the feature | the diversity checklist (`skills/adversarial-testing`): ≥1 workspace-inspecting command, ≥1 negative case, ≥1 edge case, no shared verb sets, the cheapest gaming fake named and made to fail |
| Build with TDD (the tdd phase precedes build; red first is the proof of understanding) | integrity floor: `ship ⇒ build ∧ audit ∧ tdd` (tdd waived only for trivial or document deliverables — ADR-0099) |
| Self-verify, write reports, file follow-ups into `.evolve/inbox` | reports are graded by the auditor and by machine gates; an item an agent files cannot override a protected derivation with `route:lane` |
| Land through the cycle ship class | audit binding, commit-claim coherence (ADR-0012), consumption rides the landing, repo-contract scanner (a lane cannot red main) |
| Learn: retrospectives on FAIL/WARN, memos on PASS, carryover todos | judgment phases are on a hard deny-list for remediation — the pass rate is never bought by weakening judges |

## 3. The anti-gaming protocol

Each rule closes one way an agent could make a cycle *look* successful; they are layered so that defeating one still trips the next.

1. **The verdict source is the artifact plane.** A phase with a deliverable contract is graded from the on-disk report and the executed predicates; the terminal pane is never classified (ADR-0072 coherence rule). An agent narrating success in its transcript changes nothing.
2. **Executed evidence, not asserted evidence.** ACS predicates run as Go; `acs-durable` re-executes them in CI; the eval gate requires executed acceptance evidence; a predicate that would also pass on an empty repo, a no-op change, a file that merely exists — each is a red-team case the auditor is briefed to catch.
3. **Three refusals for the control plane.** Plan-time (the wave planner's routed resolver), claim-time (`inboxmover` `Mover.Claim`, exit 3) and commit-time (triage refuses any top_n card whose files hit the manifest — even a card the LLM wrote from the scout's own backlog). The third exists because the first two only see inbox-declared files.
4. **Adversarial audit by a different model.** The auditor (Opus) grades the builder (Sonnet) with the goal-integrity rubric (ADR-0064) on metric-affecting cycles; explanation-review and solution gates check that what the report explains matches what the diff did.
5. **Declared effects are verified at the boundary.** A persona that must claim an inbox item, land a commit, or produce a deliverable has that effect checked by the gate that owns it (ADR-0100); a FAIL is issued when the effect is missing, not when the agent says it happened.
6. **Identity cannot be invented.** Every FAIL gets a deterministic digest (fingerprint, pre-class, recurrence) that the retro's `disposition.json` is cross-checked against; the inbox-lifecycle ledger is hash-chained (`prev_hash`, `entry_seq`); cycle numbers are never fabricated (CRITICAL violation).
7. **Every failure is coded and signed.** The Signal Center (ADR-0101, ADR-0103) tags each event with module, origin, kind, severity and a registered code — `evolve signals codes check` fails the build on an unregistered code — so "why did this cycle fail" is a few stream lines, not a transcript read.
8. **Breakers stop the retry treadmill.** A task-level failure counts toward the S5 ceiling and parks the item in quarantine; a system-level failure halts the loop and auto-files a P0; two consecutive zero-ship cycles halt the batch; and a deterministic refusal routes the item to the operator on the first hit (§4).

## 4. Failure handling and the newest breaker

Failures are *routed*, never fatal to the queue: a task-level FAIL classifies, learns and continues; only a system-level failure halts — and the halt files its own fix item. The classification decides whether the *item* pays or the *pipeline* does.

> **Added 2026-09-14 — the triage-refusal breaker** ([incident](../incidents/2026-09-14-triage-refusal-poison-loop.md)). One item drew nine lanes in a row (cycles 1650–1675): triage refused its card for naming a protected surface, but the refusal had no failure class, so the closeout read it as the pipeline's fault and put the item straight back. Now the triage gate stamps a structured code (`TRIAGE_PROTECTED_SURFACE`) and the card it refused on the C1 record; the classifier reads the record before any prose; the closeout routes that item to `console-manual` in place and says so on the stream (`INBOX_ITEM_ROUTED_CONSOLE`). No lane draws it again; the operator sees one WARN line naming the item, the surface and the disposition.

| Failure class | Who pays | What happens |
|---|---|---|
| build-fail · audit-fail · ship-gate-config | the item | `failure_count` bump; quarantine at the ceiling (ADR-0072 S5) |
| phase-refusal (coded, e.g. `TRIAGE_TOPN_EMPTY`) | the item | bump toward the ceiling — the gate said the work was malformed |
| phase-refusal `TRIAGE_PROTECTED_SURFACE` | the operator | routed console-manual on the first hit; the cycle's other items released untouched |
| phase-refusal `TRIAGE_COMMITMENT_INVALID` (I/O fault) · infrastructure · integrity-breach · exit-transport-hang | the pipeline | no bump; recurring fingerprints and guard aborts halt at policy ceilings; P0 auto-filed |

Where whose-fault lives: `cyclestate.RefusalDisposition` (the table beside the refusal vocabulary) and `cycleoutcome.IsTaskLevelResult`.

## 5. The operator's half

Pipeline-integrity defects — false verdicts, corrupted shared state, a red main, undiagnosable failures — are never queued only. A console session fixes them immediately at maximum reasoning: isolated worktree, red-first tests, dual review, wiring proof, sanctioned ship, CI watch. Salvage a failed cycle's worktree before re-queueing. Write tracked paths only at batch boundaries (`.evolve/inbox/` is the one path safe while lanes run). Release on four consecutive PASS verdicts or on the operator's word.

## 6. Where each rule is enforced

| Concern | Enforcement | Override |
|---|---|---|
| Gates (eval / contract / EGPS / tdd) | compiled defaults, `internal/policy` | `.evolve/policy.json` `gates`/`workflow` |
| Routing authority | `inboxbatch.ConsoleRouted` + plan-time gate + claim floor + triage commit-time check | item `route` field (operator-authored `lane` only) |
| Failure thresholds & breakers | `internal/policy` `failure_policy.thresholds`; the FAIL closeout (`cycleoutcome`) | `.evolve/policy.json` |
| Protected surfaces | `guards.ProtectedSurfaceManifest` (compiled) | operator manual ship only |
| Verdict coherence | the verdict engine (`phases/runner/verdict`) + ADR-0072 floor | none — the floor cannot be overridden |
| Failure notification | Signal Center: module-tagged, registered codes; console sink at WARN; `signals.ndjson` per cycle and per batch | none — wiring is non-optional |
| Prompt delivery timing | `internal/bridge/paste_settle.go` — one delivery tail for the prompt paste and injects; the settle and stability outcome recorded in the submit-verify ledger | none |
| Docs floor | `internal/docsfloor` + build handoff floor | `.evolve/policy.json` `docs_floor.stage` |

*Living document. Research behind it: [verification-wave findings, 2026-09-14](../research/verification-wave-findings-2026-09-14.md).*
