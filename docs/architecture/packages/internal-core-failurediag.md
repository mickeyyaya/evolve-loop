# internal/core/failurediag

> Component unit 02 of ADR-0103. The design record: [decomposition unit 02](../decomposition/02-failure-diagnostics.md).

## Purpose

`internal/core/failurediag` writes the `<phase>-failure-diag.json` sidecar when a mandatory phase aborts. It also holds the delivery-failure classifier that the sidecar and the retro relaunch key on.

## Design

- **Injected collaborators.** The orchestrator injects its clock and its artifact-timeout gate. The gate is a predicate, so the unit never imports the core sentinel `ErrArtifactTimeout`. `NewWriter` needs a non-nil gate; a nil gate panics at first use. A nil gate never means "nothing is a timeout". The gate is the one predicate that decides both exit code 81 and if a delivery cause can be attributed.
- **Live reads.** Build the `Writer` once per orchestrator. The clock is a closure and the Signal Center is an accessor, so an option applied later still takes. A nil accessor, or one that returns nil, is the Null Object. Only tests do this, and `SignalsWired` proves that the production roots wire a Center.
- **Sidecar contract.** `Sidecar` is the declared shape of `<phase>-failure-diag.json`. The field order is the wire order: compact `json.Marshal`, no `omitempty`, every field always present. `ErrorMessage` is `phaseErr.Error()` verbatim. `ExitCode` is 81 when the gate says artifact timeout, else the code of an `*exec.ExitError`, else 1.
- **Best effort.** `Write` renders the sidecar and lands it through `<path>.tmp` and `os.Rename` with mode 0o644. A temp-write or rename failure gives one `FAILUREDIAG_SIDECAR_WRITE_FAILED` WARN (`CodeSidecarWriteFailed`). The phase error is never masked, and `Write` returns nothing.
- **No marshal branch.** The sidecar holds strings and ints only. `encoding/json` has no error path for that shape (it fails on unsupported types, non-finite floats, cycles or `Marshaler` errors). So the earlier marshal branch can never fire. It was removed (ADR-0103 item 3). A float field added later must bring the branch back with a code, as in unit 01.
- **Wire contract of the timeout marker.** `MarkerPrefix`, `CauseField`, `ReasonField`, `CauseSubmitWedged` and `ExitCodeArtifactTimeout` are the tokens as the decoder spells them. The producer in `internal/bridge` (`stopreview.go`, `driver_tmux_wait_diagnostic.go`, `attempt_telemetry.go`) spells the same tokens. Follow-up F2 points the producer at these constants, so the two cannot drift.
- **Classifier gate.** `DeliveryFailureCause` classifies nothing unless the injected timeout gate says the error is an artifact timeout. Reviewer prose then cannot forge a delivery failure.
- **Classifier tokens.** With a host-owned `cause=` token, the token is authoritative. Any cause but `submit_wedged` gives "". For `submit_wedged` it returns the quoted reason when the reason names the token, and the bare cause if not. `reason=` counts only immediately after the cause token.
- **Legacy marker.** Without `cause=`, the quoted `reason=` classifies only when it names `submit_wedged`. A malformed or truncated marker classifies as nothing.
- **Quoted values.** `quotedValue` decodes a leading Go-quoted string and walks the escapes. It returns false when the value is not quoted or the quote never ends.

## Invariants

- The sidecar write never masks the phase abort. Pinned by the tests in `diag_test.go`.
- The classifier never attributes a delivery cause without the timeout gate. Pinned by the tests in `delivery_test.go`.

## Findings

- None open.
