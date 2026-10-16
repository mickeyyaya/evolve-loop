# internal/bridge/channel

## Purpose

`internal/bridge/channel` is the bidirectional channel for long-running tmux-REPL phases ([ADR-0037](../adr/0037-bidirectional-channel.md)). A `Producer` writes a filtered live feed beside the observer. A `Supervisor` sends correlated questions over the inbox and reads the answers back from the feed.

## Design

- **Single writer.** `Producer` is the only writer of the per-agent feed. It opens the file `O_APPEND` and runs one `Normalizer` over the phase's raw logs. One producer per agent enforces this.
- **Canonical feed path.** `FeedPath` is the one path that the producer, the supervisor and `evolve bridge watch` all call. An empty agent name maps to `agent`, as `inbox.Path` does.
- **Source files.** A tmux-family phase tails its driver's live pair, because its stdout log stays empty until the exit dump. A headless phase keeps the `<phase>-stdout.log` and `<phase>-stderr.log` defaults.
- **Silent-empty guard.** If the content source does not appear for `sourceMissThreshold` consecutive polls, the producer warns once. At the 2 s default this is about 40 s of grace. That covers a driver's boot time. The warning shows a name mismatch, or a CLI family that resolved to the wrong driver.
- **Correlation.** `Ask` writes a question with a fresh correlation ID through the inbox. It then polls the feed for a `response_complete` span. The answer is every content envelope whose `seq` lies in `[start_seq, end_seq]`. Timeouts use the injected clock, so tests are deterministic.
- **Transport guard.** Only drivers whose name ends in `-tmux` take live injection. Any other transport returns `ErrTransportNoInject`.
- **Long answers.** A content line can exceed the scanner's default 64 KB token cap. `collectSpan` uses a 1 MB start buffer and a 10 MB maximum. A scan error still surfaces as a warning, so an answer is never truncated without a trace.
- **Policy seam.** `Policy` decides to ask or not, from one envelope. `StallPolicy` is the minimal default. A later LLM-driven policy can implement the same interface.
- **Rollout dial.** `ResolveStage` is the one rule for the `EVOLVE_PHASE_RECOVERY` word, for both recovery dials. An unset value means shadow. The words `off`, `shadow` and `enforce` pass through, and `0` means off. Any other value means off, so a typo never turns on a kill path. The channel is on only in the enforce stage.

## Invariants

- The feed has one writer per agent. Pinned by `TestProducer_RunWritesFeed`.
- The enablement word resolves the same way for every reader, and a typo resolves to off. Pinned by `TestResolveStage` and `TestEnabled_UsesStageOnly`.
- A missing content source warns exactly once, and a present but quiet source does not warn. Pinned by `TestProducer_WarnsWhenSourceNeverAppears` and `TestProducer_NoWarnWhenSourceExists`.
- A non-tmux transport cannot inject. Pinned by `TestSupervisor_AskRefusesNonTmux`.
- A span longer than 64 KB is recovered whole. Pinned by `TestCollectSpan_RecoversLineOver64KB`.
- The end-to-end ask and answer path holds the sequence rules of the normalizer. Pinned by `TestChannel_EndToEnd`.
