# internal/bridge/inbox

## Purpose

`internal/bridge/inbox` is the file-based channel for live command injection into a running tmux REPL agent. A sender (`evolve bridge send`, or the nudge hook of the phase observer) appends NDJSON envelopes to `<workspace>/.bridge-inbox/<agent>.ndjson`. The tmux driver drains them from its artifact-wait poll loop. It injects them into the live session. The package is a leaf with no internal imports. The bridge driver, the CLI command and the observer can thus import it without a cycle.

## Design

- **Kinds set the injection rule.**
  - `command` is injected when the agent is idle (the prompt marker is visible).
  - `interrupt` sends ESC first, then injects whatever the agent state is.
  - `nudge` is a command that the observer originates. It is idle-gated like `command`.
  - `system_rule` is a late rule injection. It is idle-gated and prefixed with `## Rules`.
  - `keystroke` sends raw `tmux send-keys` tokens. It has no idle gate, no ESC prefix and no Enter suffix. The body is one tmux key spec, sent verbatim. It is literal text, a named key such as `Enter`, `Escape`, `C-c` or `Up`, or several tokens with spaces between them. It is the hatch of the operator to full tmux control (ADR-0023): dismiss modals, move through menus, confirm y/N prompts. The operator is responsible for what reaches the REPL.
- **`Valid` is the gate of the CLI.** A new `Kind` constant needs a matching case in `Valid` in the same change. If not, an operator typo can reach the dispatch switch of the driver.
- **`Path` is shared by the sender and the driver.** Both must call it, so both compute the same file. An empty agent name becomes `agent`, as the prompt-file convention of the engine does.
- **The reader is byte-offset based.** `Cursor` tracks a byte offset, not a sequence number. `Drain` advances only past the last complete newline-terminated line, so a partial final line from a concurrent sender is never half-parsed. A malformed line is skipped and the offset still moves past it. The caller logs it.
- **The concurrency contract is many writers and one reader per agent.** The reader never writes the file. It changes only its in-memory offset.
- **The driver seeks to the end on entry** (`SetOffset`), so a resumed named session or a stale file never replays an old backlog. Live injection delivers only envelopes that were appended after the agent started.
- **A file shorter than the offset means rotation.** `Drain` resets the offset to 0 and reads again. A missing file gives no envelopes and no error.
- **`Append` is atomic by size.** POSIX guarantees that one `write()` of at most `PIPE_BUF` bytes (4096 on darwin and linux) to an `O_APPEND` file is atomic. The ceiling `maxLine` uses this, so concurrent senders never interleave lines. An envelope that serializes larger is rejected with an error. It is never written.
- **`Seq` is a hint.** The writer may set it. The cursor of the reader is the authority.
- **`DeferCount` bounds the re-queue.** The driver re-queues a mid-turn command while the agent is busy. The bound stops an agent that is never idle from looping the inbox for ever.
- **`CorrID` ties an ask to its reply** in the channel feed (ADR-0037). The driver echoes it in its `inject_applied` and `idle_reached` breadcrumbs.

## Invariants

- A partial final line is not consumed or parsed until its newline arrives (`TestCursorPartialTrailingLineNotParsed`).
- An error from `Stat` other than not-exist, or from `Open`, reaches the caller. `Drain` never swallows it (`TestDrainOpenErrorPropagates`).
- **`Append` ignores the marshal error.** `Envelope` holds only strings and integers, so `json.Marshal` cannot fail on it. A new field of another kind must bring the error check back.

## Findings

- A rewrite in place to the exact same byte length between two `Drain` calls cannot be detected from the offset. The inbox is append-only in practice and is drained every few seconds, so the worst case is one missed injection on a pathological input. The package does not handle it, by decision.
