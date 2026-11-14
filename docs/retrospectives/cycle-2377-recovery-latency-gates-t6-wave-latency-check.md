# Recovery of `latency-gates-t6-wave-latency-check` in cycle 2377

- **Kind:** a recovery retrospective. The host wrote it from the committed cycle dossiers.
- **Item:** `latency-gates-t6-wave-latency-check`. **Shipped:** cycle 2377, commit `47a37029`.
- **Failed tries:** 3, since the last pass of the item.

## Tries

| Cycle | Verdict | Class | Fingerprint | First reason | Mark |
|---|---|---|---|---|---|
| 2092 | FAIL | `gate-block` | `build\|gate-block\|24340c431be8` | `review gate: phase "build" deliverable rejected after 1 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff: 8 of 23 changed function(s) are below` |  |
| 2109 | FAIL | — | — | `defect ledger: this workspace holds no continuation manifest, but the root-owned /Users/danleemh/ai/claude/evolve-loop/runtime/.evolve/continuation-registry.json binds this lane's scope to cycle-2092` |  |
| 2362 | FAIL | `unknown` | `ship\|unknown\|c0b08d152afc` | `phase ship: ship: native: [GIT_PUSH_REJECTED/precondition @atomic-ship] ship: origin rejected the push of e63c2de1d407fdaa821ab5702581d6ee6771e14c because origin/main diverged; Audit must run again on` | infrastructure? |
| 2377 | PASS | — | — | commit `47a37029` | |

The mark "infrastructure?" shows the class `infra-error` or `unknown`. Such a failure can come from the host, not from the item.

## Difference

The host compares the last failed try (cycle 2362) with the passed try (cycle 2377).

- Phases in the last failed try only: `spec-verify` and `retro`.
- No phase changed its model.
- The passed try used no ladder rung above `R0`.
- The 3 failed tries have 2 fingerprints.

## Lesson

The item `latency-gates-t6-wave-latency-check` failed in 3 tries with the classes `gate-block` and `unknown`. Cycle 2377 shipped it. Before a new try of a similar item, read the difference above.

## Judgment

The host states only the facts. Write the reason for the pass here.
