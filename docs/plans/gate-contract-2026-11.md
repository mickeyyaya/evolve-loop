# Plan: every gate is a typed unit with a conformance contract

> Design set: [ADR-0165](../architecture/adr/0165-every-gate-is-a-typed-unit-with-a-conformance-contract.md) · research: [gate-contract-2026-11.md](../research/gate-contract-2026-11.md) · spec: [gate-contract.md](../architecture/gate-contract.md).

The operator asked on 2026-11-10 for an improvement "from the fundamental unit" after the incident of 2026-11-09. This plan builds the gate contract of ADR-0165 in seven components. Each component is one pull request, and each gets an inbox item.

## Approaches

| Approach | Cost | Stops a repeat of 2026-11-09? | Holds for the next gate? | Score |
|---|---|---|---|---|
| A. Fix the known gates one by one | low | yes, for `testdup` only | no (research F1.2, F1.4) | 1 |
| B. A review checklist for gate changes | low | sometimes (F1.7: review missed one of four) | no test enforces it | 2 |
| C. A gate registry plus a conformance harness (chosen) | medium | yes | yes: each registry entry is tested | 5 |
| D. Run each gate only on the merged tree of the train | high | yes, after the merge | yes, but a lane gets no verdict that it can fix before ship | 3 |

C wins because it is the only approach that tests the next gate as well as the known ones. D stays the target of ADR-0147 and needs C for its lane verdicts to mean the same thing.

## Components

| # | Component | ADR-0165 decisions | Inbox item | Proof (red without the change) |
|---|---|---|---|---|
| G1 | Protected gate registry: name, kind, owners by artifact pattern, inputs with their trust class from `gate-input-inventory.md`, stage and a run function on `GateInput{Root, Base}`. Seed it with the scanner pack, `testdup` and the phase floors. | 1, 2, 15, 16 | `gate-contract-g1-registry` (filed) | A pack package with no entry fails. A lane diff that moves a stage back to shadow fails. |
| G2 | Call-graph hermeticity guard. It reaches each function from a registered run function and refuses git calls, `.git` reads and mtime reads outside `repostate`. `repostate` reads no mtime. `testdup` loses `Date`, `since`, `loadAcsAges`, the `HEAD` diff and the mtime walk. An `acs/` directory is new when the lane tree changes it against `Base`. The floors, `fleet` and the `phasecoherence` scanners move their git reads into `repostate`. | 3, 4 | `gate-contract-g2-hermetic-guard` (filed) | The current `testdup` fails. A fixture gate whose helper package runs `git log` fails. A gate on `TrackedTree` judges a staged new file that `Base` does not hold. |
| G3 | Parity harness: each entry runs on one fixture unstaged, staged and committed, with one base, and the three verdicts must match. The harness names the base of a main-side run. Diff-scoped gates diff against the base. | 2, 5 | `gate-contract-g3-three-state-parity` (filed) | The current `testdup` fails: its `HEAD` diff gives two verdicts. A gate that diffs against `HEAD` fails. |
| G4 | Base-red attribution at build, audit and ship, and the main-red stop on both landing paths with a fix exemption and a bound. | 10, 11, 12, 13, 14 | `gate-contract-g4-base-red-attribution` (filed) | The 2026-11-09 timeline replay below. |
| G5 | One TDD floor: a named check list with `tdd-helper-copy`, `tdd-comment-floor` and `unreachable-frozen-pin`. The host enforces it, and `phase verify tdd` binds the worktree from `run.json` and runs the same list. | 6, 7 | `tdd-self-check-worktree-checks-never-run` (filed) | The host rejects each of the three checks, with the predicate files untracked as TDD leaves them. `phase verify tdd --workspace <ws>` without `--worktree` rejects each of the three. An equality test of the two check lists. |
| G6 | Key baseline: `go/.test-dup-baseline` holds keys of a body hash and a site. It is an `authority` input on the protected surface. Each profile denies it, as the inventory test requires. Only the boundary prune step writes it. The predicates of cycle 2231 that read the count get a retire note. | 8, 9 | `testdup-equal-count-conflicts-across-lanes` and `testdup-baseline-date-is-a-lane-writable-off-switch` (filed) | A new site of a body with two baseline sites fails. Two fixture lanes that each remove one key merge clean. A lane diff of the baseline fails. |
| G7 | Protected inputs: a test checks that each registry entry has manifest rows for its file and each `authority` input. A `verified` input keeps its base-against-tree check from `gate-input-inventory.md`. | 16 | `gate-contract-g7-new-gate-protection` (filed) | An entry whose file or `authority` input is off the manifest fails. |

## Order

1. G1, because each other component reads the registry, and the registry must be protected from its first commit.
2. G4, because it saves the most cycle time. The incident lost 513 LLM minutes, and 333 of them were at a build floor.
3. G2 and G3 together, because the harness needs the guard to name the inputs.
4. G5.
5. G6, then G7, because G7 checks the manifest row of the baseline that G6 protects.

## Proof of the whole plan

- The current `testdup` fails G2 and G3 in a fixture, and it passes both after its G2 migration.
- A replay of the 2026-11-09 timeline:
  1. a lane on the red base `7452aeec4` with a green main parks nothing. It rebases onto main and runs again (cycle 2240);
  2. a lane on a red base with a red main parks with its continuation (cycles 2242 and 2249). The loop signals main-red;
  3. the fix lands while the stop holds, and the stop then releases.
- Each registered gate has an entry, a parity fixture and its manifest rows.

## Limits

- The harness proves parity for the fixtures that it builds, not for every tree.
- Base-red attribution reruns only the red tests. A red that depends on the interaction of two changes still needs the train (ADR-0147).
- The guard reaches the static call graph. A call through reflection or a plugin is outside it, and review still checks for it.
