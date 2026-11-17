# Recovery of `expected-signal-warning-names-its-cycle-and-phase` in cycle 2443

- **Kind:** a recovery retrospective. The host wrote it from the committed cycle dossiers.
- **Item:** `expected-signal-warning-names-its-cycle-and-phase`. **Shipped:** cycle 2443, commit `b681e55a`.
- **Failed tries:** 4, since the last pass of the item.

## Tries

| Cycle | Verdict | Class | Fingerprint | First reason | Mark |
|---|---|---|---|---|---|
| 2392 | FAIL | `gate-block` | `ship\|gate-block\|7d238a5a6e5c` | `phase ship: ship repo-contract gate: [REPO_CONTRACT_INFRA/precondition @atomic-ship] repo-contract importer backstop exited nonzero with no test-level failure (signal: killed) — not retried: the pac` |  |
| 2403 | FAIL | `gate-block` | `ship\|gate-block\|7d238a5a6e5c` | `phase ship: ship repo-contract gate: [REPO_CONTRACT_INFRA/precondition @atomic-ship] repo-contract importer backstop exited nonzero with no test-level failure (signal: killed) — not retried: the pac` |  |
| 2423 | FAIL | `gate-block` | `ship\|gate-block\|7d238a5a6e5c` | `phase ship: ship repo-contract gate: [REPO_CONTRACT_INFRA/precondition @atomic-ship] repo-contract importer backstop exited nonzero with no test-level failure (signal: killed) — not retried: the pac` |  |
| 2435 | FAIL | `gate-block` | `ship\|gate-block\|7d238a5a6e5c` | `phase ship: ship repo-contract gate: [REPO_CONTRACT_INFRA/precondition @atomic-ship] repo-contract importer backstop exited nonzero with no test-level failure (signal: killed) — not retried: the pac` |  |
| 2443 | PASS | — | — | commit `b681e55a` | |

The mark "infrastructure?" shows the class `infra-error` or `unknown`. Such a failure can come from the host, not from the item.

## Difference

The host compares the last failed try (cycle 2435) with the passed try (cycle 2443).

- Phases in the last failed try only: `retro`.
- No phase changed its model.
- The passed try used no ladder rung above `R0`.
- The 4 failed tries have 1 fingerprint.

## Lesson

The item `expected-signal-warning-names-its-cycle-and-phase` failed in 4 tries with the classes `gate-block`. Cycle 2443 shipped it. Before a new try of a similar item, read the difference above.

## Judgment

The host states only the facts. Write the reason for the pass here.
