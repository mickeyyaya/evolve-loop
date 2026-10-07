# Pipeline Factory Rules — what is secured, what agents own, and the anti-gaming protocol

> **Purpose:** this page is the operator digest of the rules that the AI-driven pipeline enforces.
> It tells which surfaces no agent can touch and what a lane does on its own. It also gives the
> protocol that stops an agent that tries to game its own grades. Every rule names where it is enforced.
> The canonical, environment-independent policy is [operating-policy.md](operating-policy.md).
> This page projects that policy (plus ADR-0072/0074/0084/0099/0100/0101/0103 and
> `guards.ProtectedSurfaceManifest`) into one set of tables.

> A rendered copy is published as the "Pipeline Factory Rules" artifact (2026-09-14). **The repo is
> the source of truth**: when a rule and the code disagree, file a pipeline-integrity item.

| Tier | Meaning |
|---|---|
| **Secured** (operator-owned) | The control plane: gates, guards, rubrics, hooks, the ship path, releases, policy. An agent can propose a change. Only a console session with a reviewed, gated `evolve ship --class manual` can change them. |
| **Delegated** (agents own) | Everything that a lane reaches through the spine (scout → triage → tdd → build → audit → ship). The lane works inside an isolated worktree, on inbox items routed `lane`. Gates that the agent cannot edit grade the work. |
| **Protocol** (anti-gaming) | Verdicts come from artifacts that the pipeline verifies, never from what the agent says. The pipeline cross-checks every claim against evidence that a different component produced. The pipeline codes and signs every failure on the stream. |

## 1. What is secured — the control plane

The list is a compiled manifest, `guards.ProtectedSurfaceManifest` (`go/internal/guards/integrity_surface.go`, 123 path fragments, each with its rationale). A cycle must not write these paths. The same list feeds three independent refusals (§3). A change lands only through the operator flow: isolated branch → red-first tests → simplifier + two reviews → `commit-gate` → `evolve ship --class manual` → CI.

| Surface | Why it is out of an agent's hands | Enforced by |
|---|---|---|
| `go/acs/regression/`, `internal/acssuite/` | the permanent deterministic gates and their runner: the judges of every cycle | manifest · triage commit-time refusal · claim floor |
| `internal/guards/` | the guards themselves, this manifest, `role.go` | manifest (it protects itself) |
| `skills/audit/`, `skills/adversarial-testing/`, `skills/fable/`, `skills/solution-*` | the rubrics that grade the work, and the personas that the kernel preloads on its own authority. An agent that edits its own rubric grades itself. | manifest |
| `.claude/settings.json`, hook wiring | PreToolUse denies (in-process `Agent` during a live cycle, `git worktree`, bare commits) | manifest · ADR-0075 |
| `internal/commitgate/`, `internal/commentaudit/` | the commit gate and the comment rules that it trusts. These are the comment-only proof behind its review waiver, and the added-comment, removed-history and archive-rewrite refusals ([ADR-0115](../architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md)). | manifest |
| Ship path: bare `git commit` / `git push origin main` | every landing carries an attestation, a gate run and a ship class (`cycle` = full audit binding, `manual` = operator) | ship-gate hook (denies) · `phases/ship/commitgate.go` |
| `.evolve/policy.json` gates & thresholds | the eval / contract / repo-contract / EGPS gates, which are ON by default as compiled Go; the failure ceilings and breakers | `internal/policy` compiled defaults; operator override only |
| Registry SSOT (`phase-registry.json`, flag registry, campaign contract) | the phase catalogue, the metric ratchets: the definition of "progress" for the loop | manifest · ratchet gate tests |
| Releases | `evolve release X.Y.Z`: preflight, changelog, atomic bump, 15-asset verify, auto-rollback; "publish" ≠ "push" | release preflight · operator word or 4 consecutive PASS |
| `internal/bridge/` | the drivers and the OS-sandbox fail-closed enforcement for versioned builds | manifest |
| `internal/core/bridge_completion.go` | the one predicate that lets a correction re-dispatch complete on worktree evidence, with its deliverable carried unchanged ([ADR-0113](../architecture/adr/0113-correction-completes-on-worktree-evidence.md)). If someone widens it, it will let a lane carry its own stale deliverable past the baseline guard of the bridge. | manifest |

## 2. What agents handle — in the meantime

Agents own the work, not the judgment of the work. A lane draws an inbox item, plans it, writes a test that fails first, and builds. A different model audits the work. The ship phase lands only what the gates verified. Nothing here needs an operator in the loop.

