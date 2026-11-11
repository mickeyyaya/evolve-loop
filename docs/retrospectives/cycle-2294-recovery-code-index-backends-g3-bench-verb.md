# Recovery of `code-index-backends-g3-bench-verb` in cycle 2294

- **Kind:** a recovery retrospective. The host wrote it from the committed cycle dossiers.
- **Item:** `code-index-backends-g3-bench-verb`. **Shipped:** cycle 2294, commit `54097188`.
- **Failed tries:** 2, since the last pass of the item.

## Tries

| Cycle | Verdict | Class | Fingerprint | First reason | Mark |
|---|---|---|---|---|---|
| 2188 | FAIL | — | — | `the integration tier ('go test -tags integration') reported 13 offender(s) locally. NOTE — host/CI parity gap: this host HAS tmux and CI runners do not, so every test guarded by requireTmux runs her` |  |
| 2281 | FAIL | `gate-block` | `build\|gate-block\|e0c6d5013170` | `review gate: phase "build" deliverable rejected after 1 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff: the tests of the changed packages fai` |  |
| 2294 | PASS | — | — | commit `54097188` | |

The mark "infrastructure?" shows the class `infra-error` or `unknown`. Such a failure can come from the host, not from the item.

## Difference

The host compares the last failed try (cycle 2281) with the passed try (cycle 2294).

- Phases in the passed try only: `test-amplification`, `audit` and `ship`.
- Phases in the last failed try only: `retro`.
- No phase changed its model.
- The passed try used no ladder rung above `R0`.
- The 2 failed tries have 1 fingerprint.

## Lesson

The item `code-index-backends-g3-bench-verb` failed in 2 tries with the classes `gate-block`. Cycle 2294 shipped it. Before a new try of a similar item, read the difference above.

## Judgment

The host states only the facts. Write the reason for the pass here.
