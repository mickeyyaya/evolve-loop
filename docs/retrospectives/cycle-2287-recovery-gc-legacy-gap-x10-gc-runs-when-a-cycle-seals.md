# Recovery of `gc-legacy-gap-x10-gc-runs-when-a-cycle-seals` in cycle 2287

- **Kind:** a recovery retrospective. The host wrote it from the committed cycle dossiers.
- **Item:** `gc-legacy-gap-x10-gc-runs-when-a-cycle-seals`. **Shipped:** cycle 2287, commit `3b9fe5a8`.
- **Failed tries:** 3, since the last pass of the item.

## Tries

| Cycle | Verdict | Class | Fingerprint | First reason | Mark |
|---|---|---|---|---|---|
| 2238 | FAIL | `infra-error` | `build\|infra-error\|ae4cbae9fd8b` | `phase build: build: bridge: bridge: launch exit=81: artifact-timeout: cause=review_pause reason="agent hung: stalled for 3 consecutive busy intervals — fast-fail before 6-interval backstop" phase=bu` | infrastructure? |
| 2252 | FAIL | `unknown` | `ship\|unknown\|647627e835cc` | `phase ship: ship: native: [GIT_FLEET_REBASE_NEEDED/transient @atomic-ship] ship: fleet ff-merge cycle-cd3ae73e-2252 into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the me` | infrastructure? |
| 2271 | FAIL | `verdict-fail` | `audit\|verdict-fail\|931a6d991c19` | `the integration tier ('go test -tags integration') reported 6 offender(s) locally. NOTE — host/CI parity gap: this host HAS tmux and CI runners do not, so every test guarded by requireTmux runs here` |  |
| 2287 | PASS | — | — | commit `3b9fe5a8` | |

The mark "infrastructure?" shows the class `infra-error` or `unknown`. Such a failure can come from the host, not from the item.

## Difference

The host compares the last failed try (cycle 2271) with the passed try (cycle 2287).

- Phases in the passed try only: `spec-verify`.
- Phases in the last failed try only: `retro`.
- No phase changed its model.
- The passed try used no ladder rung above `R0`.
- The 3 failed tries have 3 fingerprints.

## Lesson

The item `gc-legacy-gap-x10-gc-runs-when-a-cycle-seals` failed in 3 tries with the classes `infra-error`, `unknown` and `verdict-fail`. Cycle 2287 shipped it. Before a new try of a similar item, read the difference above.

## Judgment

The host states only the facts. Write the reason for the pass here.
