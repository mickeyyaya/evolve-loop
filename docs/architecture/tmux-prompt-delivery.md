# Tmux prompt delivery

## Issue and supported boundary

A September 2026 live verification attempt reached TDD with only the tail of
its prompt. Claude reported that no actual task had been supplied. Earlier
Codex phases left prompt text parked until a native idle nudge resent Enter,
despite recording `submit_verified` at initial delivery. The operator stopped
cycles 1613–1615 and preserved their reports and worktrees. This interrupted
attempt is not a completed improvement wave.

The shared tmux transport used an unbracketed paste. Tmux's default newline
conversion turns LF into CR, which a terminal UI can treat as Enter rather than
message content. The short shell-line fixture did not model terminal UI input:
it could produce an artifact after receiving only part of a prompt.

`execTmux.PasteBuffer` now preserves LF with `-r` and requests bracketed paste
with `-p`. Tmux sends paste delimiters only when the receiving application has
enabled bracketed-paste mode. Applications without that mode retain plain paste
behavior. Session-named buffers and deletion after paste remain unchanged.

Both normal and human input cadence use the same load/paste/submit operation.
The first transport error stops delivery; later operations, successful delivery
logging and submission verification do not run. Errors retain their operation
and underlying cause and use the existing delivery-failure exit code 81. They
do not count as provider boot failures because boot already succeeded.

Codex 0.153's `[Pasted Content …]` input chip is recognized at initial prompt
verification. This additional echo applies only to Codex and only to the
just-pasted prompt. Existing bounded resend behavior and protections against
matching historical output remain in effect.

## Evidence and limits

`TestRealTmux_BracketedMultilinePromptPreservesBytes` drives the actual shared
REPL flow, real tmux and a raw-terminal fixture that requests bracketed paste.
It compares every byte of a roughly 51 KB UTF-8 multiline payload, including the
one LF that the existing resolved-prompt materialization adds. Before repair,
the first submitted message contained only 30 bytes. A shell-line compatibility
test remains alongside it. This native fixture replaces only the provider UI;
it makes no network request and does not read private documents.

`TestTmuxPromptTransportErrorStopsDelivery` injects each load/paste/submit error
through both cadences. It asserts the original error survives, later operations
do not execute, and no verified-submission record is emitted.
`TestTmuxPromptCodexPasteChipRecovery` drives the shared REPL call site and proves
immediate bounded recovery for the current Codex chip, without treating quoted
history or another driver's input as that chip.

The opt-in `TestLiveCLI_FullRoundtrip` asks actual Claude, Codex and Agy instances
to produce an exact result that requires values near the beginning, middle and
end of a phase-sized message. It retains strict output equality and a bounded
per-provider deadline. This is a delivery smoke test, not evidence of useful
improvement cycles. The pane-based `submit_verified` result remains a heuristic
observation that no recognized pending echo was found; it is not a provider
acknowledgment or proof that a model understood the task.

```sh
cd go
SHELL=/bin/sh EVOLVE_TMUX_SOCKET=evolve-delivery-tests go test -tags=integration ./internal/bridge -run 'TestRealTmux_BracketedMultilinePromptPreservesBytes|TestRealTmux_MultilineSpecialCharPrompt|TestTmuxPrompt' -count=1
# Opt-in: launches real provider CLIs and consumes model usage.
SHELL=/bin/sh EVOLVE_TMUX_SOCKET=evolve-delivery-live EVOLVE_BRIDGE_LIVE_CLI_ROUNDTRIP=1 go test -tags=integration ./internal/bridge -run '^TestLiveCLI_FullRoundtrip$' -count=1 -v
```
