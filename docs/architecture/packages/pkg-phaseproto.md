# pkg/phaseproto

## Purpose

`pkg/phaseproto` is the public wire protocol. It lets the Go orchestrator drive out-of-process phase agents (Node, Python or Go subprocesses) through the same `core.PhaseRunner` interface as in-process runners. Every exported name, JSON tag and value is part of the contract.

## Design

- **Wire shape.** One JSON `Envelope` per line on stdin and stdout. The envelope has a versioned header (`Version`, `Kind`, `CorrelationID`), a `Payload` and a structured `WireError` for the failure path. The envelope kinds are request, response and error. A request or response carries `Payload`, and an error envelope carries `Error`.
- **`Payload` is `json.RawMessage`.** The decoding of the inner payload stays with the receiver. The bytes keep their order through middleware, and a relay can pass envelopes along without knowledge of the inner schema. The inner `core.PhaseRequest` and `core.PhaseResponse` schema can then change on its own.
- **`WireError`.** `Code` is the load-bearing field: the failure adapter dispatches on it. `Message` is for people. `Retryable` steers the retry policy of the orchestrator without parsing English. `errors.Is` matches a `*WireError` by `Code`, for example `errors.Is(err, &WireError{Code: "BUDGET_EXCEEDED"})`.
- **One process per `Run`.** `SubprocessRunner.Run` starts one child per call, writes one request envelope to its stdin and reads one response envelope from its stdout. The protocol stays synchronous and no state leaks between cycles in the child. The orchestrator owns the life cycle and the child owns one phase call. `ServeStdio` is the matching helper for a server in Go, and it handles exactly one envelope and then returns.
- **Run outcomes.** A response is returned to the caller. An error envelope is returned as a `*WireError`. Garbage, or no envelope, gives `CodeDecodeFailed`. A non-zero exit gives `CodeChildCrashed` that wraps the exit error. A canceled context signals the child through `exec.CommandContext`.
- **Handler errors.** `Handler` returns a `*WireError` unchanged. `ServeStdio` wraps any other error as `CodeHandlerError`.
- **Line reading.** `readOneEnvelopeLine` returns the line without its newline. An EOF after some bytes is not an error, because `io.Reader` may return data and `io.EOF` together.
- **Correlation id.** `defaultCorrelationID` is the process id plus nanoseconds. It does not need to be cryptographic. The orchestrator tags the ledger entries, and the id only has to match a request to its response in one `Run`.
- **Seams.** `marshalJSON` and `correlationIDFn` are test seams. They let tests reach the error branches of `EncodeRequest` and `EncodeResponse`.

## Invariants

- **The error codes are stable strings.** Subprocess agents in other languages must agree on them. Do not rename one without a version bump.
- **`WireVersion` changes only in a coordinated rollout.** All subprocess agents must be deployed again before an orchestrator speaks the new version. The test pins the value.
- **The wire bytes are deterministic JSON** in the default order of `encoding/json`. Pinned in `wire_test.go`.

## Findings

- None.
