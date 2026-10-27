# internal/adapters/runner

## Purpose

`internal/adapters/runner` gives the two `core.PhaseRunner` adapters of the "Approach C" plan. The first is the in-process Go implementation of the phase, which is itself a `PhaseRunner`. The orchestrator calls it directly and needs no wrapper. The second is `SubprocessRunner`. It forks an external binary and speaks JSON on stdin and JSON on stdout. A third party can then implement a phase in any language.

## Design

- **Independence.** The orchestrator sees only `core.PhaseRunner`, so it never knows which adapter is in play.
- **No environment switch.** X18d removed `PerPhase` and its `EVOLVE_PHASE_<NAME>_BIN` read (ADR-0156 I-16). No production code calls the package now. Its removal is a follow-up of X18d in [llm-isolation-2026-10.md](../../plans/llm-isolation-2026-10.md).
- **`SubprocessRunner.Run`.** It writes the `PhaseRequest` as JSON to the subprocess stdin and reads stdout as the `PhaseResponse` JSON. It captures stderr for the diagnosis of a non-zero exit. The stderr text in the error is bounded.
- **`CmdRunner` seam.** `NewSubprocess` takes a `CmdRunner`. A nil runner selects the default `exec.CommandContext` runner (the production wiring). Tests inject a fake.
- **`execRunner`.** It maps an `exec.ExitError` to the exit code and a nil error, so the exit-code branches of the adapter work.

## Invariants

- A non-zero exit surfaces as an error. A malformed response surfaces as an error and never as a zero value. A start failure (for example a missing binary) wraps into an adapter error. Long stderr is truncated. Pinned by `runner_test.go`.
- The in-process runner is not called when the env var selects a binary. Pinned by `runner_test.go`.

## Findings

- None open.
