# Shadow stage contract: a shadow stage must prove itself before it flips

> Related: [gate-contract](gate-contract.md) (the registry home of §4) · [ADR-0165](adr/0165-every-gate-is-a-typed-unit-with-a-conformance-contract.md) · [integration-driver-enforce](integration-driver-enforce.md) (fix F1) · [retry-ladder](retry-ladder.md) (fix F6) · [policy-config](policy-config.md) · [llm-quality-indexes](llm-quality-indexes.md) (§4.6, stage flips) · [ADR-0157](adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md) (fix F4).
> **Status:** Revised (2026-11-15) after review round 3 (N1, N2 fixed, patch reviewed).
> This file is a design. It has no code. The code follows in later cycles, in the order of §8.
> This file is the spec of record for the rules that each stage key must obey.
>
> **Retired stage (2026-11-19):** fix F5b retired the `regression_tia` stage, because 0 of 44 predicates are pure (D5). The key, the `TIA_SHADOW` code, `emitTIADecision` and the registry row no longer exist. The examples in this file that name them show the shape of an entry. They do not describe live code.
>
> **Terms used throughout:**
> - **Stage key:** a field of the typed policy schema whose type is `Stage`, with the values `off`, `shadow` and `enforce`. Example: `regression_tia.stage`.
> - **Shadow:** the feature computes its decision and records it. The pipeline acts as if the feature were off.
> - **Enforce:** the pipeline acts on the decision.
> - **Forecast field:** a field such as `would_stop`, `would_skip` or `would_hold` in a signal. It states what enforce will do.
> - **False stop:** a `would_stop` on a cycle that then shipped with a PASS.
> - **Escape:** a `would_skip` on a check that then failed.
> - **Conflict:** a `would_hold` on an item that routing sent to a lane.
> - **Registry:** the typed Go table of §4. It is the existing gate registry, extended. It has one entry for each stage key.
> - **Scorer:** the function that compares a shadow decision with the outcome.
> - **Evidence record:** one shadow decision, written into the hash chain (§10).
> - **Evidence digest:** a hash over the ordered evidence records of one cycle. The ledger entry of the cycle commits to it.
> - **Decision version:** a content digest of the Go package closure of the decider, plus the policy subtree that the decider reads (§10 E4).
> - **Intact chain:** a chain that `evolve stage verify` accepts. That verb runs `evolve ledger verify` first.
> - **Grandfather record:** a flip record for a key that was at `enforce` before this contract existed (§6.2).

## Table of contents

1. Goals
2. Issues I1 to I9: shadow features that cannot prove themselves
3. The five rules and the seven integrity rules
4. The registry
5. The conformance tests
6. The stage flip procedure and the migration
7. Fix items F1 to F7
8. Order of work
9. Guards
10. Evidence and contract integrity (E1 to E4, C1 to C3)
11. Limits

> **Addendum (operator, 2026-11-15):** "The evidence and contract is the core of this feature, we must keep both intact and be able to verify, otherwise the whole pipeline is untrustworthy." §10 holds the rules that answer this. They are first-class rules, equal to S1 to S5.

## 1. Goals

The operator asked on 2026-11-15: "Carefully review the shadow features and fix the issues observed", and then "build a strong set of test cases to prevent this type of issue".

| Goal | Measure | Target |
|---|---|---|
| A shadow stage can be seen | stage keys with one registered signal code that a non-test function emits | 100% of the stage keys of the schema |
| A shadow stage can be scored | stage keys with a scorer and a verdict `ready` or `not ready` | 100% |
| A flip is never blind | flips to `enforce` with a green contract test and a `ready` verdict | 100% of new flips |
| An existing enforced key is never refused | keys at `enforce` at the migration that the policy still accepts | 100% |
| The seven known faults are closed | red tests of §7 that turn green | 7 of 7 |
| Evidence cannot change unseen | edits, deletions and reorders of an evidence record that `evolve stage verify` detects | 100% |
| The contract cannot weaken unseen | removals of a registry entry or a rule that the ratchet test detects | 100% |

## 2. Issues I1 to I9: shadow features that cannot prove themselves

A feature at `shadow` looks healthy but cannot prove itself, or proves the wrong thing. The flip to `enforce` is then blind or harmful. Each issue cites the fault-localization reports A, B and C of 2026-11-15. The line numbers were read at HEAD `64bcd83c4`. The issue IDs are `I1` to `I9`, so that they do not collide with the fix IDs `F1` to `F7`.

| Issue | Fix |
|---|---|
| I1 redelivery | F1 |
| I2 scan skip | F2 |
| I3 main-red stop | F3 |
| I4 hold | F4 |
| I5 no signal | F5 |
| I6 ladder | F6 |
| I7 spend limits | F7 |
| I8 signals not tamper-evident | E1 to E4 (§10) |
| I9 contract not protected | C1 to C3 (§10) |

**I1** Every pool record of the integration train is "not clean" only because of a redelivery. Pool 2373 has cycle 2374.
Pool 2378 has cycles 2379, 2382, 2383 and 2386.

Each of them says "cycle N was delivered again under new patches after it was composed".
*Implication:* the gate "two clean shadow waves" cannot be met by good behavior. [integration-driver-enforce](integration-driver-enforce.md) (C3, R-a to R-d) holds the redesign. This file only links to it.

**I2** The ship scan-skip stage keys its verdict cache on the whole tree. `scanShadow.keyOf` passes `Log{Unknown: true}` (`go/internal/phases/ship/scanshadow.go:220`). The escape counter fired on `internal/acssuite` (cycle 2362) and on `internal/guards` (2026-11-13).

These packages read the ambient environment, the `.git` state and run subprocesses (report A). A tree hash does not cover these inputs.
*Implication:* about 2 escapes in 62 scans is 3%. That is too high to enforce. I did not see the failing log of cycle 2362. The cause (environment or flake) is a ranked hypothesis, not a proof.

**I3** The main-red stop opens only from `go test` keys of the lane floor. `reportAttributed` calls `openMainRedStop` (`go/internal/core/base_red_floor.go:182-207`). `stopOpening.through` returns when `MainRed` is empty (`go/internal/core/main_red_stop_open.go:~99`). The required CI check "golangci-lint: Lint" was red on main from 2026-11-14T16:03Z for 5 commits.

The stop saw nothing. `checkMainRedStop` returns `nil, nil` when no stop holds (`go/internal/phases/ship/main_red_stop.go:27-30`).
*Implication:* a lint fix has no `Package.Test` key. The stop cannot prove such a fix, and it cannot open for such a red.

**I4** `heldproposal.Decide` holds any kind with the prefix `pipeline-` (`go/internal/heldproposal/heldproposal.go:52-56`). `inboxbatch.ConsoleRouted` sends an operator-filed `pipeline-*` item with `route: lane` to a lane (`go/internal/inboxbatch/consoleroute.go:39-48`). Two copies of one rule disagree.

At `enforce`, the stage holds 5 PASS landings (cycles 2379, 2383, 2384, 2386 and 2388, reason `kind:pipeline-repair`). I did not open those cycle records, so I cannot say that each item had `route: lane`.
*Implication:* this is a design bug that matters on the day of the flip. It breaks the law "never duplicate logic".

**I5** Three features give no signal, or no behavior:
- `audit_cascade` writes `audit-cascade-shadow.json` and emits no signal (`go/internal/phases/audit/cascade_shadow.go:14-37`).
- `regression_tia` writes `acs-tia-shadow.json` and emits no signal (`go/internal/phases/audit/audit.go:382-396`). The write error is dropped: `_, _ = regressiontia.Emit(req.Workspace, d)` (`audit.go:395`). A failed write is therefore invisible. This is a defect, and F5 fixes it. The stage can also never skip: `resolveDeps` drops each predicate whose dependency closure reaches `os`, `os/exec`, `os/user`, `syscall`, `net` or `net/http` (`go/internal/regressiontia/regressiontia.go:114-149`). In `go/acs/regression/`, 35 of 44 predicate directories import one of them directly.
- `parallel_evaluate` at `shadow` acts as `off`. The only reader tests `== config.StageEnforce` (`go/internal/core/orchestrator.go:624`). It records nothing.

*Implication:* no soak data can exist for these three keys.