| Agents can | Bound by |
|---|---|
| Draw inbox items routed `lane` (or items without a route whose fix surface is not protected) | The plan-time gate and the claim floor refuse `console-*` and protected derivations (ADR-0074). Weights are priority, routes are authority. |
| Write source in the worktree of the cycle | Read-only phases are fenced (the pipeline restores their writes). A write outside the worktree is a guard abort. |
| Author evals / predicates for the feature | The diversity checklist (`skills/adversarial-testing`): ≥1 command that inspects the workspace, ≥1 negative case, ≥1 edge case and no shared verb sets. The checklist also requires the agent to name the cheapest fake that games the check, and to make that fake fail. |
| Build with TDD (the tdd phase comes before build; red first is the proof of understanding) | The integrity floor: `ship ⇒ build ∧ audit ∧ tdd`. The floor waives tdd only for trivial or document deliverables (ADR-0099). |
| Self-verify, write reports, file follow-ups into `.evolve/inbox` | The auditor and the machine gates grade the reports. An item that an agent files cannot override a protected derivation with `route:lane`. |
| Land through the cycle ship class | audit binding, commit-claim coherence (ADR-0012), consumption rides the landing, repo-contract scanner (a lane cannot red main) |
| Learn: retrospectives on FAIL/WARN, memos on PASS, carryover todos | Judgment phases are on a hard deny-list for remediation. The pipeline never buys the pass rate with weaker judges. |

## 3. The anti-gaming protocol

Each rule closes one way in which an agent can make a cycle *look* successful. The rules are layered, so an agent that defeats one rule still trips the next.

1. **The verdict source is the artifact plane.** For a phase with a deliverable contract, the grade comes from
   the on-disk report and the executed predicates. The pipeline never classifies the terminal pane
   (ADR-0072 coherence rule). If an agent narrates success in its transcript, nothing changes.
2. **Executed evidence, not asserted evidence.** ACS predicates run as Go. `acs-durable` runs them again in CI.
   The eval gate requires executed acceptance evidence. A predicate can also pass on an empty repo, on a no-op
   change, or on a file that only exists. Each of these is a red-team case that the brief of the auditor tells it to catch.
3. **Three refusals for the control plane.**
   - Plan-time: the routed resolver of the wave planner.
   - Claim-time: `inboxmover` `Mover.Claim`, exit 3.
   - Commit-time: triage refuses any top_n card whose files hit the manifest. This applies even to a card that
     the LLM wrote from the backlog of the scout itself.

   The third refusal exists because the first two see only the files that the inbox declares.
4. **Adversarial audit by a different model.** On cycles that affect metrics, the auditor (Opus) grades the builder
   (Sonnet) with the goal-integrity rubric (ADR-0064). Explanation-review and solution gates check that the
   explanation of the report matches what the diff did.
5. **Declared effects are verified at the boundary.** Some personas must claim an inbox item, land a commit
   or produce a deliverable. The gate that owns each such effect checks it (ADR-0100). The gate issues a FAIL
   when the effect is not there, not when the agent says that it happened.
6. **Identity cannot be invented.** Every FAIL gets a deterministic digest (fingerprint, pre-class, recurrence).
   The pipeline cross-checks the `disposition.json` of the retro against that digest. The inbox-lifecycle ledger
   is hash-chained (`prev_hash`, `entry_seq`). Cycle numbers are never fabricated (CRITICAL violation).
7. **Every failure is coded and signed.** The Signal Center (ADR-0101, ADR-0103) tags each event with module,
   origin, kind, severity and a registered code. `evolve signals codes check` fails the build on an unregistered
   code. So "why did this cycle fail" is a few stream lines, not a read of a transcript.
8. **Breakers stop the retry treadmill.**
   - A task-level failure counts toward the S5 ceiling and parks the item in quarantine.
   - A system-level failure halts the loop and auto-files a P0.
   - Two consecutive zero-ship cycles stop the batch. This is an operator guardrail, not a compiled breaker.
     [operating-policy.md §4.1](operating-policy.md) states it once. (The compiled `consecutive-failures`
     rule halts at three cycles that fail.)
   - A deterministic refusal routes the item to the operator on the first hit (§4).

## 4. Failure handling and the newest breaker

Failures are *routed*, never fatal to the queue. A task-level FAIL classifies, learns and continues. Only a system-level failure halts, and the halt files its own fix item. The classification decides if the *item* pays or the *pipeline* pays.

