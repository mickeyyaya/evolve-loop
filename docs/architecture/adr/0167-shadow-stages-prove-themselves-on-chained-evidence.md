# ADR-0167: Shadow stages prove themselves on chained evidence

- **Status:** Proposed (2026-11-15). The operator asked for this record on 2026-11-15: "Start from the research, design, ADR, then building directly from console."
- **Plan:** [shadow-stage-evidence-2026-11.md](../../plans/shadow-stage-evidence-2026-11.md). **Research:** [shadow-stage-evidence-2026-11.md](../../research/shadow-stage-evidence-2026-11.md). **Design:** [shadow-stage-contract](../shadow-stage-contract.md) (the spec of record). **Card:** [integration-driver-enforce](../integration-driver-enforce.md).
- **Evidence:** research F2.1 to F2.8. The two that force the decision:
  - shadow decisions live in a plain append file, so no one can check a `ready` verdict (F2.8);
  - three shadow features give no signal, and two escapes in about 62 scans stop the scan skip from flipping (F2.5, F2.2).

## Relation to other records

- **Amends ADR-0157 decision 8** ([ADR-0157](0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md)). Fix F4 of the spec makes the hold use the routing predicate. A `pipeline-*` item that routing sends to a lane then lands as a normal PASS, unless it touches a protected path. The text of ADR-0157 stays unchanged. The same change as F4 adds an "Amended by ADR-0167" note to it.
- **Extends ADR-0165** ([ADR-0165](0165-every-gate-is-a-typed-unit-with-a-conformance-contract.md)). A stage is a gate, so decisions 15 and 16 of ADR-0165 apply. Only a held proposal or the operator moves a stage. The registry is a protected surface.
  - This record adds the proof that a stage move needs. It adds no second registry and no second protected surface. It extends `gatecontract.Entry` and the seeds of `gateregistry`.

## Context

The pipeline has many stage keys. Each key runs a feature at `shadow` before the operator flips it to `enforce`. The review of 2026-11-15 found seven faults. 10 keys are already at `enforce`, and none has a flip record (spec section 6.2).

Three features gave no signal. Two copies of one rule disagreed. The main-red stop saw only `go test` keys, so a red lint step on main stood for 5 commits. The ladder record "unknown at R0 stop R0" comes from `nextItemStartRung`, a hand-coded copy of the decision outside `retryladder`.

The common cause is that no rule says what a shadow stage must emit, compare and prove before the flip. The operator then ranked the evidence first: "The evidence and contract is the core of this feature, we must keep both intact and be able to verify, otherwise the whole pipeline is untrustworthy." Evidence in a plain file fails that test. The registry and the tests fail it too, because a lane can weaken them.

## Decision

The five contract rules (S1 to S5) apply to each stage key. The spec gives the detail of each rule and does not repeat here.

1. S1: a stage emits one registered signal for each decision, with its `would_*` fields. A `shadow` equal to `off` is refused.
2. S2: `shadow` and `enforce` run one decision function. Only the decider builds a decision value.
3. S3: the enforce branch is a typed function value in the registry entry. `core` and `ship` each own a `StageActor` port, and `cmd/evolve` builds the entries and injects them. A wiring test drives the stage at `enforce` through the production entry point and asserts that a spy acted. A test of symbol existence does not satisfy S3.
4. S4: good behavior can meet the promotion condition. A scorer counts false stops, escapes and conflicts.
5. S5: a rule that two features share has one predicate.

The integrity rules protect the evidence and the contract:

6. E1: each decision is an evidence record in a hash chain. An edit, a deletion or a reorder fails verify.
7. E2: the decider adds one to an in-memory decision counter before it writes. The seal commits the counter, the record count and the digest. The seal home is in spec section 10.2 (E2). It is its own unconditional `defer`, registered before the `abnormalEpilogue` defer. It writes one `stage-evidence-seal` entry on every exit. A gap, or a failed write, marks the cycle `incomplete`.
8. E3: the verdict is a pure function of the chain. A broken chain gives `not ready`.
9. E4: each record names the decision version.
10. The version is a content digest of the decider package closure and its policy subtree.
11. It is not the build id or the whole policy. A build id changes each cycle and keeps evidence below the sample size.
12. C1: the registry, its baseline and the contract tests are on the protected surface of ADR-0165, with a ratchet. The baseline of known violations may only shrink.
13. C2 and C3: a new `enforce` needs a verified flip record. The loop-start preflight checks this. The `policy` loader does not. `evolve stage status` shows every rule as pass or fail.