**I6** The retry ladder has a second rule outside its home. `applyDispatchPolicy` asks the ladder, and `retryladder.Next` raises an `Unknown` verdict at R0 to R1 (`go/internal/retryladder/decide.go:84-87`). `cycleRun.escalatedBuildTier` (`go/internal/core/retry_tier_escalation.go:33`) does not call `retryladder.Next`. It calls `nextItemStartRung` (`go/internal/core/retry_ladder.go:209-230`). That function is a hand-coded copy of the decision:
- It starts from the literal `Decision{Action: ActionStop, Rung: R0}`.
- It raises only for the class `Capability`.
- It emits that literal with an empty `retryladder.Verdict{}` (`retry_ladder.go:210, 228`).

This matches the recorded fields of cycles 2362 and 2377 exactly: action `stop`, class `unknown`, fingerprint empty, tier empty. The cause is therefore not open. It is not a stale binary and not an incomplete input. It is a second decision rule outside `retryladder`.

*Implication:* at `enforce`, `escalatedBuildTier` returns no tier raise when the start rung is R0 (`retry_tier_escalation.go:48-50`). The deterministic tier escalation of ADR-0076 is then suppressed for each build whose failure class is `unknown`. The effect is a lost escalation, not a stopped build.

**I7** The cycle spend limits stop shipped cycles. `max_audit_rounds` 2 stops 40 of 143 shipped cycles in 2200 to 2390 (28%). `max_llm_minutes` 90 stops 9 shipped cycles in 2356 to 2390.

These counts come from the review of the operator. I did not recompute them.
*Implication:* the limit value is wrong for the data. The limiter value also differs from the sum of the phase durations, which §7 (F7) explains.

**I8** The signal stream is a plain append file. `openAppend` opens it with `O_APPEND|O_CREATE|O_WRONLY` (`go/internal/signalcenter/sinks.go:22-28`). The name is `signals.ndjson` (`go/internal/signalcenter/event.go:222`). The file has no hash.

The ledger is different. Each entry has `entry_seq` and `prev_hash` (`go/internal/core/ports.go:69-70`). `sealChainsFromPrev` checks the link (`go/internal/adapters/ledger/anchor.go:30-38`). Nothing detects an edit, a cut or a loss of a shadow decision that lives only in signals.
*Implication:* a `ready` verdict that reads only signals is a claim that no one can check. Every flip that follows is blind.

**I9** The registry and the conformance tests are ordinary files unless they join the protected surface. The protected-surface manifest (`ProtectedSurfaceManifest`, `go/internal/guards/integrity_surface.go:37`; `IsProtectedSurface`, line 270) lists the paths that a lane cannot change without the operator. A lane can remove a row, drop a rule or lower a `promotion` threshold, and the tests stay green.
*Implication:* the contract protects the stages only while the contract is itself protected.

The common cause: no rule says what a shadow stage must emit, what it must compare, and what must exist before the flip.

## 3. The five rules

Each stage key obeys these rules. A test pins each rule (§5). The scope is every stage key of the typed policy schema, at any value (§6.2 gives the migration).

| Rule | Statement | Issue that it catches |
|---|---|---|
| S1 Observable | At `shadow`, the stage emits one registered signal code for each decision, with `would_*` fields, from the one place that decides. A stage whose `shadow` equals `off` is refused. | I5 |
| S2 Same decision | `shadow` and `enforce` run the same decision function with the same inputs. The decision function has one home. No function outside it builds a decision value or emits the signal with a decision that it did not get from the decider. | I6 |
| S3 Enforce path is wired | The enforce branch is a typed function value in the registry entry. The consumer reaches it only through its `StageActor` port, which the composition root fills from the entry (D6). A test injects a spy through the port, drives the stage at `enforce` and asserts that the spy acted. | the #1459 lesson: a stage that flipped with no enforce code |
| S4 Reachable and correct evidence | Good behavior can meet the promotion condition. A shadow decision is scored against the outcome. `evolve` prints, for each stage: decisions, false stops, escapes, conflicts and the verdict `ready` or `not ready`. | I1, I2, I3, I7 |
| S5 One rule, one home | A rule that two features share has one predicate. | I4, I6 |

The seven integrity rules E1 to E4 and C1 to C3 are in §10. They have the same force as S1 to S5. The flip procedure (§6) needs both sets.

| Rule | Statement | Issue that it catches |
|---|---|---|
| E1 Durable evidence | Each shadow decision is a record bound into the hash chain. | I8 |
| E2 Complete evidence | The decider counts each decision in memory before it writes. The cycle seal, at cycle termination on every path, commits that count. A gap is detected. | I8 |
| E3 Reproducible score | The verdict is a pure function of the chain. | I8 |
| E4 Bound verdict | Evidence names its decision version, a content digest. | I8 |
| C1 Protected contract | The registry and the tests cannot weaken silently. | I9 |
| C2 Gated flip | A new `enforce` needs a verified, chained flip record. | I9 |
| C3 Checkable claim | `evolve stage status` shows every row as pass or fail. | I9 |

**Decision D1.** The rules are data-driven through the registry (§4), not through a flag.
*Because:* the top law and the hard rules ban feature flags. A registry entry is configuration data.
*Rejected:* a test that lists the stage keys by hand. A new stage key does not fail it.

## 4. The registry

**Decision D2.** There is no second registry. The shadow contract extends the existing gate registry: `gatecontract.Entry` (`go/internal/gatecontract/gatecontract.go`) and its seeds in `go/internal/gateregistry/`.

[gate-contract](gate-contract.md) specifies that registry. It is one Go table on the protected surface. Only a held proposal or the operator moves its stage. It has the enforced-gate ratchet (its test 4) and a refusal from each writer.

*Because:* a second table with its own stage value and its own ratchet beside the first is an unwired twin. One copy can lose.

*Rejected:* the file `go/internal/stagecontract/registry.json` of round 1. It held symbol names as strings. A rename breaks a string silently, and dead code satisfies it.

### 4.1 The shape

`gatecontract.Entry` today has: `Name`, `Kind`, `Files`, `Covers`, `Owners`, `Inputs`, `Stage` and `Run`. The extension adds one typed field, `Shadow`, which holds the fields below. An entry without a shadow contract has `Shadow == nil` and keeps the rules of gate-contract unchanged.

| Field of `Shadow` | Type | Meaning | Example |
|---|---|---|---|
| `PolicyKey` | path into the typed policy schema | the one stage key of the entry | `regression_tia.stage` |
| `Signal` | `signalcenter.Code` | the registered signal code | `TIA_SHADOW` |
| `Decide` | `func(ctx, StageInput) (Decision, error)` | the one decision function, as an adapter over the real decider | `regressiontia.Compute` |
| `Enforce` | `func(ctx, StageInput, Decision) error` | the enforce actor. The consumer reaches it only through its port (D6). | the TIA skip actor of the audit phase |
| `Score` | `func(ctx, []EvidenceRecord) (Verdict, error)` | the evidence scorer | `regressiontia.ScoreEscapes` |
| `Fields` | list of strings | the `would_*` fields that the signal must carry | `would_skip_count`, `selected` |
| `EmitSites` | list of function values | each function that emits `Signal`. These are the emit sites of the signal code, not the callers of the decider. | `emitTIADecision` |
| `Promotion` | struct of numbers | the condition for the verdict `ready`, as structured data | see 4.2 |
| `Closure` | list of package paths | the Go packages whose content is the decider (§10 E4) | `internal/regressiontia` |
| `PolicySubtree` | policy path | the subtree of the policy that the decider reads (§10 E4) | `regression_tia` |

A function value is a typed reference. A rename that leaves a stale reference fails the build. For a stage feature that is not a gate, `Run` is the adapter that calls `Score` and returns the verdict as a `gatecontract.Verdict`. Test 3 of gate-contract (each entry has a run function) then holds unchanged.

**The stage value has two homes today and must have one.** `Entry.Stage` is seed data in Go. The stage key is operator data in `policy.json`.

One accessor, `Entry.EffectiveStage`, resolves the stage for every entry. It returns the policy key when the entry has `Shadow.PolicyKey`. Otherwise it returns the seed. No caller reads the `Stage` field directly. A test fails when an entry has both a `Shadow` field and a seed `Stage` value. The builder removes that seed value.

Test 4 of gate-contract says that an enforced gate is an entry at `enforce`. It extends to this rule: an entry at `enforce` in the policy has a flip record (§6).

