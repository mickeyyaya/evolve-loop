# Batch Retrospective — Cycles 243–248 (2026-06-07): Six Cycles, Zero Ships

> Operator-led architecture post-mortem, written after halting the batch. Companion to the
> campaign retrospective (cycles 215–231); this one diagnoses the **ship-path**, where the
> campaign retro diagnosed the **belief-coherence** disease.
>
> The verdict in one line: **the pipeline optimizes for never shipping a bad change, with no
> architectural budget for shipping a good change despite noise** — every enforcement point is
> a cycle-killer, no enforcement point has a correction tier, audited work has no identity
> beyond its cycle, and the learning loop that should compound these lessons is the least
> reliable component in the system.

## 1. The failure ledger

| Cycle | Got to | Killed by | Class | Substance lost |
|---|---|---|---|---|
| 243 | tdd done | tree-diff guard: operator edited 2 profile files in the live tree mid-cycle (reverted in minutes; guard snapshot caught it) | operator/env | finished RED tests for step-6 bridge work |
| 244 | scout | operator pause (kill during stop), sealed by reset — no learning recorded by reset.go | operator/env | none (early) |
| 245 | mid-build | operator pause for refactor decision; abort cleanup ALSO deleted go/bin/evolve (2nd time) | operator/env | P0 guard-hardening build, salvaged manually |
| 246 | **audit PASS** → ship | SELF_SHA_TAMPERED: stale TOFU pin from operator manual ships vs rebuilt binary; ship died AFTER committing locally, BEFORE push (left main ahead-1) | integrity formality | Wave-2 (4 phases) fully audited; rescued as dangling `aea56ca` |
| 247 | **75 ACS green, red 0** → audit | audit FAIL at 0.98 confidence — *sole* cause: `build-report.md` missing the `challenge-token` header | provenance formality | Wave-2 re-land + **all of Wave-3** — rescued via reflog `a8ffa69` |
| 248 | tdd | operator stop for this retrospective | operator | none (work preserved in plan) |