The choices of the plan:

14. The hold uses the routing predicate. A protected path still holds. This amends decision 8 of ADR-0157.
15. The evidence store is a per-cycle digest that the seal entry of the cycle commits to.
16. The scan-skip key uses one purity predicate, shared with the TIA (the closure of `regressiontia.escapeHatches`). The cache is default-deny: only a package that the predicate proves pure is cacheable. A hand list holds overrides only. An escape proposes an override and never adds a package to the cache.
17. A stage flips only on a `ready` verdict from `evolve stage verify` on an intact chain.
18. Contract tests ship with a baseline of known violations. A fix removes its entries. Only the operator adds one.
19. Migration: `evolve stage migrate` writes one grandfather flip record for each key already at `enforce`, and one genesis record. C2 applies to new flips. The preflight check is inactive until the genesis record exists, so the loop never refuses its own policy.
20. F6 deletes `nextItemStartRung`. The ladder decision comes from `retryladder.Next`. The ADR-0076 floor stays a separate named rule. The build tier is `max(tier of the ladder target, floor tier)`. No ADR is reversed.
21. A redelivery at `shadow` is information, not a failure (card part 0). Write-once at `enforce` stays in the card's PR (c). The clean-wave count of `integration.stage` is the `Promotion` data of its registry entry. The scorer counts chained pool outcomes.

## Consequences

- Good: a `ready` verdict is a claim that anyone can recompute from the chain. A tamper shows at verify.
- Good: a stage that records nothing scores `not ready`. It cannot flip by silence.
- Good: a new stage key with no registry entry fails the test. The next feature follows the fix by construction.
- Good: a lane-routed `pipeline-repair` lands as a normal PASS. The routing decision of 2026-11-13 takes effect at `enforce`.
- Good: the scan skip caches only what it proves pure, so a new impure package cannot escape.
- Good: one registry home. No stage has two rows with two stage values.
- Bad: each stage needs a registry entry, an emitter and a scorer. The cost falls on every new stage.
- Bad: each cycle appends one more ledger entry, `stage-evidence-seal`, on every exit (spec section 10.2, E2). Verify reads one more file for each cycle.
- Bad: the protected surface grows. A change of a threshold needs the operator.
- Bad: a party that rewrites the ledger and the sidecar together can forge evidence. The protected paths limit who can. A checkpoint outside the repository is the remedy (plan Q3).
- What changes this decision: a need for a defense against a full rewrite adds candidate C of the plan.

## Rejected

Scores are from the plan, section 4. A higher total is better.

| Option | Score | Why not |
|---|---|---|
| Evidence store B: a separate chain with its own tip | 14 | Two tips can diverge. Verify replays the whole log. |
| Evidence store C: a Merkle log with signed checkpoints | 15 | It adds a signer, a key to custody and a second trust system to run. It stays a later option. |
| Scan-skip key 2: reuse the Go test cache | 10 | The cache state sits outside the ledger. It misses `TestMain` files, child processes and files that tests read through `..`. |
| Scan-skip key 3: record the inputs of each package | 13 | It needs two runs and an unstable test-binary option. It stays a later option. |
| Scan-skip key 1: a hand list of impure packages | 17 | It fails open: a new impure package is cached until it escapes once. It gives purity a second home. |
| Scan-skip key 1+: a hand list plus an escape denylist | 18 | It learns only after an escape, which is the failure class of F2.2. It gives purity a second home. The chosen key 4 scores 21. |
| A second registry (`stagecontract`) beside `gatecontract` | n/a | An unwired twin. A ship check will get two rows, two stage values and two ratchets. |
| A build id as the decision version | n/a | It changes about once per cycle, so evidence never reaches the sample size. |
| The check of C2 in the policy loader | n/a | `policy` will import the ledger and the scorers. The preflight is the composition root. |
| One ledger entry for each decision | n/a | The ledger grows with the keys and the decisions, and verify slows down. |
| A digest file beside the signals with no ledger link | n/a | An attacker edits both files together. |
| An automatic flip on a `ready` verdict | n/a | A wrong score flips a stage with no human check. The report is a read verb. |
| A registry with optional scorers | n/a | The stages with no scorer are the ones that failed (F2.1, F2.2). |
| A test that lists the stage keys by hand | n/a | A new key does not fail it. A feature flag is also banned by the hard rules. |
