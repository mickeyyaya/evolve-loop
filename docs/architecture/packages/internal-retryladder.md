# internal/retryladder

## Purpose

`internal/retryladder` classifies a failed phase try into one class: `infra`, `quota`, `halt`, `spec`, `capability` or `unknown` (component L1). It then picks the next rung (component L2). See [ADR-0142](../adr/0142-a-capability-failure-climbs-one-bounded-retry-ladder.md), [spec §3](../retry-ladder.md#3-the-classifier) and [spec §4](../retry-ladder.md#4-the-rungs).

## Design

- `Classify` reads `Facts` and calls no model. The order is fixed: halt exit, benched quota, infra exit or teardown error, spec, capability, unknown.
- Exit 85 is also `ExitUnknownPrompt`. It is `quota` only with a bench, else `infra`.
- A floor failure is `spec` when it is protected or outside the lane scope. It is `capability` when it is inside the scope.
- `Fingerprint` is the sorted set of bracketed violation codes. Without codes it is a hash of the normalized text.
- `Next` reads `Input` and returns a `Decision`. It calls no model and reads no clock.
- `infra` and `quota` retry on the same rung. `halt` and `spec` stop. `unknown` raises R0 to R1 and stops after that.
- `capability` goes R0 to R1 on a new fingerprint when the base effort has a step. Otherwise it goes R0 to R2. It goes R1 to R2 on the next fail.
- R2 is one tier above the base tier. The first family of the chain with headroom at that tier supplies the model. The effort is the larger of the R1 effort and the tier default, at most `max_effort`.
- With no headroom at R2, the decision carries `LADDER_NO_HEADROOM`. R0 goes to R1 and R1 stops. A bound that blocks the step records no signal.
- A capability failure at R2 stops. It becomes `spec` with `LADDER_STOP_SPEC` when its fingerprint is an earlier one.
- Unset bounds are `max_tier` deep and `max_effort` high.
- `Context` and `Stage` name the try-record values of [spec §6](../retry-ladder.md#6-the-try-record). Until L5 wires the decider, the bridge writes `ContextNew` and `StageShadow` on each try.
- `Input.Critic` is the rung of the shadow critic arm. `Decision.CriticRung` copies it and nothing reads it.

## Invariants

- The package does not import `internal/core`. Predicate `TestC1973_003` checks this.
- The normalizer copies the rules of `core/failure_digest.go`. A rule change must change both until a later cycle moves the shared rules into one package.
- A raise moves the rung up, so a chain has at most two raised tries. No target has a tier above `max_tier` or an effort above `max_effort`.
- `Next` has no production caller yet. A later ladder component wires it into the loop.
- `BuildReport` groups try records by rung and class (component L7). A fix is a try above R0 with exit code 0. It sorts the rows by rung, then class, and skips a record without a rung.
- `LoadRuns` reads `llm-calls.ndjson` of each run dir in name order. It skips a dir without a ledger and fails on an absent runs dir. `evolve ladder report` prints the result and writes nothing.
