# internal/phases/debugger

## Purpose

`internal/phases/debugger` is the ship-recovery diagnosis phase. When the ship phase fails with a novel or unresolved error, the orchestrator routes the structured `core.ShipError` here. The persona `evolve-debugger` diagnoses the root cause and writes `debug-decision.json` with one recovery action: `RESHIP`, `RERUN_PHASE` or `BLOCK`. `Classify` reads that file and maps it to a verdict and the cross-phase signals that the orchestrator's `decideAfterDebugger` consumes. The phase boilerplate is in `internal/phases/runner`.

## Design

- **Mapping (`Classify`).**
  - `RESHIP`: PASS, `debugger.action=RESHIP`, next phase ship.
  - `RERUN_PHASE`: PASS, `debugger.action=RERUN_PHASE` and `debugger.rerun_phase=<phase>`. With no phase named, it defaults to audit, the dominant precondition-recovery target.
  - `BLOCK`: FAIL, `debugger.action=BLOCK`.
  - Any other action, an empty action, malformed JSON or a missing file: the safe default, BLOCK (FAIL). An unknown non-empty action also adds an error diagnostic.
- **`Classify` is a pure function** (it reads one file) so a test drives it without the bridge. The action vocabulary is a closed set; anything else is BLOCK.
- **`Run` wraps the base runner** to put the decision into `PhaseResponse.Signals`. The base runner does not fill them from `Classify`. The decision is a contracted file, not stdout. The hook therefore re-reads it from the workspace through `Classify`, and the hook and the unit-tested function share one code path. On the skip and error paths of the base runner (an empty workspace, a bridge failure) the file is absent. `Classify` then safe-defaults to BLOCK, so the signals say so loudly and do not drop out.
- **The default model is `opus`:** diagnosing a ship failure is high-reasoning work.
- **The prompt renders the ship-failure envelope** from `PhaseRequest.Context` (keys `ship_error_code`, `class`, `stage`, `debug`). Each key is optional and only what is present renders, so a partial envelope still gives a clean prompt. At enforce the typed `ErrorContext` replaces those keys (ADR-0050 section 3.10, slice 2). The output is byte-identical, because the zero `ErrorContext` renders nothing, as an empty map does.
- **`Config`:** `ContractVerifier` is the deliverables gate's verifier accessor for the verdict engine (one verifier for gate and engine); nil means the catalog-aware default. `CompactPrompts` flows from `workflow.compact_prompts` in `policy.json` and is never a bare literal here.

## Invariants

- **Never RESHIP on a parse failure.** A wrong reship corrupts history, and replaying a corrupt decision is strictly worse than stopping loudly. `TestClassify` pins the safe default for every malformed input, and `TestClassifyNeverReshipOnParseFailure` pins that a truncated file never yields even a partial RESHIP signal.
