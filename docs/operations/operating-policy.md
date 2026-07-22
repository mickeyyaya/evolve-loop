# Operating Policy (canonical, environment-independent)

> **This document is the canon.** Operator-session memory, CLI-local notes, and
> platform config are advisory mirrors of THIS file — a clean environment (new
> machine, new operator, any AI CLI) inherits the full policy by reading the
> repo alone. Machine-enforceable rules live as **compiled Go defaults**
> (`internal/policy`) with `.evolve/policy.json` as the override surface; this
> document carries the process rules no gate can fully mechanize, each with its
> evidence. Update it the way code is updated: through the sanctioned review +
> ship flow, with the incident that motivated the change cited.

## 1. Pipeline-integrity policy (highest severity)

A **pipeline-integrity defect** is anything that causes false cycle verdicts,
corrupts shared state, breaks gates/CI on main, or makes failures
undiagnosable. Its blast radius is every subsequent cycle.

1. **Fix directly, never queue-only.** A loop cannot repair its own foundation
   while standing on it. The defect gets an inbox item as the *record*
   (evidence, acceptance criteria), but a console/operator session executes
   the fix immediately: isolated worktree branch → TDD red-first →
   dual review (+ security review when trust-kernel-adjacent) → wiring proof →
   sanctioned commit-gate/ship → CI watch.
   *Evidence: cycles 999–1001 burned on the exact state defect the queue was
   being asked to fix; the console fix landed in hours (PR #345).*
2. **Maximum reasoning, implicitly.** Pipeline issues always get the deepest
   analysis and the highest model tier — for the operator session, for every
   review subagent, and for loop phases touching pipeline classes. Never
   default-tier a pipeline diagnosis.
   *Evidence: the verdict-incoherence family consumed five refuted fix
   attempts made from inference; one fingerprint-first deep pass found the
   real cause. Deterministic defect strings outrank every narrative.*
3. **The loop halts itself on blockers.** Forged verdicts halt instantly
   (ADR-0072 floor). Recurring failure fingerprints, guard-abort classes, and
   reason-less failure runs halt at policy ceilings (blocker breaker,
   `failure_policy.thresholds`). Every halt auto-files a P0 `pipeline-defect`
   item — the queue is injected even as the loop stops.
4. **Salvage before requeue.** On any failed/aborted cycle, inventory the
   preserved worktree before re-queueing; land recoverable work through the
   operator flow. *Evidence: batch-5's seven FAILs yielded three complete,
   landable work items (PRs #350/#351/#352) — zero re-implementation.*

## 2. Queue policy

1. **Never stop the queue.** Task-level failure → classify, learn, continue.
   Only SYSTEM-level failure halts (ADR-0072) — and the halt itself files the
   P0 fix item.
2. **Routing is typed plumbing, not prose** (ADR-0074). Inbox items carry
   `route`: `console-*` = operator-owned, structurally refused at plan time
   and at claim (exit 3); `"lane"` = explicit override for derivation false
   positives; absent = derived from the protected-surface manifest.
   *Evidence: prose fences were ignored twice on one item in batch-5; six of
   seven FAILs were control-plane draws the gate now makes impossible.*
3. **Weights are priority, routes are authority** — never conflate them.
   Pipeline-class items occupy the 0.85–0.97 band.
4. **Tracked-path main-tree writes happen only at batch boundaries** — from
   operators too. Only `.evolve/inbox/` is safe while lanes run.
   *Evidence: three innocent cycles (1011/1023/1027) died to mid-batch console
   writes; one (1043) to a mid-wave queue hold.*

## 3. Engineering standards (all work, loop or console)

1. **TDD red-first; every bug fix lands with a regression test.** A test that
   never failed proves nothing.
2. **Dual review before commit** (simplifier + language reviewer via
   commit-gate); architecture changes additionally get an adversarial
   architect review. Pure-docs diffs may skip the simplifier.
3. **Wiring proof (the I2 invariant).** A mechanism ships only with proof its
   output is consumed on the composed live path. Unit-green ≠ live-green.
   *Evidence: retrofile, the recurrence escalator, the claim→quarantine chain,
   and the S1 assembler all shipped inert exactly this way; two were caught
   only by live-fire monitoring.*
4. **No feature flags; config-injected policy.** Behavior comes from compiled
   defaults + `.evolve/policy.json` (Strategy/DI), never env-flag sprawl or
   Go literals for thresholds.
5. **Single source with projection.** Never a second writer/copy of state,
   vocabulary, or logic that can drift (statemap writers, enum vocabularies,
   the routing classifier).
6. **Git discipline.** All development on branches; bare `git commit`/`push`
   are gate-denied; explicit paths staged (never `git add -A` outside
   `/commit`); ships via `evolve ship` classes; releases via `evolve release`.
7. **Fail loudly.** Every degraded path WARNs with specifics; "missing
   artifact" errors are writer defects; silent narrowing is treated as a bug.

## 4. Failure handling (the routed-not-fatal contract)

Every FAIL produces: a machine-readable reason artifact (floor-written or
fallback-written by the retro paths), a deterministic failure digest
(fingerprint + pre-class + recurrence), and a retro `disposition.json`
(legitimacy / root-cause layer / salvage / urgency / routing — cross-checked
against the digest so identity cannot be invented). Honest rejections stay
rejected; the pass-rate metric is never bought by weakening judges — judgment
phases are excluded from remediation by hard deny-list.

## 5. Release policy

Release trigger: 4 consecutive PASS verdicts in a batch (or operator word).
One entry point: `/evo:publish` (`evolve release X.Y.Z`) — preflight gates,
changelog, atomic version bump, marketplace propagation, post-release CI watch
+ 15-asset verification, auto-rollback on failure. "Publish" ≠ "push".

## 6. Model & CLI routing

Tiers, not model names (`fast < balanced < deep < top`); any CLI × any phase ×
any model must execute; cross-family fallback on quota exhaustion; per-model
exhaustion regexes validated against real provider surfaces (false-positive-
safe: a spurious bench costs a pause, a missed signal costs a livelock).
Pipeline-class phases and reviews run deep/top.

## 7. Where the machine-enforced half lives

| Concern | Enforcement | Override |
|---|---|---|
| Gates (eval/contract/EGPS/tdd) | compiled defaults, `internal/policy` | `.evolve/policy.json` `gates`/`workflow` |
| Routing authority | `inboxbatch.ConsoleRouted` + plan-time gate + claim floor | item `route` field |
| Failure thresholds & breaker ceilings | `internal/policy` `failure_policy.thresholds` | `.evolve/policy.json` |
| Build handoff floor / remediation | `workflow.build_floor`, `remediation_rounds` | `.evolve/policy.json` |
| Protected surfaces | `guards.ProtectedSurfaceManifest` (compiled) | operator manual ship only |

Related: [runtime-reference.md](runtime-reference.md) ·
[control-flags.md](../architecture/control-flags.md) · ADRs 0064/0072/0073/0074/0075 ·
[lessons-and-resolutions-2026-07](../../knowledge-base/research/lessons-and-resolutions-2026-07.md)
(the incident evidence behind every rule above).
