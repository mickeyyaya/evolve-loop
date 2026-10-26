# The narrow fence and held proposals

> Decision: [ADR-0157](adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md). Plan: [narrow-fence-2026-10](../plans/narrow-fence-2026-10.md). Research: [narrow-fence-2026-10](../research/narrow-fence-2026-10.md). Owning package notes: [internal-guards](packages/internal-guards.md), [internal-phases-ship](packages/internal-phases-ship.md).

This spec tells how the loop works on its own pipeline code. Two parts do it. The fence says which files a lane cannot change. The held proposal lets a lane change such a file, but the change waits for the operator before it lands.

## Contents

- [1. Goals](#1-goals)
- [2. The fence](#2-the-fence)
- [3. The held proposal](#3-the-held-proposal)
- [4. The approval](#4-the-approval)
- [5. What the operator sees](#5-what-the-operator-sees)
- [Guards](#guards)
- [Limits](#limits)

## 1. Goals

| Goal | Measure | Target |
|---|---|---|
| The fence keeps each guarantee | each gate file has an entry and a class | `TestProtectedGateFiles_EachKeepsItsEntryAndItsClass` passes |
| Lanes get the non-gate pipeline work | the lane-routable supply on a copy of the inbox | 27 or more (2 before) |
| A held change never lands without the operator | a held ship with no approval leaves `main` alone | the ship tests pass |
| No lane sets an approval | the writers of an approval | one: the operator verb |

## 2. The fence

`guards.ProtectedSurfaceManifest` is the list of the fence. Each entry has a path fragment, a guarantee class and a rationale. ADR-0157 lists the ten classes under "What stays protected".

The fence protects a file, not a package, unless each file of the package enforces the guarantee. Example: the bridge has 94 source files. The fence holds 18 bridge files with a sandbox, credential or kill rule, and 7 guard tests. A lane can change `driver_codex.go`, but a symbol pin keeps its sandbox check.

Four oracles keep the fence right:

1. `protectedGateFiles` (`go/internal/guards/integrity_surface_gate_files_test.go`) names each protected file and its class. A removed entry fails it.
2. `TestEveryGateShapedFileIsProtectedSurface` fails when a new `*_gate.go` or `*guard*` file has no entry.
3. `go/acs/regression/gatepins` fails when a pinned call loses its shape. It runs at audit.
4. `TestRefusalCallSitesAreFencedOrPinned` fails when a file outside the fence calls a refusal with no pin.

The supply census and the lane admission read the fence through `guards.IsProtectedScope`, so a narrower fence admits more items with no other change.

## 3. The held proposal

A held proposal is a lane cycle whose change needs the operator. The ship decides it at the landing, after the other landing checks and before any change to `main`.

The rule of `heldproposal.Decide`: a lane ship is held when one of these is true.

- An inbox item that the cycle claimed has a kind that starts with `pipeline-`.
- A path that the cycle changed is on the fence (`guards.IsProtectedSurface`).

The policy key `held_proposals.stage` selects what the ship does with a held ship:

| Stage | What the ship does |
|---|---|
| `off` | It does not check. |
| `shadow` (default) | It writes the record `.evolve/landing/held/cycle-<N>.json` with the status `would-hold`, emits `SHIP_HOLD_SHADOW` and lands. |
| `enforce` | With no approval for the tree, it writes the record with the status `held-for-operator`, emits `SHIP_HELD_FOR_OPERATOR` and stops with `HELD_FOR_OPERATOR`. The commit stays on the lane ref. `main` does not move. |

A manual ship (`--class manual`) is the console path, and the check does not run on it.

The record names the cycle, the commit, the tree digest, the reasons and the requirements. The requirements are the operator approval and an adversarial audit from a family other than the builder, at the deep tier.

## 4. The approval

`evolve landing approve --cycle <N> --tree <sha> --audit-report <path>` writes `.evolve/landing/approvals/cycle-<N>.json`. `evolve landing reject --cycle <N> --tree <sha>` writes a rejection. The approval holds the cycle, the tree digest, the decision and the time.

When the ship runs again for that cycle, for example through `evolve loop resume`, it reads the approval:

- An approval with the same tree digest lets the landing go on.
- A rejection stops the ship with `HELD_PROPOSAL_REJECTED`.
- An approval of a different tree counts as no approval. The ship holds again.

Four controls stop a lane from approving its own change:

1. The approvals live under `.evolve/landing/`, which is on the fence. The role guard denies a write there during a cycle.
2. A cell has a read-only repository. The OS sandbox denies the write.
3. `evolve landing approve` and `evolve landing reject` are class D verbs of the dispatch gate. A cell does not hold the dispatch credential.
4. A guard test closes the set of callers of `heldproposal.WriteApproval` to the operator verb.

## 5. What the operator sees

- `evolve landing list` prints each held record with its cycle, status, tree and reasons.
- `evolve wave status` adds "held for operator" to the line of a held cycle.
- The Signal Center carries `SHIP_HELD_FOR_OPERATOR` and `SHIP_HOLD_SHADOW`.

## Guards

| Rule | Guard | State |
|---|---|---|
| Each gate file keeps its entry and class | `TestProtectedGateFiles_EachKeepsItsEntryAndItsClass` | built (PR 2) |
| A new gate-shaped file needs an entry | `TestEveryGateShapedFileIsProtectedSurface` | built |
| A pinned sandbox call stays | `TestSandboxPinsHoldInLaneEditableFiles` | built (PR 2) |
| A held ship never lands without its approval | `heldproposal` and ship tests | PR 3 |
| No lane writes an approval | the caller-set guard, the fence and the dispatch gate | PR 3 |

## Limits

- A symbol pin checks that a name is referenced. It does not check that the code uses the result.
- PR 3 records the adversarial-audit requirement. It does not dispatch the audit. A held proposal builds the dispatch.
- At `shadow` and `enforce`, a lane still cannot claim a protected or `pipeline-*` item. The admission of such an item is a held proposal that comes after the dispatch.
- The explanation lifecycle call sites stay protected as a group.