**The gate kinds.** `gatecontract` has the kinds `scanner`, `phase-floor` and `ship-check`. The ship checks of this design join as `ship-check` entries. The features that are not gates today join under one new kind, `stage-feature`.

### 4.2 The promotion data

`Promotion` is a struct of numbers, so the ratchet compares numbers and no interpreter exists.

| Field | Meaning | Direction of the ratchet |
|---|---|---|
| `MaxEscapes` | most escapes allowed | may only fall |
| `MaxFalseStops` | most false stops allowed | may only fall |
| `MinDecisions` | fewest scored decisions in the current decision version | may only rise |
| `WindowCycles` | the number of last cycles that the scorer reads | may only rise |
| `MaxFalseStopPct` | most percent of shipped cycles that a limit may stop (F7 only) | may only fall |

### 4.3 What is a gate today, and how each feature joins

At HEAD, the seeds of `gateregistry` are the floors, `rawgitratchet`, `sizeratchet` and `testdup`. None of the features below is an entry today.

| Policy key | Signal | State of the code | Decider | Joins as |
|---|---|---|---|---|
| `audit_cascade.stage` | `AUDIT_CASCADE_SHADOW` | proposed, no signal exists | `auditcascade.Decide` | `stage-feature` (not a gate) |
| `regression_tia.stage` | `TIA_SHADOW` | proposed, no signal exists | `regressiontia.Compute` | `stage-feature` (not a gate) |
| `parallel_evaluate.stage` | `PARALLEL_EVALUATE_SHADOW` | proposed, no signal exists | `parallelevaluate.Decide` (new package, moved out of the unexported `core.cycleRun.evaluateBatchAt`) | `stage-feature` (not a gate) |
| `workflow.retry_ladder.stage` | `LADDER_DECISION` | exists (`retryladder/tryrecord.go:25`) and is emitted | `retryladder.Next` | `stage-feature` (not a gate) |
| `gates.main_red_stop` | `SHIP_MAIN_RED_STOP_SHADOW` | proposed | the stop check of the ship | `ship-check` |
| `held_proposals.stage` | `SHIP_HOLD_SHADOW` | exists (`phases/ship/held_proposal_gate.go:20`) | `heldproposal.Decide` | `ship-check` (the builder confirms the kind) |
| `workflow.cycle_spend.stage` | `CYCLE_SPEND_SHADOW` | proposed. `cyclespend.Judge` exists (`cyclespend/limits.go:34`) | `cyclespend.Judge` | `stage-feature` (not a gate) |
| the ship scan skip: policy key `ship.scan_skip.stage` (proposed, none exists) | `SHIP_SCAN_SHADOW_ESCAPE` and `SHIP_SCAN_SHADOW_FAULT` exist (`scanshadow.go:22-23`). A decision code is proposed. | the escape and fault codes exist | `scanShadow.settle` | `ship-check` |
| `integration.stage` | per [integration-driver-enforce](integration-driver-enforce.md) | per that file | `Driver` | `stage-feature` (not a gate) |

The scan skip has no stage key in the policy today. A stage with no policy key cannot be walked by the schema test, so the key joins the schema in the same change.

**Decision D3.** A stage with no scorer cannot have an entry. Therefore it cannot flip.
*Because:* S4 needs a verdict for each stage.
*Rejected:* an entry with an optional scorer. The faults of I1 and I2 are exactly the stages that had no scorer.

**Decision D6.** The dependency points inward. `core` and `ship` each own a small consumer-side port, `StageActor`. The binary composition root, `cmd/evolve`, builds the registry entries and injects them into `core` and `ship`. Neither `core` nor `ship` imports `gateregistry`.

```
cmd/evolve (composition root)
   |  builds entries from gateregistry
   |  injects StageActor values
   v
core, ship        (each owns its StageActor port)
   ^
   |  a seed names the deciders, not core
gateregistry ---> parallelevaluate, regressiontia, retryladder, heldproposal
```

The port has one uniform signature for each field. `Decide` is `func(ctx, StageInput) (Decision, error)`. `Enforce` is `func(ctx, StageInput, Decision) error`. An adapter in the seed wraps each real decider, such as `regressiontia.Compute` or `retryladder.Next`, to this shape. No field uses `any`.

The `parallel_evaluate` decider moves out of the unexported method `core.cycleRun.evaluateBatchAt` into the package `parallelevaluate`. A seed can name it. `core` keeps only the call through its port.

*Because:* the registry is then the wiring, and a test can inject a counting spy through the port. `gateregistry` already imports `core` and `ship`, so a reverse import is a cycle.

*Because:* a decider in its own package has a small closure. The version digest of §10 E4 then resets only when that decider changes. The closure of `internal/core` covers 133 packages and changes about every 7 cycles, which can keep `MinDecisions` out of reach.

*Rejected:* a name check in the Go tree. Dead code satisfies it.

*Rejected:* `core` imports the registry. That is an import cycle.

### 4.4 The typed key list and the effective stage

The registry covers each stage key through one typed list. The list is `policy.StageKeys()` in `go/internal/policy/stage_keys.go`. It has 48 keys:

| Part | Keys | Source |
|---|---|---|
| The schema walk | 42 | `policy.StageFields()` walks the type `policy.Policy`. It lists each field whose JSON name ends in `stage`. |
| The scan cache gate | 1 | `gates.scan_cache_gate`. The name ends in `_gate`, so the walk does not find it. |
| The handoff pairs | 5 | `handoff.<pair>.stage` for each pair in `knownHandoffPairs`. The type `HandoffPolicy` keeps its pairs in an unexported map, so the walk does not find them. |

The six keys outside the walk have seed entries in `go/internal/gateregistry/seed_stage_keys.go`. The scan cache gate names the signal `SHIP_SCAN_SHADOW_ESCAPE` and its emit site `escape`. The five handoff pairs have no signal yet. Each pair has one `no-signal` row in `go/internal/gateregistry/testdata/stage_violations.json`. The ceiling of that baseline moves from 82 to 87.

The rule `string-typed-stage` covers only the 42 walked fields. The six keys outside the walk are exempt from it.

The other `_gate` fields of `GatesPolicy` have no stage key. `TestStageKeysCoverGateAndHandoffFields` names them in one list. A new `_gate` string field fails the test until a person puts it in `StageKeys` or in that list. This is open question Q2.

`policy.Policy.EffectiveStages()` returns the stage of each key in `StageKeys()`. It reads the typed configuration, so a key that has its default in Go reads as that default. A read of the `policy.json` text misses these keys. The function returns an error that names the key when a key has no reader or when its reader fails.

Each key has one reader in `stageReaders`. The reader calls the accessor that the feature uses at run time. Four rules apply to the value:

- A key with an optional block that is absent reads `off` when the feature is then off. This holds for `fleet.budget.stage` and `fleet.orchestrator.window.adaptive.stage`.
- An isolation key reads the value that its consumer acts on. `policy.ResolveIsolationStage`, `policy.ResolveFlowStage` and `policy.ResolveQueueStage` are the one home of that rule. The bridge, the flow decision and the queue verb call them. A value such as `ENFORCE` reads `enforce` for `i2`, and `bogus` reads `off`.
- An empty isolation key reads the default of its consumer. `isolation.i2`, `i3`, `i5` and `i7` read `shadow`. `isolation.i15` reads `enforce` for the exact word `enforce` and `shadow` for every other value. `isolation.i8` reads `legacy`, because an empty flow stage means the legacy path.
- `workflow.findings_repair.code-review.stage` reads `shadow` when the policy says `enforce`. The resolver holds that stage at `shadow` today.

The values of `pr.merge_window.stage` are `off` and `on`. Its reader returns them unchanged.

`TestEffectiveStagesMatchEachKeyDefault` pins the default of each key. `TestEffectiveStages_ReadEachKeyFromItsOwnBlockAndNoOther` sets one key at a time through `policy.Parse`. It checks that only that key moves. `TestEveryStageKeyHasRegistryEntry` walks `StageKeys()` and fails for a key with no entry.

## 5. The conformance tests

The tests live with the registry in `go/internal/gateregistry/` (the protected surface of gate-contract §6.7). Each test walks the typed policy schema. It does not walk the live `.evolve/policy.json`. That file omits every key that uses its default. At HEAD, `held_proposals`, `gates.main_red_stop`, `workflow.cycle_spend` and `integration` are absent. It does not walk the registry first.

