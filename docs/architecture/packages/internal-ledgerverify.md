# internal/ledgerverify

## Purpose

`internal/ledgerverify` is the Go port of `verify_cycle` from the legacy dispatch script. It reads the `ledger.jsonl` entries of one cycle and asserts that the pipeline ran end to end. Scout, builder and auditor each need at least one entry with `exit_code` zero. Intent is also needed when the cycle began with `intent_required=true`, and memo is also needed when the audit verdict was PASS. The package is pure: it depends on `core.Ledger` and small data types. The caller in `cmd_loop` gives it the context.

## Design

- **Two ledger vocabularies.** The bash `subagent-run.sh` records `kind="agent_subprocess"` with the AGENT name as role (`scout`, `builder`, `auditor`). The Go native orchestrator records `kind="phase"` with the PHASE name (`scout`, `build`, `audit`). The `evolve loop` path writes only `phase` entries.
- **Both vocabularies verify.** A verifier that matched the bash shape alone reported `missing [scout builder auditor]` for every native cycle (cycle 137). `canonicalRole` and the kind allow-list fold both vocabularies onto the buckets scout, builder, auditor, intent and memo. Bash and Go cycles then verify the same way. A role that is not recognized gives an empty string and is not counted. A cycle with both spellings does not fault twice, because either spelling satisfies a role.
- **Only real runs count.** `countsTowardVerify` accepts the kinds `agent_subprocess` and `phase`. Every other kind (`agent_fanout`, `cycle_terminal`, `routing_decision` and so on) is bookkeeping and cannot satisfy a role. An entry of another cycle does not count, and neither does an entry with a non-zero exit code, such as a timeout.
- **One declaration of a complete cycle.** The required roles come from `phasecontract`, so `cyclehealth`, `redteamcheck` and this verifier share them.
- **Result.** `OK` is true if and only if `Missing` is empty. The counts reflect the `exit_code==0` entries. `Required` is a snapshot of the role set that `Options` resolved to. It serves diagnostics and the log line of `cmd_loop` that mirrors the bash log.
- **Errors.** `VerifyCycle` returns `OK=false` with a nil error when roles are missing. The caller decides to halt, classify or continue under `EVOLVE_DISPATCH_POLICY`. A non-nil error is only for ledger I/O failures. `roleCount` returns 0 for an unknown role. That branch is defensive, because `VerifyCycle` never passes one.
- **Empty triage termination.** The caller can validate the terminal no-work result of the orchestrator and its explicit empty triage artifact. The completed chain is then scout plus triage, and not the normal floor.
- **Optional roles.** When `IntentRequired` is set, an intent entry must exist. When `CycleVerdict` is PASS, a memo entry must exist (the Layer P and Layer E3 contract), except in an empty triage termination. Any other verdict skips the memo check.
- **Context loading.** `VerifyContext` is separate from `VerifyCycle` so that unit tests do not need to create `cycle-state.json` and `.cycle-verdict` files. `LoadVerifyContext` resolves `IntentRequired` as `cycle-state.json:intent_required` if that file exists, then `state.json:intent_required`, then false. The per-cycle file wins over the global state. `CycleVerdict` is the trimmed content of `<workspace>/.cycle-verdict`, or empty if the file is missing. A missing or unreadable file is not fatal, because the strictest behavior is no extra requirement, as in the bash defaults.
- **`readIntentRequired`.** It reads the top-level bool without a full struct decode. `cycle-state.json` has many fields that `core.CycleState` does not model, so a `map[string]any` is the safe portable option. It matches `jq -r '.intent_required // false'`. A string `"true"` is not coerced to a bool.

## Invariants

- **The native vocabulary verifies.** Pinned by the `kind="phase"` tests in `verify_test.go` (the cycle-137 regression) and by the mixed-vocabulary test.
- **Bookkeeping kinds never satisfy a role.** Pinned in `verify_test.go`.
- **The writer and the counter agree on the entry shape.** A test writes entries through the real ledger adapter and verifies them, which catches drift in the JSON shape of `LedgerEntry`.

## Findings

- None.
