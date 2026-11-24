# internal/promptboundary

> The plan: [token-telemetry-2026-07.md](../../plans/token-telemetry-2026-07.md), phase-2 item `tokenopt-cache-discipline-audit`. The research: [token-concurrency-2026](../../research/token-concurrency-2026/).

## Purpose

`internal/promptboundary` owns the line that splits a composed prompt in two parts. The part above the line is the static prefix. It must be the same bytes in each cycle and each lane, so the provider prompt cache can reuse it. The part below the line is the cycle context. It holds the values that change in each cycle or try.

The package has three symbols:

- `Boundary`, the heading `\n\n## Cycle Context\n`;
- `StaticPrefix(prompt)`, the text before the first `Boundary`, or the whole prompt when there is none;
- `Place(prompt, block)`, which puts `block` directly after the first `Boundary`.

## Design

- **A leaf.** The package imports the standard library only. `Place` and `StaticPrefix` use one `boundaryIndex`, so both find the same first boundary.
- **The `Place` contract.**
  - An empty `block` returns the prompt unchanged. It adds no boundary.
  - A prompt with no boundary gets the boundary and the block appended at the end.
  - A prompt with a boundary gets the block directly after the first one.
- **One home for the literal.** `runner.BaseCycleContext`, the retro phase and the bridge use `Boundary`. No other non-test Go file may write the heading.
- **The placement rule.**
  - `CorrectionDirective` changes in each try. The bridge puts it below the boundary, as `### Correction`, before `### Cycle facts`. It goes in before `injectContract`, so the deliverable-contract tail stays at the end of the prompt.
  - `OperatorDirectives` stay above the boundary, at the top. This is the one named exemption. The reason: they are the operator's top authority, and they are fixed for the cycle (`core/cyclerun_dispatch.go`), so the first attempts already hit the cache.
  - The postmortem section of a try is appended after the boundary ([internal-phases-runner.md](internal-phases-runner.md)).

## Guards

The tests in `go/internal/promptboundary` run in the default `go test`:

- `TestSites_NoDynamicBytesAboveBoundary` runs the real runner and the real bridge `ComposePrompt` for each registered phase. It puts a sentinel in each text field of `PhaseRequest`, except the `static` and `unfillable` fields, and compares the prefix of two variants.
- `TestSites_BoundaryExactlyOnce` fails when a composed prompt holds the boundary more than once, for example when a persona quotes the heading.
- `TestSites_CoverAllRegistered` fails when a registered phase is neither a site nor exempt with a reason.
- `TestRequestFields_AllClassified` fails when a `BridgeRequest` field has no class, or when a field cannot take a sentinel and has no reason.
- `TestBoundaryLiteral_SingleSource` fails when another non-test Go file writes the heading.

The guard tests import the phase packages, so `go test ./internal/promptboundary` compiles the phase stack.
