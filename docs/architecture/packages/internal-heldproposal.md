# internal/heldproposal

> Decision: [ADR-0157](../adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md). Design: [narrow-fence-and-held-proposals](../narrow-fence-and-held-proposals.md).

## Purpose

`internal/heldproposal` holds the rule, the record and the approval of a held proposal. A held proposal is a lane change that needs the operator before it lands. It changes a file on the fence, or it works an inbox item of kind `pipeline-*`. The package decides nothing about the landing itself. The ship check in `phases/ship/held_proposal_gate.go` calls it, and the operator verb `evolve landing` writes the approval.

## Design

- **The rule.** `Decide` holds a change when a claimed kind starts with `pipeline-` or a changed path is on the fence. It returns the sorted, unique reasons, for example `kind:pipeline-repair` and `protected:go/internal/guards/role.go`.
- **The record.** `WriteRecord` writes `.evolve/landing/held/cycle-<N>.json`. The status is `would-hold` at `shadow` and `held-for-operator` at `enforce`. The record names the commit, the tree digest, the reasons and the two requirements: the operator approval and the adversarial audit.
- **The approval.** `WriteApproval` writes `.evolve/landing/approvals/cycle-<N>.json` with the cycle, the tree, the decision and the audit report. `Judge` approves only an `approve` decision of the same tree. Another tree or another decision word is `Pending`.
- **The terminal status.** The verb sets the record to `approved` or `rejected` (`StatusAfter`), so `evolve wave status` stops the listing and a second decision is refused. The ship accepts an approval only when a record of the same tree is `held-for-operator` or carries the matching terminal status.
- **The home.** Both directories are under `.evolve/landing/`, which is on the fence. The role guard denies a write there during a cycle, and a cell has a read-only repository.

The ship check runs in `worktreeShip.checkLanding`, after the committed-document and foreign-predicate checks and before any change to `main`:

1. It does nothing for a manual ship or at stage `off`. When the cycle is not known, it refuses at `enforce` and does nothing at `shadow`.
2. It reads the kinds of the items in `.evolve/inbox/processing/cycle-<N>/` and the changed paths of the landing.
3. At `shadow`, a held change lands. The ship writes the `would-hold` record and emits `SHIP_HOLD_SHADOW`. If the record write fails, the event carries `record_error`.
4. At `enforce`, an approved tree lands. A rejected tree stops with `HELD_PROPOSAL_REJECTED`. Any other held change writes the `held-for-operator` record and stops with `HELD_FOR_OPERATOR`. The commit stays on the lane ref.

The policy key is `held_proposals.stage` (`off`, `shadow` or `enforce`; default `shadow`). If the policy cannot be read, the check holds as at `enforce`.

The operator verbs:

- `evolve landing list` prints each record.
- `evolve landing approve --cycle N --tree SHA --audit-report P --operator` approves a held tree. It refuses in a process that does not hold the dispatch credential, whatever the credential stage. It also refuses with no `--operator`, no audit report file, no `held-for-operator` record or a different tree.
- `evolve landing reject --cycle N --tree SHA --operator` rejects it.
- `evolve wave status` prints a `held:` line for each `held-for-operator` record, and its JSON has the `held` list.

## Invariants

- **Only the operator verb writes an approval.** `TestHeldApproval_OnlyTheOperatorVerbWritesAnApproval` (in `internal/guards`) closes the set of callers of `WriteApproval` to `cmd/evolve/cmd_landing.go`.
- **A cell cannot run the verb.** The verb refuses without the dispatch credential at each credential stage (`TestLanding_AProcessWithoutTheDispatchCredentialCannotDecide`). `landing approve` and `landing reject` are also class D verbs of the dispatch gate (`dispatch_gate_test.go`).
- **The approvals and the code are on the fence.** `TestLanding_TheApprovalsLiveOnTheFence` pins the two directories, this package, the ship check and the verb.
- **A held ship never moves `main`.** `TestWorktreeShipLand_AHeldShipNeverMovesMain` lands a protected change at `enforce` in a real repository and reads `main` after the refusal.
- **An approval lands only its tree.** `TestHeldProposal_AnApprovalLandsOnlyTheTreeThatItNames` and `TestJudge_OnlyAnApprovalOfTheSameTreeApproves`.
- **Each read failure fails closed.** `TestHeldProposal_EachReadFailureFailsClosed` covers the cycle state, the claimed items, the diff, the tree, the approval and the record write.

## Limits

- The package records the adversarial-audit requirement, and the verb asks for the report path. It does not dispatch the audit or check the family of its author. The inbox item `held-proposal-adversarial-audit-dispatch` builds that.
- A lane still cannot claim a protected or `pipeline-*` item. The inbox item `held-proposal-lane-admission` builds the admission at `enforce`.
- The resume of a prepared landing intent runs no gate. An intent that was prepared at `shadow` lands after a flip to `enforce`. Flip the stage only at a wave boundary with no prepared intent.
- A manual ship (`--class manual`) skips the check. It is the console path, and it needs the commit-gate attestation of the tree.