A stage key of the schema with no entry fails the test.

**One stage type.** A schema walk finds stage keys by their type. At HEAD the stage value is not one type. There are more than 15 separate `"shadow"` constants. The builder unifies them into one type `Stage` in a leaf package that `policy` and `gatecontract` both import. The constants to unify:

| Package and file | Constant |
|---|---|
| `inputindex/layers.go:19` | `stageShadow` |
| `phasecontract/flow_decision.go:27` | `FlowStageShadow` |
| `qualitypanel/index.go:8` | `StageShadow` |
| `gc/remotesweep.go:20` | `RemoteStageShadow` |
| `gcpolicy/gcpolicy.go:163` | `remoteStageShadow` |
| `phases/specrunner/verdict_from_sentinel.go:16` | `SentinelStageShadow` |
| `gatecontract/gatecontract.go:13` | `StageShadow` |
| `orchestrator/orchestrator.go:13` | `StageShadow` |
| `interaction/rulepromote.go:20` | `RuleStageShadow` |
| `policy/convergence.go:10` | `defaultConvergenceStage` |
| `policy/isolation.go:12` | `DispatchRequestStageShadow` |
| `policy/healing.go:10` | `HealingStageShadow` |
| `policy/audit_cascade.go:11` | `AuditCascadeShadow` |
| `policy/workflow_repair.go:12` | `FixabilityShadow` |
| `policy/workflow_retry_ladder.go:9` | `RetryLadderShadow` |
| `policy/inbox_curator.go:10` | `inboxCuratorShadow` |
| `policy/fleet_integration.go:6` | `FleetIntegrationShadow` |
| `policy/held_proposals.go:10` | `defaultHeldProposalsStage` |

Some of these name a different concept (for example `auditfix.ReasonShadow` and `integration.fieldShadow` are labels and not stage values). The builder reads each one and keeps a constant outside the unification only with a reason in the package note. A second test, `TestNoStringTypedStageField`, fails when a policy field whose name ends in `stage` has a plain string type. That blocks a new key from escaping the walk.

| Test | Rule | It fails when | Red proof on today's tree |
|---|---|---|---|
| `TestEveryStageKeyHasRegistryEntry` | S1 to S5 | a stage key of the typed schema has no entry with a `Shadow` field | the shadow keys `audit_cascade`, `regression_tia` and `parallel_evaluate` have no signal. The keys of the ship gates have no entry. |
| `TestNoStringTypedStageField` | S1 to S5 | a policy field named `*stage` has a plain string type | the fields that use the separate constants above |
| `TestEverySignalCodeHasNonTestEmitter` | S1 | the code of an entry is not registered, or an emit site is only in a `_test.go` file | `AUDIT_CASCADE_SHADOW` and `TIA_SHADOW` do not exist |
| `TestShadowIsNotOff` | S1 | a run at `shadow` emits no event, or the pipeline output equals the output at `off` | `parallel_evaluate` at `shadow` records nothing |
| `TestShadowSignalCarriesWouldFields` | S1 | the emitted event lacks a field that the entry lists | the ladder event of `nextItemStartRung` has an empty fingerprint and an empty tier |
| `TestNoDecisionLiteralOutsideDecider` | S2 | a non-test function outside the decider builds a decision value (a literal such as `retryladder.Decision{`) or emits the signal with a foreign decision | `nextItemStartRung` builds `retryladder.Decision{...}` (`retry_ladder.go:210, 218`) |
| `TestEnforceBranchActsAtEnforce` | S3 | a run at `enforce` through the production entry point, with a counting spy injected by the `StageActor` port, does not call the spy. A run at `shadow` that calls the spy also fails | the enforce path of `audit_cascade`, `regression_tia` and `parallel_evaluate` has no registered actor |
| `TestStageGateRefusesEnforceWithoutActor` | S3 | the preflight (§6.3) accepts `enforce` for an entry whose `Enforce` is nil | the preflight does not check it today |
| `TestEveryStageHasScorerAndVerdict` | S4 | `evolve stage report` has no line for a key | the command does not exist |
| `TestPromotionReachableByGoodBehavior` | S4 | a replay of a clean run (a redelivery with no real fault) gives the verdict `not ready` forever. The same replay with N cycles of unrelated code and policy edits must also give `ready` (§10 E4). | the integration train: redelivery marks the pool "not clean" |
| `TestScorerCountsFalseStopEscapeConflict` | S4 | the scorer misses a false stop (PASS after `would_stop`), an escape (red after `would_skip`) or a conflict (`would_hold` on a lane-routed item) | the scan skip has an escape counter only |
| `TestSharedRuleHasOnePredicate` | S5 | two entries name different predicates for one rule id | routing and hold use two copies of the `pipeline-` rule |
| `TestEvidenceEditFailsVerify` | E1 | an edit of one evidence record leaves `evolve stage verify` green | no evidence record exists in the chain |
| `TestEvidenceDeleteFailsVerify` | E1, E2 | a deletion of one record leaves verify green | same |
| `TestEvidenceReorderFailsVerify` | E1 | a swap of two records leaves verify green | same |
| `TestDeciderReturnWithoutWriteMarksIncomplete` | E2 | a fixture decider that returns without a write leaves the cycle `complete` | no decision counter exists |
| `TestEvidenceWriteFailureMarksCycleIncomplete` | E2 | a failed evidence write leaves the cycle `complete`, or counts it toward readiness | the TIA write error is dropped (`audit.go:395`) |
| `TestLateDecisionNotCounted` | E2 | a decision after the seal is counted toward readiness | no seal exists |
| `TestFailedCycleEvidenceIsSealedAndScored` | E2 | a cycle that ends in fail or abort has no seal, or its decisions are not scored | no seal exists, and a cycle that ends before ship has no commitment |
| `TestVerdictRecomputesFromFixtureChain` | E3 | the verdict from the fixture chain differs from the golden | no verb exists |
| `TestScorerMutationChangesGolden` | E3 | a changed scorer gives the same verdict as the golden | no golden exists |
| `TestVersionDigestStableUnderUnrelatedChange` | E4 | an edit of a package outside the closure, or of a policy key outside the subtree, changes the digest or resets the evidence count | evidence has no version |
| `TestVersionDigestChangesWithDeciderClosure` | E4 | an edit inside the closure, or inside the subtree, leaves the digest unchanged | same |
| `TestEvidenceOfOtherVersionNotCounted` | E4 | evidence of a different digest counts toward the current verdict | same |
| `TestViolationBaselineOnlyShrinks` | C1 | the baseline gains a key, or a stage key outside the baseline breaks a rule | the baseline does not exist |
| `TestRegistryRatchet` | C1 | the registry lost an entry, a rule, a field or an emit site that the ratchet list names, or it loosened a number (§4.2) | the ratchet does not exist |
| `TestContractPathsAreProtectedSurface` | C1 | a path of the shadow-contract code or tests is not matched by `IsProtectedSurface` | the manifest lacks these paths |
| `TestPreflightRefusesEnforceWithoutFlipRecord` | C2 | after the genesis record exists, the preflight accepts a key at `enforce` that has no flip record of any kind | the preflight does not check it today |
| `TestHeadPolicyAccepted` | C2 | the preflight refuses the policy of HEAD with the migration records present, or before the genesis record exists | the 10 keys at `enforce` have no record |
| `TestStageGateRefusesNotReadyVerdict` | C2 | `evolve stage flip` accepts a key whose verify returns `not ready` or reports a broken chain | the verb does not exist |
| `TestStageStatusListsEveryRow` | C3 | `evolve stage status` omits a key, or shows a rule row as pass when its test fails | the command does not exist |

### Mutations

The mutation set proves that the tests catch a defect. Each mutation must turn a named test red. A mutation that stays green is a gap in the test. Every E-test below has its mutation in this table. A red proof needs a change that makes the test fail, and not only a missing verb.

