# Cycle 1825: ship's carry rule refused the re-ship's own inbox consumption (wave 75, 2026-10-07)

## What happened

Cycle 1825 passed audit, met a moved main at ship, carried its audited verdict across a byte-identical rebase, and was then sealed FAIL by a false `INTEGRITY_TREE_DRIFT`. Nothing unaudited was in the tree. Ship's two drift rules each explained part of the drift, but ship tried them one at a time, and neither alone explained all of it.

### Timeline

| Step | What happened |
|---|---|
| Build | Cycle 1826 landed on main as `839a819ca` while 1825 was still in Build. |
| Audit | 1825's audit bound the base `b0860ce1` and the audited tree `92e7ba18`. |
| Ship attempt 1 | Ship committed the audited tree plus its inbox consumption, and the consumption rule accepted the drift. The ff-merge was refused because main had moved. |
| Unwind and rebase | The recovery unwound ship's commit, which took the consumption out, and rebased onto `839a819ca`. The identity carry wrote its record (plane ledger line 161629, `entry_seq` 161180, 2026-10-07 09:03:43Z): carried tree `fcfb917`. The record is correct: `92e7ba18 → fcfb917` is exactly 1826's 12 paths. |
| Ship attempt 2 | The verify-class check re-proved the carry on `fcfb917`. Ship then consumed the inbox again, as ADR-0105 B1 says it does, so the staged tree became `9a96ef`, which is `fcfb917` plus the 4 consumption paths. The pre-commit check refused the commit. |
| Outcome | The router classed the integrity-class error `integrity-block` (`router/recovery.go`) and the cycle was sealed FAIL. |

