# internal/bridge/launchoutcome

> Decision record: [ADR-0103](../adr/0103-component-breakdown-program.md), unit 10. Design: [10-bridgeengine.md](../decomposition/10-bridgeengine.md).

## Purpose

`internal/bridge/launchoutcome` says what one bridge launch exit means. It owns the exit vocabulary as one table. A row holds the class name, the registered `BRIDGE_EXIT_*` signal code, the cause-ledger code and the port sentinel the exit wraps. It also owns the miners of the cause line: the first diagnostic line, the artifact-timeout summary and the typed sub-cause.

`Classify` projects the table onto one `Outcome`. `CauseCode` projects the same row for the cause ledger. The package is pure. It has no clock, no filesystem and no Signal Center. `bridge.Engine.Launch` owns the emit and the request gauntlet.

## Design

- **One table, many projections.** Every reading of an exit code goes through `exitClass`. The error chain, the ledger and the signal cannot disagree.
- **Transient exits** are 80, 85, 86 and 124, and a signal death under a cancelled context. 124 (a command-level timeout) joins as the sibling of 81. 127 stays plain on purpose. An absent CLI is an environment defect and must fail loud. Its only recovery is the exit-code-triggered family fallback, which sees the raw 127.
- **A signal death (`ExitSignalDeath`, -1) is transient only under a cancelled context.** That is the deliverable-authority door (cycle 859): our own cancellation killed the driver. With a live context it is a start failure and stays plain.
- **The numbers are spelled twice on purpose.** `bridge/exitcodes.go` declares the same literals for the drivers, `cmd/evolve` and the ACS predicate that regex-scans that file. `TestExitCodes_HostAliasesAreTheLeafValues` (package bridge) makes the two spellings one belief.
- **The error text is a contract.** `Outcome.Err` reads `bridge: launch exit=<code>[: <cause>]`. It wraps `core.ErrArtifactTimeout` (81) or `core.ErrTransientBridgeFailure` (transient). Other exits get a plain error. `retry_backoff.go` byte-parses the prefix. The breaker fingerprints the whole string.
- **The exit chooses the cause line.**
  - `firstDiagnosticLine` takes the first `[bridge]`-prefixed line, else the last non-empty line. Driver failures put the causal line last (cycle 286).
  - On exit 81 the marker summary of the driver replaces it. The death then says what was awaited, how many extensions were used, and the last review verdict.
  - Only exit 81 has this rule. No other cause changes.
- **The marker.** `ArtifactTimeoutMarker` prefixes the one self-describing summary line. It is hyphenated and followed by colon-space. It can then never collide with the `artifact_timeout` stop-kind string. The classifier takes the last `[bridge]`-prefixed candidate. Earlier free-form diagnostics are sanitized, and closeout writes the authoritative marker last.
- **The typed sub-cause never comes from prose.** `timeoutCauseCode` reads only the `cause=` value. It accepts the value only if `Known()` says so. On exit 81 the cause code of the ledger is that sub-cause, else the code of the row.
- **Exit 85 (unknown prompt).** The report line is the escalation line of the auto-responder, else the corroborated quota wall, else nothing. The ledger sub-cause is the pattern that the responder matched. A corroborated wall gives `rate_limit`. A prompt that nobody recognised gives an empty sub-cause.
- **Bounds.** The cause line and the summary are cut at a rune boundary, never inside a UTF-8 sequence. `maxCauseRunes` bounds the cause line. The larger `maxSummaryRunes` bounds the summary. A cut ends in an ellipsis.

## Invariants

- **The zero `Outcome` is `ExitOK`.** It has no error, no cause and no code. Success carries no signal code.
- **The signal naming rule** `code == "BRIDGE_EXIT_" + upper(name)` is a test, not a computation. The host emits exactly one signal for each failed launch. It emits after the launch-error persist and the boot-strike record. The classified error is the reason.
