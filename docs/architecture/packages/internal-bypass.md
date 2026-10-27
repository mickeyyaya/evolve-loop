# internal/bypass

## Purpose

`internal/bypass` is the audit of the bypass parameters (ADR-0156 I-16, X18d of [llm-isolation.md](../llm-isolation.md) §27). A bypass is an explicit CLI parameter, never an environment name. Each use writes a record of who used it, when and why.

## Design

- **`Check` gates a use.** The parameter takes the reason as its value, for example `--sandbox-off=<reason>`. An empty value means that the caller does not use the bypass, so `Check` sees only a set reason from the CLI. `Check` refuses an empty reason with `ErrNoReason` for a direct caller. It refuses a use inside an LLM cell with `ErrInCell`. A cell has `EVOLVE_PHASE` set or a dispatch depth more than 0.
- **`Record` writes the audit.** It first appends one ledger entry of kind `control_bypass` and role `operator`. The entry has the bypass name in `action`, `<actor>: <reason>` in `message` and the code origin in `source`. Then it calls `Emit`. A ledger error is returned, and no signal fires, so no signal reports a use that the command refused.
- **`Emit` is the signal alone.** It emits one `CONTROL_BYPASS_USED` signal (module `control`, kind `control.bypass`, severity INFO) with the fields `name`, `actor`, `reason` and `at`. A nil center is a no-op.
- **`Use` carries who, when and why.** It has the name, the reason, the actor, the origin, the cycle and the time `At`. A CLI use has cycle 0, because the cycle is not known when the command starts.
- **`Actor` names the operator.** It gives the account name of `os/user`, or `unknown`.
- **`Appender` is the narrow port.** `Record` needs only `Append`, so a test needs no full ledger.

## Consumers

- `cmd/evolve`: `bypassAuditor.record` writes the record for `evolve ship --class manual --auto-confirm`, for `--sandbox-off` of `evolve loop` and `evolve cycle run`, and for `evolve bridge launch --human-input` through the `bridge.Deps.RecordBypass` sink. A failed record stops the command before the bypass takes effect.
- `internal/adapters/bridge`: `NewDefault` calls `Emit` once for each credential opt-in of the policy that is on, with the actor `policy.json`.

## Invariants

- No reason, or a use inside a cell, gives a refusal (`TestCheck_NeedsAReasonAndRefusesInsideACell`).
- One use gives one ledger entry and one signal with who, when and why (`TestRecord_WritesOneLedgerEntryAndOneSignalWithWhoWhenWhy`).
- A ledger failure is returned, and no signal claims the use (`TestRecord_ALedgerFailureIsReturnedAndNoSignalClaimsTheUse`).
- `Emit` is the signal alone, and a nil center is a no-op (`TestEmit_IsTheSignalAloneAndANilCenterIsANoOp`).
- `Actor` falls back to `unknown` (`TestActor_NamesTheOperatorOrUnknown`).
