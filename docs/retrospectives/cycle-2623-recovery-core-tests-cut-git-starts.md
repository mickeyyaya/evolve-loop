# Recovery of `core-tests-cut-git-starts` in cycle 2623

- **Kind:** a recovery retrospective. The host wrote it from the committed cycle dossiers.
- **Item:** `core-tests-cut-git-starts`. **Shipped:** cycle 2623, commit `d75f2267`.
- **Failed tries:** 2, since the last pass of the item.

## Tries

| Cycle | Verdict | Class | Fingerprint | First reason | Mark |
|---|---|---|---|---|---|
| 2560 | FAIL | `gate-block` | `build\|gate-block\|784622c39439` | `review gate: phase "build" deliverable rejected after 2 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff: ./acs/cycle2560 (-tags acs): unit tes` |  |
| 2577 | FAIL | `gate-block` | `build\|gate-block\|c245fe6b9e73` | `review gate: phase "build" deliverable rejected after 2 correction(s): build handoff floor: 2 deterministic check failure(s) — fix these exactly before handoff: ./acs/cycle2577 (-tags acs): unit tes` |  |
| 2623 | PASS | — | — | commit `d75f2267` | |

The mark "infrastructure?" shows the class `infra-error` or `unknown`. Such a failure can come from the host, not from the item.

## Difference

The host compares the last failed try (cycle 2577) with the passed try (cycle 2623).

- Phases in the passed try only: `audit` and `ship`.
- Phases in the last failed try only: `retro`.
- No phase changed its model.
- The passed try used no ladder rung above `R0`.
- The 2 failed tries have 2 fingerprints.

## Lesson

The item `core-tests-cut-git-starts` failed in 2 tries with the classes `gate-block`. Cycle 2623 shipped it. Before a new try of a similar item, read the difference above.

## Judgment

The host states only the facts. Write the reason for the pass here.