Cross-cutting recurrences: LLM retro bridge timeout exit=81 **3/3 times** (every failure's lesson lost; operator hand-injected lessons via goal-text); agy builder timeout ×1 (fallback worked, +10 min); "spine not satisfied … fail-open" WARNs ×4 (artifact-name coherence class); `go/bin/evolve` deleted by abort cleanup ×2 (inbox: abort-cleanup-binary-wipe).

## 2. Architectural findings

### F1 — Enforcement is binary: every check is a cycle-killer, none has a correction tier
The system HAS a correction mechanism — contract-gate correction re-dispatch (PR #60,
`EVOLVE_CONTRACT_CORRECTION_RETRIES=2`) — but it covers **only deliverable-contract
violations**. The three killers today live outside it:
- **challenge-token in audit judgment** (cycle 247): a missing header in an otherwise
  fully-green cycle should trigger "builder: re-emit the report with the token" (a
  60-second correction), not a FAIL that costs a 50-minute pipeline re-run.
- **tree-diff guard** (cycle 243): benign-class churn (2 config files, content reverted)
  has no quarantine/recover tier; the only recognized benign class is the tracked-binary
  allowlist. Punishment = death of unrelated audited work.
- **SELF_SHA pin** (cycle 246): when the commit-gate attestation chain is otherwise intact,
  a pin mismatch is an operator-rebuild signature, not tampering — yet remedy is manual
  state surgery and the audited ship dies.

### F2 — Audited work has no identity beyond its cycle (no content-addressed trust)
A PASS audit binds to the *cycle*, not the *tree*. Cycle 247 had to re-run the full
pipeline to re-land bit-identical content that cycle 246 had already audited; cycle 248 was
re-running it a third time. The provenance machinery to fix this **already exists** (challenge
tokens, step-5 provenance headers `{cycle, phase, tree_sha, inputs_digest}`): bind audit PASS
to tree-SHA; a re-land whose tree-SHA matches an audited verdict fast-paths to ship.
Formality failures become 5-minute re-ships instead of 50-minute re-pipelines.

### F3 — The learning loop is the least reliable component while being the most load-bearing
Retro bridge failed 3/3 today; reset.go learns nothing; loop fatals learn nothing. Each
failure was therefore re-discovered by the next cycle (or hand-carried by the operator in
goal-text — a human doing the machine's job). **Status: the fix is built** — the concurrent
failure-floor session completed all 5 phases of the Advisor-Aware Failure Routing plan
(branch `worktree-failure-floor`, faillearn substrate → deterministic failure floor →
advisor failure vocabulary → `policy.json:failure_floor` → ADR-0039). Landing that PR is
the single highest-leverage merge available.

### F4 — Operator and loop share one mutable environment
Three of six failures trace to the operator sharing the checkout with the loop (profile
edit → 243; binary rebuilds vs TOFU pin → 246; pause kills → 244/245; plus cwd accidents
creating phantom scaffolding). The project's own playbook has the answer — the
**clone-on-branch campaign isolation pattern** (domain-phase-catalog campaign) — and it was
not used. Operator changes must flow through inbox/carryover (which worked every time it
was used today); batches should run in a dedicated clone when an operator session is active.

### F5 — Ship is non-transactional: it can die half-way and strand state
Cycle 246's ship committed to local main, THEN died at verify-self-sha, leaving main ahead-1
unpushed (`f01a323`) — a state that confused every subsequent observer. Ship's steps
(verify-sha → verify-attestation → commit → merge → push → re-pin) need
all-or-nothing semantics, or at minimum a recorded "ship-in-progress" marker with a
deterministic resume (the ship-closure-idempotency work fixed report regeneration but not
the commit/push atomicity).

## 3. Prescriptions (ranked by ship-unblocking power)

| # | Prescription | Mechanism | Status |
|---|---|---|---|
| P0 | **Ship the stranded product NOW** via the sanctioned manual path | cherry-pick `a8ffa69` (Wave-2+3, audited green in 247) onto main; full /commit gate (simplifier + reviewer + commit-gate attestation + `evolve ship --class manual`); push includes `f01a323` | THIS SESSION |
| P1 | Graduated enforcement: correction tier before cycle-death | extend correction re-dispatch to (a) missing/incorrect challenge token → builder re-emits report; (b) benign tree-churn classes → quarantine+warn; (c) SELF_SHA mismatch + intact attestation → warn+repin | inbox (new) |
| P2 | Content-addressed audit reuse (fast re-land) | bind audit verdicts to tree-SHA via existing provenance headers; ship accepts SHA-matched audited trees without pipeline re-run | inbox (new) |
| P3 | Deterministic failure floor + advisor failure vocabulary | land branch `worktree-failure-floor` (ADR-0039, all 5 phases complete) | other session: create PR + merge |
| P4 | Operator/loop isolation | batches run in a dedicated clone (clone-on-branch pattern); operator edits only via inbox/carryover; document in runtime-reference | inbox (new) |
| P5 | Ship transactionality | all-or-nothing ship steps or ship-in-progress marker + deterministic resume | inbox (new, fold into ship-executor backlog) |

## 4. What went RIGHT (keep these)

- **Nothing was ever lost.** Worktrees + dangling commits + reflog preserved every artifact
  of every killed cycle. Work-preservation architecture is sound.
- **Every defense fired correctly**: tree-diff guard caught a real (if benign) mutation;
  the adversarial codex auditor enforced provenance honestly; cross-CLI fallback rescued the
  agy builder; leak-recover discarded binary churn; the integrity floor clamped a tdd-skip.
  The components work — the *composition* lacks proportionality (F1) and memory (F2, F3).
- **Advisory routing behaved**: plans were sensible, vetoes held, no cycle-238-style
  pile-ons; the lesson-injected goal-text in cycle 248 produced the cleanest plan of the day.

## 5. References

- Campaign retrospective: `knowledge-base/research/campaign-retrospective-cycles-215-231-2026-06-06.md`
- Refactor plan: `~/.claude/plans/gleaming-finding-kahn.md` → branch `worktree-failure-floor` (ADR-0039)
- Sealed runs: `.evolve/runs/cycle-24{3,4,5,6,7,8}*` (243/244/245/248 carry operator-written
  retrospective records per the retro-always discipline; 246/247 have orchestrator failure records)
- Evidence log: `/tmp/evolve-batch-20260607.log` (machine-local)
- Inbox items born today: `abort-cleanup-binary-wipe`, `retro-always-invariant` (parked → P3 supersedes)