| Mutation | Test that must fail |
|---|---|
| Delete the `Emit` call in `emitTIADecision` | `TestEverySignalCodeHasNonTestEmitter` |
| Make `parallel_evaluate` at `shadow` return before it computes the batch | `TestShadowIsNotOff` |
| Set the `Enforce` function value of an entry to a function that nothing calls | `TestEnforceBranchActsAtEnforce` |
| Re-add a `retryladder.Decision{` literal in `nextItemStartRung` | `TestNoDecisionLiteralOutsideDecider` |
| Count a PASS after `would_stop` as correct | `TestScorerCountsFalseStopEscapeConflict`; `TestScorerMutationChangesGolden` |
| Make the promotion condition need a cycle with no redelivery | `TestPromotionReachableByGoodBehavior` |
| Add a second `pipeline-` prefix check in `Decide` | `TestSharedRuleHasOnePredicate` |
| Add a policy stage key with no entry | `TestEveryStageKeyHasRegistryEntry` |
| Add a policy field `foo_stage` of a plain string type | `TestNoStringTypedStageField` |
| Change one byte of one evidence record in the fixture chain | `TestEvidenceEditFailsVerify`; verify names the record by its sequence number |
| Delete one evidence record from the fixture chain | `TestEvidenceDeleteFailsVerify`; verify names the gap |
| Swap two evidence records | `TestEvidenceReorderFailsVerify` |
| Make a decider return without the write | `TestDeciderReturnWithoutWriteMarksIncomplete` |
| Make the evidence writer return an error | `TestEvidenceWriteFailureMarksCycleIncomplete` |
| Count a decision after the seal | `TestLateDecisionNotCounted` |
| Seal only when the cycle ships | `TestFailedCycleEvidenceIsSealedAndScored` |
| Hash the whole policy, or the build id, into the version digest | `TestVersionDigestStableUnderUnrelatedChange` |
| Leave one closure package out of the version digest | `TestVersionDigestChangesWithDeciderClosure` |
| Ignore the digest when the verdict counts evidence | `TestEvidenceOfOtherVersionNotCounted` |
| Delete an entry, or loosen a `Promotion` number (§4.2), in the registry | `TestRegistryRatchet` |
| Add a key to the baseline | `TestViolationBaselineOnlyShrinks` |
| Remove the registry path from the protected manifest | `TestContractPathsAreProtectedSurface` |
| Make the preflight skip the flip-record check | `TestPreflightRefusesEnforceWithoutFlipRecord` |
| Make the migration verb skip one enforced key | `TestHeadPolicyAccepted` |

## 6. The stage flip procedure and the migration

### 6.1 A new flip

A stage flips from `shadow` to `enforce` only when all three conditions hold:

1. `evolve stage status` shows every contract row of the key as pass (§10 C3). The contract tests of §5 and §10 are green for the key.
2. `evolve stage verify <key>` returns `ready` on an intact chain (§10 E3). The verdict follows the `Promotion` data of the entry and the sample rule of [llm-quality-indexes](llm-quality-indexes.md) §4.6.
3. The key has an `Enforce` function value in its entry (S3).

The flip is one verb, `evolve stage flip <key>`. It runs conditions 1 to 3. Then it does two writes in this order. First, it appends the flip record (who, when, the verified digest) to the chain. Second, it changes the `stage` value in `policy.json` with an atomic rename.

**The chain record is the single truth.** The `stage` value in `policy.json` is a projection of it. A hand edit of `policy.json` to `enforce` has no record, and the preflight refuses it (§6.3).

A record with no policy change has no effect on the pipeline. This happens when the verb stopped between the two writes. `evolve stage status` reports it as `flip pending`, and a second run of the verb finishes it. The operator cites the output of `evolve stage verify` in the commit message.

The flip procedure uses only the output of `evolve stage verify` and `evolve stage status`. `evolve stage report` is a summary for people and is not an input of the flip. A flip with a red test or a `not ready` verdict is a defect.

**Decision D4.** The report is a read verb. It never flips a stage.
*Because:* a listener observes and does not decide ([signal-center-design](signal-center-design.md) §3).
*Rejected:* an auto-flip. A wrong score flips a stage with no human check.

### 6.2 The migration of keys that are already at enforce

At HEAD, the review counts 10 keys at `enforce` in `.evolve/policy.json` (for example `comment_floor.stage`, `docs_floor.ste_stage`, `catalog.refresh_stage` and `isolation.i2.stage`). None has a flip record or an entry. The migration verb prints the real list when it runs.

- **Scope.** Every stage key of the typed schema obeys the contract, at any value. The contract does not exempt a key because it is old.
- **Known-violations baseline.** A data file lists each stage key that does not yet meet the contract, with the rule it breaks. A key at `off`, or a grandfathered key that is not a gate, needs only a stub entry until it changes stage. The baseline is held by the same ratchet as C1: it may only shrink, and a new key that is not in it fails. Step 1 of §8 ships green against it.
- **The migration verb.** `evolve stage migrate` writes, once, one grandfather record for each key that is at `enforce`. The record holds the key, the value, the policy digest and the evidence that the chain has for the key. For most keys the chain has none. The record then says `enforced before the contract` as its reason. The verb also writes one genesis record that lists the keys it covered.
- **Once.** A second run of the verb writes nothing and prints the first genesis record. The ratchet (C1) holds the genesis record: it cannot be removed.
- **Visible.** `evolve stage status` shows a grandfathered key as `grandfathered`, never as `verified`. The key still needs its entry, its scorer and its tests. The scorer then produces the evidence that the key never had, and the operator may re-flip the key by the normal procedure.
- **C2 applies to new flips only.** The preflight (§6.3) accepts a key at `enforce` when it has a flip record of either kind. It refuses a key at `enforce` that has none. A key that was at `enforce` at the migration has a record, so the policy is never refused for it.
- **Before the genesis record exists, the check is inactive.** The preflight then only emits a loud signal for each key at `enforce` that has no record. It never refuses. The check becomes active when the genesis record is in the chain. This order (migrate first, then enforce the check) is what `TestHeadPolicyAccepted` pins.

### 6.3 Where the check runs

The check runs in the loop-start preflight, in the composition root where `cli-routing` already halts the loop. It does not run in the policy loader. The package `policy` stays pure: it reads the file and imports neither the ledger nor the scorers.

The dependency direction: the preflight calls a port `FlipRecords`. An adapter on the ledger implements it. The evidence writer (the recorder of §10 E2) lives in a new package, `stageevidence`, with its adapter in `go/internal/adapters/ledger`. `policy`, `gatecontract` and `stageevidence` do not import each other in a cycle. The preflight is the only place that joins them. The spec has no "or at loop start" choice.

## 7. Fix items

Each fix is one component with a red test. A fix makes the tests of §5 green for its key.

### F1 Train redelivery

Follow [integration-driver-enforce](integration-driver-enforce.md) (C3, R-a to R-d). This file adds only the S4 test: `TestPromotionReachableByGoodBehavior` replays pool 2378 (cycles 2379, 2382, 2383, 2386) and expects a "clean" wave.

### F2 Scan skip

Source: report A.

**One purity predicate, one home.** The rule "a package is pure" has a home today: `regressiontia.escapeHatches` and `resolveDeps` (`regressiontia.go:114-149`). F2 does not add a second rule. It moves that rule behind one function, `regressiontia.Pure(package)`, and both consumers call it:
- the TIA, in place of the inline test of `resolveDeps`;
- the scan skip, in `cacheable` (`scanshadow.go:208`).

`Pure` is true when the dependency closure of the package and of its tests has none of the escape hatches. The builder reads `listCorpusDeps` and states in the package note which imports the closure covers. The hatch list is data. The builder checks if some standard library paths belong in the list. These paths read files without an `os` import, for example `path/filepath` walks and `io/ioutil`.

**The cache is default-deny.** `cacheable` returns true only for a package that `Pure` proves.

A new package that nobody has listed is not cached. A hand list stays only as overrides, as data. Each override carries a reason. A forced-impure entry is one kind, for example `internal/acssuite`. A forced-pure entry is the other kind.

A learning step never adds a package to the cache.

**The escape scorer** stays, as the check on the rule. When `settle` finds a hit and the package failed, it counts an escape. The scorer proposes an override entry. The console decides. Nothing writes an override on its own.

**Why the key is sound, and the comparison with Go's own test cache.**

Go's test cache records, at run time, the environment variables and the files that the test binary read (`internal/testlog` receives them from package `os`). It reruns the test when one of them changed. It has known gaps:
- files that `TestMain` opens are not tracked (golang.org/issue/24122);
- a subprocess that the test starts is not tracked;
- files younger than 2 s are not cached.

