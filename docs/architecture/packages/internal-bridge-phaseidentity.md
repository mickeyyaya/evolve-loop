# internal/bridge/phaseidentity

> Decision record: [ADR-0106](../adr/0106-logic-first-delivery.md) (logic-first delivery, component P3). Design: [logic-first-delivery-design.md §5.4](../logic-first-delivery-design.md#54-the-agents-identity-and-its-pane).

## Purpose

`phaseidentity` renders the statement a tmux driver appends to the bytes it pastes into a phase pane: which phase and cycle the agent is, which tmux session it runs in, which files the bridge wrote to deliver the prompt, and which artifact it alone writes. An agent that inspects its environment then recognises its own traces. Cycle 1707's tdd agent listed the tmux sessions, found its own, read its own prompt file, and refused the phase as a prompt injection racing "the real agent"; an operator's one-line identity clarification resumed it an hour later. The block says that line before the agent needs it.

## Design

- **A pure projection of facts.** `Block(Facts) string`: no I/O, no clock, no CLI branch. `Facts` holds `Agent`, `Cycle`, `Session`, `PromptFile` (the prompt the engine composed), `PastedFile` (`resolved-prompt.txt`, the exact bytes pasted) and `Artifact`.
- **Only true claims.** A phase whose answer the bridge reads from the pane (`completion: stdout`, the router and the advisor) never writes the artifact, so the driver passes no artifact and the sole-writer line becomes "The bridge reads your answer from this pane". `TestBlock_WithoutAnArtifactStatesThePaneIsRead` and `TestTmuxDispatch_StdoutCompletionClaimsNoArtifact` pin it.
- **Null object for a pane that is no phase.** `Block` returns `""` when `Agent` or `Session` is empty: a probe pane has no phase to be, and a headless run has no pane to speak of. The driver appends nothing rather than a half-true statement.
- **The wording is the component.** Five lines: the phase (and the cycle when numbered); the session and the command that prints it (`tmux display-message -p '#S'`); the two prompt files and that finding them, or the session in `tmux ls`, is expected; the sole-writer fact and the standing instruction not to kill, pause or hand off the session or wait for an operator; and that instruction files addressed to the console operator describe the operator's sessions, not this one. `TestBlock_StatesEveryTraceTheAgentCanFind` pins the block byte for byte.
- **`Heading`** (`## Who you are (stated by the evolve bridge)`) is the one spelling the drivers and their tests look for.
- **Placement is the caller's.** `driver_tmux_prepare.go` appends the block after the composed prompt, so the engine's bytes stay a stable prefix (the cached skill and policy blocks hold across dispatches) and the deliverable path stays the last thing the agent reads.

## Invariants

- **Every trace is named verbatim.** Session, both prompt paths and the artifact path appear exactly as the driver holds them; a swapped path is caught by the golden (`TestBlock_StatesEveryTraceTheAgentCanFind`).
- **No agent, no session → no block.** Pinned by `TestBlock_NeedsAnAgentAndASession`.
- **Cycle 0 is unstated**, never "cycle 0". Pinned by `TestBlock_UnnumberedCycleIsNotStated`.
- **One heading, a terminated last line.** Pinned by `TestBlock_OpensWithTheHeadingOnce`.
- **A fact stays on its line and inside its code span.** Control bytes and backticks are dropped from every fact before it is rendered, so a hostile agent name or path cannot append markdown that reads as the bridge's own instruction. Pinned by `TestBlock_AFactCannotBreakOutOfItsLine`.
- **No manifest pattern matches the block.** The auto-responder drops pane lines that echo the injected prompt before it scans for a wall; `TestIdentityBlockMatchesNoManifestPattern` (package `bridge`) keeps every `interactive_prompts`, `exhausted_regex` and `transient_regex` from matching the block's text.
- **Standard library only**, so the package never imports `bridge`.

## Findings

- 2026-09-27: review found the sole-writer line false for stdout-completion phases (the router never writes its artifact); the driver now passes no artifact for them and the block states the pane is read.
- 2026-09-26: built as ADR-0106 P3 with ten mutants (both gates, the cycle guard, a doubled heading, each fact swapped or dropped, the trailing LF) all killed by the four tests.
