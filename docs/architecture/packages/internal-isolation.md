# internal/isolation

> Decision: [ADR-0154](../adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md). Spec of record: [llm-isolation.md](../llm-isolation.md) §14 and §15. Plan: [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md) component X1. Linked into the binary by `go/cmd/evolve/cmd_signals.go`, so `evolve signals codes check` sees the codes.

## Purpose

`internal/isolation` holds two things of the agent cell. The first is the conformance matrix: the 19 rows M1 to M5 and N1 to N14 of spec §14, with the state of each row. The second is the set of Signal Center codes of module `isolation`: one `ISOLATION_I<n>_VIOLATION` code for each invariant I-1 to I-9.

The package runs no check. Each row has an owner test in the package that owns the behavior. The owner test runs the check and gives the result to `Judge`.

## Design

- **The catalog.** `Rows` returns a copy of the rows in spec order. A `Row` has the id, the check and the invariant. It also has the component that turns it green (`ClosedBy`), the state (`Open`) and the owner test file under `go/`. An open row has a `Cause`: a text that its failure must contain. `Lookup` finds one row.
- **The verdict.** `Judge(id, failure)` returns a log note or an error. A green row must pass. An open row must fail with its cause, and the note names the component that closes it. An open row that passes is an error, so a fix cannot go unseen. An open row that fails for another reason, or an unknown id, is an error.
- **No silent drop.** `TestRows_EachOwnerTestJudgesItsRowSoNoRowIsSilentlyDropped` parses each owner file with `go/parser`. A test function must call `isolation.Judge` with the row id, or with a table field whose keyed value is the id. A comment, a stray literal or a helper outside a test function does not count.
- **The codes.** `init` registers the nine codes with `signalcenter.RegisterCode`. `ViolationCode(n)` gives the code of invariant I-n. `docs/architecture/signal-codes.md` is regenerated from the registry.

| Owner test | Rows |
|---|---|
| `go/cmd/evolve/e2e_cycle_cli_matrix_test.go` (tags `e2e evolve_test_phases`) | M1 to M5, N1, N4, N6 |
| `go/internal/bridge/sandbox_contract_integration_test.go` (tags `integration`, `darwin`) | N3, N5, N7 to N10 |
| `go/internal/sysexec/isolation_matrix_test.go` | N2 |
| `go/internal/guards/isolation_matrix_test.go` | N11 |
| `go/internal/core/isolation_matrix_test.go` | N12, N13 |
| `go/internal/bridge/isolation_matrix_test.go` | N14 |

## Invariants

- The package imports only `signalcenter` and the standard library. It reads no file and starts no process.
- A component that closes a gap sets `Open` to false for its row. The same PR marks the row green in spec §14.
- No row launches a real LLM CLI. The matrix and the exec row use `fakeclitest` stubs.

## Findings

- At X1, rows M1 and N9 are green. The other 17 rows are open. Rows M2 to M5 fail on the Claude floor in `go/internal/cliroute/compile.go`.