The refusal, verbatim (the plane's cycle-1825 run record; the path list is cut at 8 by design):

```
[INTEGRITY_TREE_DRIFT/integrity @atomic-ship] INTEGRITY BREACH (pre-commit): audit-bound tree SHA 92e7ba18c2f7b57321eccbc1f0ef02dd82d27e67 != staged tree SHA 9a96ef68112ad3e7f00212189465bce18204a020 — refused to commit; worktree changes preserved (staged) for operator triage (unsanctioned drift path(s): .evolve/evals/cli-dossier-publish.md, .evolve/inbox/2026-09-30T10-51-14Z-cli-dossier-publish.json, .evolve/inbox/consumed/2026-09-30T10-51-14Z-cli-dossier-publish.json, docs/architecture/packages/internal-dossier.md, docs/explain/builds/cycle-1826-01m4amd6bdtvc6t99arbqfr3nq.md, docs/operations/runtime-reference.md, go/acs/cycle1826/predicates_test.go, go/cmd/evolve/cmd_dossier.go and 4 more) (carry not re-proven: the carry of cycle 1825 names the tree fcfb917f613f4ad26487907a4b310877fd609eda, not 9a96ef68112ad3e7f00212189465bce18204a020)
```

Every "unsanctioned" path it names is one of 1826's landed paths.

### The three trees

| Tree | SHA | What it is | Which rule explains the step to it |
|---|---|---|---|
| Audited | `92e7ba18c2f7b57321eccbc1f0ef02dd82d27e67` | the lane's change on `b0860ce1`, as the audit bound it | — |
| Carried | `fcfb917f613f4ad26487907a4b310877fd609eda` | the same change on `839a819ca` (the carry record's `tree_state_sha`) | the carry: audited → carried is 1826's 12 paths, proven byte-identical |
| Staged | `9a96ef68112ad3e7f00212189465bce18204a020` | the carried tree plus the re-ship's consumption | the consumption rule: carried → staged is the 4 consumption paths |

## Root cause

Ship has two rules that excuse a drift from the audited tree, and `auditBindingSatisfied` tried them as alternatives:

- **The carry rule** (`carrySatisfied`, `go/internal/phases/ship/carry.go`) accepted only a held tree exactly equal to the carry record's tree (`boundCarryRecord`'s `rec.TreeStateSHA != actual` case).
- **The consumption rule** (`treeDriftExplainedByConsumption`, called from `worktree_integrity.go`) measured from the audited tree, so main's newly landed paths counted as unsanctioned.

So for any carried lane with inbox items, the re-ship stages carry-tree plus consumption, and neither rule accepts it. The post-push check (`verifyCommittedTree`) and the direct path's checks (`gitops.go`) go through the same `auditBindingSatisfied`, so they had the same gap.

The design said both things at once. ADR-0105 B1 says the re-ship consumes its inbox items again; B4's component table says the carry needs a record "that names the held tree" and that "the consumption explanation stays the next rule". The gap dates from `d68091650` (2026-09-27, the B4 commit). It stayed invisible while no carry reached ship's later stages: until #788 (2026-10-06) every carry was refused earlier, at the ledger Verify ([incident](2026-10-06-carry-refused-ledger-verify-f5.md)). 1825 is the first of the 8 live carries to reach the pre-commit check.

### Why the tests missed it

Every carry test built a lane with no inbox items, so the re-ship never consumed anything, and the held tree was always the carry's tree. The B4 tests stop at `auditBindingSatisfied` or `verifyAuditBinding`, which run before ship consumes; nothing drove a carried lane through `verifyStagedTree` or `verifyCommittedTree` (the open inbox item `carry-recovery-to-ship-end-to-end-proof` names exactly that gap).

## The fix

The carry composes with the consumption, in the one place both binding checks share (`carry.go`):

- `boundCarryRecord` no longer compares the record's tree with the held tree; it checks only that the record names the bound audit and the bound audited tree.
- `carryExplains` (new, called by `carrySatisfied`): when the held tree is not the record's tree, the drift from the record's tree must be only the re-ship's sanctioned consumption (`treeDriftExplainedByConsumption(rec.TreeStateSHA, held)`); otherwise the decline names the record's tree, the held tree and the paths the consumption does not explain.
- The carry is then re-proven on the record's own tree: the ledger chain, both ancestry edges, the byte identity and the patch-id. Before, the byte identity ran on the held tree, which was the record's tree by the exact-equality check.

`auditBindingSatisfied` is unchanged. The pre-commit check, the post-push check, the direct path's two checks and the verify-class predicate-execution check all call it, so the composition has one home and applies at every stage.

**A known limit, carried over rather than introduced.** `treeDriftExplainedByConsumption` checks which paths changed, not what they now hold. So any content at a sanctioned inbox path passes, measured from the audited tree (the old rule) or now from the carried tree. The composition grants no trust the consumption rule did not already grant. The exposure is bounded where the sanctioned set is built: its one writer (`consume.go`) only ever names `.evolve/inbox/<base>` and `.evolve/inbox/consumed/<base>` for a root-level `*.json` item found by task id, so no code path can match; the source bytes are provenance-pinned to the audit-bound tree before the move, and the destination bytes are ship-written JSON. The review of this change (go-reviewer, APPROVE, one MEDIUM) suggested closing it with a content or disjointness check on the consumed paths, as a follow-up to the consumption rule itself.

### The fallback: not built here

The brief asked that a combination ship cannot prove end as a recoverable re-audit, not `integrity-block`, if that is a small, local change. It is not:

- An unprovable carry already returns to audit, at the verify-class check (`verifyExecutionTree`, `AUDIT_BINDING_TREE_MISMATCH`, precondition class), which runs before ship stages its consumption.
- After the fix, the pre-commit check on a carried lane refuses only two shapes: a path neither rule explains, where integrity is the right class, or a carry that stops re-proving between the two checks (a ledger append that breaks Verify, a moved HEAD).
- Making the second shape recoverable needs the carry's state out of `auditBindingSatisfied` (five callers), and the re-audit route would first have to take ship's staged consumption out of the worktree. Today core's unwind handles only a ship commit on the fleet-rebase path, so the audit would otherwise bind the consumed tree. A pushed commit cannot be re-audited at all.

Filed as inbox `pre-commit-carry-refusal-returns-to-audit`.

## Tests (red first)

All in `go/internal/phases/ship/carry_consumption_compose_test.go`, on a lane whose inbox item is tracked on its base (`laneWithAnInboxItem`; `rebasedLane` now builds on `rebasedLaneOn`, so the two share one fixture):

| Test | Before the fix | After |
|---|---|---|
| `TestVerifyStagedTree_ACarriedRebaseShipsWithTheReShipsInboxConsumption` (the pin; its precondition re-proves the carry before consumption, as live) | **red**, with the live message: `(unsanctioned drift path(s): peer.txt) (carry not re-proven: the carry of cycle 1825 names the tree …, not …)` | green |
| `TestVerifyStagedTree_ACarryPlusConsumptionStillRefusesAnUnsanctionedExtraPath` (preservation) | green | green |
| `TestVerifyCommittedTree_ACarriedRebaseShipsWithTheReShipsInboxConsumption` (the post-push twin) | **red**, the same message as `INTEGRITY BREACH: … worktree-to-main tree drift` | green |
| `TestVerifyCommittedTree_ACarryPlusConsumptionStillRefusesAnUnsanctionedExtraPath` (the post-push preservation twin) | green | green |
| `TestCarrySatisfied_DeclinesWhatTheReShipsConsumptionCannotCompose` (a path the consumption does not sanction; a record whose patch-id is another change's; an audited tree that holds no change) | **red** (each declined with the old exact-tree reason) | green |

Mutation: 13 overlay mutants of the changed lines, all killed, after a control mutant proved the harness fails: exact equality restored; consumption measured from the audited tree; consumption measured from the held tree to itself; the re-proof skipped on the composed path; the re-proof run on the held tree; the consumption verdict inverted; the consumption verdict ignored; the post-push path left uncomposed; the pre-commit path left uncomposed; the tree condition inverted; the offending paths dropped from the reason; a consumption check that names no path accepted; the re-proof run on the audited tree. Two equivalent mutants were set aside: swapping the consumption's two trees (`diff-tree --name-only` is symmetric) and dropping its directory (a linked worktree shares the object store).

## Recurrence data

- Since 2026-09-29, 8 of 14 fleet rebases took the carry path.
- Every carried lane with inbox items to consume would have met this refusal once it reached ship's pre-commit check. Before #788, none got that far.

## What it taught

- **Two rules that each excuse part of a change must compose, not race.** A chain of "or" rules is right only when each rule explains the whole drift on its own.
- **A design that says "this happens again" in one step and "the old rule stays next" in another has to say how they meet.** ADR-0105 B1 and B4 were each true; together they described a tree neither rule accepted.
- **The fixture decides what the proof covers.** A carried lane with no inbox items never exercises the re-ship's consumption.

## Related

- [ADR-0105](../architecture/adr/0105-identity-preserving-fleet-rebase.md): B1, B4 and the 2026-10-07 amendment.
- [logic-first-delivery-design §5.8](../architecture/logic-first-delivery-design.md), C9.
- [phases/ship package page](../architecture/packages/internal-phases-ship.md).
- [The F5 incident](2026-10-06-carry-refused-ledger-verify-f5.md), the refusal that hid this one.
- Inbox `carry-recovery-to-ship-end-to-end-proof` (still open: no Run-level test drives the real recovery through ship) and `pre-commit-carry-refusal-returns-to-audit` (the fallback).