The Go cache is therefore dynamic. It is sound for what the test binary itself reads. It is unsound for what a child process reads.

The key of the scan skip is static. It is the content hash of the tree of the package closure, plus the toolchain version and the module files. It is sound only if the closure has no input that the hash does not cover.

The default-deny `Pure` rule gives that. A package that reaches no escape hatch cannot read the environment, the file system or a child process through module code. The hash then covers every input. A package that reaches a hatch is not cached.

The Go cache can cache such a package when the test reads the same files. It hits wrongly when a child process such as `git` reads changed state. The static rule is therefore stricter than the Go cache for exactly the case of I2. It is weaker for a pure-looking path that reaches the file system through the standard library. The hatch list data and the escape scorer cover that second case.

I did not check the toolchain source. The description of the Go cache comes from the research notes of 2026-11-15, not from a test.

- Red tests: `TestScanShadow_AmbientEnvChangeDoesNotHit` and `TestCacheable_RefusesPackagesThatReadTheEnvironmentOrGit` in `go/internal/phases/ship/scanshadow_unit_test.go`. The second test is red today because `cacheable` returns true for both packages. A third test, `TestCacheable_DefaultDenyForUnlistedImpurePackage`, adds a fixture package that imports `os/exec` and is on no list. It is red today for the same reason.

*Rejected:* a key that hashes all of the environment. The set of inputs of a subprocess is open. A proven-pure closure is a smaller and checkable claim.

### F3 Main-red stop

Source: report B1.
- Add a source: the required-CI conclusion of main. The ship reads it at the start of `checkMainRedStop` through the existing CLI path (`evolve ci classify`). When main is red and no stop holds, the ship opens a stop.
- Add step keys. `baseattr.Key` gets a `Step` field. The stop record moves to schema `main-red-stop/v2`. `mainstop/record.go` already refuses an unknown schema, so v1 files need a migration.
- Proof for a step key: a rerun of the step in the lane tree when the step can run locally. Otherwise the stop releases on a green conclusion of main (`greenOnMain`, `main_red_stop.go:~119`).
- The CI read fails open with a signal. An unreadable source never refuses a lane.
- Red tests in `go/internal/phases/ship/main_red_stop_test.go`: a fake CI source reports "golangci-lint: Lint" red on main with no stop on disk. The test expects an opened stop and `CodeMainRedPark` for a lane that does not fix it.
- A second test admits a lane that turns the step green.

### F4 Hold

Source: report B2.
- `Decide` uses the routing predicate. The `pipeline-` loop of `heldproposal.go:52-56` goes. `Inputs` carries the result of `inboxbatch.ConsoleRouted`. A kind reason remains only for an item that routing marks console-owned and that landed anyway.
- Protected paths still hold (`protected:<path>`).
- Amend [ADR-0157](adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md) decision 8 in the same change. The text must say that a `pipeline-*` item routed to a lane lands as a normal PASS unless it touches a protected path.
- Red tests in `go/internal/heldproposal/heldproposal_test.go`: `pipeline-repair` with `go/internal/foo/foo.go` changed must give `Hold == false`. This test is red today, because `Decide` returns `Hold == true`. A second test with a fence path must give a `protected:` reason. That test passes today and pins the fence.

### F5 Signals for audit_cascade, regression_tia and parallel_evaluate

Source: report C1.
- Register `AUDIT_CASCADE_SHADOW`, `TIA_SHADOW` and `PARALLEL_EVALUATE_SHADOW`. Emit each from the one function that builds the record: `recordCascadeShadowWith`, `emitTIADecision` and the branch at `orchestrator.go:624`. The JSON files stay.
- Fix the dropped error: `_, _ = regressiontia.Emit(req.Workspace, d)` at `audit.go:395` returns its error to the recorder of §10 E2. A failed write marks the cycle `incomplete` and emits a loud signal.
- `parallel_evaluate` at `shadow` computes the batch, emits the signal and dispatches serially.
- Red tests: `TestRecordCascadeShadow_EmitsSignal` and `TestOrchestrator_ParallelEvaluateShadowRecordsBatch`. Fix F5b deleted the two `TestEmitTIADecision` tests with `emitTIADecision`.
- **Decision D5 (closed 2026-11-19: retired by fix F5b, because 0 of 44 predicates are pure): retire or split `regression_tia`.** Recommendation: split the corpus into pure predicates and I/O predicates. Apply TIA to the pure ones only, using `regressiontia.Pure`. Retire the stage if fewer than 9 predicates are pure.
- *Because:* 35 of 44 predicate directories import an escape hatch, so at most 9 can skip.
- *Rejected:* allow `os` reads in the closure. A predicate that reads files can depend on a file outside its own package, so a skip can hide a regression.
- This is a product decision. Until the operator decides, the stage stays at `shadow` and its verdict stays `not ready`.

### F6 Ladder second rule

The root cause is that `nextItemStartRung` (`retry_ladder.go:209-230`) is a hand-coded copy of the decision (I6). A shared builder of inputs changes nothing, because `escalatedBuildTier` never builds an `Input`. The fix removes the copy.

- `nextItemStartRung` builds a complete `retryladder.Input` and calls `retryladder.Next`. It builds no `Decision` itself and no `Verdict{}` literal. The `Input` has the rung R0 and the worst failure class of the scoped items as `Verdict.Class`. It takes the base tier, chain, headroom and bounds from the source of `askLadder` (`retry_ladder.go:54-87`). Both functions then share one function that builds the `Input`. `Next` is the one decision home (S5).
- The event carries the decision that `Next` returned. It never carries a literal. When the scoped items have no failure class, there is nothing to ask and the function emits no event.
- **Decision (b): two named rules, one composition.** The ADR-0076 floor stays a separate rule: an item with failure_count >= `system_failure_policy.thresholds.build_deep_escalate_at_failures` (default 1) routes its next build to the deep tier. The floor has its own registry entry and its own signal. The entry lists that subtree in its `PolicySubtree`. It is removed from the `EmitSites` of the ladder entry. No ADR is reversed.
- The threshold check at the start of `escalatedBuildTier` (`retry_tier_escalation.go:44-46`) is no longer the entry gate of the function. It becomes the floor predicate, which is one input of the composition. The function asks the ladder for each scoped item that has a failure class, at each failure count.
- The build tier is the composition `max(tier of Decision.Target, floor tier)`. One function names it, and it has its own test. The ladder decision comes from `retryladder.Next` with a complete `Input`. The hand-coded `nextItemStartRung` copy is deleted. The mapping `cs.Ladder.Tier = retryEscalationTier` is deleted.
- `Next` for `Unknown` at R0 returns R1 with the same tier and a higher effort (`decide.go:85-86, 188-194`). `Capability` at R0 returns R1 when an effort step exists (`decide.go:122-124`). The old copy sent `Capability` to R2. The decision of `Next` is the truth for the ladder part.
- **Red test, verified red today:** `TestEscalatedBuildTier_UnknownClassStartsNotStop`. It drives the production caller `escalatedBuildTier` with a failure count at the threshold, a failure class `unknown`, and the ladder at `enforce`.
- The test asserts two cases. First, an `Unknown` failure at R0 of an item at the threshold dispatches at the deep tier with the effort of the ladder. Second, the threshold is 2 and `failure_count` is 1 with class `Unknown`: the ladder runs and the floor does not, and the tier equals `Decision.Target`. A mutant that returns the floor tier without condition fails the second case. It also asserts that the emitted decision has the action `raise` and the rung R1.
- A direct test of the composition function covers three orderings: `Decision.Target` below the floor tier, `Decision.Target` above it, and the floor absent.
- Today the function returns no raise, because `nextItemStartRung` returns R0 (`retry_tier_escalation.go:48-50`). The test fails for that reason.
- Second red test: `TestNoDecisionLiteralOutsideDecider` of §5. It is red today at `retry_ladder.go:210`.
- The S2 test in the registry has the form "no function emits the ladder event with a decision that did not come from `retryladder`". The field `EmitSites` lists the emit sites of the signal, and the test checks each.

### F7 Cycle spend limit

What the limiter counts:
- `max_llm_minutes` is the sum of `DurationMS` over all records of the `llm-calls` ledger of the cycle workspace, divided by 60,000 (`go/internal/cyclespend/spend.go:33-37,90`). A call without a duration adds 0. Calls that overlap in time add up. The value is therefore not the wall-clock time and not the sum of the phase durations.
- `max_audit_rounds` is `AuditDispatches` from `cycle-state.json` (`auditRounds`, `spend.go:67-78`), not the count of audit phases in the ledger.