> **Added 2026-09-14 — the triage-refusal breaker** ([incident](../incidents/2026-09-14-triage-refusal-poison-loop.md)).
> One item drew nine lanes in a row (cycles 1650–1675). Triage refused its card because the card named a
> protected surface. But the refusal had no failure class, so the closeout read it as a fault of the
> pipeline and put the item straight back.

> Now the triage gate stamps a structured code (`TRIAGE_PROTECTED_SURFACE`) and the card that it refused
> on the C1 record. The classifier reads the record before any prose. The closeout routes that item to
> `console-manual` in place and says so on the stream (`INBOX_ITEM_ROUTED_CONSOLE`). No lane draws it again.
> The operator sees one WARN line that names the item, the surface and the disposition.

| Failure class | Who pays | What happens |
|---|---|---|
| build-fail · audit-fail · ship-gate-config | the item | `failure_count` bump; quarantine at the ceiling (ADR-0072 S5) |
| phase-refusal (coded, for example `TRIAGE_TOPN_EMPTY`) | the item | a bump toward the ceiling: the gate said that the work was malformed |
| `triage-empty-commitment-claimable-work` (a lane whose triage committed nothing and left a pinned item unanswered) | the item | The closeout records the end at C1 with `TRIAGE_SCOPE_UNANSWERED`, so it classifies as a task-level refusal. (Before, it was `integrity-breach`, system-level: released, never bumped.) The closeout bumps each unanswered pin toward the ceiling (`committedset.Unanswered`: the pin minus what the decision answered). A deferral is owed work; a reasoned drop or an escalation is an answer. So a lane that continues to leave its item unanswered reaches quarantine, and the loop does not draw the item again every wave. A sequential cycle has no pin: its classification and batch halt are unchanged (R1c, 2026-09-29). |
| phase-refusal `TRIAGE_PROTECTED_SURFACE` | the operator | Since ADR-0106 R1 (2026-09-27), the host moves the card into `escalate_block`. The planned no-work closeout routes the item console-manual on the first hit. The other cards of the cycle continue, and the cycle is never a FAIL because of the refusal. The route can empty the top_n of a lane that is bound to one undeferred item. Then that item is escalated too, for any id that triage gave its cards (R1b, cycle 1757). The failure-path route remains for the fail-closed case. |
| phase-refusal `TRIAGE_COMMITMENT_INVALID` (I/O fault) · infrastructure · integrity-breach · exit-transport-hang | the pipeline | no bump; fingerprints that recur and guard aborts halt at policy ceilings; P0 auto-filed |

The decision about whose fault it is lives in `cyclestate.RefusalDisposition` (the table beside the refusal vocabulary) and `cycleoutcome.IsTaskLevelResult`.

## 5. The operator's half

Pipeline-integrity defects (false verdicts, corrupted shared state, a red main, undiagnosable failures) never go to the queue only. A console session fixes them immediately at maximum reasoning: isolated worktree, red-first tests, dual review, wiring proof, sanctioned ship, CI watch. Salvage the worktree of a failed cycle before you queue the item again. Write tracked paths only at batch boundaries (`.evolve/inbox/` is the one path that is safe while lanes run). Release on four consecutive PASS verdicts or on the word of the operator.

## 6. Where each rule is enforced

| Concern | Enforcement | Override |
|---|---|---|
| Gates (eval / contract / EGPS / tdd) | compiled defaults, `internal/policy` | `.evolve/policy.json` `gates`/`workflow` |
| Routing authority | `inboxbatch.ConsoleRouted` + plan-time gate + claim floor + triage commit-time check | item `route` field (operator-authored `lane` only) |
| Failure thresholds & breakers | `internal/policy` `failure_policy.thresholds`; the FAIL closeout (`cycleoutcome`) | `.evolve/policy.json` |
| Protected surfaces | `guards.ProtectedSurfaceManifest` (compiled) | operator manual ship only |
| Verdict coherence | the verdict engine (`phases/runner/verdict`) + ADR-0072 floor | none: the floor cannot be overridden |
| Failure notification | Signal Center: module-tagged, registered codes; console sink at WARN; `signals.ndjson` per cycle and per batch | none: wiring is non-optional |
| Prompt delivery timing | `internal/bridge/paste_settle.go`: one delivery tail for the prompt paste and the injects. The submit-verify ledger records the settle and stability outcome. | none |
| Docs floor | `internal/docsfloor` + build handoff floor | `.evolve/policy.json` `docs_floor.stage` |

*This document changes over time. The research behind it: [verification-wave findings, 2026-09-14](../research/verification-wave-findings-2026-09-14.md).*
