# Recovery of `recycle-x3-archive-leaf-writer` in cycle 2619

- **Kind:** a recovery retrospective. The host wrote it from the committed cycle dossiers.
- **Item:** `recycle-x3-archive-leaf-writer`. **Shipped:** cycle 2619, commit `e854f84d`.
- **Failed tries:** 3, since the last pass of the item.

## Tries

| Cycle | Verdict | Class | Fingerprint | First reason | Mark |
|---|---|---|---|---|---|
| 2576 | FAIL | `verdict-fail` | `audit\|verdict-fail\|ac639195d500` | `the integration tier ('go test -tags integration') reported 4 offender(s) locally. NOTE — host/CI parity gap: this host HAS tmux and CI runners do not, so every test guarded by requireTmux runs here` |  |
| 2589 | FAIL | `verdict-fail` | `audit\|verdict-fail\|ac639195d500` | `the integration tier ('go test -tags integration') reported 4 offender(s) locally. NOTE — host/CI parity gap: this host HAS tmux and CI runners do not, so every test guarded by requireTmux runs here` |  |
| 2593 | FAIL | `unknown` | `ship\|unknown\|64ed90dec2ed` | `phase ship: ship: native: [GIT_FLEET_REBASE_NEEDED/transient @atomic-ship] ship: fleet ff-merge cycle-cd3ae73e-2593 into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the me` | infrastructure? |
| 2619 | PASS | — | — | commit `e854f84d` | |

The mark "infrastructure?" shows the class `infra-error` or `unknown`. Such a failure can come from the host, not from the item.

## Difference

The host compares the last failed try (cycle 2593) with the passed try (cycle 2619).

- Phases in the passed try only: `test-amplification`.
- Phases in the last failed try only: `retro`.
- No phase changed its model.
- The passed try used no ladder rung above `R0`.
- The 3 failed tries have 2 fingerprints.

## Lesson

The item `recycle-x3-archive-leaf-writer` failed in 3 tries with the classes `verdict-fail` and `unknown`. Cycle 2619 shipped it. Before a new try of a similar item, read the difference above.

## Judgment

The host states only the facts. Write the reason for the pass here.