Proposal: set each limit from the distribution of the shipped cycles, not from a guess.
- Rule, as row data: a limit is the smallest value that stops at most `MaxFalseStopPct` (2) of the shipped cycles of the last `WindowCycles` (100). The two numbers are `Promotion` fields (§4.2), not prose.
- Numbers: the operator's counts give the failing cases only (2 rounds stops 28%; 90 minutes stops 9 of 35 cycles). **F7 is incomplete: I did not have the per-cycle ledger values, so this file proposes no final number.** The scorer prints the distribution and the number. The builder of F7 must fill in the numbers from `evolve stage report` and record them in `policy.json`.
- Red test: `TestSpendLimitWouldStopShippedCycles` replays a fixture of shipped cycles (`testdata/spend_distribution.json`, rows of rounds and minutes per cycle). It fails when the false-stop rate is above `MaxFalseStopPct`. The fixture holds the shape of the live data and does not read the live ledger. A second fixture with a known 28% rate must give a red. That proves the test can fail.

## 8. Order of work

1. Build the type unification and the registry extension (§4, §5) and the baseline (§6.2). This is one `Stage` type, the `Shadow` field, the `stage-feature` kind and the schema walk. The tests run green against the baseline. A key outside the baseline that breaks a rule fails. Each later step removes keys from the baseline.
2. The integrity layer of §10 (recorder, decision counter, evidence record, digest, `evolve stage verify`, ratchet).
3. The migration verb and the genesis record (§6.2), then the preflight check of §6.3. The check must not be active before the genesis record exists.
4. F6, then F5, F4, F2, F1, F3, F7. The order puts the cheap fixes that remove a lost escalation or a false stop first.

Each step turns more entries green. A step ends when the tests of its key are green.

## 9. Guards

| Rule | Guard | State |
|---|---|---|
| Each stage key has an entry | `TestEveryStageKeyHasRegistryEntry`, `TestNoStringTypedStageField` | proposed |
| Each signal has an emitter | `TestEverySignalCodeHasNonTestEmitter` | proposed |
| Shadow differs from off | `TestShadowIsNotOff` | proposed |
| One decision home | `TestNoDecisionLiteralOutsideDecider` | proposed |
| Enforce path is wired | `TestEnforceBranchActsAtEnforce`, `TestStageGateRefusesEnforceWithoutActor` | proposed |
| Evidence is reachable and scored | `TestPromotionReachableByGoodBehavior`, `TestScorerCountsFalseStopEscapeConflict` | proposed |
| One rule, one home | `TestSharedRuleHasOnePredicate` | proposed |
| Evidence is tamper-evident | `TestEvidenceEditFailsVerify`, `TestEvidenceDeleteFailsVerify`, `TestEvidenceReorderFailsVerify` | proposed |
| Evidence is complete | `TestDeciderReturnWithoutWriteMarksIncomplete`, `TestEvidenceWriteFailureMarksCycleIncomplete`, `TestLateDecisionNotCounted`, `TestFailedCycleEvidenceIsSealedAndScored` | proposed |
| Score is reproducible | `TestVerdictRecomputesFromFixtureChain`, `TestScorerMutationChangesGolden` | proposed |
| Verdict is bound to its version | `TestVersionDigestStableUnderUnrelatedChange`, `TestVersionDigestChangesWithDeciderClosure`, `TestEvidenceOfOtherVersionNotCounted` | proposed |
| Contract cannot weaken silently | `TestRegistryRatchet`, `TestViolationBaselineOnlyShrinks`, `TestContractPathsAreProtectedSurface` | proposed |
| New flip is gated, old keys accepted | `TestPreflightRefusesEnforceWithoutFlipRecord`, `TestHeadPolicyAccepted`, `TestStageGateRefusesNotReadyVerdict` | proposed |
| Every claim is checkable | `TestStageStatusListsEveryRow` | proposed |

## 10. Evidence and contract integrity

The operator's order: keep the evidence and the contract intact, and make both verifiable. This section holds the seven rules.

### 10.1 What exists today

| Store | Tamper-evident? | Source |
|---|---|---|
| Ledger | yes: `entry_seq` and `prev_hash` bind each entry to the one before | `go/internal/core/ports.go:69-70`, `go/internal/adapters/ledger/anchor.go:30-38` |
| `evolve ledger verify` per cycle | yes for the ledger | `go/internal/ledgerverify/verify.go:65` (`VerifyCycle`) |
| `.evolve/runs/cycle-N/signals.ndjson` | no: a plain append file | `go/internal/signalcenter/sinks.go:22-28`, `go/internal/signalcenter/event.go:222` |

*Implication:* evidence must move from the signal stream into the chain. The signal stays as the live view for people. The chain holds the proof.

**One verify verb.** `evolve stage verify` is the verb for evidence tamper. It runs `evolve ledger verify` for the cycle first, and then it checks the evidence chain and the commitment in the ledger entry. A reader needs one verb, and the output names the failing layer.

**Decision D7.** Use a per-cycle evidence digest. The ledger entry of the cycle commits to it. Do not write one ledger entry for each decision.

- *Because:* a cycle can make many decisions for many keys. One digest keeps the ledger small and still binds every record.
- *Rejected:* one ledger entry for each decision. It grows the ledger with the number of keys and decisions, and it slows `evolve ledger verify`.
- *Rejected:* a digest file beside the signals with no ledger link. An attacker edits both files together.
- *What changes the decision:* if a cycle makes more than the entry size limit allows, the evidence moves to a sidecar file. The ledger entry still commits to its digest, so the rule E1 does not change.

### 10.2 The evidence rules

**E1 Evidence is durable and tamper-evident.**
- Each shadow decision is an evidence record. Its fields are: scope, scope id, session, stage key, kind, decision version digest, `would_*` fields, inputs digest, build revision and a sequence number. The kind is `open`, `decision`, `late` or `close`.
- The chain is one chain for each scope key. A scope key is a pair of a scope kind and an id. A cycle, a pool, a fleet tick and a curator run are scopes. Records of two scopes with the same id never share a chain.
- Each record holds the hash of the record before it. The first record of a scope has an empty `prev`. The ledger entry of the scope commits to the hash of the last record and to the count (E2).
- An edit, a deletion or a reorder of any record fails `evolve stage verify`. The verb names the first record that fails, by cycle and sequence number.
- Tests: `TestEvidenceEditFailsVerify`, `TestEvidenceDeleteFailsVerify`, `TestEvidenceReorderFailsVerify`. Each has a mutation in §5.

**E2 Evidence is complete.**
- The decider does not write evidence on its own. It goes through the recorder, and the recorder adds one to an in-memory decision counter of the key before it writes the record. The counter does not depend on the write.
- A session is one process that writes evidence. A session writes an `open` marker before its first record and a `close` marker at its end. The close marker holds the counter of each key. A cycle has one working file, shared by its sessions under a file lock. The ship child process and a resumed cycle are sessions of the same cycle.
- The seal copies the working file into the write-once store `ledgerartifacts` and appends one ledger entry. The entry has the role `stage-evidence`, the kind `stage-evidence-seal`, the scope label in `cycle_label`, and the digest of the stored copy in `artifact_path` and `artifact_sha256`. Its `message` holds the version, the count, the last hash, the counters and the cause of an incomplete scope. A second seal of a scope must contain the first as a prefix. The reader uses the last seal.
- A counter that differs from the count of records of its session is a gap. Three more faults mark the scope `incomplete`: a session without a close marker, a torn last line and a failed write. The causes are `gap`, `open-session`, `torn-tail` and `write-failed`. No signal file is an input.
- A write failure of evidence emits a loud signal and marks the evidence of the cycle `incomplete` in the ledger entry. An `incomplete` cycle never counts toward readiness. The scorer lists such cycles by number. The reader treats a working file as `incomplete` when no seal pins it or when it differs from its last seal.
- **The seal point.** The seal is its own unconditional `defer`. It is registered in both `runCycle` and the resume path (`resume_execution.go`) before the `abnormalEpilogue` defer, so it runs after it. It is not inside `abnormalEpilogue` or the resume defer, because their guards skip the normal, parked and deadline exits. The seal reads the exit kind from `retErr` and the context. Every exit passes through it: normal completion, `abnormalEpilogue`, `deadlineStamp`, the parked return and a failed `finalizeCycle`.
- **The ledger entry of the seal.** The seal appends its own typed entry through `o.ledger.Append`, the port that `cyclerun_record.go` uses. It does not depend on `emitCycleClose`, whose sink is a signal and not the ledger. The seal writes one ledger entry of kind `stage-evidence-seal` on every exit. The test asserts the ledger entry, not a signal.
- **Why every path.** A failed cycle holds the decisions that matter most: the ladder decides on failing cycles, and an escape or a wrong raise shows only there. A seal at `evolve ship` alone drops these cycles and skews the score toward `ready`.
- **Order.** The seal comes after the last decision of the cycle. The ship-phase decisions (the scan skip, the hold and the main-red stop) run before the seal. The test `TestFailedCycleEvidenceIsSealedAndScored` pins this order. It drives each of the five exits (abort, deadline, parked, failed `finalizeCycle`, normal) through both entry points, fresh and resume: 10 cases.
- **A decision after the seal.** The recorder is closed at the seal. A decision after it is written as a record of the kind `late`. The scorer lists a `late` record and never counts it. The next verify shows it by cycle and sequence number.
- Tests: `TestDeciderReturnWithoutWriteMarksIncomplete`, `TestEvidenceWriteFailureMarksCycleIncomplete`, `TestLateDecisionNotCounted`, `TestFailedCycleEvidenceIsSealedAndScored`.

