# internal/agentrequest

> Decision: [ADR-0154](../adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) I-3 (amended 2026-10-26) and [ADR-0155](../adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md). Spec of record: [llm-isolation.md](../llm-isolation.md) §7.2 and §7.2.1, [cli-contract.md](../cli-contract.md) §4.3 and §4.5. Plan: [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md) component X4, item `isolation-dispatch-request-channel`.

## Purpose

A phase agent cannot start another phase agent. The operator said on 2026-10-26: "if it was to spawn other phase agent, it must send the request back to the evo loop for orchestrator to justify if it is the reasonable request."

`internal/agentrequest` is that path back. The agent writes a request file in its workspace. At the end of the phase, the pipeline validates the file in Go, the orchestrator agent judges it, and the pipeline runs an approved request. The result goes into the index of the requester.

## Design

The steps, the policy, the record, the signals and the refusal order are in [llm-isolation.md](../llm-isolation.md) §7.2.1. This note names the parts of the package.

- **`Request` and `parse`.** The strict decoder of `evolve.request/1`. `Role` reads `what.args.role`.
- **`Channel.Handoff` and `read`.** The walk over the request files of one `frame`: the cell, the workspace, the name prefix and the depth. `claim` finds the record of a name and compares the `sha256`.
- **`Channel.check`.** The Go validator. `inputEntry` applies the one input rule, `inputindex.FileEntry` inside the workspace.
- **`writeEvidence` and `EvidenceWorkspace`.** The evidence index of the judge seat: `q1` is a copy of the request, and `i1` and up are the checked inputs.
- **`Channel.judge`.** `phasecontract.ParseDiagnosis`, then `selfheal.Validator.Decide` at tier T2 with the request as validated.
- **`Record`, `readRecord` and `writeRecord`.** One file for each decision in `inputindex.RequestsDir`.
- **`emitRequest` and `emitDecision`.** The two signal kinds of cli-contract.md §4.5.

The ports `Judge`, `Dispatch`, `Emit` and `Now` keep the package free of the CLI. `cmd/evolve` (`requestHandoff`) runs both seats through `evolve agent run`, and `core.WithRequestHandoff` calls the channel after each phase dispatch.

## Invariants

- The validator runs before the judge, so a request that fails a Go check costs no LLM launch.
- No agent text decides a run alone. The Go checks, the schema of the judgement and the catalog precondition `request-validated` must all pass.
- A record is written before its decision signal. A record that cannot be written gives an error that names the request, and no decision signal.
- `TestDispatchRequest_AValidRequestIsDispatchedByThePipeline` proves the path from the file to the index entry.

## Limits

- Only the kind `dispatch` has an executor. Item `cli-agent-tool-surface` adds the others.
- The cycle cap counts each record of the cycle, also a refused request.
- The channel trusts the `Dispatch` port to give a workspace that the pipeline owns. It removes that workspace.