**E3 Scoring is reproducible.**
- The verdict of a stage is a pure function of the chained evidence plus the chained cycle outcomes. It reads no signal file and no other input.
- `evolve stage verify <key>` recomputes the verdict from the chain alone. It prints the evidence range (first and last cycle) and the digest that it verified.
- It also prints the counts: decisions, false stops, escapes, conflicts and incomplete cycles. It lists each counted case with its cycle and sequence number.
- A broken chain gives the verdict `not ready` and a typed error. It never gives a partial verdict.
- Tests: `TestVerdictRecomputesFromFixtureChain` recomputes the verdict from a fixture chain and compares it with a golden. `TestScorerMutationChangesGolden` changes the scorer and expects the golden to fail.

**E4 The verdict is bound to what it measured.**
- The decision version is a content digest of two parts.
  - The source of the Go package closure of the decider: the `Closure` field, its module-local imports and the toolchain line of `go.mod`.
  - The policy subtree that the decider reads: the `PolicySubtree` field.
- It is not the build id of the binary. It is not the digest of the whole `policy.json`.
- *Because:* the loop ships its own code each cycle, so a build id or a VCS revision changes about once per cycle. A rule that counts only the current build holds about one cycle of evidence and never reaches `MinDecisions`. That repeats the fault of I1 one layer up. An unrelated policy edit has the same effect.
- **The rule.** Evidence from earlier cycles counts while the digest of the decision version is unchanged. When the digest changes, the evidence of the key restarts for the new version. The verb prints the count of records for each version. Records of other versions are reported and not counted.
- Q2 and Q3 of round 1 are closed by this rule: no policy digest rule and no build id source are needed.
- Tests: `TestVersionDigestStableUnderUnrelatedChange` edits an unrelated package and an unrelated policy key, and expects the same digest and the same evidence count. `TestVersionDigestChangesWithDeciderClosure` edits a closure package and a subtree key, and expects a new digest. `TestEvidenceOfOtherVersionNotCounted` expects that old-version evidence does not count after the digest changed. `TestPromotionReachableByGoodBehavior` also replays N cycles that ship unrelated code and policy edits, and expects `ready`.

### 10.3 The contract rules

**C1 The contract cannot be weakened silently.**
- The registry (`gatecontract`, `gateregistry` and the shadow-contract fields), the recorder package and the conformance tests are protected surfaces. Their paths join `ProtectedSurfaceManifest` (`go/internal/guards/integrity_surface.go:37`). `go/internal/gatecontract/` and `go/internal/gateregistry/` are already `GateGuarantee` rows by gate-contract §6.7. A change goes through the operator approval path that protected paths use.
- A ratchet file lists each entry, each rule, each `Fields` item, each emit site and each `Promotion` number with its direction (§4.2). The known-violations baseline (§6.2) is in the same file and may only shrink. A change that removes an entry, drops a rule, drops a field or an emit site, or loosens a number fails `TestRegistryRatchet`. A tighter number or an added entry passes. A lane cannot drop an emit site from the S2 table. The genesis record of §6.2 is in the ratchet.
- Tests: `TestRegistryRatchet`, `TestViolationBaselineOnlyShrinks`, `TestContractPathsAreProtectedSurface`.

**C2 The flip is gated by the verified verdict.**
- A new flip needs `evolve stage flip <key>` (§6.1), which runs `evolve stage verify <key>` and needs `ready` on an intact chain. The flip record (who, when, the verified digest) is written into the chain by the verb.
- The loop-start preflight (§6.3) refuses a key at `enforce` that has no flip record of any kind, with a typed error. A grandfather record counts as a record (§6.2). The check is active only after the genesis record exists. `policy` does not do this check.
- Tests: `TestPreflightRefusesEnforceWithoutFlipRecord`, `TestHeadPolicyAccepted`, `TestStageGateRefusesNotReadyVerdict`.

**C3 Every claim is checkable.**
- `evolve stage status` lists each stage key with its rows S1 to S5 and E1 to E4 as pass or fail, with the evidence verdict from `evolve stage verify`. It shows a grandfathered key and a pending flip apart.
- The flip procedure of §6 uses only this output and the output of `evolve stage verify`.
- Test: `TestStageStatusListsEveryRow`.

### 10.4 Worked example

Stage `regression_tia.stage`, cycle 2400, two decisions.
1. The audit phase asks the recorder twice. The counter is 2. It writes two evidence records, sequence 1 and 2. Record 2 holds the hash of record 1. The audit phase also emits two `TIA_SHADOW` signals.
2. The seal of cycle 2400 commits to the hash of record 2, the count 2 and the counter 2.
3. An attacker deletes record 1. `evolve stage verify` finds that the count is 2 and the file has 1 record. It names cycle 2400, sequence 1.
4. An attacker edits `would_skip_count` in record 1. The hash of record 1 differs, so record 2 fails its link. Verify names sequence 2.
5. A decider returns without a write. The counter is 3 and the count of records is 2. Verify marks cycle 2400 `incomplete`.
6. The operator runs `evolve stage verify regression_tia.stage`. The output lists cycles 2380 to 2400, the digest of the decision version, and each escape. The verdict is `not ready`.

### 10.5 Open questions

- Q1 is closed by K2: the working file is `.evolve/stage-evidence/<scope>.ndjson`, outside `runs/`. The stored copy is in the write-once store `ledgerartifacts`, and the ledger entry pins it by digest.
- Q5: which standard library paths belong in the escape hatch list (§7 F2).

## 11. Limits

- This file has no code. The registry extension, the report verb and the tests are a design.
- The line numbers are from HEAD `64bcd83c4`. They move with edits.
- The claims about cycles 2356 to 2390 and the count of 10 enforced keys come from the reports and the operator. The raw cycle artifacts were not in the worktree.
- F7 is incomplete: it gives the method and not the final numbers.
- `evolve docs ste-lint --strict` reports 0 findings on this revision.
- The chain proves that evidence did not change after it was written. It does not prove that the decision function was right when it wrote the record. S2 and the scorer tests cover that.
- A party that can rewrite the whole chain and the head anchor can forge evidence. The hash chain detects an edit by a party that cannot do this. The protected paths (C1) and the ship gate limit who can.
- The file:line cites of §10 were read at HEAD `64bcd83c4`. The recorder, the seal writer and the ledger reader exist as packages from K2a and K2b. The wiring of the recorder into the cycle, the default reader of `evolve stage report` and the verbs `evolve stage status`, `evolve stage flip` and `evolve stage migrate` do not exist yet.
- A scorer proves a stage only for the cases that the data holds. A rare failure can stay unseen until the flip. The stop-and-revert path of each enforce branch is the second defense.
- The description of the Go test cache in §7 F2 comes from the research notes of 2026-11-15 and not from a test run.
